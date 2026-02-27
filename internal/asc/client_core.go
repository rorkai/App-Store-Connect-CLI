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
	// BaseURL is the App Store Connect API base URL
	BaseURL = "https://api.appstoreconnect.apple.com"
	// DefaultTimeout is the default request timeout
	DefaultTimeout = 30 * time.Second
	// DefaultUploadTimeout is the default timeout for upload operations.
	DefaultUploadTimeout = 300 * time.Second
	// tokenLifetime is the JWT token lifetime for App Store Connect API authentication.
	// 10 minutes is a good balance between security (shorter-lived tokens) and usability.
	tokenLifetime = 10 * time.Minute
	// jwtRefreshSkew refreshes a token a bit early to avoid edge-of-expiry races.
	jwtRefreshSkew = 30 * time.Second

	// Retry defaults
	DefaultMaxRetries = 3
	DefaultBaseDelay  = 1 * time.Second
	DefaultMaxDelay   = 30 * time.Second

	defaultMaxIdleConns        = 128
	defaultMaxIdleConnsPerHost = 32
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
	if err != nil {
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
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
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

// RetryOptions configures retry behavior.
//   - MaxRetries: Number of retry attempts. 0 = no retries (fail fast),
//     negative = use DefaultMaxRetries.
//   - BaseDelay: Initial delay between retries (with exponential backoff).
//   - MaxDelay: Maximum delay cap for backoff.
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
		if !IsRetryable(err) {
			return zero, err
		}

		// Check if we've exceeded max retries
		if retryCount >= opts.MaxRetries {
			return zero, fmt.Errorf("retry limit exceeded after %d retries: %w", retryCount+1, err)
		}

		// Calculate delay
		delay := GetRetryAfter(err)
		if delay == 0 {
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
		}

		if ResolveRetryLogEnabled() {
			logRetry(delay, retryCount+1, opts.MaxRetries, err)
		}

		if debugEnabled {
			debugLogger.Info("⟳ Retrying request",
				"attempt", retryCount+1,
				"max_retries", opts.MaxRetries,
				"delay", delay.String(),
				"error", err.Error(),
			)
		}

		retryCount++

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry
		}
	}
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
	cfg := loadConfig()
	var uploadTimeout config.DurationValue
	var uploadTimeoutSeconds config.DurationValue
	if cfg != nil {
		uploadTimeout = cfg.UploadTimeout
		uploadTimeoutSeconds = cfg.UploadTimeoutSeconds
	}
	return resolveTimeoutWithDefaultAndEnv(DefaultUploadTimeout, "ASC_UPLOAD_TIMEOUT", "ASC_UPLOAD_TIMEOUT_SECONDS", uploadTimeout, uploadTimeoutSeconds)
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
	httpClient    *http.Client
	keyID         string
	issuerID      string
	privateKey    *ecdsa.PrivateKey
	notaryBaseURL string // override for testing; empty uses NotaryBaseURL constant

	jwtMu              sync.Mutex
	cachedJWT          string
	cachedJWTExpiresAt time.Time
}

// NewClient creates a new ASC client.
func NewClient(keyID, issuerID, privateKeyPath string) (*Client, error) {
	return newClientWithHTTPClient(keyID, issuerID, privateKeyPath, newDefaultHTTPClient(ResolveTimeout()))
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
