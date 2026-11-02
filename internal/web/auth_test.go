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
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAuthStatusErrorsExposeHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  interface{ HTTPStatusCode() int }
	}{
		{name: "session info", err: &sessionInfoStatusError{Status: http.StatusUnauthorized}},
		{name: "two-factor verification", err: &twoFAVerificationFailedError{Status: http.StatusForbidden}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.HTTPStatusCode(); got < 400 || got > 599 {
				t.Fatalf("HTTPStatusCode() = %d, want an HTTP error status", got)
			}
		})
	}
}

func TestInvalidCredentialsClassificationScansUnboundedRawCodes(t *testing.T) {
	codes := make([]map[string]string, 12)
	for i := 0; i < 11; i++ {
		codes[i] = map[string]string{"code": fmt.Sprintf("NOISE-%02d", i)}
	}
	codes[11] = map[string]string{"code": "-20101"}
	body, err := json.Marshal(map[string]any{"serviceErrors": codes})
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}

	if !isInvalidAppleAccountCredentialsSigninComplete(http.StatusUnauthorized, body) {
		t.Fatal("invalid-credentials classifier did not inspect the raw code beyond the display cap")
	}
}

func TestNewClientPreservesProviderIdentity(t *testing.T) {
	client := NewClient(&AuthSession{
		Client:           &http.Client{},
		PublicProviderID: " TEAM123 ",
		ProviderName:     " Example Team ",
	})

	if client.publicProviderID != "TEAM123" {
		t.Fatalf("publicProviderID = %q", client.publicProviderID)
	}
	if client.providerName != "Example Team" {
		t.Fatalf("providerName = %q", client.providerName)
	}
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

type diagnosticCaptureHandler struct {
	values map[string]string
}

func (h *diagnosticCaptureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *diagnosticCaptureHandler) Handle(_ context.Context, record slog.Record) error {
	record.Attrs(func(attr slog.Attr) bool {
		h.values[attr.Key] = fmt.Sprint(attr.Value.Any())
		return true
	})
	return nil
}

func (h *diagnosticCaptureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *diagnosticCaptureHandler) WithGroup(string) slog.Handler { return h }

func TestLogWebAuthHTTPBoundsAndSanitizesProviderDiagnostics(t *testing.T) {
	origLogger := webDebugLogger
	origDebugEnabled := webDebugEnabledFn
	t.Cleanup(func() {
		webDebugLogger = origLogger
		webDebugEnabledFn = origDebugEnabled
	})

	handler := &diagnosticCaptureHandler{values: make(map[string]string)}
	webDebugLogger = slog.New(handler)
	webDebugEnabledFn = func() bool { return true }

	codes := make([]map[string]string, 12)
	for i := range codes {
		codes[i] = map[string]string{
			"code": fmt.Sprintf("CODE-%02d-%s", i, strings.Repeat("x", 300)),
		}
	}
	codes[0]["code"] = "CODE-00-bad\x1b[2J\n" + strings.Repeat("x", 300)
	body, err := json.Marshal(map[string]any{"errors": codes})
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}
	requestID := "request\x1b[31m\n" + strings.Repeat("é", 200) + string([]byte{0xff})
	correlationKey := "correlation\u202e" + strings.Repeat("c", 400)
	resp := &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Header: http.Header{
			"X-Apple-Request-Uuid":           []string{requestID},
			"X-Apple-Jingle-Correlation-Key": []string{correlationKey},
		},
	}

	logWebAuthHTTP("signin_complete", nil, resp, body, nil)

	for _, key := range []string{"request_id", "correlation_key", "codes"} {
		value := handler.values[key]
		if !utf8.ValidString(value) {
			t.Fatalf("%s is not valid UTF-8: %q", key, value)
		}
		if asc.HasInterpretedTerminalSequence(value) {
			t.Fatalf("%s retained interpreted terminal characters: %q", key, value)
		}
	}
	for _, key := range []string{"request_id", "correlation_key"} {
		if value := handler.values[key]; len(value) > 256 || !strings.HasSuffix(value, "...") {
			t.Fatalf("%s projection is not bounded to 256 bytes: len=%d value=%q", key, len(value), value)
		}
	}
	codeValue := handler.values["codes"]
	if strings.Contains(codeValue, "CODE-10-") || strings.Contains(codeValue, "CODE-11-") || !strings.Contains(codeValue, "... and 2 more") {
		t.Fatalf("codes projection is not count-bounded: %q", codeValue)
	}
}

func TestSanitizeWebAuthURLForLogRedactsTransactionTaxJobID(t *testing.T) {
	const jobID = "txn-tax-job-secret-9f3c"
	rawURL := "https://appstoreconnect.apple.com/WebObjects/iTunesConnect.woa/ra/paymentConsolidation/providers/123/sapVendorNumbers/456/reports/" + jobID + "/status?year=2026&regionCurrencyIds=13%2C88&signature=secret"

	got := sanitizeWebAuthURLForLog(rawURL)
	for _, secret := range []string{jobID, "/providers/123/", "/sapVendorNumbers/456/", "13%2C88", "13,88", "secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("sanitized URL leaked Transaction Tax value %q: %q", secret, got)
		}
	}
	if !strings.Contains(got, "reports/%5BREDACTED%5D/status") {
		t.Fatalf("sanitized URL did not redact Transaction Tax job path: %q", got)
	}
	if !strings.Contains(got, "providers/%5BREDACTED%5D") || !strings.Contains(got, "sapVendorNumbers/%5BREDACTED%5D") {
		t.Fatalf("sanitized URL did not redact provider and SAP vendor path values: %q", got)
	}
	if !strings.Contains(got, "regionCurrencyIds=%5BREDACTED%5D") {
		t.Fatalf("sanitized URL did not redact region currency IDs: %q", got)
	}
}

func TestLogWebAuthHTTPRedactsTransactionTaxTransportError(t *testing.T) {
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

	const jobID = "txn-tax-job-secret-9f3c"
	pollURL := "https://appstoreconnect.apple.com" + transactionTaxFinancePath + "/providers/123/sapVendorNumbers/456/reports/" + jobID + "/status"
	req, err := http.NewRequest(http.MethodGet, pollURL, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp := &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header)}
	logWebAuthHTTP("iris_request", req, resp, nil, &url.Error{Op: "Get", URL: pollURL, Err: errors.New("poll transport secret")})

	output := logs.String()
	if strings.Contains(output, jobID) || strings.Contains(output, "poll transport secret") || strings.Contains(output, "/providers/123/") || strings.Contains(output, "/sapVendorNumbers/456/") {
		t.Fatalf("Transaction Tax transport error leaked into debug output: %q", output)
	}
	if !strings.Contains(output, "stage=iris_request") || !strings.Contains(output, "status=502") || !strings.Contains(output, "transaction tax request failed") {
		t.Fatalf("expected safe stage/status/error diagnostics, got %q", output)
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

func TestSigninCompleteReportsLockedAppleAccount(t *testing.T) {
	// #3111: Apple answers signin/complete with 403 and -20209 once it has
	// locked the account for API sign-in. Only that pair means locked; other
	// 403 refusals keep the generic service error.
	for _, tc := range []struct {
		name       string
		body       string
		wantLocked bool
	}{
		{name: "-20209 locked", body: `{"serviceErrors":[{"code":"-20209","message":"This Apple Account has been locked for security reasons. Visit iForgot to reset your account (https://iforgot.apple.com)","suppressDismissal":false}]}`, wantLocked: true},
		{name: "other 403 code", body: `{"serviceErrors":[{"code":"-20751","message":"Your account cannot sign in at this time."}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusForbidden,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(tc.body)),
					}, nil
				}),
			}

			err := signinComplete(context.Background(), client, "user@example.com", "m1-proof", "m2-proof", json.RawMessage(`{"v":1}`), "service-key", "hashcash-token")
			if errors.Is(err, ErrAppleAccountLocked) != tc.wantLocked {
				t.Fatalf("errors.Is(err, ErrAppleAccountLocked) = %v, want %v for %v", !tc.wantLocked, tc.wantLocked, err)
			}
			var serviceErr *SigninServiceError
			if !errors.As(err, &serviceErr) || serviceErr.Status != http.StatusForbidden {
				t.Fatalf("error = %v, want the 403 service error with Apple's message", err)
			}
			if !strings.Contains(err.Error(), "signin_complete") || !strings.Contains(err.Error(), "403") {
				t.Fatalf("error = %q, want the stage and status", err)
			}
		})
	}
}

func TestSigninCompleteReturnsAppleAccountActionRequiredForPreconditionFailed(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusPreconditionFailed,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"authType":"hsa2"}`)),
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
	if !errors.Is(err, errAppleAccountActionRequired) {
		t.Fatalf("expected account-action-required error, got %v", err)
	}
}

func TestGetHashcashAllowsMissingChallengeHeaders(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if got, want := req.URL.String(), authServiceURL+"/signin?widgetKey=service-key"; got != want {
				t.Fatalf("expected request URL %q, got %q", want, got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}),
	}

	hashcash, err := getHashcash(context.Background(), client, "service-key")
	if err != nil {
		t.Fatalf("expected missing-hashcash headers to be tolerated, got %v", err)
	}
	if hashcash != "" {
		t.Fatalf("expected empty hashcash when headers are absent, got %q", hashcash)
	}
}

func TestEnsureTwoFactorCodeRequestedRequestsPhoneCodeWhenNoTrustedDevicesHasMultipleNumbers(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": true,
							"trustedPhoneNumbers": [
								{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"},
								{"id": 9, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••88"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						Mode string `json:"mode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone request payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.Mode != "sms" {
						t.Fatalf("expected sms mode, got %q", payload.Mode)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	challenge, err := EnsureTwoFactorCodeRequested(context.Background(), session)
	if err != nil {
		t.Fatalf("EnsureTwoFactorCodeRequested returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodPhone {
		t.Fatalf("expected phone method, got %q", challenge.Method)
	}
	if challenge.Destination != "+1 (•••) •••-••66" {
		t.Fatalf("expected first phone destination, got %q", challenge.Destination)
	}
	if !challenge.Requested {
		t.Fatal("expected phone code request for multiple fallback numbers")
	}
	if session.twoFactorPhoneID != 7 {
		t.Fatalf("expected stored phone id 7, got %d", session.twoFactorPhoneID)
	}
}

func TestTwoFactorChallengeIsPhoneMethod(t *testing.T) {
	if challenge := (*TwoFactorChallenge)(nil); challenge.IsPhoneMethod() {
		t.Fatal("expected nil challenge not to report phone method")
	}

	if challenge := (&TwoFactorChallenge{Method: twoFactorMethodPhone}); !challenge.IsPhoneMethod() {
		t.Fatal("expected phone challenge to report phone method")
	}

	if challenge := (&TwoFactorChallenge{Method: twoFactorMethodTrustedDevice}); challenge.IsPhoneMethod() {
		t.Fatal("expected trusted-device challenge not to report phone method")
	}
}

func TestEnsureTwoFactorCodeRequestedRequestsPhoneCodeForSingleTrustedPhone(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": true,
							"trustedPhoneNumbers": [
								{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	challenge, err := EnsureTwoFactorCodeRequested(context.Background(), session)
	if err != nil {
		t.Fatalf("EnsureTwoFactorCodeRequested returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodPhone {
		t.Fatalf("expected phone method, got %q", challenge.Method)
	}
	if !challenge.Requested {
		t.Fatal("expected single trusted phone to trigger a code request before prompting")
	}
}

func TestEnsureTwoFactorCodeRequestedRequestsSMSWhenPreferredOverTrustedDevices(t *testing.T) {
	var phoneRequest map[string]any
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": false,
							"trustedDevices": [{"id": "device"}],
							"trustedPhoneNumbers": [
								{"id": 1, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••11"},
								{"id": 2, "pushMode": "voice", "numberWithDialCode": "+1 (•••) •••-••22"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					if err := json.NewDecoder(req.Body).Decode(&phoneRequest); err != nil {
						t.Fatalf("decode phone request: %v", err)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}
	session.SetTwoFactorPreference(appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS, PhoneID: 2})

	challenge, err := EnsureTwoFactorCodeRequested(context.Background(), session)
	if err != nil {
		t.Fatalf("EnsureTwoFactorCodeRequested returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodPhone || !challenge.Requested {
		t.Fatalf("expected a requested phone challenge, got %+v", challenge)
	}
	if challenge.Destination != "+1 (•••) •••-••22" {
		t.Fatalf("expected selected phone destination, got %q", challenge.Destination)
	}
	want := map[string]any{"phoneNumber": map[string]any{"id": float64(2)}, "mode": "sms"}
	if !reflect.DeepEqual(phoneRequest, want) {
		t.Fatalf("unexpected phone request payload: %#v", phoneRequest)
	}
}

func TestPrepareTwoFactorChallengeWrapsAmbiguousSMSPhoneSelection(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.String() != authServiceURL {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"trustedPhoneNumbers": [
							{"id": 1, "numberWithDialCode": "+1 (•••) •••-••11"},
							{"id": 2, "numberWithDialCode": "+1 (•••) •••-••22"}
						]
					}`)),
				}, nil
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}
	session.SetTwoFactorPreference(appleauth.TwoFactorPreference{Delivery: appleauth.TwoFactorDeliverySMS})

	_, err := PrepareTwoFactorChallenge(context.Background(), session)
	var selectionErr *appleauth.TrustedPhoneSelectionError
	if !errors.As(err, &selectionErr) {
		t.Fatalf("expected TrustedPhoneSelectionError, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "2fa failed: multiple trusted phone numbers are available") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrepareTwoFactorChallengeKeepsPhoneFallbackForTrustedDeviceFlow(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.String() != authServiceURL {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"trustedDevices": [{"id":"device-1"}],
						"trustedPhoneNumbers": [
							{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"}
						]
					}`)),
				}, nil
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	challenge, err := PrepareTwoFactorChallenge(context.Background(), session)
	if err != nil {
		t.Fatalf("PrepareTwoFactorChallenge returned error: %v", err)
	}
	if challenge.Method != twoFactorMethodTrustedDevice {
		t.Fatalf("expected trusted-device method, got %q", challenge.Method)
	}
	if !challenge.PhoneFallbackAvailable {
		t.Fatal("expected phone fallback to remain available")
	}
	if session.twoFactorPhoneID != 7 {
		t.Fatalf("expected stored fallback phone id 7, got %d", session.twoFactorPhoneID)
	}
	if challenge.Destination != "+1 (•••) •••-••66" {
		t.Fatalf("expected fallback destination on initial trusted-device challenge, got %q", challenge.Destination)
	}

	cachedChallenge, err := PrepareTwoFactorChallenge(context.Background(), session)
	if err != nil {
		t.Fatalf("PrepareTwoFactorChallenge (cached) returned error: %v", err)
	}
	if cachedChallenge.Destination != challenge.Destination {
		t.Fatalf("expected cached challenge destination %q, got %q", challenge.Destination, cachedChallenge.Destination)
	}
}

// Regression: preparing the same FSA challenge twice must preserve its bytes
// without requesting another challenge or routing the account into code delivery.
func TestPrepareTwoFactorChallengeCachesSecurityKeyContinuation(t *testing.T) {
	requests := 0
	session := &AuthSession{
		ServiceKey: "service-key", AppleIDSessionID: "session-id", SCNT: "scnt-token",
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if req.Method != http.MethodGet || req.URL.String() != authServiceURL {
				t.Fatalf("unexpected request %s %s", req.Method, req.URL)
			}
			for name, want := range map[string]string{"X-Apple-Widget-Key": "service-key", "X-Apple-ID-Session-Id": "session-id", "scnt": "scnt-token"} {
				if got := req.Header.Get(name); got != want {
					t.Fatalf("%s = %q, want %q", name, got, want)
				}
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"fsaChallenge":{"challenge":"AQID","allowedCredentials":"BAUG","keyHandles":["BAUG"]},"trustedDevices":[{"id":"device"}]}`))}, nil
		})},
	}
	for range 2 {
		challenge, err := PrepareTwoFactorChallenge(context.Background(), session)
		if err != nil {
			t.Fatal(err)
		}
		if challenge.Method != appleauth.TwoFactorMethodSecurityKey || challenge.SecurityKey == nil {
			t.Fatalf("challenge = %+v, want security key", challenge)
		}
		if challenge.SecurityKey.Challenge != "AQID" || challenge.SecurityKey.AllowedCredentials != "BAUG" {
			t.Fatalf("continuation bytes changed: %+v", challenge.SecurityKey)
		}
		if challenge.PhoneFallbackAvailable || challenge.Requested {
			t.Fatal("security-key continuation exposed code delivery")
		}
	}
	if requests != 1 {
		t.Fatalf("auth-options requests = %d, want 1", requests)
	}
	if session.TwoFactorMethod() != appleauth.TwoFactorMethodSecurityKey {
		t.Fatalf("prepared method = %q", session.TwoFactorMethod())
	}
}

func TestSubmitTwoFactorCodeUsesPreparedPhoneFlow(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						Mode string `json:"mode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone delivery payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected delivery phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.Mode != "voice" {
						t.Fatalf("expected delivery mode voice, got %q", payload.Mode)
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						SecurityCode struct {
							Code string `json:"code"`
						} `json:"securityCode"`
						Mode string `json:"mode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone verification payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.SecurityCode.Code != "123456" {
						t.Fatalf("expected 2fa code 123456, got %q", payload.SecurityCode.Code)
					}
					if payload.Mode != "voice" {
						t.Fatalf("expected voice mode, got %q", payload.Mode)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == olympusSessionURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 123, "publicProviderId": "public-123"},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:           "service-key",
		AppleIDSessionID:     "session-id",
		SCNT:                 "scnt-token",
		twoFactorMethod:      twoFactorMethodPhone,
		twoFactorPhoneID:     7,
		twoFactorPhoneMode:   "voice",
		twoFactorDestination: "+1 (•••) •••-••66",
	}

	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode returned error: %v", err)
	}
	if session.ProviderID != 123 {
		t.Fatalf("expected provider id 123, got %d", session.ProviderID)
	}
	if session.PublicProviderID != "public-123" {
		t.Fatalf("expected public provider id public-123, got %q", session.PublicProviderID)
	}
	if session.UserEmail != "user@example.com" {
		t.Fatalf("expected user email to be refreshed, got %q", session.UserEmail)
	}
}

func TestSubmitTwoFactorCodeSubmitsPreparedPhoneFallbackAfterTrustedDeviceFailure(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/trusteddevice/securitycode":
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"serviceErrors":[{"code":"-21669"}]}`)),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						SecurityCode struct {
							Code string `json:"code"`
						} `json:"securityCode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone verification payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected fallback phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.SecurityCode.Code != "123456" {
						t.Fatalf("expected 2fa code 123456, got %q", payload.SecurityCode.Code)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == olympusSessionURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 123, "publicProviderId": "public-123"},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:             "service-key",
		AppleIDSessionID:       "session-id",
		SCNT:                   "scnt-token",
		twoFactorMethod:        twoFactorMethodPhone,
		twoFactorPhoneID:       7,
		twoFactorPhoneMode:     "sms",
		twoFactorCodeRequested: true,
	}

	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode returned error: %v", err)
	}
}

func TestSubmitTwoFactorCodePreservesPreparedPhoneFallbackStateWhenFinalizeFails(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/trusteddevice/securitycode":
					return &http.Response{
						StatusCode: http.StatusBadRequest,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"serviceErrors":[{"code":"-21669"}]}`)),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:             "service-key",
		AppleIDSessionID:       "session-id",
		SCNT:                   "scnt-token",
		twoFactorMethod:        twoFactorMethodPhone,
		twoFactorPhoneID:       7,
		twoFactorPhoneMode:     "sms",
		twoFactorDestination:   "+1 (•••) •••-••66",
		twoFactorCodeRequested: true,
	}

	err := SubmitTwoFactorCode(context.Background(), session, "123456")
	if err == nil {
		t.Fatal("expected finalize failure")
	}
	if !strings.Contains(err.Error(), "2fa trust failed with status 500") {
		t.Fatalf("expected finalize failure, got %v", err)
	}
	if session.twoFactorMethod != twoFactorMethodPhone {
		t.Fatalf("expected phone fallback method to be preserved, got %q", session.twoFactorMethod)
	}
	if !session.twoFactorCodeRequested {
		t.Fatal("expected phone fallback state to mark code delivery as requested")
	}
}

func TestSubmitTwoFactorCodeRequestsPhoneDeliveryBeforeVerification(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"noTrustedDevices": true,
							"trustedPhoneNumbers": [
								{"id": 7, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••66"},
								{"id": 9, "pushMode": "sms", "numberWithDialCode": "+1 (•••) •••-••88"}
							]
						}`)),
					}, nil
				case req.Method == http.MethodPut && req.URL.String() == authServiceURL+"/verify/phone":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone delivery payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected delivery phone id 7, got %d", payload.PhoneNumber.ID)
					}
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case req.Method == http.MethodPost && req.URL.String() == authServiceURL+"/verify/phone/securitycode":
					var payload struct {
						PhoneNumber struct {
							ID int `json:"id"`
						} `json:"phoneNumber"`
						SecurityCode struct {
							Code string `json:"code"`
						} `json:"securityCode"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode phone verification payload: %v", err)
					}
					if payload.PhoneNumber.ID != 7 {
						t.Fatalf("expected phone id 7, got %d", payload.PhoneNumber.ID)
					}
					if payload.SecurityCode.Code != "123456" {
						t.Fatalf("expected 2fa code 123456, got %q", payload.SecurityCode.Code)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == authServiceURL+"/2sv/trust":
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case req.Method == http.MethodGet && req.URL.String() == olympusSessionURL:
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 123, "publicProviderId": "public-123"},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
	}

	if err := SubmitTwoFactorCode(context.Background(), session, "123456"); err != nil {
		t.Fatalf("SubmitTwoFactorCode returned error: %v", err)
	}
}

func TestSelectProviderByPublicProviderID(t *testing.T) {
	requests := 0
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				switch requests {
				case 1:
					if req.Method != http.MethodGet || req.URL.String() != olympusSessionURL {
						t.Fatalf("unexpected initial request %s %s", req.Method, req.URL.String())
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 111, "publicProviderId": "TEAM111", "name": "Old Team"},
							"availableProviders": [
								{"providerId": 111, "publicProviderId": "TEAM111", "name": "Old Team"},
								{"providerId": 222, "publicProviderId": "TEAM222", "name": "New Team"}
							],
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				case 2:
					if req.Method != http.MethodPost || req.URL.String() != olympusSessionURL {
						t.Fatalf("unexpected provider selection request %s %s", req.Method, req.URL.String())
					}
					if got := req.Header.Get("X-Requested-With"); got != "olympus-ui" {
						t.Fatalf("expected X-Requested-With olympus-ui, got %q", got)
					}
					var payload struct {
						Provider struct {
							ProviderID int64 `json:"providerId"`
						} `json:"provider"`
					}
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode provider selection payload: %v", err)
					}
					if payload.Provider.ProviderID != 222 {
						t.Fatalf("expected providerId 222, got %d", payload.Provider.ProviderID)
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{}`)),
					}, nil
				case 3:
					if req.Method != http.MethodGet || req.URL.String() != olympusSessionURL {
						t.Fatalf("unexpected refresh request %s %s", req.Method, req.URL.String())
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`{
							"provider": {"providerId": 222, "publicProviderId": "TEAM222", "name": "New Team"},
							"user": {"emailAddress": "user@example.com"}
						}`)),
					}, nil
				default:
					t.Fatalf("unexpected extra request %d: %s %s", requests, req.Method, req.URL.String())
					return nil, nil
				}
			}),
		},
	}

	err := SelectProvider(context.Background(), session, ProviderSelection{PublicProviderID: "team222"})
	if err != nil {
		t.Fatalf("SelectProvider returned error: %v", err)
	}
	if session.ProviderID != 222 {
		t.Fatalf("expected provider id 222, got %d", session.ProviderID)
	}
	if session.PublicProviderID != "TEAM222" {
		t.Fatalf("expected public provider id TEAM222, got %q", session.PublicProviderID)
	}
	if session.TeamID != "222" {
		t.Fatalf("expected team id 222, got %q", session.TeamID)
	}
	if session.UserEmail != "user@example.com" {
		t.Fatalf("expected user email user@example.com, got %q", session.UserEmail)
	}
	if requests != 3 {
		t.Fatalf("expected 3 requests, got %d", requests)
	}
}

func TestSelectProviderRejectsUnknownPublicProviderID(t *testing.T) {
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.String() != olympusSessionURL {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"provider": {"providerId": 111, "publicProviderId": "TEAM111", "name": "Old Team"},
						"availableProviders": [
							{"providerId": 111, "publicProviderId": "TEAM111", "name": "Old Team"}
						],
						"user": {"emailAddress": "user@example.com"}
					}`)),
				}, nil
			}),
		},
	}

	err := SelectProvider(context.Background(), session, ProviderSelection{PublicProviderID: "TEAM404"})
	if err == nil {
		t.Fatal("expected unknown provider error")
	}
	if !strings.Contains(err.Error(), "TEAM404") || !strings.Contains(err.Error(), "TEAM111") {
		t.Fatalf("expected error to include requested and available providers, got %v", err)
	}
}

func TestSelectProviderRejectsMismatchedProviderIDs(t *testing.T) {
	requests := 0
	session := &AuthSession{
		Client: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet || req.URL.String() != olympusSessionURL {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.String())
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"provider": {"providerId": 111, "publicProviderId": "TEAM111", "name": "Old Team"},
						"availableProviders": [
							{"providerId": 111, "publicProviderId": "TEAM111", "name": "Old Team"},
							{"providerId": 222, "publicProviderId": "TEAM222", "name": "New Team"}
						],
						"user": {"emailAddress": "user@example.com"}
					}`)),
				}, nil
			}),
		},
	}

	err := SelectProvider(context.Background(), session, ProviderSelection{ProviderID: 999, PublicProviderID: "TEAM222"})
	if err == nil {
		t.Fatal("expected provider mismatch error")
	}
	if !strings.Contains(err.Error(), "TEAM222") || !strings.Contains(err.Error(), "999") || !strings.Contains(err.Error(), "222") {
		t.Fatalf("expected error to include mismatched provider ids, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected mismatch to stop before provider selection POST, got %d requests", requests)
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

// A stale cached cookie jar makes the post-2FA App Store Connect session
// bootstrap fail with 401 even though Apple already accepted the 2FA code.
// The error must identify that as a finalization failure carrying the status.
func TestSubmitTwoFactorCodeReportsStaleSessionBootstrapAfterAcceptedCode(t *testing.T) {
	var trustedDeviceCalls, trustCalls, sessionCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/appleauth/auth/verify/trusteddevice/securitycode":
			trustedDeviceCalls++
			w.WriteHeader(http.StatusNoContent)
		case "/appleauth/auth/2sv/trust":
			trustCalls++
			w.WriteHeader(http.StatusNoContent)
		case "/olympus/v1/session":
			sessionCalls++
			w.WriteHeader(http.StatusUnauthorized)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	session := &AuthSession{
		Client:           newTestServerRoutedClient(t, server),
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
		twoFactorMethod:  twoFactorMethodTrustedDevice,
	}

	err := SubmitTwoFactorCode(context.Background(), session, "123456")
	if err == nil {
		t.Fatal("expected stale session bootstrap error")
	}
	if trustedDeviceCalls != 1 || trustCalls != 1 || sessionCalls != 1 {
		t.Fatalf("expected one call per 2fa stage, got trusted-device=%d trust=%d session=%d", trustedDeviceCalls, trustCalls, sessionCalls)
	}

	var finalizeErr *TwoFactorFinalizationError
	if !errors.As(err, &finalizeErr) {
		t.Fatalf("expected *TwoFactorFinalizationError, got %T: %v", err, err)
	}
	if finalizeErr.Status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", finalizeErr.Status)
	}
	if finalizeErr.HTTPStatusCode() != http.StatusUnauthorized {
		t.Fatalf("expected HTTPStatusCode 401, got %d", finalizeErr.HTTPStatusCode())
	}
	if !IsStaleSessionAfterTwoFactor(err) {
		t.Fatal("expected a stale-session finalization failure to be retryable with a fresh login")
	}
	if got := err.Error(); !strings.Contains(got, "401") {
		t.Fatalf("expected finalization error to report the HTTP status, got %q", got)
	}
	if got := err.Error(); strings.Contains(got, "verification") {
		t.Fatalf("expected finalization error to avoid verification wording, got %q", got)
	}
}

func TestIsStaleSessionAfterTwoFactorIgnoresUnrelatedFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unauthorized", err: &TwoFactorFinalizationError{Status: http.StatusUnauthorized}, want: true},
		{name: "forbidden", err: &TwoFactorFinalizationError{Status: http.StatusForbidden}, want: true},
		{name: "server error", err: &TwoFactorFinalizationError{Status: http.StatusInternalServerError}, want: false},
		{name: "wrapped", err: fmt.Errorf("wrapped: %w", &TwoFactorFinalizationError{Status: http.StatusUnauthorized}), want: true},
		{name: "rejected code", err: &twoFAVerificationFailedError{Kind: "trusted-device", Status: http.StatusBadRequest}, want: false},
		{name: "bare session info", err: &sessionInfoStatusError{Status: http.StatusUnauthorized}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsStaleSessionAfterTwoFactor(tc.err); got != tc.want {
				t.Fatalf("IsStaleSessionAfterTwoFactor(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// newTestServerRoutedClient routes Apple's hardcoded auth hosts at a local
// httptest server while preserving request paths, headers, and bodies.
func newTestServerRoutedClient(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server url: %v", err)
	}
	transport := server.Client().Transport
	return &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		routed := req.Clone(req.Context())
		routed.URL.Scheme = serverURL.Scheme
		routed.URL.Host = serverURL.Host
		routed.Host = ""
		return transport.RoundTrip(routed)
	})}
}

// Apple already consumed the submitted code before the trust step runs, so a
// 401 from /2sv/trust means the reused cookie jar is stale, not that the code
// was wrong. It must classify like the session-bootstrap failure so callers
// discard the proven-stale jar and retry fresh.
func TestSubmitTwoFactorCodeReportsStaleTrustFailureAfterAcceptedCode(t *testing.T) {
	var trustedDeviceCalls, trustCalls, sessionCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/appleauth/auth/verify/trusteddevice/securitycode":
			trustedDeviceCalls++
			w.WriteHeader(http.StatusNoContent)
		case "/appleauth/auth/2sv/trust":
			trustCalls++
			w.WriteHeader(http.StatusUnauthorized)
		case "/olympus/v1/session":
			sessionCalls++
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	session := &AuthSession{
		Client:           newTestServerRoutedClient(t, server),
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
		twoFactorMethod:  twoFactorMethodTrustedDevice,
	}

	err := SubmitTwoFactorCode(context.Background(), session, "123456")
	if err == nil {
		t.Fatal("expected the trust failure to be reported")
	}
	if trustedDeviceCalls != 1 || trustCalls != 1 || sessionCalls != 0 {
		t.Fatalf("expected the flow to stop at the trust step, got trusted-device=%d trust=%d session=%d", trustedDeviceCalls, trustCalls, sessionCalls)
	}

	var finalizeErr *TwoFactorFinalizationError
	if !errors.As(err, &finalizeErr) {
		t.Fatalf("expected *TwoFactorFinalizationError, got %T: %v", err, err)
	}
	if finalizeErr.Status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", finalizeErr.Status)
	}
	if !IsStaleSessionAfterTwoFactor(err) {
		t.Fatal("expected a 401 from the trust step to be retryable with a fresh login")
	}
	if got := err.Error(); !strings.Contains(got, "401") {
		t.Fatalf("expected the error to report the HTTP status, got %q", got)
	}
	if got := err.Error(); strings.Contains(got, "verification") {
		t.Fatalf("expected the error to avoid verification wording, got %q", got)
	}
}

// A trust step that fails for a reason unrelated to authorization must not be
// treated as a stale cached session: retrying fresh burns another 2FA code.
func TestSubmitTwoFactorCodeKeepsNonAuthorizationTrustFailuresUnretryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/appleauth/auth/verify/trusteddevice/securitycode":
			w.WriteHeader(http.StatusNoContent)
		case "/appleauth/auth/2sv/trust":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	session := &AuthSession{
		Client:           newTestServerRoutedClient(t, server),
		ServiceKey:       "service-key",
		AppleIDSessionID: "session-id",
		SCNT:             "scnt-token",
		twoFactorMethod:  twoFactorMethodTrustedDevice,
	}

	err := SubmitTwoFactorCode(context.Background(), session, "123456")
	if err == nil {
		t.Fatal("expected the trust failure to be reported")
	}
	if IsStaleSessionAfterTwoFactor(err) {
		t.Fatalf("expected a 500 from the trust step to stay unretryable, got %v", err)
	}
	if got := err.Error(); !strings.Contains(got, "500") {
		t.Fatalf("expected the error to report the HTTP status, got %q", got)
	}
}
