package asc

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

const (
	ascAPIHost        = "api.appstoreconnect.apple.com"
	enterpriseAPIHost = "api.enterprise.developer.apple.com"

	// BaseURL is the App Store Connect API base URL
	BaseURL = "https://" + ascAPIHost
	// EnterpriseBaseURL is the Apple Developer Enterprise Program API base URL
	EnterpriseBaseURL = "https://" + enterpriseAPIHost

	ascJWTAudience        = "appstoreconnect-v1"
	enterpriseJWTAudience = "apple-developer-enterprise-v1"

	// DefaultTimeout is the default request timeout
	DefaultTimeout = 30 * time.Second
	// DefaultUploadTimeout is the default timeout for upload operations.
	DefaultUploadTimeout = 300 * time.Second
	// tokenLifetime is the JWT token lifetime for App Store Connect API authentication.
	// 10 minutes is a good balance between security (shorter-lived tokens) and usability.
	tokenLifetime = 10 * time.Minute
	// jwtRefreshSkew refreshes a token a bit early to avoid edge-of-expiry races.
	jwtRefreshSkew = 30 * time.Second
	// jwtIssuedAtSkew backdates the issued-at claim so a client clock running
	// ahead of Apple's does not produce a token rejected as issued in the future.
	jwtIssuedAtSkew = 60 * time.Second

	// Retry defaults
	DefaultMaxRetries = 3
	DefaultBaseDelay  = 1 * time.Second
	DefaultMaxDelay   = 30 * time.Second

	defaultMaxIdleConns         = 128
	defaultMaxIdleConnsPerHost  = 32
	defaultMutatingRequestLimit = 8

	// BulkMutatingRequestLimit is the client-wide ceiling on concurrent
	// mutating requests. Ordinary writes are held to
	// defaultMutatingRequestLimit beneath it; only writes whose context is
	// marked with WithBulkMutatingRequestLimit may use the extra slots, so a
	// bulk fan-out can run more writes at once without raising the limit for
	// every other command.
	BulkMutatingRequestLimit = 16
)

var retryLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
	ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if attr.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return attr
	},
}))

var debugLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
	ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if attr.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return attr
	},
}))

var retryLogOverride struct {
	mu  sync.RWMutex
	val *bool
}

var debugOverride struct {
	mu          sync.RWMutex
	enabled     *bool
	verboseHTTP *bool
}

var (
	loadConfigFn = config.Load
	loadConfigMu sync.Mutex
	cachedConfig *config.Config
	configLoaded bool
)

type debugSettings struct {
	enabled     bool
	verboseHTTP bool
}

// SetRetryLogOverride sets an explicit retry-log override.
// When set, it takes precedence over env/config. When unset (nil), behavior falls back to env/config.
func SetRetryLogOverride(value *bool) {
	retryLogOverride.mu.Lock()
	defer retryLogOverride.mu.Unlock()
	retryLogOverride.val = value
}

// SetDebugOverride sets an explicit debug override.
// When set, it takes precedence over env/config. When unset (nil), behavior falls back to env/config.
func SetDebugOverride(value *bool) {
	debugOverride.mu.Lock()
	defer debugOverride.mu.Unlock()
	debugOverride.enabled = value
}

// SetDebugHTTPOverride sets an explicit HTTP-debug override.
// When set, it takes precedence over env/config for HTTP logging only.
func SetDebugHTTPOverride(value *bool) {
	debugOverride.mu.Lock()
	defer debugOverride.mu.Unlock()
	debugOverride.verboseHTTP = value
}

// ResolveRetryLogEnabled returns whether retry logging should be enabled.
// Precedence: explicit override > env > config.
func ResolveRetryLogEnabled() bool {
	retryLogOverride.mu.RLock()
	override := retryLogOverride.val
	retryLogOverride.mu.RUnlock()
	if override != nil {
		return *override
	}
	if override, ok := envValue("ASC_RETRY_LOG"); ok {
		return override != ""
	}
	cfg := loadConfig()
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.RetryLog) != ""
}

// ResolveDebugEnabled returns whether debug logging should be enabled.
// Precedence: explicit override > env > config.
func ResolveDebugEnabled() bool {
	return resolveDebugSettings().enabled
}

func resolveDebugSettings() debugSettings {
	settings := debugSettings{}
	if value, ok := envValue("ASC_DEBUG"); ok {
		settings = resolveDebugValue(value)
	} else {
		cfg := loadConfig()
		if cfg != nil {
			settings = resolveDebugValue(cfg.Debug)
		}
	}

	debugOverride.mu.RLock()
	enabledOverride := debugOverride.enabled
	verboseOverride := debugOverride.verboseHTTP
	debugOverride.mu.RUnlock()

	if verboseOverride != nil {
		settings.verboseHTTP = *verboseOverride
		if *verboseOverride {
			settings.enabled = true
		}
	}

	if enabledOverride != nil {
		if !*enabledOverride {
			return debugSettings{}
		}
		settings.enabled = true
		if verboseOverride == nil {
			settings.verboseHTTP = false
		}
	}

	return settings
}

func resolveDebugValue(value string) debugSettings {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return debugSettings{}
	}
	normalized := strings.ToLower(trimmed)
	switch normalized {
	case "0", "false", "no":
		return debugSettings{}
	}
	return debugSettings{
		enabled:     true,
		verboseHTTP: strings.Contains(normalized, "api"),
	}
}

func loadConfig() *config.Config {
	loadConfigMu.Lock()
	defer loadConfigMu.Unlock()

	if configLoaded {
		return cachedConfig
	}

	cfg, err := loadConfigFn()
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return nil
	}
	cachedConfig = cfg
	configLoaded = true
	return cachedConfig
}

func resetConfigCacheForTest() {
	loadConfigMu.Lock()
	defer loadConfigMu.Unlock()

	loadConfigFn = config.Load
	cachedConfig = nil
	configLoaded = false
}

func setConfigLoaderForTest(loader func() (*config.Config, error)) {
	loadConfigMu.Lock()
	defer loadConfigMu.Unlock()

	if loader == nil {
		loadConfigFn = config.Load
	} else {
		loadConfigFn = loader
	}
	cachedConfig = nil
	configLoaded = false
}

func envValue(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	return strings.TrimSpace(value), ok
}

// RetryableError is returned when a request can be retried (e.g., rate limiting).
type RetryableError struct {
	Err        error
	RetryAfter time.Duration
	// PreserveErrorOnDeadline requests terminal classification when a computed
	// fallback wait cannot finish before the context deadline. Explicit context
	// cancellation still takes precedence.
	PreserveErrorOnDeadline bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// HTTPStatusCode reports the HTTP status code carried by the wrapped error,
// or 0 when no status is known (e.g., transport failures).
func (e *RetryableError) HTTPStatusCode() int {
	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(e.Err, &statusErr) {
		return statusErr.HTTPStatusCode()
	}
	return 0
}

// IsRetryable checks if an error indicates the request can be retried.
func IsRetryable(err error) bool {
	_, ok := errors.AsType[*RetryableError](err)
	return ok
}

// GetRetryAfter extracts the retry-after duration from an error.
func GetRetryAfter(err error) time.Duration {
	if re, ok := errors.AsType[*RetryableError](err); ok {
		return re.RetryAfter
	}
	return 0
}

type retryBudgetExceededError struct {
	retries int
	err     error
}

func (e *retryBudgetExceededError) Error() string {
	return fmt.Sprintf("retry limit exceeded after %d retries: %v", e.retries, e.err)
}

func (e *retryBudgetExceededError) Unwrap() error {
	return e.err
}

// IsRetryBudgetExhausted reports whether the retry helper consumed its
// configured request retry budget before returning the error. Callers with a
// second, higher-level recovery loop should not replay the same request after
// this marker is present.
func IsRetryBudgetExhausted(err error) bool {
	var exhausted *retryBudgetExceededError
	return errors.As(err, &exhausted)
}

type retryCancelledError struct {
	contextErr error
	err        error
}

func (e *retryCancelledError) Error() string {
	return fmt.Sprintf("retry cancelled: %v", e.contextErr)
}

func (e *retryCancelledError) Unwrap() []error {
	return []error{e.contextErr, e.err}
}

// retryDelayExceededError marks a retryable failure whose requested or
// computed delay could not be honored by this request. It preserves the
// original retryable cause for status and Retry-After inspection, while
// allowing outer recovery loops to treat the already-diagnosed wait as
// terminal.
type retryDelayExceededError struct {
	err error
}

func (e *retryDelayExceededError) Error() string {
	return e.err.Error()
}

func (e *retryDelayExceededError) Unwrap() error {
	return e.err
}

// IsRetryDelayExceeded reports whether a retryable failure was returned after
// its server-provided delay could not be honored by the request. The original
// retryable cause remains available through the error chain.
func IsRetryDelayExceeded(err error) bool {
	var delayErr *retryDelayExceededError
	return errors.As(err, &delayErr)
}

// RetryOptions configures retry behavior.
//   - MaxRetries: Number of retry attempts. 0 = no retries (fail fast),
//     negative = use DefaultMaxRetries.
//   - BaseDelay: Initial delay between retries (with exponential backoff).
//   - MaxDelay: Maximum delay cap for backoff and honored Retry-After hints;
//     an explicit hint above this cap fails fast with the requested and
//     configured durations instead of sleeping at the cap.
type RetryOptions struct {
	MaxRetries int           // 0=disabled, negative=default, positive=retry count
	BaseDelay  time.Duration // Initial delay for exponential backoff
	MaxDelay   time.Duration // Maximum delay cap
}

// ResolveRetryOptions returns retry options, optionally overridden by config/env.
func ResolveRetryOptions() RetryOptions {
	opts := RetryOptions{
		MaxRetries: DefaultMaxRetries,
		BaseDelay:  DefaultBaseDelay,
		MaxDelay:   DefaultMaxDelay,
	}

	cfg := loadConfig()

	if override, ok := envValue("ASC_MAX_RETRIES"); ok {
		if override != "" {
			if parsed, err := strconv.Atoi(override); err == nil && parsed >= 0 {
				opts.MaxRetries = parsed
			}
		}
	} else if cfg != nil {
		if override := strings.TrimSpace(cfg.MaxRetries); override != "" {
			if parsed, err := strconv.Atoi(override); err == nil && parsed >= 0 {
				opts.MaxRetries = parsed
			}
		}
	}

	if override, ok := envValue("ASC_BASE_DELAY"); ok {
		if override != "" {
			if parsed, err := time.ParseDuration(override); err == nil && parsed > 0 {
				opts.BaseDelay = parsed
			}
		}
	} else if cfg != nil {
		if override := strings.TrimSpace(cfg.BaseDelay); override != "" {
			if parsed, err := time.ParseDuration(override); err == nil && parsed > 0 {
				opts.BaseDelay = parsed
			}
		}
	}

	if override, ok := envValue("ASC_MAX_DELAY"); ok {
		if override != "" {
			if parsed, err := time.ParseDuration(override); err == nil && parsed > 0 {
				opts.MaxDelay = parsed
			}
		}
	} else if cfg != nil {
		if override := strings.TrimSpace(cfg.MaxDelay); override != "" {
			if parsed, err := time.ParseDuration(override); err == nil && parsed > 0 {
				opts.MaxDelay = parsed
			}
		}
	}
	return opts
}

// WithRetry executes a function with retry logic for rate limiting.
// It uses exponential backoff with jitter and respects Retry-After headers.
func WithRetry[T any](ctx context.Context, fn func() (T, error), opts RetryOptions) (T, error) {
	return withRetry(ctx, fn, opts, IsRetryable)
}

// withRetry executes a function with the shared backoff policy, retrying only
// the errors accepted by shouldRetry. Callers that can replay a request safely
// only under narrower conditions (writes, which are retryable when App Store
// Connect rejects them outright) supply their own predicate.
func withRetry[T any](ctx context.Context, fn func() (T, error), opts RetryOptions, shouldRetry func(error) bool) (T, error) {
	var zero T
	debugEnabled := ResolveDebugEnabled()

	// If MaxRetries is negative, use the default; if zero, fail on first error
	if opts.MaxRetries < 0 {
		opts.MaxRetries = DefaultMaxRetries
	}
	if opts.MaxRetries == 0 {
		return fn()
	}

	if opts.BaseDelay <= 0 {
		opts.BaseDelay = DefaultBaseDelay
	}
	if opts.MaxDelay <= 0 {
		opts.MaxDelay = DefaultMaxDelay
	}

	retryCount := 0

	for {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		// Check if error is retryable
		if !shouldRetry(err) {
			return zero, err
		}

		// Calculate delay
		retryAfter := GetRetryAfter(err)
		delay := retryAfter
		if retryAfter > 0 {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return zero, &retryCancelledError{contextErr: ctxErr, err: err}
			}
			if deadline, ok := ctx.Deadline(); ok {
				remaining := time.Until(deadline)
				if delay >= remaining {
					if delay > opts.MaxDelay {
						return zero, &retryDelayExceededError{err: fmt.Errorf(
							"%s: upstream server asked to wait %s, exceeding the %s retry cap and the context deadline (%s remaining); raise ASC_MAX_DELAY and the request timeout to wait longer: %w",
							retryDelayCategory(err), delay, opts.MaxDelay, remaining.Round(time.Millisecond), err,
						)}
					}
					return zero, &retryDelayExceededError{err: fmt.Errorf(
						"%s: upstream server asked to wait %s, which cannot be honored before the context deadline (%s remaining): %w",
						retryDelayCategory(err), delay, remaining.Round(time.Millisecond), err,
					)}
				}
			}
		}

		switch {
		case delay <= 0:
			// Exponential backoff with jitter, capped to prevent overflow
			expDelay := opts.BaseDelay
			if retryCount > 0 && retryCount < 31 { // Prevent overflow for reasonable retry counts
				expDelay = opts.BaseDelay * time.Duration(1<<retryCount)
			}
			if expDelay > opts.MaxDelay || expDelay <= 0 {
				expDelay = opts.MaxDelay
			}
			// Add jitter: ±25% of the delay
			jitter := float64(expDelay) * 0.25 * (2*rand.Float64() - 1)
			delay = expDelay + time.Duration(jitter)
			if delay < 0 {
				delay = expDelay / 2 // minimum delay
			}
		case delay > opts.MaxDelay:
			// The server asked for longer than this run is willing to wait.
			// Retrying at the cap would just collect the same rejection, and
			// sleeping the full hint hides the response behind an eventual
			// context deadline, so report both numbers now.
			category := retryDelayCategory(err)
			return zero, &retryDelayExceededError{err: fmt.Errorf(
				"%s: upstream server asked to wait %s, exceeding the %s retry cap (raise ASC_MAX_DELAY to wait longer): %w",
				category, delay, opts.MaxDelay, err,
			)}
		}

		if retryPreservesErrorOnDeadline(err) {
			if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
				if ctx.Err() == nil {
					remaining := time.Until(deadline)
					return zero, &retryDelayExceededError{err: fmt.Errorf(
						"%s: computed fallback backoff %s cannot be honored before the context deadline (%s remaining): %w",
						retryDelayCategory(err), delay, remaining.Round(time.Millisecond), err,
					)}
				}
			}
		}

		// Check if we've exceeded max retries after classifying any explicit
		// Retry-After hint. A final attempt carrying an unhonored hint must
		// remain terminal to outer recovery loops instead of being hidden by
		// the retry-budget marker.
		if retryCount >= opts.MaxRetries {
			if contextErr := ctx.Err(); contextErr != nil {
				return zero, &retryCancelledError{contextErr: contextErr, err: err}
			}
			return zero, &retryBudgetExceededError{retries: retryCount + 1, err: err}
		}

		if ResolveRetryLogEnabled() {
			logRetry(delay, retryCount+1, opts.MaxRetries, err)
		}

		if debugEnabled {
			debugLogger.Info(
				"⟳ Retrying request",
				"attempt", retryCount+1,
				"max_retries", opts.MaxRetries,
				"delay", delay.String(),
				"error", err.Error(),
			)
		}

		observeRequestRetry(ctx, err)
		retryCount++

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			// Preserve the last retryable failure as well as the cancellation
			// cause. Callers that reconcile an ambiguous mutation must not lose
			// a 429 (or its Retry-After hint) when the wait outlives the request
			// deadline.
			return zero, &retryCancelledError{
				contextErr: ctx.Err(),
				err:        &retryBudgetExceededError{retries: retryCount, err: err},
			}
		case <-time.After(delay):
			// Continue to next retry
		}
	}
}

func retryPreservesErrorOnDeadline(err error) bool {
	retryErr, ok := errors.AsType[*RetryableError](err)
	return ok && retryErr.PreserveErrorOnDeadline
}

func retryDelayCategory(err error) string {
	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(err, &statusErr) && statusErr.HTTPStatusCode() == http.StatusTooManyRequests {
		return "rate limited"
	}
	return "retry delayed"
}

func logRetry(delay time.Duration, attempt, maxRetries int, err error) {
	retryLogger.Info("retrying request", "delay", delay.String(), "attempt", attempt, "maxRetries", maxRetries, "error", err)
}

// ResolveTimeout returns the request timeout, optionally overridden by config/env.
func ResolveTimeout() time.Duration {
	return ResolveTimeoutWithDefault(DefaultTimeout)
}

// ResolveUploadTimeout returns the upload timeout, optionally overridden by config/env.
func ResolveUploadTimeout() time.Duration {
	return ResolveUploadTimeoutWithDefault(DefaultUploadTimeout)
}

// ResolveUploadTimeoutWithDefault returns the upload timeout using a custom default.
// ASC_UPLOAD_TIMEOUT and ASC_UPLOAD_TIMEOUT_SECONDS override the default when set.
func ResolveUploadTimeoutWithDefault(defaultTimeout time.Duration) time.Duration {
	cfg := loadConfig()
	var uploadTimeout config.DurationValue
	var uploadTimeoutSeconds config.DurationValue
	if cfg != nil {
		uploadTimeout = cfg.UploadTimeout
		uploadTimeoutSeconds = cfg.UploadTimeoutSeconds
	}
	return resolveTimeoutWithDefaultAndEnv(defaultTimeout, "ASC_UPLOAD_TIMEOUT", "ASC_UPLOAD_TIMEOUT_SECONDS", uploadTimeout, uploadTimeoutSeconds)
}

// ResolveTimeoutWithDefault returns the request timeout using a custom default.
// ASC_TIMEOUT and ASC_TIMEOUT_SECONDS override the default when set.
func ResolveTimeoutWithDefault(defaultTimeout time.Duration) time.Duration {
	cfg := loadConfig()
	var timeout config.DurationValue
	var timeoutSeconds config.DurationValue
	if cfg != nil {
		timeout = cfg.Timeout
		timeoutSeconds = cfg.TimeoutSeconds
	}
	return resolveTimeoutWithDefaultAndEnv(defaultTimeout, "ASC_TIMEOUT", "ASC_TIMEOUT_SECONDS", timeout, timeoutSeconds)
}

func resolveTimeoutWithDefaultAndEnv(defaultTimeout time.Duration, durationEnv, secondsEnv string, durationConfig, secondsConfig config.DurationValue) time.Duration {
	timeout := defaultTimeout
	if override, ok := envValue(durationEnv); ok {
		if override != "" {
			if parsed, err := time.ParseDuration(override); err == nil && parsed > 0 {
				timeout = parsed
			}
		}
		return timeout
	}
	if override, ok := envValue(secondsEnv); ok {
		if override != "" {
			if parsed, err := strconv.Atoi(override); err == nil && parsed > 0 {
				timeout = time.Duration(parsed) * time.Second
			}
		}
		return timeout
	}
	if override, ok := durationConfig.Value(); ok {
		timeout = override
	} else if override, ok := secondsConfig.Value(); ok {
		timeout = override
	}
	return timeout
}

// Client is an App Store Connect API client
type Client struct {
	uploadTransportMu    sync.Mutex
	uploadTransport      http.RoundTripper
	ownedUploadTransport *http.Transport

	httpClient    *http.Client
	keyID         string
	issuerID      string
	privateKey    *ecdsa.PrivateKey
	enterprise    bool
	notaryBaseURL string // override for testing; empty uses NotaryBaseURL constant

	jwtMu              sync.Mutex
	cachedJWT          string
	cachedJWTExpiresAt time.Time

	// bearerToken is a token signed elsewhere (ASC_BEARER_TOKEN); when set,
	// the client has no private key and sends it until it expires.
	bearerToken          string
	bearerTokenExpiresAt time.Time

	mutatingRequestLimiterOnce sync.Once
	mutatingRequestLimiter     chan struct{}

	bulkMutatingRequestLimiterOnce sync.Once
	bulkMutatingRequestLimiter     chan struct{}
}

// NewClient creates a new ASC client.
func NewClient(keyID, issuerID, privateKeyPath string) (*Client, error) {
	return newClientWithHTTPClient(keyID, issuerID, privateKeyPath, newDefaultHTTPClient(ResolveTimeout()))
}

// NewClientWithTimeout creates a new ASC client with an explicit HTTP timeout.
func NewClientWithTimeout(keyID, issuerID, privateKeyPath string, timeout time.Duration) (*Client, error) {
	return newClientWithHTTPClient(keyID, issuerID, privateKeyPath, newDefaultHTTPClient(timeout))
}

// NewClientWithHTTPClient creates a new ASC client using the provided HTTP client.
// If httpClient is nil, a default client with ASC timeouts is used.
func NewClientWithHTTPClient(keyID, issuerID, privateKeyPath string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = newDefaultHTTPClient(ResolveTimeout())
	}
	return newClientWithHTTPClient(keyID, issuerID, privateKeyPath, httpClient)
}

// NewClientFromPEM creates a new ASC client from in-memory private key PEM content.
func NewClientFromPEM(keyID, issuerID, privateKeyPEM string) (*Client, error) {
	return newClientFromPEMWithHTTPClient(keyID, issuerID, privateKeyPEM, newDefaultHTTPClient(ResolveTimeout()))
}

// NewClientFromPEMWithTimeout creates a new ASC client from in-memory PEM
// content with an explicit HTTP timeout.
func NewClientFromPEMWithTimeout(keyID, issuerID, privateKeyPEM string, timeout time.Duration) (*Client, error) {
	return newClientFromPEMWithHTTPClient(keyID, issuerID, privateKeyPEM, newDefaultHTTPClient(timeout))
}

func newDefaultHTTPClient(timeout time.Duration) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		// Some tests replace http.DefaultTransport with a custom RoundTripper.
		// Keep that behavior and skip transport tuning in that case.
		return &http.Client{
			Timeout:   timeout,
			Transport: http.DefaultTransport,
		}
	}

	clonedTransport := transport.Clone()
	clonedTransport.MaxIdleConns = defaultMaxIdleConns
	clonedTransport.MaxIdleConnsPerHost = defaultMaxIdleConnsPerHost

	return &http.Client{
		Timeout:   timeout,
		Transport: clonedTransport,
	}
}

func newClientWithHTTPClient(keyID, issuerID, privateKeyPath string, httpClient *http.Client) (*Client, error) {
	if err := auth.ValidateKeyFile(privateKeyPath); err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	key, err := auth.LoadPrivateKey(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	return newClientWithPrivateKey(keyID, issuerID, key, httpClient), nil
}

func newClientFromPEMWithHTTPClient(keyID, issuerID, privateKeyPEM string, httpClient *http.Client) (*Client, error) {
	key, err := auth.LoadPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	return newClientWithPrivateKey(keyID, issuerID, key, httpClient), nil
}

func newClientWithPrivateKey(keyID, issuerID string, privateKey *ecdsa.PrivateKey, httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
		keyID:      keyID,
		issuerID:   issuerID,
		privateKey: privateKey,
	}
}

// UseEnterpriseProgramAPI points the client at the Enterprise Program API, which
// takes its own host and token audience. Call it before the first request.
func (c *Client) UseEnterpriseProgramAPI() {
	c.enterprise = true
	c.httpClient = clientWithoutRedirects(c.httpClient)
}

// IsAPIHost reports whether host serves the App Store Connect or Enterprise
// Program API. A *Client still only sends requests to its own host.
func IsAPIHost(host string) bool {
	return host == ascAPIHost || host == enterpriseAPIHost
}

func (c *Client) apiBaseURL() string {
	if c.enterprise {
		return EnterpriseBaseURL
	}
	return BaseURL
}

func (c *Client) getMutatingRequestLimiter() chan struct{} {
	c.mutatingRequestLimiterOnce.Do(func() {
		if c.mutatingRequestLimiter == nil {
			c.mutatingRequestLimiter = make(chan struct{}, defaultMutatingRequestLimit)
		}
	})
	return c.mutatingRequestLimiter
}

// getBulkMutatingRequestLimiter returns the client-wide write ceiling shared
// by bulk and ordinary mutating requests.
func (c *Client) getBulkMutatingRequestLimiter() chan struct{} {
	c.bulkMutatingRequestLimiterOnce.Do(func() {
		if c.bulkMutatingRequestLimiter == nil {
			c.bulkMutatingRequestLimiter = make(chan struct{}, BulkMutatingRequestLimit)
		}
	})
	return c.bulkMutatingRequestLimiter
}

type bulkMutatingRequestLimitKey struct{}

// WithBulkMutatingRequestLimit marks ctx for a bulk write fan-out. Mutating
// requests sent with the returned context share the client-wide
// BulkMutatingRequestLimit instead of the lower default limit for ordinary
// writes. Retry and backoff behavior is unchanged. Callers must size their
// worker pools to at most BulkMutatingRequestLimit.
func WithBulkMutatingRequestLimit(ctx context.Context) context.Context {
	return context.WithValue(ctx, bulkMutatingRequestLimitKey{}, true)
}

func usesBulkMutatingRequestLimit(ctx context.Context) bool {
	bulk, _ := ctx.Value(bulkMutatingRequestLimitKey{}).(bool)
	return bulk
}
