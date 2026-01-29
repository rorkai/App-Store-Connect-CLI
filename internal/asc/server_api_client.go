package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
)

const serverAPITokenLifetime = 5 * time.Minute

// ServerAPIClient is an App Store Server API client.
type ServerAPIClient struct {
	httpClient *http.Client
	keyID      string
	issuerID   string
	bundleID   string
	privateKey *ecdsa.PrivateKey
	baseURL    string
}

// NewServerAPIClient creates a new App Store Server API client.
func NewServerAPIClient(keyID, issuerID, privateKeyPath, bundleID string, env ServerEnvironment) (*ServerAPIClient, error) {
	if err := auth.ValidateKeyFile(privateKeyPath); err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	key, err := auth.LoadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	baseURL, err := serverAPIBaseURL(env)
	if err != nil {
		return nil, err
	}

	return &ServerAPIClient{
		httpClient: &http.Client{Timeout: ResolveTimeout()},
		keyID:      strings.TrimSpace(keyID),
		issuerID:   strings.TrimSpace(issuerID),
		bundleID:   strings.TrimSpace(bundleID),
		privateKey: key,
		baseURL:    baseURL,
	}, nil
}

func serverAPIBaseURL(env ServerEnvironment) (string, error) {
	switch env {
	case ServerEnvironmentProduction:
		return "https://api.storekit.itunes.apple.com", nil
	case ServerEnvironmentSandbox:
		return "https://api.storekit-sandbox.itunes.apple.com", nil
	}
	return "", fmt.Errorf("unsupported server environment %q", env)
}

func (c *ServerAPIClient) newRequest(ctx context.Context, method, path string, body io.Reader, contentType string) (*http.Request, error) {
	token, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *ServerAPIClient) generateJWT() (string, error) {
	now := time.Now()
	claims := struct {
		BID string `json:"bid"`
		jwt.RegisteredClaims
	}{
		BID: strings.TrimSpace(c.bundleID),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    c.issuerID,
			Audience:  jwt.ClaimStrings{"appstoreconnect-v1"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(serverAPITokenLifetime)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = c.keyID

	signedToken, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return signedToken, nil
}

func (c *ServerAPIClient) do(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	request := func() ([]byte, error) {
		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}
		return c.doOnce(ctx, method, path, reader, contentType)
	}

	if shouldRetryMethod(method) {
		retryOpts := ResolveRetryOptions()
		return WithRetry(ctx, request, retryOpts)
	}

	return request()
}

func (c *ServerAPIClient) doOnce(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, error) {
	req, err := c.newRequest(ctx, method, path, body, contentType)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			return nil, &RetryableError{
				Err:        buildServerAPIRetryableError(resp.StatusCode, retryAfter, respBody),
				RetryAfter: retryAfter,
			}
		}

		if err := parseServerAPIError(resp.StatusCode, respBody); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("server API request failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

type serverAPIError struct {
	StatusCode int
	Code       int
	Message    string
}

func (e *serverAPIError) Error() string {
	if e == nil {
		return "server API error"
	}
	if e.Code != 0 && strings.TrimSpace(e.Message) != "" {
		return fmt.Sprintf("server API error %d (status %d): %s", e.Code, e.StatusCode, e.Message)
	}
	if e.Code != 0 {
		return fmt.Sprintf("server API error %d (status %d)", e.Code, e.StatusCode)
	}
	if strings.TrimSpace(e.Message) != "" {
		return fmt.Sprintf("server API error (status %d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("server API error (status %d)", e.StatusCode)
}

func parseServerAPIError(statusCode int, body []byte) error {
	var errResp struct {
		ErrorCode    int    `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && (errResp.ErrorCode != 0 || strings.TrimSpace(errResp.ErrorMessage) != "") {
		return &serverAPIError{
			StatusCode: statusCode,
			Code:       errResp.ErrorCode,
			Message:    strings.TrimSpace(errResp.ErrorMessage),
		}
	}

	if len(body) > 0 {
		return &serverAPIError{
			StatusCode: statusCode,
			Message:    sanitizeErrorBody(body),
		}
	}
	return nil
}

func buildServerAPIRetryableError(statusCode int, retryAfter time.Duration, respBody []byte) error {
	base := "server API request failed"
	switch statusCode {
	case http.StatusTooManyRequests:
		base = "rate limited by App Store Server API"
	case http.StatusServiceUnavailable:
		base = "App Store Server API service unavailable"
	}

	message := fmt.Sprintf("%s (status %d)", base, statusCode)
	if len(respBody) > 0 {
		if err := parseServerAPIError(statusCode, respBody); err != nil {
			message = fmt.Sprintf("%s: %s", message, err)
		}
	}
	if retryAfter > 0 {
		message = fmt.Sprintf("%s (retry after %s)", message, retryAfter)
	}
	return errors.New(message)
}

func addQueryValues(values url.Values, key string, items []string) {
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		values.Add(key, item)
	}
}

func addStatusQueryValues(values url.Values, statuses []ServerStatus) {
	for _, status := range statuses {
		values.Add("status", strconv.Itoa(int(status)))
	}
}

func (c *ServerAPIClient) GetAllSubscriptionStatuses(ctx context.Context, transactionID string, statuses []ServerStatus) (*StatusResponse, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, fmt.Errorf("transactionID is required")
	}

	path := fmt.Sprintf("/inApps/v1/subscriptions/%s", transactionID)
	values := url.Values{}
	addStatusQueryValues(values, statuses)
	if query := values.Encode(); query != "" {
		path += "?" + query
	}

	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}

	var response StatusResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse subscription status response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) GetTransactionHistory(ctx context.Context, transactionID string, request TransactionHistoryRequest, revision string) (*HistoryResponse, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, fmt.Errorf("transactionID is required")
	}

	path := fmt.Sprintf("/inApps/v2/history/%s", transactionID)
	values := url.Values{}
	if revision = strings.TrimSpace(revision); revision != "" {
		values.Set("revision", revision)
	}
	if request.StartDate != nil {
		values.Set("startDate", strconv.FormatInt(*request.StartDate, 10))
	}
	if request.EndDate != nil {
		values.Set("endDate", strconv.FormatInt(*request.EndDate, 10))
	}
	addQueryValues(values, "productId", request.ProductIDs)
	if len(request.ProductTypes) > 0 {
		items := make([]string, 0, len(request.ProductTypes))
		for _, item := range request.ProductTypes {
			items = append(items, string(item))
		}
		addQueryValues(values, "productType", items)
	}
	addQueryValues(values, "subscriptionGroupIdentifier", request.SubscriptionGroupIdentifiers)
	if request.InAppOwnershipType != nil {
		values.Set("inAppOwnershipType", string(*request.InAppOwnershipType))
	}
	if request.Revoked != nil {
		values.Set("revoked", strconv.FormatBool(*request.Revoked))
	}
	if request.Sort != nil {
		values.Set("sort", string(*request.Sort))
	}
	if query := values.Encode(); query != "" {
		path += "?" + query
	}

	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}

	var response HistoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse transaction history response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) GetRefundHistory(ctx context.Context, transactionID, revision string) (*RefundHistoryResponse, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, fmt.Errorf("transactionID is required")
	}

	path := fmt.Sprintf("/inApps/v2/refund/lookup/%s", transactionID)
	if revision = strings.TrimSpace(revision); revision != "" {
		values := url.Values{}
		values.Set("revision", revision)
		path += "?" + values.Encode()
	}

	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}

	var response RefundHistoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse refund history response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) GetTransactionInfo(ctx context.Context, transactionID string) (*TransactionInfoResponse, error) {
	transactionID = strings.TrimSpace(transactionID)
	if transactionID == "" {
		return nil, fmt.Errorf("transactionID is required")
	}

	path := fmt.Sprintf("/inApps/v1/transactions/%s", transactionID)
	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}

	var response TransactionInfoResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse transaction info response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) LookUpOrderID(ctx context.Context, orderID string) (*OrderLookupResponse, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("orderID is required")
	}

	path := fmt.Sprintf("/inApps/v1/lookup/%s", orderID)
	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}

	var response OrderLookupResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse order lookup response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) RequestTestNotification(ctx context.Context) (*SendTestNotificationResponse, error) {
	data, err := c.do(ctx, http.MethodPost, "/inApps/v1/notifications/test", nil, "")
	if err != nil {
		return nil, err
	}

	var response SendTestNotificationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse test notification response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) GetTestNotificationStatus(ctx context.Context, token string) (*CheckTestNotificationResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("testNotificationToken is required")
	}

	path := fmt.Sprintf("/inApps/v1/notifications/test/%s", token)
	data, err := c.do(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}

	var response CheckTestNotificationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse test notification status response: %w", err)
	}
	return &response, nil
}

func (c *ServerAPIClient) GetNotificationHistory(ctx context.Context, request NotificationHistoryRequest, paginationToken string) (*NotificationHistoryResponse, error) {
	path := "/inApps/v1/notifications/history"
	if paginationToken = strings.TrimSpace(paginationToken); paginationToken != "" {
		values := url.Values{}
		values.Set("paginationToken", paginationToken)
		path += "?" + values.Encode()
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, path, body, "application/json")
	if err != nil {
		return nil, err
	}

	var response NotificationHistoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse notification history response: %w", err)
	}
	return &response, nil
}
