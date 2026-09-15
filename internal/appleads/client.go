package appleads

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

const (
	// BaseURL is the Apple Ads Campaign Management API v5 base URL.
	BaseURL = "https://api.searchads.apple.com/api/"
	// PlatformBaseURL is the Apple Ads Platform API v1 base URL.
	PlatformBaseURL = "https://api.ads.apple.com/v1/"
)

// APIVersion identifies an Apple Ads API transport contract.
type APIVersion string

const (
	APIVersionCampaignV5 APIVersion = "campaign-v5"
	APIVersionPlatformV1 APIVersion = "platform-v1"
)

// ContextKind identifies the X-AP-Context behavior for a request.
type ContextKind uint8

const (
	ContextNone ContextKind = iota
	ContextOrg
	ContextAdAccount
	ContextAdAccountOptional
)

// RawResponse preserves the Apple Ads response envelope.
type RawResponse json.RawMessage

// MarshalJSON implements json.Marshaler.
func (r RawResponse) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return json.RawMessage(r).MarshalJSON()
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// Client is an Apple Ads Campaign Management API client.
type Client struct {
	httpClient      *http.Client
	baseURL         string
	platformBaseURL string
	tokenURL        string
	now             func() time.Time

	credentials Credentials

	tokenMu         sync.Mutex
	token           tokenCache
	privateKeyMu    sync.Mutex
	privateKeyValue *ecdsa.PrivateKey
	rateLimitMu     sync.RWMutex
	lastRateLimit   RateLimit
}

// NewClient constructs an Apple Ads API client.
func NewClient(credentials Credentials, opts ...ClientOption) (*Client, error) {
	if err := ValidateOrgID(credentials.OrgID); err != nil {
		return nil, err
	}
	if err := ValidateAdAccountID(credentials.AdAccountID); err != nil {
		return nil, err
	}
	client := &Client{
		httpClient:      &http.Client{Timeout: asc.ResolveTimeout()},
		baseURL:         BaseURL,
		platformBaseURL: PlatformBaseURL,
		tokenURL:        tokenURL,
		now:             time.Now,
		credentials:     normalizeCredentials(credentials),
	}
	for _, opt := range opts {
		opt(client)
	}
	if client.httpClient == nil {
		client.httpClient = &http.Client{Timeout: asc.ResolveTimeout()}
	}
	if strings.TrimSpace(client.baseURL) == "" {
		client.baseURL = BaseURL
	}
	if strings.TrimSpace(client.platformBaseURL) == "" {
		client.platformBaseURL = PlatformBaseURL
	}
	if strings.TrimSpace(client.tokenURL) == "" {
		client.tokenURL = tokenURL
	}
	if client.now == nil {
		client.now = time.Now
	}
	if err := validateCredentials(client.credentials); err != nil {
		return nil, err
	}
	return client, nil
}

// ValidateAdAccountID rejects values that cannot safely be placed in the
// Apple Ads ad-account context header.
func ValidateAdAccountID(value string) error {
	if strings.ContainsRune(value, ';') {
		return fmt.Errorf("invalid ad account ID: semicolons are not allowed")
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid ad account ID: control characters are not allowed")
	}
	return nil
}

// ValidateOrgID rejects values that cannot safely be placed in the legacy
// Apple Ads organization context header.
func ValidateOrgID(value string) error {
	if strings.ContainsRune(value, ';') {
		return fmt.Errorf("invalid organization ID: semicolons are not allowed")
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid organization ID: control characters are not allowed")
	}
	return nil
}

func normalizeCredentials(credentials Credentials) Credentials {
	credentials.ClientID = strings.TrimSpace(credentials.ClientID)
	credentials.TeamID = strings.TrimSpace(credentials.TeamID)
	credentials.KeyID = strings.TrimSpace(credentials.KeyID)
	credentials.PrivateKeyPath = strings.TrimSpace(credentials.PrivateKeyPath)
	credentials.PrivateKeyPEM = strings.TrimSpace(credentials.PrivateKeyPEM)
	credentials.AccessToken = strings.TrimSpace(credentials.AccessToken)
	credentials.OrgID = strings.TrimSpace(credentials.OrgID)
	credentials.AdAccountID = strings.TrimSpace(credentials.AdAccountID)
	credentials.Profile = strings.TrimSpace(credentials.Profile)
	return credentials
}

func validateCredentials(credentials Credentials) error {
	if credentials.AccessToken != "" {
		return nil
	}
	if credentials.ClientID == "" {
		return fmt.Errorf("client ID is required")
	}
	if credentials.TeamID == "" {
		return fmt.Errorf("team ID is required")
	}
	if credentials.KeyID == "" {
		return fmt.Errorf("key ID is required")
	}
	if credentials.PrivateKeyPath == "" && credentials.PrivateKeyPEM == "" {
		return fmt.Errorf("private key is required")
	}
	return nil
}

// WithHTTPClient configures the HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

// WithBaseURL configures the Apple Ads API base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/"
	}
}

// WithPlatformBaseURL configures the Apple Ads Platform API v1 base URL.
func WithPlatformBaseURL(baseURL string) ClientOption {
	return func(client *Client) {
		trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if trimmed == "" {
			client.platformBaseURL = ""
			return
		}
		client.platformBaseURL = trimmed + "/"
	}
}

// WithTokenURL configures the OAuth token URL.
func WithTokenURL(tokenURL string) ClientOption {
	return func(client *Client) {
		client.tokenURL = strings.TrimSpace(tokenURL)
	}
}

// WithNow configures the clock used by token caching.
func WithNow(now func() time.Time) ClientOption {
	return func(client *Client) {
		client.now = now
	}
}

// Do executes a documented Apple Ads endpoint.
func (c *Client) Do(ctx context.Context, spec EndpointSpec, pathParams map[string]string, query url.Values, body json.RawMessage) (RawResponse, error) {
	path, err := expandPath(spec.Path, pathParams)
	if err != nil {
		return nil, err
	}
	version := spec.Version
	contextKind := spec.Context
	if version == "" {
		version = APIVersionCampaignV5
	}
	if version == APIVersionCampaignV5 && contextKind == ContextNone && spec.RequiresOrg {
		contextKind = ContextOrg
	}
	retrySafe := spec.RetrySafe || spec.Method == http.MethodGet || spec.Method == http.MethodHead
	if spec.ReadOnlyRequest() {
		ctx = readonly.WithReadIntent(ctx)
	}
	return c.requestForVersion(ctx, version, spec.Method, path, query, body, contextKind, retrySafe)
}

// Request executes an Apple Ads API request for a relative v5 path.
func (c *Client) Request(ctx context.Context, method, path string, query url.Values, body json.RawMessage, requiresOrg bool) (RawResponse, error) {
	contextKind := ContextNone
	if requiresOrg {
		contextKind = ContextOrg
	}
	return c.RequestForVersion(ctx, APIVersionCampaignV5, method, path, query, body, contextKind)
}

// RequestForVersion executes an Apple Ads request against an explicit API version.
func (c *Client) RequestForVersion(ctx context.Context, version APIVersion, method, path string, query url.Values, body json.RawMessage, contextKind ContextKind) (RawResponse, error) {
	retrySafe := version == APIVersionPlatformV1 && (method == http.MethodGet || method == http.MethodHead)
	return c.requestForVersion(ctx, version, method, path, query, body, contextKind, retrySafe)
}

func (c *Client) requestForVersion(ctx context.Context, version APIVersion, method, path string, query url.Values, body json.RawMessage, contextKind ContextKind, retrySafe bool) (RawResponse, error) {
	if err := validateVersionContext(version, contextKind); err != nil {
		return nil, err
	}
	contextHeader, err := c.contextHeader(contextKind)
	if err != nil {
		return nil, err
	}
	requestURL, err := c.requestURLForVersion(version, path, query)
	if err != nil {
		return nil, err
	}
	retrySafe = version == APIVersionPlatformV1 && retrySafe
	var retryOptions asc.RetryOptions
	if retrySafe {
		retryOptions = asc.ResolveRetryOptions()
	}
	request := func() (RawResponse, error) {
		return c.requestOnce(ctx, version, method, requestURL, body, contextHeader, retrySafe, retryOptions.MaxDelay)
	}
	if retrySafe {
		return asc.WithRetry(ctx, request, retryOptions)
	}
	return request()
}

func (c *Client) requestOnce(ctx context.Context, version APIVersion, method, requestURL string, body json.RawMessage, contextHeader string, retrySafe bool, maxRetryDelay time.Duration) (RawResponse, error) {
	if err := readonly.Check(ctx, method, readonly.Target(requestURL)); err != nil {
		return nil, err
	}
	token, err := c.bearerToken(ctx)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if contextHeader != "" {
		req.Header.Set("X-AP-Context", contextHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if retrySafe && ctx.Err() == nil {
			return nil, &asc.RetryableError{Err: err}
		}
		return nil, err
	}
	defer resp.Body.Close()
	if version == APIVersionPlatformV1 {
		c.rateLimitMu.Lock()
		c.lastRateLimit = rateLimitFromHeaders(resp.Header)
		c.rateLimitMu.Unlock()
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		readErr := fmt.Errorf("read response: %w", err)
		if retrySafe && ctx.Err() == nil {
			return nil, &asc.RetryableError{Err: readErr}
		}
		return nil, readErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := parseErrorForVersion(respBody, resp.StatusCode, resp.Header, version)
		if retrySafe && isRetryableAdsStatus(resp.StatusCode) {
			retryAfter := adsRetryDelay(resp.Header, c.now(), maxRetryDelay)
			return nil, &asc.RetryableError{
				Err:        apiErr,
				RetryAfter: retryAfter,
			}
		}
		return nil, apiErr
	}
	if len(strings.TrimSpace(string(respBody))) == 0 {
		if version == APIVersionPlatformV1 {
			return RawResponse(`{}`), nil
		}
		return RawResponse(`{"data":null}`), nil
	}
	return RawResponse(respBody), nil
}

func validateVersionContext(version APIVersion, kind ContextKind) error {
	switch version {
	case APIVersionCampaignV5:
		if kind == ContextNone || kind == ContextOrg {
			return nil
		}
	case APIVersionPlatformV1:
		if kind == ContextNone || kind == ContextAdAccount || kind == ContextAdAccountOptional {
			return nil
		}
	default:
		return fmt.Errorf("unsupported Apple Ads API version %q", version)
	}
	return fmt.Errorf("version %q does not support Apple Ads context kind %d", version, kind)
}

func adsRetryDelay(headers http.Header, now time.Time, maxDelay time.Duration) time.Duration {
	var delay time.Duration
	if value := strings.TrimSpace(headerValue(headers, "Retry-After")); value != "" {
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 {
			delay = adsRetryDelayFromSeconds(seconds, maxDelay)
		} else if deadline, err := http.ParseTime(value); err == nil {
			if delay := deadline.Sub(now); delay > 0 {
				return capAdsRetryDelay(delay, maxDelay)
			}
		}
	}
	if delay == 0 {
		if seconds, err := strconv.ParseInt(strings.TrimSpace(headerValue(headers, "RateLimit-Reset")), 10, 64); err == nil && seconds > 0 {
			delay = adsRetryDelayFromSeconds(seconds, maxDelay)
		}
	}
	return capAdsRetryDelay(delay, maxDelay)
}

func adsRetryDelayFromSeconds(seconds int64, maxDelay time.Duration) time.Duration {
	if seconds <= 0 {
		return 0
	}
	if maxDelay > 0 && seconds > int64(maxDelay/time.Second) {
		return maxDelay
	}

	const maxDuration = time.Duration(1<<63 - 1)
	if seconds > int64(maxDuration/time.Second) {
		return maxDuration
	}
	return time.Duration(seconds) * time.Second
}

func capAdsRetryDelay(delay, maxDelay time.Duration) time.Duration {
	if delay > 0 && maxDelay > 0 && delay > maxDelay {
		return maxDelay
	}
	return delay
}

func headerValue(headers http.Header, name string) string {
	if value := headers.Get(name); value != "" {
		return value
	}
	for key, values := range headers {
		if strings.EqualFold(key, name) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

// LastRateLimit returns the rate-limit metadata from the most recent Platform API response.
func (c *Client) LastRateLimit() RateLimit {
	c.rateLimitMu.RLock()
	defer c.rateLimitMu.RUnlock()
	return c.lastRateLimit
}

func isRetryableAdsStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (c *Client) contextHeader(kind ContextKind) (string, error) {
	switch kind {
	case ContextNone:
		return "", nil
	case ContextOrg:
		orgID := strings.TrimSpace(c.credentials.OrgID)
		if orgID == "" {
			return "", fmt.Errorf("org ID is required")
		}
		if err := ValidateOrgID(orgID); err != nil {
			return "", err
		}
		return "orgId=" + orgID, nil
	case ContextAdAccount, ContextAdAccountOptional:
		if err := ValidateAdAccountID(c.credentials.AdAccountID); err != nil {
			return "", err
		}
		adAccountID := strings.TrimSpace(c.credentials.AdAccountID)
		if adAccountID == "" {
			if kind == ContextAdAccountOptional {
				return "", nil
			}
			return "", fmt.Errorf("ad account ID is required")
		}
		return "adAccountId=" + adAccountID + ";", nil
	default:
		return "", fmt.Errorf("unsupported Apple Ads context kind %d", kind)
	}
}

func (c *Client) requestURLForVersion(version APIVersion, path string, query url.Values) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	if parsed.IsAbs() {
		baseURL, _, absolutePrefix, description, err := c.versionRouting(version)
		if err != nil {
			return "", err
		}
		base, err := url.Parse(baseURL)
		if err != nil {
			return "", err
		}
		if parsed.Scheme != "https" || parsed.User != nil || parsed.Host != base.Host || !strings.HasPrefix(parsed.Path, absolutePrefix) || hasPathTraversal(parsed.Path) {
			return "", fmt.Errorf("--path must be %s", description)
		}
		if len(query) > 0 {
			values := parsed.Query()
			for key, items := range query {
				for _, item := range items {
					values.Add(key, item)
				}
			}
			parsed.RawQuery = values.Encode()
		}
		return parsed.String(), nil
	}
	clean := strings.TrimPrefix(path, "/")
	baseURL, relativePrefix, _, _, err := c.versionRouting(version)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(clean, relativePrefix) {
		return "", fmt.Errorf("--path must start with %s", relativePrefix)
	}
	if hasPathTraversal(clean) {
		return "", fmt.Errorf("--path must not contain path traversal")
	}
	relPath := clean
	if version == APIVersionPlatformV1 {
		relPath = strings.TrimPrefix(clean, relativePrefix)
	}
	rel, err := url.Parse(relPath)
	if err != nil {
		return "", err
	}
	if rel.IsAbs() || rel.Host != "" || rel.User != nil || hasPathTraversal(rel.Path) {
		return "", fmt.Errorf("--path must not escape the Apple Ads API base URL")
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	resolved := base.ResolveReference(rel)
	if version == APIVersionPlatformV1 &&
		(resolved.Scheme != base.Scheme || resolved.Host != base.Host || !strings.HasPrefix(resolved.Path, base.Path)) {
		return "", fmt.Errorf("--path must not escape the Apple Ads Platform API v1 base URL")
	}
	if len(query) > 0 {
		values := resolved.Query()
		for key, items := range query {
			for _, item := range items {
				values.Add(key, item)
			}
		}
		resolved.RawQuery = values.Encode()
	}
	return resolved.String(), nil
}

func (c *Client) versionRouting(version APIVersion) (baseURL, relativePrefix, absolutePrefix, description string, err error) {
	switch version {
	case APIVersionCampaignV5:
		baseURL = c.baseURL
		relativePrefix = "v5/"
		description = "an Apple Ads v5 URL"
	case APIVersionPlatformV1:
		baseURL = c.platformBaseURL
		relativePrefix = "v1/"
		description = "an Apple Ads Platform API v1 URL"
	default:
		err = fmt.Errorf("unsupported Apple Ads API version %q", version)
		return
	}
	parsed, parseErr := url.Parse(baseURL)
	if parseErr != nil {
		err = parseErr
		return
	}
	absolutePrefix = parsed.Path
	if version == APIVersionCampaignV5 {
		absolutePrefix += relativePrefix
	}
	return
}

func hasPathTraversal(path string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func expandPath(path string, params map[string]string) (string, error) {
	result := path
	for key, value := range params {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("%s is required", key)
		}
		result = strings.ReplaceAll(result, "{"+key+"}", url.PathEscape(value))
	}
	if strings.Contains(result, "{") || strings.Contains(result, "}") {
		return "", fmt.Errorf("missing path parameter for %s", path)
	}
	return result, nil
}
