package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAPIErrorRedactsRawBodyInErrorString(t *testing.T) {
	err := &APIError{
		Status:         422,
		AppleRequestID: "abc-request-id",
		CorrelationKey: "abc-correlation-key",
		rawBody:        []byte(`{"detail":"super-secret-token-123"}`),
	}
	message := err.Error()
	if strings.Contains(message, "super-secret-token-123") {
		t.Fatalf("expected redacted error string, got %q", message)
	}
	if !strings.Contains(message, "status 422") {
		t.Fatalf("expected status in error message, got %q", message)
	}
}

func TestAPIErrorPreservesOrdinaryDiagnosticFormat(t *testing.T) {
	err := &APIError{
		Status:         http.StatusUnauthorized,
		AppleRequestID: "req-123",
		CorrelationKey: "corr-456",
		rawBody:        []byte(`{"serviceErrors":[{"code":"AUTH-401"}]}`),
	}
	if got, want := err.Error(), "web api error (status 401), request_id=req-123, correlation_key=corr-456, codes=[AUTH-401]"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestTwoFAVerificationFailedErrorBoundsAndSanitizesCodes(t *testing.T) {
	codes := make([]map[string]string, 12)
	for i := range codes {
		codes[i] = map[string]string{
			"code": fmt.Sprintf("CODE-%02d-%s", i, strings.Repeat("é", 200)),
		}
	}
	codes[0]["code"] = "CODE-00-bad\x1b[2J\n" + strings.Repeat("x", 300)
	body, err := json.Marshal(map[string]any{"serviceErrors": codes})
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}
	bodyBefore := bytes.Clone(body)
	verificationErr := &twoFAVerificationFailedError{
		Kind:   "trusted-device",
		Status: http.StatusBadRequest,
		Body:   body,
	}

	message := verificationErr.Error()
	if !utf8.ValidString(message) || asc.HasInterpretedTerminalSequence(message) {
		t.Fatalf("2FA diagnostic is not terminal-safe UTF-8: %q", message)
	}
	if strings.Contains(message, "CODE-10-") || strings.Contains(message, "CODE-11-") || !strings.Contains(message, "... and 2 more") {
		t.Fatalf("2FA diagnostic is not count-bounded: %q", message)
	}
	if verificationErr.HTTPStatusCode() != http.StatusBadRequest || !bytes.Equal(verificationErr.Body, bodyBefore) {
		t.Fatalf("2FA structured error data changed: %#v", verificationErr)
	}

	ordinary := &twoFAVerificationFailedError{
		Kind:   "trusted-device",
		Status: http.StatusBadRequest,
		Body:   []byte(`{"serviceErrors":[{"code":"-21669"}]}`),
	}
	if got, want := ordinary.Error(), "trusted-device 2fa failed (status 400, codes=[-21669])"; got != want {
		t.Fatalf("ordinary 2FA diagnostic = %q, want %q", got, want)
	}
}

func TestAPIErrorBoundsAndSanitizesProviderDiagnostics(t *testing.T) {
	requestID := "  request\x1b[31m\n" + strings.Repeat("é", 200) + string([]byte{0xff}) + "tail  "
	correlationKey := "  correlation\u202e" + strings.Repeat("c", 400) + "  "
	codes := make([]map[string]string, 12)
	for i := range codes {
		codes[i] = map[string]string{
			"code": fmt.Sprintf("CODE-%02d-%s", i, strings.Repeat("x", 300)),
		}
	}
	codes[0]["code"] = "CODE-00-bad\x1b[2J\n" + strings.Repeat("x", 300)
	rawBody, err := json.Marshal(map[string]any{"errors": codes})
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}
	rawBodyBefore := bytes.Clone(rawBody)

	apiErr := &APIError{
		Status:         http.StatusUnprocessableEntity,
		AppleRequestID: requestID,
		CorrelationKey: correlationKey,
		rawBody:        rawBody,
	}
	message := apiErr.Error()

	if !utf8.ValidString(message) {
		t.Fatalf("Error() returned invalid UTF-8: %q", message)
	}
	if asc.HasInterpretedTerminalSequence(message) {
		t.Fatalf("Error() retained interpreted terminal characters: %q", message)
	}
	if strings.Contains(message, "CODE-10-") || strings.Contains(message, "CODE-11-") {
		t.Fatalf("Error() included service codes beyond the display cap: %q", message)
	}
	if !strings.Contains(message, "... and 2 more") {
		t.Fatalf("Error() omitted the service-code truncation marker: %q", message)
	}
	if got := diagnosticValueBetween(t, message, "request_id=", ", correlation_key="); len(got) > 256 || !strings.HasSuffix(got, "...") {
		t.Fatalf("request ID projection is not bounded to 256 bytes: len=%d value=%q", len(got), got)
	}
	if got := diagnosticValueBetween(t, message, "correlation_key=", ", codes="); len(got) > 256 || !strings.HasSuffix(got, "...") {
		t.Fatalf("correlation key projection is not bounded to 256 bytes: len=%d value=%q", len(got), got)
	}

	if apiErr.AppleRequestID != requestID || apiErr.CorrelationKey != correlationKey {
		t.Fatalf("Error() mutated structured provider identifiers: %#v", apiErr)
	}
	if !bytes.Equal(apiErr.rawBody, rawBodyBefore) {
		t.Fatal("Error() mutated the raw response body")
	}
	rawCodes := extractServiceErrorCodes(apiErr.rawBody)
	if len(rawCodes) != 12 || !strings.Contains(rawCodes[0], "\x1b[2J\n") {
		t.Fatalf("structured service codes were altered: %#v", rawCodes)
	}
}

func TestBoundedWebAuthDiagnosticCodesPreservesOrdinaryValuesAndDropsEmptyOnes(t *testing.T) {
	if got := boundedWebAuthDiagnosticCodes([]string{"AUTH-401"}); len(got) != 1 || got[0] != "AUTH-401" {
		t.Fatalf("ordinary service code changed: %#v", got)
	}

	codes := []string{"\x1b"}
	for i := 0; i < 12; i++ {
		codes = append(codes, fmt.Sprintf("CODE-%02d-%s", i, strings.Repeat("é", 200)))
	}
	got := boundedWebAuthDiagnosticCodes(codes)
	if len(got) != 11 {
		t.Fatalf("bounded code count = %d, want 11: %#v", len(got), got)
	}
	for i, code := range got[:10] {
		if len(code) > 256 || !utf8.ValidString(code) || asc.HasInterpretedTerminalSequence(code) {
			t.Fatalf("code %d is not a safe 256-byte projection: len=%d value=%q", i, len(code), code)
		}
	}
	if got[10] != "... and 2 more" {
		t.Fatalf("unexpected truncation marker: %q", got[10])
	}
}

func diagnosticValueBetween(t *testing.T, message, prefix, suffix string) string {
	t.Helper()
	start := strings.Index(message, prefix)
	if start < 0 {
		t.Fatalf("message %q does not contain %q", message, prefix)
	}
	start += len(prefix)
	end := strings.Index(message[start:], suffix)
	if end < 0 {
		t.Fatalf("message %q does not contain %q after %q", message, suffix, prefix)
	}
	return message[start : start+end]
}

func TestExtractWebPortalErrorReasonSanitizesAndBoundsDetails(t *testing.T) {
	longDetail := strings.Repeat("x", 600)
	body := []byte(`{"errors":[{"title":"Attachment refused","detail":"` + longDetail + `\u001b[31m"},{"detail":"second reason"}]}`)
	reason := extractWebPortalErrorReason(body)
	if strings.Contains(reason, "\x1b") {
		t.Fatalf("portal reason contains terminal escape sequence: %q", reason)
	}
	if len([]rune(reason)) > 503 {
		t.Fatalf("portal reason exceeds bound: %d runes", len([]rune(reason)))
	}
	if !strings.HasSuffix(reason, "...") {
		t.Fatalf("expected bounded portal reason to carry truncation marker: %q", reason)
	}
}

func TestIsDuplicateAppNameError(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantDup bool
	}{
		{
			name: "duplicate by code and detail",
			err: &APIError{rawBody: []byte(`{
				"errors":[{
					"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.DIFFERENT_ACCOUNT",
					"detail":"The app name you entered is already being used."
				}]
			}`)},
			wantDup: true,
		},
		{
			name: "non-duplicate code",
			err: &APIError{rawBody: []byte(`{
				"errors":[{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID","detail":"invalid value"}]
			}`)},
			wantDup: false,
		},
		{
			name:    "non api error",
			err:     errors.New("nope"),
			wantDup: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsDuplicateAppNameError(tc.err); got != tc.wantDup {
				t.Fatalf("IsDuplicateAppNameError()=%v want %v", got, tc.wantDup)
			}
		})
	}
}

func TestIsMissingCompanyNameError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "required attribute detail",
			err: &APIError{rawBody: []byte(`{
				"errors":[{
					"title":"The provided entity is missing a required attribute",
					"detail":"You must provide a value for the attribute 'companyName' with this request"
				}]
			}`)},
			want: true,
		},
		{
			name: "required attribute name",
			err: &APIError{rawBody: []byte(`{
				"errors":[{
					"attributeName":"companyName",
					"detail":"This attribute is required"
				}]
			}`)},
			want: true,
		},
		{
			name: "different required attribute",
			err: &APIError{rawBody: []byte(`{
				"errors":[{
					"attributeName":"sku",
					"detail":"This attribute is required"
				}]
			}`)},
			want: false,
		},
		{
			name: "company name explicitly optional",
			err: &APIError{rawBody: []byte(`{
				"errors":[{
					"attributeName":"companyName",
					"detail":"This attribute is not required"
				}]
			}`)},
			want: false,
		},
		{
			name: "malformed body",
			err:  &APIError{rawBody: []byte(`{"errors":[`)},
			want: false,
		},
		{
			name: "non api error",
			err:  errors.New("nope"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsMissingCompanyNameError(tc.err); got != tc.want {
				t.Fatalf("IsMissingCompanyNameError()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestIsAlreadyExistsConflict(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "already exists code",
			err: &APIError{
				Status: http.StatusConflict,
				rawBody: []byte(`{
					"errors":[{
						"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
						"title":"The request entity conflicts with the current state."
					}]
				}`),
			},
			want: true,
		},
		{
			name: "already attached detail",
			err: &APIError{
				Status:  http.StatusConflict,
				rawBody: []byte(`{"errors":[{"detail":"This in-app purchase is already attached to the app version."}]}`),
			},
			want: false,
		},
		{
			name: "already exists for another target",
			err: &APIError{
				Status: http.StatusConflict,
				rawBody: []byte(`{
						"errors":[{
							"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
							"detail":"This in-app purchase is already attached to another submission."
						}]
					}`),
			},
			want: false,
		},
		{
			name: "mixed already exists and blocking conflict",
			err: &APIError{
				Status: http.StatusConflict,
				rawBody: []byte(`{
					"errors":[{
						"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS",
						"title":"The request entity conflicts with the current state."
					},{
						"code":"STATE_ERROR.INVALID",
						"detail":"This in-app purchase cannot be attached in the current state."
					}]
				}`),
			},
			want: false,
		},
		{
			name: "other conflict",
			err: &APIError{
				Status:  http.StatusConflict,
				rawBody: []byte(`{"errors":[{"code":"STATE_ERROR","detail":"Invalid state transition."}]}`),
			},
			want: false,
		},
		{
			name: "non conflict",
			err: &APIError{
				Status:  http.StatusBadRequest,
				rawBody: []byte(`{"errors":[{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.ALREADY_EXISTS"}]}`),
			},
			want: false,
		},
		{
			name: "wrapped",
			err: fmt.Errorf("wrapped: %w", &APIError{
				Status:  http.StatusConflict,
				rawBody: []byte(`already exists`),
			}),
			want: true,
		},
		{
			name: "non api error",
			err:  errors.New("nope"),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAlreadyExistsConflict(tc.err); got != tc.want {
				t.Fatalf("IsAlreadyExistsConflict()=%v want %v", got, tc.want)
			}
		})
	}
}

func TestAPIErrorAllCodesReturnsEveryErrorsEntryInOrder(t *testing.T) {
	// Codes captured live on 2026-09-29 for POST /iris/v1/apps re-creating an
	// existing app; the entries' other fields were not captured.
	captured := &APIError{Status: 409, rawBody: []byte(`{"errors":[` +
		`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE"},` +
		`{"code":" ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE "},` +
		`{"code":""},` +
		`{"code":"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT"}]}`)}
	got := captured.AllCodes()
	want := []string{
		"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
		"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE",
		"ENTITY_ERROR.ATTRIBUTE.INVALID.DUPLICATE.SAME_ACCOUNT",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("AllCodes() = %v, want %v", got, want)
	}

	for _, body := range []string{``, `not json`, `{"errors":[]}`, `{"serviceErrors":[{"code":"AUTH-401"}]}`} {
		if got := (&APIError{Status: 409, rawBody: []byte(body)}).AllCodes(); got != nil {
			t.Fatalf("AllCodes(%q) = %v, want nil", body, got)
		}
	}
	var nilErr *APIError
	if got := nilErr.AllCodes(); got != nil {
		t.Fatalf("nil AllCodes() = %v, want nil", got)
	}
}

func TestTwoFAVerificationFailedErrorIsRejectedCodeOnlyForRefusals(t *testing.T) {
	for _, test := range []struct {
		kind   string
		status int
		want   bool
	}{
		{kind: "trusted-device", status: http.StatusBadRequest, want: true},
		{kind: "phone", status: http.StatusBadRequest, want: true},
		{kind: "phone", status: http.StatusUnauthorized},
		{kind: "trusted-device", status: http.StatusForbidden},
		{kind: "trusted-device", status: http.StatusTooManyRequests},
		{kind: "trusted-device", status: http.StatusServiceUnavailable},
		{kind: "phone-request", status: http.StatusBadRequest},
	} {
		err := fmt.Errorf("wrapped: %w", &twoFAVerificationFailedError{Kind: test.kind, Status: test.status})
		if got := errors.Is(err, ErrTwoFactorCodeRejected); got != test.want {
			t.Errorf("%s status %d: errors.Is(ErrTwoFactorCodeRejected) = %t, want %t", test.kind, test.status, got, test.want)
		}
	}
}
