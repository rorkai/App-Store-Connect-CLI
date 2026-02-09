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
)

// newRequest creates a new HTTP request with JWT authentication
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	// Generate JWT token
	token, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	url := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		url = BaseURL + path
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// generateJWT generates a JWT for ASC API authentication
func (c *Client) generateJWT() (string, error) {
	return GenerateJWT(c.keyID, c.issuerID, c.privateKey)
}

// GenerateJWT generates a JWT for ASC API authentication.
func GenerateJWT(keyID, issuerID string, privateKey *ecdsa.PrivateKey) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Issuer:    issuerID,
		Audience:  jwt.ClaimStrings{"appstoreconnect-v1"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = keyID

	// Sign with the private key
	signedToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return signedToken, nil
}

// do performs an HTTP request and returns the response.
// GET/HEAD requests use retry logic for rate limiting by default.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
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
		return c.doOnce(ctx, method, path, reader)
	}

	if shouldRetryMethod(method) {
		retryOpts := ResolveRetryOptions()
		return WithRetry(ctx, request, retryOpts)
	}

	return request()
}

func (c *Client) doOnce(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	start := time.Now()
	debugSettings := resolveDebugSettings()

	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	if debugSettings.verboseHTTP {
		debugLogger.Info("→ HTTP Request",
			"method", method,
			"url", sanitizeURLForLog(req.URL.String()),
			"content-type", req.Header.Get("Content-Type"),
			"authorization", sanitizeAuthHeader(req.Header.Get("Authorization")),
		)
	}

	resp, err := c.httpClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		if debugSettings.verboseHTTP {
			debugLogger.Info("← HTTP Error",
				"error", err.Error(),
				"elapsed", elapsed.String(),
			)
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if debugSettings.verboseHTTP {
		debugLogger.Info("← HTTP Response",
			"status", resp.StatusCode,
			"elapsed", elapsed.String(),
			"content-type", resp.Header.Get("Content-Type"),
			"content-length", resp.Header.Get("Content-Length"),
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)

		// Check for rate limiting (429) or service unavailable (503)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			return nil, &RetryableError{
				Err:        buildRetryableError(resp.StatusCode, retryAfter, respBody),
				RetryAfter: retryAfter,
			}
		}

		if err := ParseErrorWithStatus(respBody, resp.StatusCode); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// sanitizeAuthHeader redacts the JWT token from Authorization header for logging.
func sanitizeAuthHeader(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "Bearer ") {
		return "Bearer [REDACTED]"
	}
	return "[REDACTED]"
}

func sanitizeURLForLog(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsedURL.User != nil {
		parsedURL.User = url.User("[REDACTED]")
	}
	values := parsedURL.Query()
	if len(values) == 0 {
		return parsedURL.String()
	}
	redactAll := hasSignedQuery(values)
	for key, vals := range values {
		if redactAll || isSensitiveQueryKey(key) {
			for i := range vals {
				vals[i] = "[REDACTED]"
			}
			values[key] = vals
		}
	}
	parsedURL.RawQuery = values.Encode()
	return parsedURL.String()
}

func shouldRetryMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

func buildRetryableError(statusCode int, retryAfter time.Duration, respBody []byte) error {
	base := "API request failed"
	switch statusCode {
	case http.StatusTooManyRequests:
		base = "rate limited by App Store Connect"
	case http.StatusServiceUnavailable:
		base = "App Store Connect service unavailable"
	}

	message := fmt.Sprintf("%s (status %d)", base, statusCode)
	if len(respBody) > 0 {
		if err := ParseErrorWithStatus(respBody, statusCode); err != nil {
			message = fmt.Sprintf("%s: %s", message, err)
		}
	}
	if retryAfter > 0 {
		message = fmt.Sprintf("%s (retry after %s)", message, retryAfter)
	}
	return errors.New(message)
}

// parseRetryAfterHeader parses the Retry-After header value.
// Supports seconds (e.g., "60") or HTTP-date format (RFC1123, RFC850, ANSIC).
func parseRetryAfterHeader(value string) time.Duration {
	if value = strings.TrimSpace(value); value == "" {
		return 0
	}

	// Try to parse as seconds first
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	// Try to parse as HTTP-date (try multiple formats)
	formats := []string{
		http.TimeFormat, // RFC1123: "Mon, 02 Jan 2006 15:04:05 GMT"
		time.RFC850,     // RFC850: "Monday, 02-Jan-06 15:04:05 MST"
		time.ANSIC,      // ANSIC: "Mon Jan _2 15:04:05 2006"
	}
	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			delay := time.Until(t)
			if delay > 0 {
				return delay
			}
		}
	}

	return 0
}

// validateNextURL validates that a pagination URL is safe to use.
// It ensures the URL is on the same host as BaseURL and uses HTTPS.
func validateNextURL(nextURL string) error {
	if nextURL == "" {
		return nil
	}

	// If it's not an absolute URL, it's relative and safe
	if !strings.HasPrefix(nextURL, "http://") && !strings.HasPrefix(nextURL, "https://") {
		return nil
	}

	// Parse the URL and compare hosts
	parsedURL, err := url.Parse(nextURL)
	if err != nil {
		return fmt.Errorf("invalid pagination URL: %w", err)
	}

	baseURL, err := url.Parse(BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}

	// Allow URLs on the same host as BaseURL
	if parsedURL.Host != baseURL.Host {
		return fmt.Errorf("rejected pagination URL from untrusted host %q (expected %q)", parsedURL.Host, baseURL.Host)
	}

	// Require HTTPS for authentication endpoints
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("rejected pagination URL with insecure scheme %q (expected https)", parsedURL.Scheme)
	}

	return nil
}

// allowedAnalyticsHosts contains the allowed host suffixes for analytics report downloads.
// Analytics reports are typically hosted on Apple-owned domains/CDNs.
// Based on Apple's enterprise network documentation and App Store Connect API behavior.
// Using suffix matching to allow subdomains (e.g., *.mzstatic.com).
var allowedAnalyticsHosts = []string{
	// Apple domains (allow subdomains)
	"itunes.apple.com",
	"apps.apple.com",
	"apple.com",
	"mzstatic.com",  // Apple static content CDN
	"cdn-apple.com", // Apple CDN
}

// allowedAnalyticsCDNHosts contains CDN host suffixes that require signed URLs.
// These hosts are used by Apple for analytics report delivery via presigned URLs.
var allowedAnalyticsCDNHosts = []string{
	"cloudfront.net",   // AWS CloudFront
	"amazonaws.com",    // AWS S3
	"s3.amazonaws.com", // AWS S3
	"azureedge.net",    // Azure CDN
}

var signedQueryKeys = map[string]struct{}{
	"x-amz-signature":     {},
	"x-amz-credential":    {},
	"x-amz-algorithm":     {},
	"x-amz-signedheaders": {},
	"signature":           {},
	"key-pair-id":         {},
	"policy":              {},
	"sig":                 {},
}

var sensitiveQueryKeys = map[string]struct{}{
	"x-amz-signature":      {},
	"x-amz-credential":     {},
	"x-amz-algorithm":      {},
	"x-amz-signedheaders":  {},
	"x-amz-security-token": {},
	"signature":            {},
	"key-pair-id":          {},
	"policy":               {},
	"sig":                  {},
	"token":                {},
	"access_token":         {},
	"id_token":             {},
	"refresh_token":        {},
}

// isAllowedAnalyticsHost checks if the host matches any allowed host suffix.
func isAllowedAnalyticsHost(host string) bool {
	for _, allowed := range allowedAnalyticsHosts {
		// Exact match or suffix match (for subdomains)
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// isAllowedAnalyticsCDNHost checks if the host matches any CDN host suffix.
func isAllowedAnalyticsCDNHost(host string) bool {
	for _, allowed := range allowedAnalyticsCDNHosts {
		if host == allowed || strings.HasSuffix(host, "."+allowed) {
			return true
		}
	}
	return false
}

// hasSignedAnalyticsQuery checks for common signed URL query parameters.
func hasSignedAnalyticsQuery(values url.Values) bool {
	return hasSignedQuery(values)
}

func hasSignedQuery(values url.Values) bool {
	for key, vals := range values {
		if _, ok := signedQueryKeys[strings.ToLower(key)]; ok && hasNonEmptyValue(vals) {
			return true
		}
	}
	return false
}

func isSensitiveQueryKey(key string) bool {
	_, ok := sensitiveQueryKeys[strings.ToLower(key)]
	return ok
}

func hasNonEmptyValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// validateAnalyticsDownloadURL validates that an analytics download URL is safe.
// It requires HTTPS and allows only trusted hosts, with signed URLs for CDNs.
func validateAnalyticsDownloadURL(downloadURL string) error {
	if downloadURL == "" {
		return fmt.Errorf("empty analytics download URL")
	}

	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("invalid analytics download URL: %w", err)
	}

	// Require HTTPS for all analytics downloads
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("rejected analytics download URL with insecure scheme %q (expected https)", parsedURL.Scheme)
	}

	host := strings.ToLower(parsedURL.Hostname())
	// Check against allowed hosts (with subdomain support)
	if isAllowedAnalyticsHost(host) {
		return nil
	}
	if isAllowedAnalyticsCDNHost(host) {
		if !hasSignedAnalyticsQuery(parsedURL.Query()) {
			return fmt.Errorf("rejected analytics download URL from CDN host %q without signed query", parsedURL.Host)
		}
		return nil
	}
	if host == "" {
		return fmt.Errorf("rejected analytics download URL with empty host")
	}
	return fmt.Errorf("rejected analytics download URL from untrusted host %q", parsedURL.Host)
}

func (c *Client) doStream(ctx context.Context, method, path string, body io.Reader, accept string) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err := ParseErrorWithStatus(respBody, resp.StatusCode); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}
	return resp, nil
}

func (c *Client) doStreamNoAuth(ctx context.Context, method, rawURL, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err := ParseErrorWithStatus(respBody, resp.StatusCode); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}
	return resp, nil
}

// BuildRequestBody builds a JSON request body
func BuildRequestBody(data interface{}) (io.Reader, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}
	return &buf, nil
}

// ParseError parses an error response (status code unknown)
func ParseError(body []byte) error {
	return ParseErrorWithStatus(body, 0)
}

// ParseErrorWithStatus parses an error response and includes the HTTP status code
func ParseErrorWithStatus(body []byte, statusCode int) error {
	var errResp struct {
		Errors []struct {
			Code   string `json:"code"`
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && len(errResp.Errors) > 0 {
		return &APIError{
			Code:       errResp.Errors[0].Code,
			Title:      errResp.Errors[0].Title,
			Detail:     errResp.Errors[0].Detail,
			StatusCode: statusCode,
		}
	}

	// Sanitize the error body to prevent information disclosure
	sanitized := sanitizeErrorBody(body)
	return fmt.Errorf("unknown error: %s", sanitized)
}

// sanitizeErrorBody limits the length and strips control characters from error bodies
// to prevent information disclosure and terminal escape sequence attacks.
func sanitizeErrorBody(body []byte) string {
	const maxLength = 200
	// Limit length
	if len(body) > maxLength {
		body = body[:maxLength]
	}
	// Strip control characters but keep printable characters and newlines
	result := make([]byte, 0, len(body))
	for _, b := range body {
		if b >= 32 || b == '\n' || b == '\r' || b == '\t' {
			result = append(result, b)
		}
	}
	return string(result)
}

// sanitizeTerminal strips control characters to prevent terminal escape injection.
// It removes ASCII control characters (0x00-0x1F) and DEL (0x7F).
func sanitizeTerminal(input string) string {
	if input == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(input))
	for _, r := range input {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// IsNotFound checks if the error is a "not found" error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if the error is an "unauthorized" error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}
