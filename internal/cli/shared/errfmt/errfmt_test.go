package errfmt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestClassify_MissingAuth(t *testing.T) {
	err := errors.New("wrapped")
	err = wrap(err, shared.ErrMissingAuth)
	ce := Classify(err)
	if ce.Hint == "" {
		t.Fatalf("expected hint, got empty")
	}
}

func TestClassify_MissingWebSessionUsesItsOwnHint(t *testing.T) {
	err := fmt.Errorf("web review show failed: %w", &shared.MissingWebSessionError{
		Message: "no Apple web session is cached",
		Hint:    "Sign in with 'asc web auth login' in a terminal.",
	})

	ce := Classify(err)
	if ce.Message != "web review show failed: no Apple web session is cached" {
		t.Fatalf("Message = %q", ce.Message)
	}
	if ce.Hint != "Sign in with 'asc web auth login' in a terminal." {
		t.Fatalf("Hint = %q, want the web session hint", ce.Hint)
	}
	if got, want := FormatStderr(err), "Error: web review show failed: no Apple web session is cached\nHint: Sign in with 'asc web auth login' in a terminal.\n"; got != want {
		t.Fatalf("FormatStderr = %q, want %q", got, want)
	}
}

func TestClassify_Forbidden(t *testing.T) {
	apiErr := &asc.APIError{Code: "FORBIDDEN", Title: "Forbidden", Detail: "Nope"}
	ce := Classify(apiErr)
	if ce.Hint == "" {
		t.Fatalf("expected hint, got empty")
	}
}

func TestClassify_AgreementForbiddenDoesNotAddPermissionHint(t *testing.T) {
	apiErr := &asc.APIError{
		Code:        "FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED",
		StatusCode:  403,
		Remediation: "An Account Holder must accept the agreement.",
	}

	classified := Classify(apiErr)
	if classified.Hint != "" {
		t.Fatalf("expected account remediation to suppress generic permission hint, got %q", classified.Hint)
	}
	if !strings.Contains(classified.Message, apiErr.Remediation) {
		t.Fatalf("expected remediation in classified message, got %q", classified.Message)
	}
}

func TestClassify_Timeout(t *testing.T) {
	ce := Classify(context.DeadlineExceeded)
	if ce.Hint != "Increase the request timeout (e.g. set `ASC_TIMEOUT=90s`)." {
		t.Fatalf("expected request timeout hint, got %q", ce.Hint)
	}
}

func TestClassify_TimeoutUploadOperation(t *testing.T) {
	err := fmt.Errorf("builds upload: upload failed: upload operation 3: %w", context.DeadlineExceeded)

	ce := Classify(err)
	if ce.Hint != "Increase the upload timeout (e.g. set `ASC_UPLOAD_TIMEOUT=45m`)." {
		t.Fatalf("expected upload timeout hint, got %q", ce.Hint)
	}
}

func TestClassify_TimeoutReportDownloadNamesGoverningSetting(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "body copy uses the transfer budget",
			err:  fmt.Errorf("analytics download: failed to write report: %w", context.DeadlineExceeded),
			want: "Increase the download timeout, which `ASC_UPLOAD_TIMEOUT` sets (e.g. `ASC_UPLOAD_TIMEOUT=45m`).",
		},
		{
			name: "waiting for headers uses the request budget",
			err:  fmt.Errorf("analytics download: failed to download report: timed out after 30s awaiting response headers: %w", context.DeadlineExceeded),
			want: "Increase the request timeout (e.g. set `ASC_TIMEOUT=90s`).",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err).Hint; got != tt.want {
				t.Fatalf("hint = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassify_TimeoutBuildsUploadsListKeepsRequestHint(t *testing.T) {
	err := fmt.Errorf("builds uploads list: failed to fetch: %w", context.DeadlineExceeded)

	ce := Classify(err)
	if ce.Hint != "Increase the request timeout (e.g. set `ASC_TIMEOUT=90s`)." {
		t.Fatalf("expected request timeout hint, got %q", ce.Hint)
	}
}

func TestClassify_ServerErrorSuggestsSystemStatus(t *testing.T) {
	err := fmt.Errorf("apps list: %w", &asc.APIError{Code: "INTERNAL_ERROR", Title: "Unavailable", StatusCode: 503})

	ce := Classify(err)
	if ce.Hint != "Check Apple's service health with `asc system-status --service \"App Store Connect\"`." {
		t.Fatalf("expected system-status hint, got %q", ce.Hint)
	}
}

func TestClassify_NonASCServerErrorsDoNotSuggestSystemStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "Apple Ads", err: &appleads.APIError{StatusCode: 503}},
		{name: "StoreKit", err: &storekit.APIError{StatusCode: 503}},
		{name: "web session", err: &web.APIError{Status: 503}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if hint := Classify(test.err).Hint; hint != "" {
				t.Fatalf("unexpected cross-service hint: %q", hint)
			}
		})
	}
}

func TestClassify_PrivacyDataUsages(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantHit bool
	}{
		{
			name:    "associated error with appDataUsages path",
			err:     errors.New("submit create: failed to submit for review: Associated errors for /v1/appDataUsages/: missing required data"),
			wantHit: true,
		},
		{
			name:    "associated error with appDataUsagesPublications",
			err:     errors.New("submit create: /v1/appDataUsagesPublications/ not published"),
			wantHit: true,
		},
		{
			name:    "unrelated error",
			err:     errors.New("submit create: failed to attach build"),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ce := Classify(tt.err)
			if tt.wantHit && ce.Hint == "" {
				t.Fatalf("expected privacy hint, got empty")
			}
			if tt.wantHit && !strings.Contains(ce.Hint, "App Store Connect web UI") {
				t.Fatalf("expected web UI hint, got: %s", ce.Hint)
			}
			if !tt.wantHit && ce.Hint != "" {
				t.Fatalf("did not expect hint for unrelated error, got: %s", ce.Hint)
			}
		})
	}
}

func TestClassify_PrivacyDataUsages_TakesPrecedenceOverForbidden(t *testing.T) {
	err := &asc.APIError{
		Code:   "FORBIDDEN",
		Title:  "Forbidden",
		Detail: "Associated resources failed validation",
		AssociatedErrors: map[string][]asc.APIAssociatedError{
			"/v1/appDataUsages/": {
				{
					Code:   "ENTITY_ERROR.ATTRIBUTE.REQUIRED",
					Detail: "Missing required privacy answers",
				},
			},
		},
	}

	ce := Classify(err)
	if !strings.Contains(ce.Hint, "App Store Connect web UI") {
		t.Fatalf("expected privacy hint to take precedence, got: %s", ce.Hint)
	}
	if strings.Contains(ce.Hint, "role/permissions") {
		t.Fatalf("expected privacy hint, got permissions hint: %s", ce.Hint)
	}
}

// wrap creates an error that Is() matches target without altering the base string.
type isWrapper struct {
	target error
}

func (e isWrapper) Error() string { return "x" }
func (e isWrapper) Is(t error) bool {
	return t == e.target
}

func wrap(base error, target error) error {
	_ = base
	return isWrapper{target: target}
}

func TestClassify_AppNotFoundByBundleIDHintsNumericAppID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantHint string
	}{
		{
			name:     "bundle ID",
			id:       "com.example.app",
			wantHint: "App IDs are numeric. Find this app's ID with `asc apps list --bundle-id \"com.example.app\"` and pass that instead.",
		},
		{name: "numeric app ID", id: "123456789"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'apps' with id '` + test.id + `'"}]}`
			err := fmt.Errorf("versions list: %w", asc.ParseErrorWithStatus([]byte(body), 404))

			if got := Classify(err).Hint; got != test.wantHint {
				t.Fatalf("Hint = %q, want %q", got, test.wantHint)
			}
		})
	}
}
