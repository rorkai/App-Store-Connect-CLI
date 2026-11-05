package asc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const agreementsForbiddenBody = `{"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED","title":"A required agreement is missing or has expired","detail":"This request requires an in-effect agreement that has not been signed or has expired."}]}`

func newForbiddenRemediationTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	return &Client{
		httpClient: server.Client(),
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: testJWTPrivateKey(t),
	}
}

// An unaccepted or expired agreement blocks the whole team, so the generic
// "check your API key permissions" reading of a 403 sends operators down the
// wrong path. The guidance has to ride the error itself.
func TestParseError_AgreementForbiddenCarriesRemediation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, agreementsForbiddenBody)
	}))
	t.Cleanup(server.Close)

	client := newForbiddenRemediationTestClient(t, server)

	_, err := client.do(context.Background(), http.MethodPost, server.URL+"/v1/appStoreVersions", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	message := err.Error()
	if !strings.Contains(message, "A required agreement is missing or has expired") {
		t.Fatalf("expected Apple's own message to be preserved, got %q", message)
	}
	if !strings.Contains(message, "Account Holder") {
		t.Fatalf("expected remediation naming who can accept the agreement, got %q", message)
	}
	if !strings.Contains(message, "https://appstoreconnect.apple.com/agreements") {
		t.Fatalf("expected remediation linking the agreements page, got %q", message)
	}

	apiErr, ok := errors.AsType[*APIError](err)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, apiErr.StatusCode)
	}
	// Callers that branch on forbidden must keep working.
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected the error to still match ErrForbidden, got %v", err)
	}
}

// A permissions 403 is a different problem with a different fix, so it must not
// pick up agreement guidance.
func TestParseError_PermissionForbiddenHasNoAgreementRemediation(t *testing.T) {
	body := `{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"This request is forbidden for security reasons","detail":"The API key in use does not allow this request."}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	client := newForbiddenRemediationTestClient(t, server)

	_, err := client.do(context.Background(), http.MethodGet, server.URL+"/v1/apps", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	want := "This request is forbidden for security reasons: The API key in use does not allow this request."
	if got := err.Error(); got != want {
		t.Fatalf("expected the message to be unchanged\n got: %q\nwant: %q", got, want)
	}
}

func TestRemediationForAPIError(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		wantAgreement bool
	}{
		{name: "required agreements missing or expired", code: "FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED", wantAgreement: true},
		// Apple returns both FORBIDDEN and FORBIDDEN_ERROR prefixes for the same
		// account-level cause, so match on the specific segment.
		{name: "alternate prefix", code: "FORBIDDEN_ERROR.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED", wantAgreement: true},
		{name: "program license agreement not valid", code: "FORBIDDEN_ERROR.PLA_NOT_VALID", wantAgreement: true},
		{name: "lowercase", code: "forbidden.required_agreements_missing_or_expired", wantAgreement: true},
		{name: "unrelated prefix", code: "STATE_ERROR.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED", wantAgreement: false},
		{name: "similar forbidden prefix", code: "FORBIDDENISH.PLA_NOT_VALID", wantAgreement: false},
		{name: "generic forbidden", code: "FORBIDDEN_ERROR", wantAgreement: false},
		{name: "unauthorized", code: "NOT_AUTHORIZED", wantAgreement: false},
		{name: "empty", code: "", wantAgreement: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remediation := remediationForAPIError(tt.code)
			if tt.wantAgreement && remediation == "" {
				t.Fatalf("expected agreement remediation for code %q, got none", tt.code)
			}
			if !tt.wantAgreement && remediation != "" {
				t.Fatalf("expected no remediation for code %q, got %q", tt.code, remediation)
			}
		})
	}
}

func TestIsRequiredAgreementErrorRecognizesWrappedAPIError(t *testing.T) {
	err := fmt.Errorf("validate request: %w", &APIError{
		Code:       "FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED",
		StatusCode: http.StatusForbidden,
	})
	if !IsRequiredAgreementError(err) {
		t.Fatalf("IsRequiredAgreementError(%v) = false", err)
	}
	if IsRequiredAgreementError(&APIError{Code: "FORBIDDEN.MISSING_ROLE", StatusCode: http.StatusForbidden}) {
		t.Fatal("unrelated forbidden error was classified as required agreement")
	}
}

// Remediation is additive: it must not displace Apple's message or the
// associated errors that already ride along with it.
func TestAPIErrorMessageKeepsRemediationSeparate(t *testing.T) {
	apiErr := &APIError{
		Title:       "A required agreement is missing or has expired",
		Detail:      "This request requires an in-effect agreement.",
		StatusCode:  http.StatusForbidden,
		Remediation: "Accept the agreement.",
		AssociatedErrors: map[string][]APIAssociatedError{
			"appStoreVersions": {{Code: "ENTITY_ERROR", Detail: "Blocked."}},
		},
	}

	message := apiErr.Error()
	wantOrder := []string{
		"A required agreement is missing or has expired: This request requires an in-effect agreement.",
		"Associated errors for appStoreVersions:",
		"Accept the agreement.",
	}
	offset := 0
	for _, section := range wantOrder {
		index := strings.Index(message[offset:], section)
		if index < 0 {
			t.Fatalf("expected %q in message after offset %d, got %q", section, offset, message)
		}
		offset += index + len(section)
	}
}

func TestAPIErrorMessageUnchangedWithoutRemediation(t *testing.T) {
	apiErr := &APIError{
		Title:      "This request is forbidden for security reasons",
		Detail:     "The API key in use does not allow this request.",
		StatusCode: http.StatusForbidden,
	}

	want := "This request is forbidden for security reasons: The API key in use does not allow this request."
	if got := apiErr.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}
