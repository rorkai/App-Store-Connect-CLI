package asc

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

// newRequest creates a new HTTP request with JWT authentication
func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if err := validateAPIPath(path); err != nil {
		return nil, err
	}
	if err := readonly.Check(ctx, method, readonly.Target(path)); err != nil {
		return nil, err
	}

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
	now := time.Now()

	c.jwtMu.Lock()
	defer c.jwtMu.Unlock()

	if c.cachedJWT != "" && now.Before(c.cachedJWTExpiresAt.Add(-jwtRefreshSkew)) {
		return c.cachedJWT, nil
	}

	signedToken, err := GenerateJWT(c.keyID, c.issuerID, c.privateKey)
	if err != nil {
		return "", err
	}

	c.cachedJWT = signedToken
	c.cachedJWTExpiresAt = now.Add(tokenLifetime)
	return signedToken, nil
}

// GenerateJWT generates a JWT for ASC API authentication.
func GenerateJWT(keyID, issuerID string, privateKey *ecdsa.PrivateKey) (string, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return "", ErrMissingKeyID
	}
	issuerID = strings.TrimSpace(issuerID)

	now := time.Now()
	claims := jwt.RegisteredClaims{
		Audience:  jwt.ClaimStrings{"appstoreconnect-v1"},
		IssuedAt:  jwt.NewNumericDate(now.Add(-jwtIssuedAtSkew)),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenLifetime)),
	}
	if issuerID == "" {
		claims.Subject = "user"
	} else {
		claims.Issuer = issuerID
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
// GET/HEAD requests use retry logic for transient failures by default.
// Mutating requests are throttled and retried only when App Store Connect
// rejects them with 429; see isRateLimitRejection.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	if err := validateMutatingRequestTarget(method, path); err != nil {
		return nil, err
	}

	request, err := c.replayableRequest(method, path, body)
	if err != nil {
		return nil, err
	}

	if shouldRetryMethod(method) {
		retryOpts := ResolveRetryOptions()
		return WithRetry(ctx, func() ([]byte, error) {
			return request(ctx)
		}, retryOpts)
	}
	if shouldLimitMutatingMethod(method) {
		return c.doMutation(ctx, request, isRateLimitRejection)
	}

	return request(ctx)
}

// doIdempotentMutation performs an explicitly idempotent mutating request with
// the same transient-failure retry policy used by reads. Callers must only use
// this for operations whose exact payload can be safely replayed.
func (c *Client) doIdempotentMutation(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	if err := validateMutatingRequestTarget(method, path); err != nil {
		return nil, err
	}

	request, err := c.replayableRequest(method, path, body)
	if err != nil {
		return nil, err
	}

	if !shouldLimitMutatingMethod(method) {
		return WithRetry(ctx, func() ([]byte, error) {
			return request(ctx)
		}, ResolveRetryOptions())
	}
	return c.doMutation(ctx, request, IsRetryable)
}

// doMutation sends a mutating request under the write concurrency limiter,
// replaying it while shouldRetry accepts the failure. Each attempt re-acquires
// a limiter slot so backoff never holds write capacity.
func (c *Client) doMutation(ctx context.Context, request func(context.Context) ([]byte, error), shouldRetry func(error) bool) ([]byte, error) {
	return withRetry(ctx, func() ([]byte, error) {
		return c.doWithMutatingRequestLimiter(ctx, request)
	}, ResolveRetryOptions(), shouldRetry)
}

// replayableRequest buffers the request body so every attempt sends the
// identical payload from a fresh reader.
func (c *Client) replayableRequest(method, path string, body io.Reader) (func(context.Context) ([]byte, error), error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	return func(requestCtx context.Context) ([]byte, error) {
		var reader io.Reader
		if bodyBytes != nil {
			reader = bytes.NewReader(bodyBytes)
		}
		return c.doOnce(requestCtx, method, path, reader)
	}, nil
}

// isRateLimitRejection reports whether err is a 429 from App Store Connect.
// A 429 rejects the request before it is processed, so replaying the identical
// payload cannot apply a mutation twice. Every other retryable failure
// (transport errors, 408, 5xx) leaves the outcome of a write unknown and must
// not be replayed automatically.
func isRateLimitRejection(err error) bool {
	retryable, ok := errors.AsType[*RetryableError](err)
	if !ok {
		return false
	}
	return retryable.HTTPStatusCode() == http.StatusTooManyRequests
}

func (c *Client) doWithMutatingRequestLimiter(ctx context.Context, request func(context.Context) ([]byte, error)) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("wait for mutating request slot: %w", err)
	}

	requestTimeout, hasDeadline := requestTimeoutBudget(ctx)
	limiter := c.getMutatingRequestLimiter()

	select {
	case limiter <- struct{}{}:
		defer func() { <-limiter }()
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait for mutating request slot: %w", err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("wait for mutating request slot: %w", ctx.Err())
	}

	requestCtx, cancel := deriveMutatingRequestContext(ctx, requestTimeout, hasDeadline)
	defer cancel()
	return request(requestCtx)
}

func requestTimeoutBudget(ctx context.Context) (time.Duration, bool) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0, false
	}
	return time.Until(deadline), true
}

func deriveMutatingRequestContext(ctx context.Context, requestTimeout time.Duration, hasDeadline bool) (context.Context, context.CancelFunc) {
	// Preserve context values, but give queued writes a fresh timeout budget once they acquire a slot.
	base := context.WithoutCancel(ctx)

	var (
		requestCtx context.Context
		cancel     context.CancelFunc
	)
	if hasDeadline {
		requestCtx, cancel = context.WithTimeout(base, requestTimeout)
	} else {
		requestCtx, cancel = context.WithCancel(base)
	}

	stop := context.AfterFunc(ctx, func() {
		if errors.Is(ctx.Err(), context.Canceled) {
			cancel()
		}
	})
	return requestCtx, func() {
		stop()
		cancel()
	}
}

func (c *Client) doOnce(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	start := time.Now()
	debugSettings := resolveDebugSettings()

	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	if debugSettings.verboseHTTP {
		debugLogger.Info(
			"→ HTTP Request",
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
			debugLogger.Info(
				"← HTTP Error",
				"error", err.Error(),
				"elapsed", elapsed.String(),
			)
		}
		requestErr := fmt.Errorf("request failed: %w", err)
		if isTransientTransportError(err) {
			return nil, &RetryableError{Err: requestErr}
		}
		return nil, requestErr
	}
	defer resp.Body.Close()

	if debugSettings.verboseHTTP {
		debugLogger.Info(
			"← HTTP Response",
			"status", resp.StatusCode,
			"elapsed", elapsed.String(),
			"content-type", resp.Header.Get("Content-Type"),
			"content-length", resp.Header.Get("Content-Length"),
			"x-rate-limit", resp.Header.Get("X-Rate-Limit"),
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)

		if isRetryableHTTPStatus(resp.StatusCode) {
			retryAfter := parseRetryAfterHeader(resp.Header.Get("Retry-After"))
			return nil, &RetryableError{
				Err:        buildRetryableError(resp.StatusCode, retryAfter, respBody),
				RetryAfter: retryAfter,
			}
		}

		if err := ParseErrorWithStatus(respBody, resp.StatusCode); err != nil {
			if isIntermittentBuildUploadsNotFound(method, path, err) {
				// No Retry-After accompanies the flake; the shared exponential
				// backoff (1s/2s/4s by default) and retry budget apply.
				return nil, &RetryableError{Err: err}
			}
			return nil, err
		}
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		readErr := &responseBodyReadError{err: err}
		if isTransientTransportError(err) {
			return nil, &RetryableError{Err: readErr}
		}
		return nil, readErr
	}
	return data, nil
}

func isTransientTransportError(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE)
}

func isRetryableHTTPStatus(statusCode int) bool {
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

// isBuildUploadsPath reports whether path targets the Build Upload API family
// (/v1/buildUploads, /v1/buildUploadFiles, and the app relationship views).
// Apple intermittently answers reads on these endpoints with 404 NOT_FOUND for
// resources that exist (fastlane/fastlane#29908), so reads there are allowed
// a bounded retry that no other 404 receives. Full next-page URLs are matched
// on their path component.
func isBuildUploadsPath(path string) bool {
	segments := apiPathSegments(path)
	if len(segments) < 2 || segments[0] != "v1" {
		return false
	}
	switch segments[1] {
	case "buildUploads", "buildUploadFiles":
		return true
	case "apps":
		// /v1/apps/{id}/buildUploads and /v1/apps/{id}/relationships/buildUploads
		switch len(segments) {
		case 4:
			return segments[3] == "buildUploads"
		case 5:
			return segments[3] == "relationships" && segments[4] == "buildUploads"
		}
	}
	return false
}

// isIntermittentBuildUploadsNotFound reports whether err is the 404 NOT_FOUND
// flake Apple emits on Build Upload API reads: the app relationship is
// reported missing for an app that exists. Only reads are eligible, and only
// when the missing resource is the app. On /v1/apps/{id}/buildUploads the app
// is the only resource that can be missing, so any NOT_FOUND qualifies. On
// /v1/buildUploads* and /v1/buildUploadFiles* the detail must name resource
// type 'apps'; a genuinely missing upload or file id ("no resource of type
// 'buildUploads'") is surfaced unchanged without consuming the retry budget.
func isIntermittentBuildUploadsNotFound(method, path string, err error) bool {
	if !shouldRetryMethod(method) || !isBuildUploadsPath(path) {
		return false
	}
	apiErr, ok := errors.AsType[*APIError](err)
	if !ok || apiErr.StatusCode != http.StatusNotFound || !strings.EqualFold(apiErr.Code, "NOT_FOUND") {
		return false
	}
	if isAppBuildUploadsPath(path) {
		return true
	}
	return notFoundNamesAppResource(apiErr)
}

// isAppBuildUploadsPath reports whether path is the app-scoped Build Upload
// relationship view (/v1/apps/{id}/buildUploads or its relationships form).
func isAppBuildUploadsPath(path string) bool {
	segments := apiPathSegments(path)
	return len(segments) >= 3 && segments[0] == "v1" && segments[1] == "apps"
}

// notFoundNamesAppResource reports whether a NOT_FOUND detail names the apps
// resource type, e.g. "There is no resource of type 'apps' with id '123'".
func notFoundNamesAppResource(apiErr *APIError) bool {
	return appResourceNotFoundDetail.MatchString(apiErr.Detail)
}

var appResourceNotFoundDetail = regexp.MustCompile(`(?i)resource of type ['"‘’“”]?apps['"‘’“”]?(?:\s|$)`)

// apiPathSegments returns the path segments of an API path or absolute URL,
// without the query string.
func apiPathSegments(path string) []string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		parsed, err := url.Parse(path)
		if err != nil {
			return nil
		}
		path = parsed.Path
	}
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	return strings.Split(strings.Trim(path, "/"), "/")
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
	return urlsanitize.SanitizeURLForLog(rawURL, signedQueryKeys, sensitiveQueryKeys)
}

func shouldRetryMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

func shouldLimitMutatingMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

func buildRetryableError(statusCode int, retryAfter time.Duration, respBody []byte) error {
	return buildRetryableErrorWithSource(statusCode, retryAfter, respBody, "App Store Connect")
}

func buildRetryableErrorWithSource(statusCode int, retryAfter time.Duration, respBody []byte, source string) error {
	base := "API request failed"
	switch statusCode {
	case http.StatusRequestTimeout:
		base = fmt.Sprintf("%s request timeout", source)
	case http.StatusTooManyRequests:
		base = fmt.Sprintf("rate limited by %s", source)
	case http.StatusInternalServerError:
		base = fmt.Sprintf("%s internal server error", source)
	case http.StatusBadGateway:
		base = fmt.Sprintf("%s bad gateway", source)
	case http.StatusServiceUnavailable:
		base = fmt.Sprintf("%s service unavailable", source)
	case http.StatusGatewayTimeout:
		base = fmt.Sprintf("%s gateway timeout", source)
	}

	message := fmt.Sprintf("%s (status %d)", base, statusCode)
	var apiErr *APIError
	if len(respBody) > 0 {
		if parsed, ok := errors.AsType[*APIError](ParseErrorWithStatus(respBody, statusCode)); ok {
			apiErr = parsed
			message = fmt.Sprintf("%s: %s", message, parsed)
		}
	}
	if apiErr == nil {
		apiErr = &APIError{
			Code:       apiErrorCodeFromStatus(statusCode),
			Title:      base,
			StatusCode: statusCode,
		}
	}
	if retryAfter > 0 {
		message = fmt.Sprintf("%s (retry after %s)", message, retryAfter)
	}
	return &retryableStatusError{message: message, apiErr: apiErr}
}

// retryableStatusError keeps the human-readable retry message while exposing
// the parsed *APIError (and its HTTP status code) through Unwrap, so that
// errors.Is/errors.As and exit-code mapping still observe the underlying
// HTTP status after retries are exhausted.
type retryableStatusError struct {
	message string
	apiErr  *APIError
}

func (e *retryableStatusError) Error() string {
	return e.message
}

func (e *retryableStatusError) Unwrap() error {
	return e.apiErr
}

// parseRetryAfterHeader parses the Retry-After header value.
// Supports seconds (e.g., "60") or HTTP-date format (RFC1123, RFC850, ANSIC).
func parseRetryAfterHeader(value string) time.Duration {
	if value = strings.TrimSpace(value); value == "" {
		return 0
	}

	// Try to parse as seconds first
	const maxRetryAfterDuration = time.Duration(1<<63 - 1)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds > 0 {
			if seconds > int64(maxRetryAfterDuration/time.Second) {
				return maxRetryAfterDuration
			}
			return time.Duration(seconds) * time.Second
		}
	} else if isPositiveDecimal(value) {
		// ParseInt reports ErrRange for values above MaxInt64. ParseUint still
		// accepts the portion that fits in uint64; values beyond that range
		// report ErrRange there as well. Both cases are an unambiguously huge
		// positive delay and must saturate instead of falling back to backoff.
		if _, unsignedErr := strconv.ParseUint(strings.TrimPrefix(value, "+"), 10, 64); unsignedErr == nil {
			return maxRetryAfterDuration
		} else {
			var numberErr *strconv.NumError
			if errors.As(unsignedErr, &numberErr) && errors.Is(numberErr.Err, strconv.ErrRange) {
				return maxRetryAfterDuration
			}
		}
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

func isPositiveDecimal(value string) bool {
	value = strings.TrimPrefix(value, "+")
	if value == "" {
		return false
	}
	positive := false
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
		positive = positive || value[i] != '0'
	}
	return positive
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

var signedQueryKeys = urlsanitize.CopyKeySet(urlsanitize.DefaultSignedQueryKeys)

var sensitiveQueryKeys = urlsanitize.CopyKeySet(urlsanitize.DefaultSensitiveQueryKeys)

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
	return urlsanitize.HasSignedQuery(values, signedQueryKeys)
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

func (c *Client) doStream(ctx context.Context, path string, accept string) (*http.Response, error) {
	req, err := c.newRequest(ctx, "GET", path, nil)
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

func (c *Client) doStreamNoAuth(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, newSanitizedNoAuthStreamError("create download request", rawURL, err)
	}
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}

	client := clientWithoutRedirects(c.httpClient)
	resp, err := client.Do(req)
	if err != nil {
		return nil, newSanitizedNoAuthStreamError("download request", rawURL, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = resp.Body.Close()
		// Presigned-CDN error bodies are untrusted and can echo the requested
		// capability. The status code is sufficient diagnostic context here.
		return nil, &APIError{
			Code:       apiErrorCodeFromStatus(resp.StatusCode),
			Title:      fmt.Sprintf("download request failed with status %d", resp.StatusCode),
			StatusCode: resp.StatusCode,
		}
	}
	return resp, nil
}

func newSanitizedNoAuthStreamError(operation, rawURL string, err error) error {
	return urlsanitize.NewTransportError(operation, urlsanitize.RedactURLForError(rawURL), err)
}

// BuildRequestBody builds a JSON request body
func BuildRequestBody(data any) (io.Reader, error) {
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
			Code   string          `json:"code"`
			Title  string          `json:"title"`
			Detail string          `json:"detail"`
			Meta   json.RawMessage `json:"meta"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && len(errResp.Errors) > 0 {
		associatedErrors := parseAssociatedErrors(errResp.Errors[0].Meta)
		return &APIError{
			Code:             errResp.Errors[0].Code,
			Title:            errResp.Errors[0].Title,
			Detail:           errResp.Errors[0].Detail,
			StatusCode:       statusCode,
			AssociatedErrors: associatedErrors,
			Remediation:      remediationForAPIError(errResp.Errors[0].Code),
		}
	}

	// Sanitize the error body to prevent information disclosure
	sanitized := sanitizeErrorBody(body)
	if strings.TrimSpace(sanitized) == "" {
		sanitized = "unknown error"
	}

	return &APIError{
		Code:       apiErrorCodeFromStatus(statusCode),
		Title:      "API request failed",
		Detail:     sanitized,
		StatusCode: statusCode,
	}
}

func parseAssociatedErrors(metaRaw json.RawMessage) map[string][]APIAssociatedError {
	if len(metaRaw) == 0 {
		return nil
	}

	var meta map[string]json.RawMessage
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return nil
	}

	associatedRaw, ok := meta["associatedErrors"]
	if !ok {
		return nil
	}

	var associatedByResourceRaw map[string]json.RawMessage
	if err := json.Unmarshal(associatedRaw, &associatedByResourceRaw); err != nil {
		return nil
	}

	associatedByResource := make(map[string][]APIAssociatedError)
	for resource, entriesRaw := range associatedByResourceRaw {
		var rawEntries []json.RawMessage
		if err := json.Unmarshal(entriesRaw, &rawEntries); err != nil {
			continue
		}

		entries := make([]APIAssociatedError, 0, len(rawEntries))
		for _, rawEntry := range rawEntries {
			var parsed struct {
				Code   string `json:"code"`
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(rawEntry, &parsed); err != nil {
				continue
			}
			if strings.TrimSpace(parsed.Code) == "" && strings.TrimSpace(parsed.Detail) == "" {
				continue
			}
			entries = append(entries, APIAssociatedError{
				Code:   parsed.Code,
				Detail: parsed.Detail,
			})
		}

		if len(entries) > 0 {
			associatedByResource[resource] = entries
		}
	}

	if len(associatedByResource) == 0 {
		return nil
	}
	return associatedByResource
}

func apiErrorCodeFromStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	default:
		return ""
	}
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

// validateAPIPath checks a relative API path for dangerous characters that
// could indicate a hallucinated or malicious resource ID. Full URLs (pagination
// cursors) are skipped — they are validated separately by validateNextURL.
//
// The check operates on path segments only (before any query string) and rejects:
//   - control characters (< 0x20, DEL)
//   - pre-encoded percent sequences (%) that would cause double-encoding
//   - fragment markers (#) that truncate the URL
//   - backslashes that may be misinterpreted across platforms
//   - path traversal segments (.. or empty segments //)
func validateAPIPath(path string) error {
	if path == "" {
		return nil
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return nil
	}

	pathOnly := path
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		pathOnly = path[:idx]
	}

	for _, r := range pathOnly {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("API path contains control character: %q", path)
		}
	}

	if strings.ContainsAny(pathOnly, "#%\\") {
		return fmt.Errorf("API path contains unsafe character (#, %%, or \\): %q", path)
	}

	if strings.Contains(pathOnly, "//") {
		return fmt.Errorf("API path contains empty segment (//): %q", path)
	}

	for _, segment := range strings.Split(pathOnly, "/") {
		if segment == ".." {
			return fmt.Errorf("API path contains traversal segment: %q", path)
		}
	}

	return nil
}

// IsNotFound checks if the error is a "not found" error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if the error is an "unauthorized" error
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized)
}
