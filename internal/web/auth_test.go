package web

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestLogWebAuthHTTPRedactsSensitiveQueryValues(t *testing.T) {
	origLogger := webDebugLogger
	origDebugEnabled := webDebugEnabledFn
	t.Cleanup(func() {
		webDebugLogger = origLogger
		webDebugEnabledFn = origDebugEnabled
	})

	var logs bytes.Buffer
	webDebugLogger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))
	webDebugEnabledFn = func() bool { return true }

	req, err := http.NewRequest(http.MethodPost, "https://idmsa.apple.com/appleauth/auth/signin?widgetKey=super-secret-widget-key&flow=login", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header: http.Header{
			"X-Apple-Request-Uuid":           []string{"req-123"},
			"X-Apple-Jingle-Correlation-Key": []string{"corr-456"},
		},
	}

	logWebAuthHTTP("signin_complete", req, resp, []byte(`{"serviceErrors":[{"code":"AUTH-401"}]}`), nil)

	output := logs.String()
	if !strings.Contains(output, "stage=signin_complete") {
		t.Fatalf("expected stage in debug output, got %q", output)
	}
	if !strings.Contains(output, "status=401") {
		t.Fatalf("expected status in debug output, got %q", output)
	}
	if !strings.Contains(output, "request_id=req-123") {
		t.Fatalf("expected request id in debug output, got %q", output)
	}
	if !strings.Contains(output, "correlation_key=corr-456") {
		t.Fatalf("expected correlation key in debug output, got %q", output)
	}
	if !strings.Contains(output, "codes=AUTH-401") {
		t.Fatalf("expected service error code in debug output, got %q", output)
	}
	if strings.Contains(output, "super-secret-widget-key") {
		t.Fatalf("expected sensitive widget key to be redacted, got %q", output)
	}
	if !strings.Contains(output, "%5BREDACTED%5D") {
		t.Fatalf("expected redacted marker in debug output, got %q", output)
	}
}

func TestLogWebAuthHTTPNoopWhenDebugDisabled(t *testing.T) {
	origLogger := webDebugLogger
	origDebugEnabled := webDebugEnabledFn
	t.Cleanup(func() {
		webDebugLogger = origLogger
		webDebugEnabledFn = origDebugEnabled
	})

	var logs bytes.Buffer
	webDebugLogger = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))
	webDebugEnabledFn = func() bool { return false }

	req, err := http.NewRequest(http.MethodGet, "https://idmsa.apple.com/appleauth/auth", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	logWebAuthHTTP("session_info", req, &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)}, nil, nil)

	if logs.Len() != 0 {
		t.Fatalf("expected no debug output when disabled, got %q", logs.String())
	}
}

func TestSigninCompleteReturnsInvalidCredentialsErrorForRejectedCredentials(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST request, got %s", req.Method)
			}
			if got, want := req.URL.String(), authServiceURL+"/signin/complete?isRememberMeEnabled=false"; got != want {
				t.Fatalf("expected request URL %q, got %q", want, got)
			}
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"serviceErrors":[{"code":"-20101"}]}`)),
			}, nil
		}),
	}

	err := signinComplete(
		context.Background(),
		client,
		"user@example.com",
		"m1-proof",
		"m2-proof",
		json.RawMessage(`{"v":1}`),
		"service-key",
		"hashcash-token",
	)
	if !errors.Is(err, errInvalidAppleAccountCredentials) {
		t.Fatalf("expected invalid-credentials error, got %v", err)
	}
	if got, want := err.Error(), errInvalidAppleAccountCredentials.Error(); got != want {
		t.Fatalf("expected error %q, got %q", want, got)
	}
}

func TestSigninCompleteKeepsGenericUnauthorizedErrorsForOtherCodes(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"serviceErrors":[{"code":"-99999"}]}`)),
			}, nil
		}),
	}

	err := signinComplete(
		context.Background(),
		client,
		"user@example.com",
		"m1-proof",
		"m2-proof",
		json.RawMessage(`{"v":1}`),
		"service-key",
		"hashcash-token",
	)
	if err == nil {
		t.Fatal("expected unauthorized error")
	}
	if errors.Is(err, errInvalidAppleAccountCredentials) {
		t.Fatalf("expected generic unauthorized error, got %v", err)
	}
	if !strings.Contains(err.Error(), "signin complete failed with status 401") {
		t.Fatalf("expected generic status error, got %q", err.Error())
	}
}

func TestPreparePasswordForProtocol(t *testing.T) {
	t.Run("s2k", func(t *testing.T) {
		prepared, err := preparePasswordForProtocol("example", "s2k")
		if err != nil {
			t.Fatalf("preparePasswordForProtocol returned error: %v", err)
		}
		if len(prepared) != 32 {
			t.Fatalf("expected 32-byte digest for s2k, got %d", len(prepared))
		}
	})

	t.Run("s2k_fo", func(t *testing.T) {
		prepared, err := preparePasswordForProtocol("example", "s2k_fo")
		if err != nil {
			t.Fatalf("preparePasswordForProtocol returned error: %v", err)
		}
		if len(prepared) != 64 {
			t.Fatalf("expected 64-byte hex digest for s2k_fo, got %d", len(prepared))
		}
	})

	t.Run("unsupported protocol", func(t *testing.T) {
		if _, err := preparePasswordForProtocol("example", "unknown"); err == nil {
			t.Fatal("expected error for unsupported protocol")
		}
	})
}

func TestMakeHashcash(t *testing.T) {
	now := time.Date(2026, 2, 24, 18, 0, 0, 0, time.UTC)
	hashcash := makeHashcash(10, "4d74fb15eb23f465f1f6fcbf534e5877", now)
	parts := strings.Split(hashcash, ":")
	if len(parts) != 6 {
		t.Fatalf("expected 6 hashcash fields, got %d (%q)", len(parts), hashcash)
	}
	if parts[0] != "1" {
		t.Fatalf("unexpected hashcash version: %q", parts[0])
	}
	if parts[1] != "10" {
		t.Fatalf("unexpected bits field: %q", parts[1])
	}
	if parts[2] != "20260224180000" {
		t.Fatalf("unexpected date field: %q", parts[2])
	}
	if parts[3] != "4d74fb15eb23f465f1f6fcbf534e5877" {
		t.Fatalf("unexpected challenge field: %q", parts[3])
	}
	sum := sha1.Sum([]byte(hashcash))
	if !hasLeadingZeroBits(sum[:], 10) {
		t.Fatalf("hashcash does not satisfy required leading-zero bits: %q", hashcash)
	}
}

func TestParseSigninInitResponseChallengeObject(t *testing.T) {
	input := []byte(`{
		"iteration": 21000,
		"salt": "c2FsdA==",
		"protocol": "s2k_fo",
		"b": "AQIDBA==",
		"c": {"v":1,"n":"test","u":"user@example.com"}
	}`)

	parsed, err := parseSigninInitResponse(input)
	if err != nil {
		t.Fatalf("parseSigninInitResponse error: %v", err)
	}
	if len(parsed.Challenge) == 0 {
		t.Fatal("expected non-empty challenge")
	}

	var challenge map[string]any
	if err := json.Unmarshal(parsed.Challenge, &challenge); err != nil {
		t.Fatalf("expected challenge to be JSON object, got decode error: %v", err)
	}
	if challenge["n"] != "test" {
		t.Fatalf("expected challenge.n=test, got %#v", challenge["n"])
	}
}

func TestParseSigninInitResponseMissingChallenge(t *testing.T) {
	input := []byte(`{
		"iteration": 21000,
		"salt": "c2FsdA==",
		"protocol": "s2k_fo",
		"b": "AQIDBA=="
	}`)
	if _, err := parseSigninInitResponse(input); err == nil {
		t.Fatal("expected missing challenge error")
	}
}

func TestClientDoRequestHonorsCanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    server.URL,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.doRequest(ctx, "GET", "/apps", nil)
	if err == nil {
		t.Fatal("expected canceled-context error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "context canceled") {
		t.Fatalf("expected context canceled in error, got %v", err)
	}
}

func TestClientDoRequestBaseUsesProvidedBaseAndHeaders(t *testing.T) {
	var (
		gotPath    string
		gotAccept  string
		gotReferer string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path + "?" + r.URL.RawQuery
		gotAccept = r.Header.Get("Accept")
		gotReferer = r.Header.Get("Referer")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient: server.Client(),
		baseURL:    "https://unused.example.test",
	}
	headers := make(http.Header)
	headers.Set("Accept", "application/vnd.api+json")
	headers.Set("Referer", "https://example.test/access/integrations/api")

	if _, err := client.doRequestBase(context.Background(), server.URL+"/iris/v2", http.MethodGet, "/apiKeys?limit=1", nil, headers); err != nil {
		t.Fatalf("doRequestBase() error: %v", err)
	}

	if gotPath != "/iris/v2/apiKeys?limit=1" {
		t.Fatalf("expected request path %q, got %q", "/iris/v2/apiKeys?limit=1", gotPath)
	}
	if gotAccept != "application/vnd.api+json" {
		t.Fatalf("expected accept header override, got %q", gotAccept)
	}
	if gotReferer != "https://example.test/access/integrations/api" {
		t.Fatalf("expected referer header override, got %q", gotReferer)
	}
	if headers.Get("Content-Type") != "" {
		t.Fatalf("expected caller headers to remain unmodified, got content-type %q", headers.Get("Content-Type"))
	}
}

func TestResolveWebMinRequestInterval(t *testing.T) {
	t.Run("default interval", func(t *testing.T) {
		t.Setenv(webMinRequestIntervalEnv, "")
		if got := resolveWebMinRequestInterval(); got != defaultWebMinRequestInterval {
			t.Fatalf("expected default interval %v, got %v", defaultWebMinRequestInterval, got)
		}
	})

	t.Run("invalid interval falls back to default", func(t *testing.T) {
		t.Setenv(webMinRequestIntervalEnv, "not-a-duration")
		if got := resolveWebMinRequestInterval(); got != defaultWebMinRequestInterval {
			t.Fatalf("expected default interval %v, got %v", defaultWebMinRequestInterval, got)
		}
	})

	t.Run("too low interval is clamped", func(t *testing.T) {
		t.Setenv(webMinRequestIntervalEnv, "5ms")
		if got := resolveWebMinRequestInterval(); got != minimumWebMinRequestInterval {
			t.Fatalf("expected clamped interval %v, got %v", minimumWebMinRequestInterval, got)
		}
	})
}

func TestClientDoRequestAppliesRateLimit(t *testing.T) {
	servedAt := make(chan time.Time, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		servedAt <- time.Now()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	client := &Client{
		httpClient:         server.Client(),
		baseURL:            server.URL,
		minRequestInterval: 80 * time.Millisecond,
	}

	if _, err := client.doRequest(context.Background(), "GET", "/apps", nil); err != nil {
		t.Fatalf("first doRequest error: %v", err)
	}
	if _, err := client.doRequest(context.Background(), "GET", "/apps", nil); err != nil {
		t.Fatalf("second doRequest error: %v", err)
	}

	first := <-servedAt
	second := <-servedAt
	if diff := second.Sub(first); diff < 55*time.Millisecond {
		t.Fatalf("expected low-rate gap between calls, got %v", diff)
	}
}

func TestLoadWebRootCAPoolFromPaths(t *testing.T) {
	certPath := filepath.Join(t.TempDir(), "roots.pem")
	pemData, cert := generateSelfSignedCertPEM(t)
	if err := os.WriteFile(certPath, pemData, 0o600); err != nil {
		t.Fatalf("write cert bundle: %v", err)
	}

	pool := loadWebRootCAPoolFromPaths([]string{
		filepath.Join(t.TempDir(), "missing.pem"),
		certPath,
	})
	if pool == nil {
		t.Fatal("expected non-nil root CA pool")
	}
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("expected generated cert to verify with loaded pool: %v", err)
	}
}

func TestLoadWebRootCAPoolFromPathsWithBaseKeepsBaseWhenNoFallbackCerts(t *testing.T) {
	basePEM, cert := generateSelfSignedCertPEM(t)
	base := x509.NewCertPool()
	if !base.AppendCertsFromPEM(basePEM) {
		t.Fatal("expected base cert to be appended")
	}

	pool := loadWebRootCAPoolFromPathsWithBase(base, []string{filepath.Join(t.TempDir(), "missing.pem")})
	if pool == nil {
		t.Fatal("expected non-nil pool")
	}
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:       pool,
		CurrentTime: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("expected base cert to remain trusted: %v", err)
	}
}

func TestLoadWebRootCAPoolFromPathsReturnsNilWhenNoValidPEM(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalidPath, []byte("not-a-pem"), 0o600); err != nil {
		t.Fatalf("write invalid pem: %v", err)
	}

	pool := loadWebRootCAPoolFromPaths([]string{invalidPath})
	if pool != nil {
		t.Fatalf("expected nil pool for invalid PEM bundle")
	}
}

func generateSelfSignedCertPEM(t *testing.T) ([]byte, *x509.Certificate) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "asc-web-test-root",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: der,
	}
	return pem.EncodeToMemory(block), cert
}
