package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/1Password/srp"
	"golang.org/x/crypto/pbkdf2"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

const (
	// Apple auth endpoints used by the web session flow.
	authServiceURL    = "https://idmsa.apple.com/appleauth/auth"
	appStoreBaseURL   = "https://appstoreconnect.apple.com"
	olympusSessionURL = "https://appstoreconnect.apple.com/olympus/v1/session"
	irisV1BaseURL     = appStoreBaseURL + "/iris/v1"
	irisV2BaseURL     = appStoreBaseURL + "/iris/v2"
	olympusBaseURL    = appStoreBaseURL + "/olympus/v1"

	// Public production widget identifier shipped by Apple's ASC login page,
	// not an account credential. Fallback when the legacy config endpoint is absent.
	// Verified 2026-09-11: https://unpkg.apple.com/@maison/preauthorization-container@0.2.1/umd/chunks/ASC.BRGDgul9.js
	appStoreConnectWidgetKey = "e0b80c3bf78523bfe80974d320935bfa30add02e1bff88ec2166c6bd5a706c42"

	// Apple currently uses RFC5054 group 2048 + 32-byte derived password.
	srpClientSecretBytes  = 256
	srpDerivedPasswordLen = 32

	// Guardrails for unofficial web/iris calls.
	webMinRequestIntervalEnv     = "ASC_WEB_MIN_REQUEST_INTERVAL"
	defaultWebMinRequestInterval = 1 * time.Second
	minimumWebMinRequestInterval = 200 * time.Millisecond
)

var (
	errTwoFactorRequired              = errors.New("two-factor authentication required")
	errInvalidAppleAccountCredentials = errors.New("incorrect Apple Account email or password")
	errAppleAccountActionRequired     = errors.New("complete the pending Apple Account web prompt in a browser (privacy acknowledgement or 2FA upgrade) and try again")

	// ErrInvalidAppleAccountCredentials reports rejected Apple Account
	// credentials during web login flows.
	ErrInvalidAppleAccountCredentials = errInvalidAppleAccountCredentials
)

var webTLSRootBundlePaths = []string{
	"/etc/ssl/cert.pem",
	"/private/etc/ssl/cert.pem",
	"/opt/homebrew/etc/openssl@3/cert.pem",
	"/usr/local/etc/openssl@3/cert.pem",
}

var webDebugLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
	ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
		if attr.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return attr
	},
}))

var webDebugEnabledFn = asc.ResolveDebugEnabled

var webAuthSignedQueryKeys = urlsanitize.CopyKeySet(urlsanitize.DefaultSignedQueryKeys)

var webAuthSensitiveQueryKeys = urlsanitize.MergeKeySets(
	urlsanitize.DefaultSensitiveQueryKeys,
	map[string]struct{}{
		"widgetkey": {},
		"code":      {},
		"scnt":      {},
	},
)

// AuthSession holds authenticated web-session state for internal API calls.
type AuthSession struct {
	Client           *http.Client
	ProviderID       int64
	PublicProviderID string
	ProviderName     string
	TeamID           string
	UserEmail        string
	DeveloperTeamID  string

	// Continuation state needed after a 409 SRP completion response.
	ServiceKey       string
	AppleIDSessionID string
	SCNT             string

	// cachedUpdatedAt is the UpdatedAt stamp of the cached entry this session was
	// loaded from, zero for a freshly logged-in session. It lets a caller that
	// proves the loaded jar unusable delete only that entry, leaving a
	// replacement another process persisted in the meantime intact.
	cachedUpdatedAt  time.Time
	cachedGeneration string

	// Prepared 2FA delivery state so callers can request code delivery before prompting.
	twoFactorMethod        string
	twoFactorPhoneID       int
	twoFactorPhoneMode     string
	twoFactorDestination   string
	twoFactorCodeRequested bool
}

// ProviderSelection identifies the App Store Connect provider/team a web
// session should use. ProviderID is Apple's numeric provider id; PublicProviderID
// is the public team/provider id users usually recognize.
type ProviderSelection struct {
	ProviderID       int64
	PublicProviderID string
}

func (s ProviderSelection) empty() bool {
	return s.ProviderID == 0 && strings.TrimSpace(s.PublicProviderID) == ""
}

// LoginCredentials holds Apple ID credentials.
type LoginCredentials struct {
	Username string
	Password string
}

// TwoFactorRequiredError signals that the caller must submit a 2FA code.
type TwoFactorRequiredError struct {
	AppleIDSessionID string
	SCNT             string
}

func (e *TwoFactorRequiredError) Error() string {
	return errTwoFactorRequired.Error()
}

type TwoFactorChallenge = appleauth.TwoFactorChallenge

const (
	twoFactorMethodTrustedDevice = appleauth.TwoFactorMethodTrustedDevice
	twoFactorMethodPhone         = appleauth.TwoFactorMethodPhone
)

// Client is an internal web API client using a web session cookie jar.
type Client struct {
	httpClient         *http.Client
	baseURL            string
	developerPortalURL string

	developerSessionMu    sync.Mutex
	developerCSRF         string
	developerCSRFTS       string
	developerTeamID       string
	developerTeamSelector string
	publicProviderID      string
	providerName          string
	session               *AuthSession

	// Requests are intentionally throttled to reduce pressure on fragile, unofficial
	// web-session endpoints and avoid bursty behavior against user accounts.
	minRequestInterval time.Duration
	rateLimitMu        sync.Mutex
	nextAllowedAt      time.Time
}

// APIError wraps non-2xx internal web API responses.
//
// The raw body is retained for internal classification and tests, but Error()
// intentionally avoids dumping response bodies that may contain sensitive data.
type APIError struct {
	Status         int
	AppleRequestID string
	CorrelationKey string
	rawBody        []byte
	// portalReason is populated only for review attachment mutations. Those
	// endpoints return the actionable refusal reason in errors[].detail, while
	// the general web API error contract intentionally keeps response details
	// redacted.
	portalReason string
}

type sessionInfoStatusError struct {
	Status int
}

func (e *sessionInfoStatusError) Error() string {
	return fmt.Sprintf("failed to get session info with status %d", e.Status)
}

func (e *sessionInfoStatusError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.Status
}

// TwoFactorFinalizationError reports that Apple accepted the submitted 2FA code
// but the follow-up App Store Connect session bootstrap failed. It is distinct
// from a rejected verification code: the code was already consumed, so callers
// must not describe this as a 2FA verification failure.
type TwoFactorFinalizationError struct {
	Status int
	Err    error
}

func (e *TwoFactorFinalizationError) Error() string {
	if e == nil {
		return "session bootstrap after 2fa failed"
	}
	if e.Status > 0 {
		if e.Err != nil {
			// Name the failing stage: trust and session-info both bootstrap the
			// session after an accepted code, and only the cause tells them apart.
			return fmt.Sprintf("session bootstrap returned status %d: %v", e.Status, e.Err)
		}
		return fmt.Sprintf("session bootstrap returned status %d", e.Status)
	}
	if e.Err != nil {
		return fmt.Sprintf("session bootstrap failed: %v", e.Err)
	}
	return "session bootstrap after 2fa failed"
}

func (e *TwoFactorFinalizationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *TwoFactorFinalizationError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.Status
}

// IsStaleSessionAfterTwoFactor reports whether a 2FA submission failed only
// because the reused cookie jar no longer authenticates against App Store
// Connect. Callers may discard the cached session and retry a fresh login once.
func IsStaleSessionAfterTwoFactor(err error) bool {
	var finalizeErr *TwoFactorFinalizationError
	if !errors.As(err, &finalizeErr) {
		return false
	}
	switch finalizeErr.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return true
	default:
		return false
	}
}

func (e *APIError) Error() string {
	parts := []string{fmt.Sprintf("web api error (status %d)", e.Status)}
	if e.AppleRequestID != "" {
		parts = append(parts, fmt.Sprintf("request_id=%s", e.AppleRequestID))
	}
	if e.CorrelationKey != "" {
		parts = append(parts, fmt.Sprintf("correlation_key=%s", e.CorrelationKey))
	}
	if codes := extractServiceErrorCodes(e.rawBody); len(codes) > 0 {
		parts = append(parts, fmt.Sprintf("codes=%v", codes))
	}
	if reason := strings.TrimSpace(e.portalReason); reason != "" {
		parts = append(parts, fmt.Sprintf("reason=%s", reason))
	}
	return strings.Join(parts, ", ")
}

func (e *APIError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.Status
}

// rawResponseBody exposes the body to package-internal helpers only.
func (e *APIError) rawResponseBody() []byte {
	return e.rawBody
}

func logWebAuthHTTP(stage string, req *http.Request, resp *http.Response, body []byte, err error) {
	if !webDebugEnabledFn() {
		return
	}

	fields := []any{
		"stage", strings.TrimSpace(stage),
	}
	if req != nil {
		fields = append(
			fields,
			"method", req.Method,
			"url", sanitizeWebAuthURLForLog(req.URL.String()),
		)
	}
	if resp != nil {
		fields = append(fields, "status", resp.StatusCode)
		if requestID := extractAppleRequestID(resp.Header); requestID != "" {
			fields = append(fields, "request_id", requestID)
		}
		if correlationKey := strings.TrimSpace(resp.Header.Get("X-Apple-Jingle-Correlation-Key")); correlationKey != "" {
			fields = append(fields, "correlation_key", correlationKey)
		}
		if codes := extractServiceErrorCodes(body); len(codes) > 0 {
			fields = append(fields, "codes", strings.Join(codes, ","))
		}
	}
	if err != nil {
		errorText := err.Error()
		if req != nil && isTransactionTaxFinanceRequestURL(req.URL) {
			errorText = sanitizeTransactionTaxTransportError(err)
		}
		fields = append(fields, "error", errorText)
	}
	webDebugLogger.Info("web auth http", fields...)
}

func sanitizeTransactionTaxTransportError(err error) string {
	if class := urlsanitize.ClassifyTransportFailure(err); class != "" {
		return "transaction tax request failed (" + class + ")"
	}
	return "transaction tax request failed"
}

func extractAppleRequestID(headers http.Header) string {
	if len(headers) == 0 {
		return ""
	}
	return headerValueCaseInsensitive(headers, "X-Apple-Request-UUID")
}

func sanitizeWebAuthURLForLog(rawURL string) string {
	sanitized := urlsanitize.SanitizeURLForLog(rawURL, webAuthSignedQueryKeys, webAuthSensitiveQueryKeys)
	return sanitizeWebAuthFinanceJobPath(sanitized)
}

// sanitizeWebAuthFinanceJobPath removes opaque finance identifiers from debug
// request paths and query values. These identifiers are not useful in a
// diagnostic log; unlike signed query values, they cannot be removed by the
// generic URL sanitizer.
func sanitizeWebAuthFinanceJobPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(segments) < 4 ||
		segments[0] != "WebObjects" ||
		segments[1] != "iTunesConnect.woa" ||
		segments[2] != "ra" ||
		segments[3] != "paymentConsolidation" {
		return rawURL
	}
	for index := 4; index+1 < len(segments); index++ {
		switch segments[index] {
		case "providers", "sapVendorNumbers", "reports":
			if strings.TrimSpace(segments[index+1]) != "" {
				segments[index+1] = "[REDACTED]"
			}
		}
	}
	parsed.Path = "/" + strings.Join(segments, "/")
	parsed.RawPath = ""
	query := parsed.Query()
	for key := range query {
		if strings.EqualFold(key, "regionCurrencyIds") {
			query.Set(key, "[REDACTED]")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

type signinInitResponse struct {
	Iteration  int             `json:"iteration"`
	Salt       string          `json:"salt"`
	Protocol   string          `json:"protocol"`
	ServerPubB string          `json:"b"`
	Challenge  json.RawMessage `json:"c"`
}

type sessionProviderInfo struct {
	ProviderID       int64  `json:"providerId"`
	PublicProviderID string `json:"publicProviderId"`
	Name             string `json:"name"`
}

type sessionInfo struct {
	Provider           sessionProviderInfo   `json:"provider"`
	AvailableProviders []sessionProviderInfo `json:"availableProviders"`
	User               struct {
		EmailAddress string `json:"emailAddress"`
	} `json:"user"`
}

type authOptionsResponse = appleauth.AuthOptionsResponse

type twoFAVerificationFailedError struct {
	Kind   string
	Status int
	Body   []byte
}

func (e *twoFAVerificationFailedError) Error() string {
	codes := extractServiceErrorCodes(e.Body)
	if len(codes) > 0 {
		return fmt.Sprintf("%s 2fa failed (status %d, codes=%v)", e.Kind, e.Status, codes)
	}
	return fmt.Sprintf("%s 2fa failed (status %d)", e.Kind, e.Status)
}

func (e *twoFAVerificationFailedError) HTTPStatusCode() int {
	if e == nil {
		return 0
	}
	return e.Status
}

func newWebHTTPClient(jar http.CookieJar) *http.Client {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{
			Jar:     jar,
			Timeout: asc.ResolveTimeout(),
		}
	}

	cloned := transport.Clone()
	cloned.TLSHandshakeTimeout = 30 * time.Second
	applyDarwinTLSRootFallback(cloned)

	return &http.Client{
		Jar:       jar,
		Timeout:   asc.ResolveTimeout(),
		Transport: cloned,
	}
}

func loadWebRootCAPoolFromPaths(paths []string) *x509.CertPool {
	return loadWebRootCAPoolFromPathsWithBase(nil, paths)
}

func loadWebRootCAPoolFromPathsWithBase(base *x509.CertPool, paths []string) *x509.CertPool {
	if len(paths) == 0 {
		if base == nil {
			return nil
		}
		return base
	}
	pool := base
	if pool == nil {
		pool = x509.NewCertPool()
	}
	appended := false
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		pemData, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if pool.AppendCertsFromPEM(pemData) {
			appended = true
		}
	}
	if !appended && base == nil {
		return nil
	}
	return pool
}

func resolveDarwinTLSRootPool() *x509.CertPool {
	systemPool, err := x509.SystemCertPool()
	if err == nil && systemPool != nil {
		return loadWebRootCAPoolFromPathsWithBase(systemPool.Clone(), webTLSRootBundlePaths)
	}
	return loadWebRootCAPoolFromPaths(webTLSRootBundlePaths)
}

func applyDarwinTLSRootFallback(transport *http.Transport) {
	if transport == nil || runtime.GOOS != "darwin" {
		return
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.RootCAs != nil {
		return
	}
	rootPool := resolveDarwinTLSRootPool()
	if rootPool == nil {
		return
	}
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{RootCAs: rootPool}
		return
	}
	clonedTLS := transport.TLSClientConfig.Clone()
	clonedTLS.RootCAs = rootPool
	transport.TLSClientConfig = clonedTLS
}

func resolveWebMinRequestInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv(webMinRequestIntervalEnv))
	if raw == "" {
		return defaultWebMinRequestInterval
	}
	if millis, err := strconv.Atoi(raw); err == nil {
		interval := time.Duration(millis) * time.Millisecond
		if interval < minimumWebMinRequestInterval {
			return minimumWebMinRequestInterval
		}
		return interval
	}
	interval, err := time.ParseDuration(raw)
	if err != nil {
		return defaultWebMinRequestInterval
	}
	if interval < minimumWebMinRequestInterval {
		return minimumWebMinRequestInterval
	}
	return interval
}

func parseSigninInitResponse(data []byte) (*signinInitResponse, error) {
	var result signinInitResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to decode signin init response: %w", err)
	}
	if len(result.Challenge) == 0 || bytes.Equal(result.Challenge, []byte("null")) {
		return nil, fmt.Errorf("signin init response missing challenge")
	}
	return &result, nil
}

// NewClient creates an internal web API client from an authenticated session.
func NewClient(session *AuthSession) *Client {
	client := &Client{
		httpClient:         session.Client,
		baseURL:            irisV1BaseURL,
		publicProviderID:   strings.TrimSpace(session.PublicProviderID),
		providerName:       strings.TrimSpace(session.ProviderName),
		developerTeamID:    strings.TrimSpace(session.DeveloperTeamID),
		session:            session,
		minRequestInterval: resolveWebMinRequestInterval(),
	}
	return client
}

// Login performs Apple ID SRP authentication and returns a web session.
//
// If 2FA is required, Login returns a non-nil partial session and an error
// wrapping *TwoFactorRequiredError. The caller can continue with SubmitTwoFactorCode.
func Login(ctx context.Context, creds LoginCredentials) (*AuthSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}
	client := newWebHTTPClient(jar)
	return loginWithHTTPClient(ctx, client, creds)
}

// LoginWithClient performs Apple ID SRP authentication reusing an existing HTTP
// client and cookie jar. This is used for best-effort relogin attempts that
// should preserve Apple trust cookies from a cached session.
func LoginWithClient(ctx context.Context, client *http.Client, creds LoginCredentials) (*AuthSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		return nil, fmt.Errorf("client is required")
	}
	if client.Jar == nil {
		return nil, fmt.Errorf("client jar is required")
	}
	return loginWithHTTPClient(ctx, client, creds)
}

func applySessionInfo(session *AuthSession, info *sessionInfo) {
	if session == nil || info == nil {
		return
	}
	session.ProviderID = info.Provider.ProviderID
	session.PublicProviderID = strings.TrimSpace(info.Provider.PublicProviderID)
	session.ProviderName = strings.TrimSpace(info.Provider.Name)
	session.TeamID = fmt.Sprintf("%d", info.Provider.ProviderID)
	session.UserEmail = strings.TrimSpace(info.User.EmailAddress)
}

func loginWithHTTPClient(ctx context.Context, client *http.Client, creds LoginCredentials) (*AuthSession, error) {
	if strings.TrimSpace(creds.Username) == "" {
		return nil, fmt.Errorf("apple id is required")
	}
	if strings.TrimSpace(creds.Password) == "" {
		return nil, fmt.Errorf("password is required")
	}

	serviceKey, err := getAuthServiceKey(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("could not load Apple login configuration; password authentication has not started. Run with --api-debug for request details: %w", err)
	}

	if err := performSRPLogin(ctx, client, creds, serviceKey); err != nil {
		var tfaErr *TwoFactorRequiredError
		if errors.As(err, &tfaErr) {
			partial := &AuthSession{
				Client:           client,
				ServiceKey:       serviceKey,
				AppleIDSessionID: tfaErr.AppleIDSessionID,
				SCNT:             tfaErr.SCNT,
				UserEmail:        strings.TrimSpace(creds.Username),
			}
			return partial, fmt.Errorf("srp login failed: %w", err)
		}
		return nil, fmt.Errorf("srp login failed: %w", err)
	}

	info, err := getSessionInfo(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get session info: %w", err)
	}

	session := &AuthSession{
		Client:     client,
		ServiceKey: serviceKey,
	}
	applySessionInfo(session, info)
	return session, nil
}

func getAuthServiceKey(ctx context.Context, client *http.Client) (string, error) {
	if key, err := getAuthServiceKeyFromSignout(ctx, client); err == nil {
		return key, nil
	} else if ctx.Err() != nil {
		return "", ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://appstoreconnect.apple.com/olympus/v1/app/config?hostname=itunesconnect.apple.com", nil)
	if err != nil {
		return "", fmt.Errorf("failed to build auth service key request: %w", err)
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		logWebAuthHTTP("auth_service_key", req, nil, nil, err)
		return "", fmt.Errorf("failed to fetch auth service key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("auth_service_key", req, resp, nil, err)
		return "", fmt.Errorf("failed to read auth service key response: %w", err)
	}
	logWebAuthHTTP("auth_service_key", req, resp, body, nil)
	if resp.StatusCode == http.StatusNotFound {
		if webDebugEnabledFn() {
			webDebugLogger.Info("using public App Store Connect widget key", "stage", "auth_service_key_fallback")
		}
		return appStoreConnectWidgetKey, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch auth service key (status %d)", resp.StatusCode)
	}

	var payload struct {
		AuthServiceKey string `json:"authServiceKey"`
		ServiceKey     string `json:"serviceKey"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("failed to decode auth service key response: %w", err)
	}
	serviceKey := strings.TrimSpace(payload.AuthServiceKey)
	if serviceKey == "" {
		serviceKey = strings.TrimSpace(payload.ServiceKey)
	}
	if serviceKey == "" {
		return "", fmt.Errorf("auth service key is empty")
	}
	return serviceKey, nil
}

func getAuthServiceKeyFromSignout(ctx context.Context, client *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, appStoreBaseURL+"/logout", nil)
	if err != nil {
		return "", err
	}

	// The Location header carries Apple's current public widget key. Sending
	// session cookies or following that redirect could sign the user out.
	// Keep the caller's transport (including TLS configuration) and timeout.
	discoveryClient := *client
	discoveryClient.Jar = nil
	discoveryClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := discoveryClient.Do(req)
	if err != nil {
		// net/http errors can include a malformed Location containing the key.
		// Discovery is best effort; log only a generic failure before fallback.
		err = errors.New("sign-out service key discovery request failed")
		logWebAuthHTTP("auth_service_key_discovery", req, nil, nil, err)
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	logWebAuthHTTP("auth_service_key_discovery", req, resp, nil, nil)

	switch resp.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
	default:
		return "", fmt.Errorf("sign-out service key discovery returned status %d", resp.StatusCode)
	}
	location, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || location.Scheme != "https" || !strings.EqualFold(location.Host, "idmsa.apple.com") || location.User != nil || location.Path != "/appleauth/signout" {
		return "", errors.New("sign-out service key discovery returned an invalid redirect")
	}
	query, err := url.ParseQuery(location.RawQuery)
	if err != nil {
		return "", errors.New("sign-out service key discovery returned an invalid query")
	}
	keys := query["widgetKey"]
	if len(keys) != 1 || strings.TrimSpace(keys[0]) == "" {
		return "", errors.New("sign-out service key discovery omitted a unique service key")
	}
	return strings.TrimSpace(keys[0]), nil
}

func performSRPLogin(ctx context.Context, client *http.Client, creds LoginCredentials, serviceKey string) error {
	group := srp.KnownGroups[srp.RFC5054Group2048]
	n := group.N()
	g := group.Generator()

	aBytes := make([]byte, srpClientSecretBytes)
	if _, err := rand.Read(aBytes); err != nil {
		return fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	a := new(big.Int).SetBytes(aBytes)
	A := new(big.Int).Exp(g, a, n)
	aBase64 := base64.StdEncoding.EncodeToString(A.Bytes())

	initResp, err := signinInit(ctx, client, strings.TrimSpace(creds.Username), aBase64, serviceKey)
	if err != nil {
		return fmt.Errorf("signin init failed: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(initResp.Salt)
	if err != nil {
		return fmt.Errorf("failed to decode salt: %w", err)
	}

	preparedPassword, err := preparePasswordForProtocol(creds.Password, initResp.Protocol)
	if err != nil {
		return err
	}
	derivedPassword := pbkdf2.Key(preparedPassword, salt, initResp.Iteration, srpDerivedPasswordLen, sha256.New)

	serverB, err := base64.StdEncoding.DecodeString(initResp.ServerPubB)
	if err != nil {
		return fmt.Errorf("failed to decode server public B: %w", err)
	}

	m1, m2, err := calculateSRPProof(strings.TrimSpace(creds.Username), a, A, n, g, serverB, derivedPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to calculate SRP proof: %w", err)
	}

	hashcash, err := getHashcash(ctx, client, serviceKey)
	if err != nil {
		return fmt.Errorf("failed to compute hashcash: %w", err)
	}

	if err := signinComplete(ctx, client, strings.TrimSpace(creds.Username), m1, m2, initResp.Challenge, serviceKey, hashcash); err != nil {
		return fmt.Errorf("signin complete failed: %w", err)
	}

	return nil
}

func preparePasswordForProtocol(password, protocol string) ([]byte, error) {
	passwordDigest := sha256.Sum256([]byte(password))
	switch protocol {
	case "s2k":
		return passwordDigest[:], nil
	case "s2k_fo":
		return []byte(hex.EncodeToString(passwordDigest[:])), nil
	default:
		return nil, fmt.Errorf("unsupported SRP protocol %q", protocol)
	}
}

func calculateSRPProof(username string, a, A, n, g *big.Int, serverB, derivedPassword, salt []byte) (string, string, error) {
	bHex := hex.EncodeToString(serverB)
	saltHex := hex.EncodeToString(salt)
	aHex := numToHex(A)
	derivedPasswordHex := hex.EncodeToString(derivedPassword)

	x, err := calcXHex(derivedPasswordHex, saltHex)
	if err != nil {
		return "", "", err
	}

	k, err := calcK(n, g)
	if err != nil {
		return "", "", err
	}

	u, err := calcU(n, aHex, bHex)
	if err != nil {
		return "", "", err
	}
	if u.Sign() == 0 {
		return "", "", fmt.Errorf("invalid SRP scrambling parameter")
	}

	B := new(big.Int).SetBytes(serverB)

	gx := new(big.Int).Exp(g, x, n)
	kgx := new(big.Int).Mul(k, gx)
	kgx.Mod(kgx, n)
	base := new(big.Int).Sub(B, kgx)
	base.Mod(base, n)
	if base.Sign() < 0 {
		base.Add(base, n)
	}
	exp := new(big.Int).Add(a, new(big.Int).Mul(u, x))
	S := new(big.Int).Exp(base, exp, n)

	kHex, err := shaHex(numToHex(S))
	if err != nil {
		return "", "", err
	}

	m1Hex, err := calcM(n, g, username, saltHex, aHex, bHex, kHex)
	if err != nil {
		return "", "", err
	}

	m2Hex, err := calcHAMK(aHex, m1Hex, kHex)
	if err != nil {
		return "", "", err
	}

	m1Bytes, err := hex.DecodeString(m1Hex)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode m1 proof: %w", err)
	}
	m2Bytes, err := hex.DecodeString(m2Hex)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode m2 proof: %w", err)
	}

	return base64.StdEncoding.EncodeToString(m1Bytes), base64.StdEncoding.EncodeToString(m2Bytes), nil
}

func calcXHex(derivedPasswordHex, saltHex string) (*big.Int, error) {
	if _, err := hex.DecodeString(derivedPasswordHex); err != nil {
		return nil, fmt.Errorf("invalid derived password hex: %w", err)
	}
	if _, err := hex.DecodeString(saltHex); err != nil {
		return nil, fmt.Errorf("invalid salt hex: %w", err)
	}

	inner, err := shaHex("3a" + derivedPasswordHex)
	if err != nil {
		return nil, err
	}
	outer, err := shaHex(saltHex + inner)
	if err != nil {
		return nil, err
	}

	x := new(big.Int)
	if _, ok := x.SetString(outer, 16); !ok {
		return nil, fmt.Errorf("failed to parse x value")
	}
	return x, nil
}

func calcK(n, g *big.Int) (*big.Int, error) {
	return hashWithPadding(n, numToHex(n), numToHex(g))
}

func calcU(n *big.Int, aHex, bHex string) (*big.Int, error) {
	return hashWithPadding(n, aHex, bHex)
}

func calcM(n, g *big.Int, username, saltHex, aHex, bHex, kHex string) (string, error) {
	hn, err := hashWithPadding(n, numToHex(n))
	if err != nil {
		return "", err
	}
	hg, err := hashWithPadding(n, numToHex(g))
	if err != nil {
		return "", err
	}
	hxor := new(big.Int).Xor(hn, hg)

	input := numToHex(hxor) + shaStringHex(username) + saltHex + aHex + bHex + kHex
	raw, err := hex.DecodeString(input)
	if err != nil {
		return "", fmt.Errorf("failed to decode M input: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func calcHAMK(aHex, mHex, kHex string) (string, error) {
	raw, err := hex.DecodeString(aHex + mHex + kHex)
	if err != nil {
		return "", fmt.Errorf("failed to decode H_AMK input: %w", err)
	}
	sum := sha256.Sum256(raw)
	return numToHex(new(big.Int).SetBytes(sum[:])), nil
}

func hashWithPadding(n *big.Int, values ...string) (*big.Int, error) {
	nHexLen := len(fmt.Sprintf("%x", n))
	nLen := 2 * (((nHexLen * 4) + 7) >> 3)

	var input strings.Builder
	for _, value := range values {
		if value == "" {
			continue
		}
		hexValue := strings.ToLower(value)
		if len(hexValue) > nLen {
			return nil, fmt.Errorf("bit width mismatch for value")
		}
		input.WriteString(strings.Repeat("0", nLen-len(hexValue)))
		input.WriteString(hexValue)
	}

	digestHex, err := shaHex(input.String())
	if err != nil {
		return nil, err
	}

	result := new(big.Int)
	if _, ok := result.SetString(digestHex, 16); !ok {
		return nil, fmt.Errorf("failed to parse hash result")
	}
	result.Mod(result, n)
	return result, nil
}

func shaHex(hexValue string) (string, error) {
	raw, err := hex.DecodeString(hexValue)
	if err != nil {
		return "", fmt.Errorf("invalid hex input: %w", err)
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func shaStringHex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func numToHex(number *big.Int) string {
	hexValue := strings.ToLower(number.Text(16))
	if len(hexValue)%2 == 1 {
		hexValue = "0" + hexValue
	}
	return hexValue
}

func getHashcash(ctx context.Context, client *http.Client, serviceKey string) (string, error) {
	endpoint := authServiceURL + "/signin?widgetKey=" + url.QueryEscape(serviceKey)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		logWebAuthHTTP("hashcash", req, nil, nil, err)
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	logWebAuthHTTP("hashcash", req, resp, nil, nil)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch hashcash challenge (status %d)", resp.StatusCode)
	}

	bitsValue := strings.TrimSpace(resp.Header.Get("X-Apple-HC-Bits"))
	challenge := strings.TrimSpace(resp.Header.Get("X-Apple-HC-Challenge"))
	if bitsValue == "" || challenge == "" {
		return "", nil
	}
	bits, err := strconv.Atoi(bitsValue)
	if err != nil {
		return "", fmt.Errorf("invalid hashcash bits %q: %w", bitsValue, err)
	}
	return makeHashcash(bits, challenge, time.Now().UTC()), nil
}

func makeHashcash(bits int, challenge string, now time.Time) string {
	date := now.Format("20060102150405")
	for counter := 0; ; counter++ {
		candidate := fmt.Sprintf("1:%d:%s:%s::%d", bits, date, challenge, counter)
		sum := sha1.Sum([]byte(candidate))
		if hasLeadingZeroBits(sum[:], bits) {
			return candidate
		}
	}
}

func hasLeadingZeroBits(sum []byte, bits int) bool {
	fullBytes := bits / 8
	remainingBits := bits % 8

	for i := 0; i < fullBytes; i++ {
		if sum[i] != 0 {
			return false
		}
	}
	if remainingBits == 0 {
		return true
	}
	mask := byte(0xFF << (8 - remainingBits))
	return (sum[fullBytes] & mask) == 0
}

// setModifiedCookieHeader mirrors fastlane's workaround where DES cookies
// require explicit quotes for some Apple auth endpoints.
func setModifiedCookieHeader(client *http.Client, req *http.Request) {
	if client == nil || client.Jar == nil || req == nil || req.URL == nil {
		return
	}
	cookies := client.Jar.Cookies(req.URL)
	if len(cookies) == 0 {
		return
	}

	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c == nil {
			continue
		}
		value := c.Value
		if strings.Contains(c.Name, "DES") && !strings.HasPrefix(value, "\"") {
			value = "\"" + value + "\""
		}
		parts = append(parts, c.Name+"="+value)
	}
	if len(parts) > 0 {
		req.Header.Set("Cookie", strings.Join(parts, "; "))
	}
}

func signinInit(ctx context.Context, client *http.Client, username, aBase64, serviceKey string) (*signinInitResponse, error) {
	reqBody := map[string]any{
		"accountName": username,
		"protocols":   []string{"s2k", "s2k_fo"},
		"a":           aBase64,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signin init payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authServiceURL+"/signin/init", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Apple-Widget-Key", serviceKey)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript")
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		logWebAuthHTTP("signin_init", req, nil, nil, err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("signin_init", req, resp, nil, err)
		return nil, fmt.Errorf("failed to read signin init response: %w", err)
	}
	logWebAuthHTTP("signin_init", req, resp, respBody, nil)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("signin init failed with status %d", resp.StatusCode)
	}
	return parseSigninInitResponse(respBody)
}

func signinComplete(ctx context.Context, client *http.Client, username, m1, m2 string, challenge json.RawMessage, serviceKey, hashcash string) error {
	reqBody := struct {
		AccountName string          `json:"accountName"`
		RememberMe  bool            `json:"rememberMe"`
		M1          string          `json:"m1"`
		M2          string          `json:"m2"`
		C           json.RawMessage `json:"c"`
	}{
		AccountName: username,
		RememberMe:  false,
		M1:          m1,
		M2:          m2,
		C:           challenge,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal signin complete payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", authServiceURL+"/signin/complete?isRememberMeEnabled=false", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Apple-Widget-Key", serviceKey)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json, text/javascript")
	if strings.TrimSpace(hashcash) != "" {
		req.Header.Set("X-Apple-HC", hashcash)
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		logWebAuthHTTP("signin_complete", req, nil, nil, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("signin_complete", req, resp, nil, err)
		return fmt.Errorf("failed to read signin complete response: %w", err)
	}
	logWebAuthHTTP("signin_complete", req, resp, respBody, nil)

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return &TwoFactorRequiredError{
			AppleIDSessionID: strings.TrimSpace(resp.Header.Get("X-Apple-ID-Session-Id")),
			SCNT:             strings.TrimSpace(resp.Header.Get("scnt")),
		}
	}
	if isAppleAccountActionRequiredSigninComplete(resp.StatusCode, respBody) {
		return errAppleAccountActionRequired
	}
	if isInvalidAppleAccountCredentialsSigninComplete(resp.StatusCode, respBody) {
		return errInvalidAppleAccountCredentials
	}
	return fmt.Errorf("signin complete failed with status %d", resp.StatusCode)
}

func getSessionInfo(ctx context.Context, client *http.Client) (*sessionInfo, error) {
	return getSessionInfoAt(ctx, client, olympusSessionURL)
}

func getSessionInfoAt(ctx context.Context, client *http.Client, endpoint string) (*sessionInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		logWebAuthHTTP("session_info", req, nil, nil, err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("session_info", req, resp, nil, err)
		return nil, fmt.Errorf("failed to read session info response: %w", err)
	}
	logWebAuthHTTP("session_info", req, resp, respBody, nil)
	if resp.StatusCode != http.StatusOK {
		return nil, &sessionInfoStatusError{Status: resp.StatusCode}
	}

	var result sessionInfo
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode session info: %w", err)
	}
	return &result, nil
}

func providerSelectionDescription(selection ProviderSelection) string {
	parts := make([]string, 0, 2)
	if selection.ProviderID != 0 {
		parts = append(parts, fmt.Sprintf("provider-id=%d", selection.ProviderID))
	}
	if publicID := strings.TrimSpace(selection.PublicProviderID); publicID != "" {
		parts = append(parts, fmt.Sprintf("public-provider-id=%s", publicID))
	}
	return strings.Join(parts, " ")
}

func providerSummary(provider sessionProviderInfo) string {
	name := strings.TrimSpace(provider.Name)
	publicID := strings.TrimSpace(provider.PublicProviderID)
	parts := []string{fmt.Sprintf("%d", provider.ProviderID)}
	if publicID != "" {
		parts = append(parts, publicID)
	}
	if name != "" {
		parts = append(parts, name)
	}
	return strings.Join(parts, " / ")
}

func availableProviderSummaries(info *sessionInfo) string {
	if info == nil {
		return ""
	}
	providers := info.AvailableProviders
	if len(providers) == 0 && info.Provider.ProviderID != 0 {
		providers = []sessionProviderInfo{info.Provider}
	}
	summaries := make([]string, 0, len(providers))
	for _, provider := range providers {
		summaries = append(summaries, providerSummary(provider))
	}
	return strings.Join(summaries, ", ")
}

func resolveProviderSelection(info *sessionInfo, selection ProviderSelection) (sessionProviderInfo, error) {
	if info == nil {
		return sessionProviderInfo{}, fmt.Errorf("session info is required")
	}
	publicID := strings.TrimSpace(selection.PublicProviderID)
	providers := info.AvailableProviders
	if len(providers) == 0 && info.Provider.ProviderID != 0 {
		providers = []sessionProviderInfo{info.Provider}
	}

	var matched *sessionProviderInfo
	for i := range providers {
		provider := providers[i]
		idMatches := selection.ProviderID != 0 && provider.ProviderID == selection.ProviderID
		publicMatches := publicID != "" && strings.EqualFold(strings.TrimSpace(provider.PublicProviderID), publicID)
		if selection.ProviderID != 0 && publicID != "" {
			switch {
			case idMatches && publicMatches:
				matched = &provider
			case idMatches:
				return sessionProviderInfo{}, fmt.Errorf("provider selection mismatch: provider-id %d is %q, not %q", selection.ProviderID, strings.TrimSpace(provider.PublicProviderID), publicID)
			case publicMatches:
				return sessionProviderInfo{}, fmt.Errorf("provider selection mismatch: public-provider-id %q is provider-id %d, not %d", publicID, provider.ProviderID, selection.ProviderID)
			}
			continue
		}
		switch {
		case idMatches && publicID != "" && !publicMatches:
			return sessionProviderInfo{}, fmt.Errorf("provider selection mismatch: provider-id %d is %q, not %q", selection.ProviderID, strings.TrimSpace(provider.PublicProviderID), publicID)
		case idMatches || publicMatches:
			matched = &provider
		}
	}
	if matched == nil {
		available := availableProviderSummaries(info)
		if available == "" {
			return sessionProviderInfo{}, fmt.Errorf("provider selection %s did not match any available providers", providerSelectionDescription(selection))
		}
		return sessionProviderInfo{}, fmt.Errorf("provider selection %s did not match any available providers (available: %s)", providerSelectionDescription(selection), available)
	}
	return *matched, nil
}

// SelectProvider switches an authenticated web session to a specific App Store
// Connect provider/team using Apple's private olympus session endpoint.
func SelectProvider(ctx context.Context, session *AuthSession, selection ProviderSelection) error {
	if selection.empty() {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if session == nil || session.Client == nil {
		return fmt.Errorf("web session is required")
	}

	info, err := getSessionInfo(ctx, session.Client)
	if err != nil {
		return fmt.Errorf("failed to get session info: %w", err)
	}
	selected, err := resolveProviderSelection(info, selection)
	if err != nil {
		return err
	}
	if info.Provider.ProviderID == selected.ProviderID {
		applySessionInfo(session, info)
		return nil
	}

	payload, err := json.Marshal(map[string]any{
		"provider": map[string]int64{
			"providerId": selected.ProviderID,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build provider selection payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, olympusSessionURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "olympus-ui")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		logWebAuthHTTP("select_provider", req, nil, nil, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("select_provider", req, resp, nil, err)
		return fmt.Errorf("failed to read provider selection response: %w", err)
	}
	logWebAuthHTTP("select_provider", req, resp, body, nil)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to select provider %s with status %d", providerSummary(selected), resp.StatusCode)
	}

	refreshed, err := getSessionInfo(ctx, session.Client)
	if err != nil {
		return fmt.Errorf("failed to refresh session info after provider selection: %w", err)
	}
	if refreshed.Provider.ProviderID != selected.ProviderID {
		return fmt.Errorf("provider selection did not take effect: selected %d but session reports %d", selected.ProviderID, refreshed.Provider.ProviderID)
	}
	applySessionInfo(session, refreshed)
	return nil
}

func isSessionInfoAuthExpired(err error) bool {
	var statusErr *sessionInfoStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.Status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return true
	default:
		return false
	}
}

func appleSessionHeaders(session *AuthSession) http.Header {
	header := make(http.Header)
	header.Set("X-Apple-ID-Session-Id", session.AppleIDSessionID)
	header.Set("X-Apple-Widget-Key", session.ServiceKey)
	header.Set("scnt", session.SCNT)
	return header
}

func getAuthOptions(ctx context.Context, session *AuthSession) (*authOptionsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", authServiceURL, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range appleSessionHeaders(session) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		logWebAuthHTTP("auth_options", req, nil, nil, err)
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("auth_options", req, resp, nil, err)
		return nil, fmt.Errorf("failed to read auth options response: %w", err)
	}
	logWebAuthHTTP("auth_options", req, resp, body, nil)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("auth options failed with status %d", resp.StatusCode)
	}

	var result authOptionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse auth options: %w", err)
	}
	return &result, nil
}

func requestPhoneCode(ctx context.Context, session *AuthSession, phoneID int, mode string) error {
	payload := map[string]any{
		"phoneNumber": map[string]int{"id": phoneID},
		"mode":        mode,
	}
	status, respBody, err := appleauth.DoTwoFactorJSONRequest(
		ctx,
		session.Client,
		appleSessionHeaders(session),
		"request_phone_2fa",
		http.MethodPut,
		authServiceURL+"/verify/phone",
		payload,
		json.Marshal,
		func(req *http.Request) {
			setModifiedCookieHeader(session.Client, req)
		},
		logWebAuthHTTP,
	)
	if err != nil {
		var marshalErr *appleauth.MarshalPayloadError
		if errors.As(err, &marshalErr) {
			return fmt.Errorf("failed to marshal phone request payload: %w", err)
		}
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "phone-request", Status: status, Body: respBody}
}

func PrepareTwoFactorChallenge(ctx context.Context, session *AuthSession) (*TwoFactorChallenge, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if session == nil || session.Client == nil {
		return nil, fmt.Errorf("session is required")
	}
	if session.ServiceKey == "" || session.AppleIDSessionID == "" || session.SCNT == "" {
		return nil, fmt.Errorf("session is missing 2fa continuation state")
	}
	challenge, err := appleauth.PrepareTwoFactorChallenge(ctx, session, func(ctx context.Context) (*appleauth.AuthOptions, error) {
		opts, err := getAuthOptions(ctx, session)
		if err != nil {
			return nil, err
		}
		return opts.AuthOptions(), nil
	})
	return challenge, wrapWebTwoFactorFlowError(err)
}

func EnsureTwoFactorCodeRequested(ctx context.Context, session *AuthSession) (*TwoFactorChallenge, error) {
	challenge, err := appleauth.EnsureTwoFactorCodeRequested(
		ctx,
		session,
		func(ctx context.Context) (*appleauth.AuthOptions, error) {
			opts, err := getAuthOptions(ctx, session)
			if err != nil {
				return nil, err
			}
			return opts.AuthOptions(), nil
		},
		func(ctx context.Context, phoneID int, mode string) error {
			return requestPhoneCode(ctx, session, phoneID, mode)
		},
	)
	return challenge, wrapWebTwoFactorFlowError(err)
}

func submitTrustedDeviceCode(ctx context.Context, session *AuthSession, code string) error {
	payload := map[string]any{
		"securityCode": map[string]string{"code": code},
	}
	status, respBody, err := appleauth.DoTwoFactorJSONRequest(
		ctx,
		session.Client,
		appleSessionHeaders(session),
		"trusted_device_2fa",
		http.MethodPost,
		authServiceURL+"/verify/trusteddevice/securitycode",
		payload,
		json.Marshal,
		func(req *http.Request) {
			setModifiedCookieHeader(session.Client, req)
		},
		logWebAuthHTTP,
	)
	if err != nil {
		var marshalErr *appleauth.MarshalPayloadError
		if errors.As(err, &marshalErr) {
			return fmt.Errorf("failed to marshal trusted-device payload: %w", err)
		}
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "trusted-device", Status: status, Body: respBody}
}

func submitPhoneCode(ctx context.Context, session *AuthSession, code string, phoneID int, mode string) error {
	payload := map[string]any{
		"securityCode": map[string]string{"code": code},
		"phoneNumber":  map[string]int{"id": phoneID},
		"mode":         mode,
	}
	status, respBody, err := appleauth.DoTwoFactorJSONRequest(
		ctx,
		session.Client,
		appleSessionHeaders(session),
		"phone_2fa",
		http.MethodPost,
		authServiceURL+"/verify/phone/securitycode",
		payload,
		json.Marshal,
		func(req *http.Request) {
			setModifiedCookieHeader(session.Client, req)
		},
		logWebAuthHTTP,
	)
	if err != nil {
		var marshalErr *appleauth.MarshalPayloadError
		if errors.As(err, &marshalErr) {
			return fmt.Errorf("failed to marshal phone payload: %w", err)
		}
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return &twoFAVerificationFailedError{Kind: "phone", Status: status, Body: respBody}
}

func finalizeTwoFactor(ctx context.Context, session *AuthSession) error {
	req, err := http.NewRequestWithContext(ctx, "GET", authServiceURL+"/2sv/trust", nil)
	if err != nil {
		return err
	}
	for key, values := range appleSessionHeaders(session) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("Accept", "application/json")
	setModifiedCookieHeader(session.Client, req)

	resp, err := session.Client.Do(req)
	if err != nil {
		logWebAuthHTTP("finalize_2fa_trust", req, nil, nil, err)
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("finalize_2fa_trust", req, resp, nil, err)
		return fmt.Errorf("failed to read 2fa trust response: %w", err)
	}
	logWebAuthHTTP("finalize_2fa_trust", req, resp, body, nil)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Apple consumed the submitted code before this request ran, so a
		// failure here is a session bootstrap failure, not a verification
		// failure. Carry the HTTP status the same way the session-info path
		// does so a stale reused cookie jar is recognized and retried fresh
		// instead of being reported as a wrong code.
		return &TwoFactorFinalizationError{
			Status: resp.StatusCode,
			Err:    fmt.Errorf("2fa trust failed with status %d", resp.StatusCode),
		}
	}
	_ = body

	info, err := getSessionInfo(ctx, session.Client)
	if err != nil {
		// The 2FA code was already accepted, so this is a session bootstrap
		// failure, not a verification failure. Carry the HTTP status so callers
		// can recognize a stale cookie jar and retry fresh.
		var statusErr *sessionInfoStatusError
		if errors.As(err, &statusErr) {
			return &TwoFactorFinalizationError{Status: statusErr.Status, Err: err}
		}
		return &TwoFactorFinalizationError{Err: err}
	}
	applySessionInfo(session, info)
	return nil
}

// SubmitTwoFactorCode completes a pending 2FA challenge for an existing session.
func SubmitTwoFactorCode(ctx context.Context, session *AuthSession, code string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if session == nil || session.Client == nil {
		return fmt.Errorf("session is required")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("2fa code is required")
	}
	if session.ServiceKey == "" || session.AppleIDSessionID == "" || session.SCNT == "" {
		return fmt.Errorf("session is missing 2fa continuation state")
	}
	err := appleauth.SubmitTwoFactorCode(
		ctx,
		session,
		code,
		func(ctx context.Context) (*appleauth.AuthOptions, error) {
			opts, err := getAuthOptions(ctx, session)
			if err != nil {
				return nil, err
			}
			return opts.AuthOptions(), nil
		},
		func(ctx context.Context, phoneID int, mode string) error {
			return requestPhoneCode(ctx, session, phoneID, mode)
		},
		func(ctx context.Context, code string) error {
			return submitTrustedDeviceCode(ctx, session, code)
		},
		func(ctx context.Context, code string, phoneID int, mode string) error {
			return submitPhoneCode(ctx, session, code, phoneID, mode)
		},
		func(ctx context.Context) error {
			return finalizeTwoFactor(ctx, session)
		},
	)
	return wrapWebTwoFactorFlowError(err)
}

func extractServiceErrorCodes(respBody []byte) []string {
	if codes := appleauth.ExtractServiceErrorCodes(respBody); len(codes) > 0 {
		return codes
	}
	var payload struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return nil
	}
	codes := make([]string, 0, len(payload.Errors))
	for _, responseError := range payload.Errors {
		if code := strings.TrimSpace(responseError.Code); code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func isReviewAttachmentMutation(method, path string) bool {
	if !strings.EqualFold(strings.TrimSpace(method), http.MethodPost) {
		return false
	}
	switch strings.TrimSpace(path) {
	case "/subscriptionSubmissions", "/inAppPurchaseSubmissions":
		return true
	default:
		return false
	}
}

// extractWebPortalErrorReason keeps the actionable refusal text from the
// review attachment endpoints without exposing a raw response body. The
// general web API error contract deliberately omits details because other
// private endpoints can echo sensitive values.
func extractWebPortalErrorReason(respBody []byte) string {
	var payload struct {
		Errors []struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return ""
	}

	reasons := make([]string, 0, len(payload.Errors))
	for _, responseError := range payload.Errors {
		title := sanitizeWebPortalErrorText(responseError.Title)
		detail := sanitizeWebPortalErrorText(responseError.Detail)
		switch {
		case title != "" && detail != "":
			reasons = append(reasons, title+": "+detail)
		case title != "":
			reasons = append(reasons, title)
		case detail != "":
			reasons = append(reasons, detail)
		}
	}
	return sanitizeWebPortalErrorText(strings.Join(reasons, "; "))
}

func sanitizeWebPortalErrorText(value string) string {
	value = strings.TrimSpace(asc.SanitizeTerminalText(value))
	const maxLength = 500
	valueRunes := []rune(value)
	if len(valueRunes) > maxLength {
		return string(valueRunes[:maxLength]) + "..."
	}
	return value
}

// Apple currently returns -20101 when signin/complete rejects SRP credentials.
func isInvalidAppleAccountCredentialsSigninComplete(status int, respBody []byte) bool {
	if status != http.StatusUnauthorized {
		return false
	}
	for _, code := range extractServiceErrorCodes(respBody) {
		if code == "-20101" {
			return true
		}
	}
	return false
}

func isAppleAccountActionRequiredSigninComplete(status int, respBody []byte) bool {
	return appleauth.IsAppleAccountActionRequiredSigninComplete(status, respBody)
}

func (c *Client) waitForRateLimit(ctx context.Context) error {
	if c == nil || c.minRequestInterval <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		c.rateLimitMu.Lock()
		now := time.Now()
		if c.nextAllowedAt.IsZero() || !now.Before(c.nextAllowedAt) {
			c.nextAllowedAt = now.Add(c.minRequestInterval)
			c.rateLimitMu.Unlock()
			return nil
		}
		wait := time.Until(c.nextAllowedAt)
		c.rateLimitMu.Unlock()

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func cloneHeaders(headers http.Header) http.Header {
	if len(headers) == 0 {
		return make(http.Header)
	}
	cloned := make(http.Header, len(headers))
	for key, values := range headers {
		copied := make([]string, len(values))
		copy(copied, values)
		cloned[key] = copied
	}
	return cloned
}

func (c *Client) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Accept", "application/json")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	headers.Set("Origin", appStoreBaseURL)
	headers.Set("Referer", appStoreBaseURL+"/")
	return c.doRequestBase(ctx, c.baseURL, method, path, body, headers)
}

func (c *Client) doRequestBase(ctx context.Context, baseURL, method, path string, body any, headers http.Header) ([]byte, error) {
	return c.doRequestBaseWithHTTPClient(c.httpClient, ctx, baseURL, method, path, body, headers)
}

func (c *Client) doRequestBaseWithHTTPClient(client *http.Client, ctx context.Context, baseURL, method, path string, body any, headers http.Header) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	fullURL := strings.TrimSpace(path)
	if !strings.HasPrefix(fullURL, "https://") && !strings.HasPrefix(fullURL, "http://") {
		fullURL = strings.TrimRight(baseURL, "/") + path
	}
	if err := readonly.Check(ctx, method, readonly.Target(fullURL)); err != nil {
		return nil, err
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header = cloneHeaders(headers)
	if strings.TrimSpace(req.Header.Get("Content-Type")) == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(req.Header.Get("Accept")) == "" {
		req.Header.Set("Accept", "application/json")
	}
	setModifiedCookieHeader(client, req)

	resp, err := client.Do(req)
	if err != nil {
		logWebAuthHTTP("iris_request", req, nil, nil, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logWebAuthHTTP("iris_request", req, resp, nil, err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	logWebAuthHTTP("iris_request", req, resp, respBody, nil)

	appleRequestID := extractAppleRequestID(resp.Header)
	correlationKey := strings.TrimSpace(resp.Header.Get("X-Apple-Jingle-Correlation-Key"))

	if resp.StatusCode >= 400 {
		apiErr := &APIError{
			Status:         resp.StatusCode,
			AppleRequestID: appleRequestID,
			CorrelationKey: correlationKey,
			rawBody:        respBody,
		}
		if isReviewAttachmentMutation(method, path) {
			apiErr.portalReason = extractWebPortalErrorReason(respBody)
		}
		return nil, apiErr
	}
	return respBody, nil
}
