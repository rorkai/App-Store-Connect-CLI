package submit

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubmissionLocalizationPreflightReadinessFailuresExposeValidationDiagnostics(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1s")

	tests := []struct {
		name              string
		prefix            string
		retryCommand      string
		localizationsBody string
		wantStderr        []string
	}{
		{
			name:              "no localizations through review submit",
			prefix:            "review submit",
			retryCommand:      "asc review submit",
			localizationsBody: `{"data":[]}`,
			wantStderr: []string{
				"Submit preflight failed: no app store version localizations found for this version.",
			},
		},
		{
			name:              "missing required fields through publish appstore",
			prefix:            "publish appstore",
			retryCommand:      "asc publish appstore --submit",
			localizationsBody: `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`,
			wantStderr: []string{
				"Submit preflight failed: submission-blocking localization fields are missing:",
				"  - en-US: description, keywords, supportUrl",
				"Fix these with `asc metadata push` or `asc apps info edit` before retrying `asc publish appstore --submit`.",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appUpdateChecked := false
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return submitJSONResponse(http.StatusOK, test.localizationsBody)
				case "/v1/apps/app-1/appStoreVersions":
					appUpdateChecked = true
					return submitJSONResponse(http.StatusOK, `{"data":[]}`)
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			}))

			stderr := captureSubmitStderr(t, func() {
				err := runSubmissionLocalizationPreflight(
					context.Background(),
					client,
					"app-1",
					"version-1",
					"IOS",
					0,
					test.prefix,
					test.retryCommand,
				)
				if err == nil {
					t.Fatal("expected submit preflight error")
				}
				if got, want := err.Error(), test.prefix+": submit preflight failed"; got != want {
					t.Fatalf("error = %q, want %q", got, want)
				}
				if !shared.IsValidationError(err) {
					t.Fatalf("error = %v, want validation marker", err)
				}
				diagnostic, ok := shared.DiagnosticFromError(err)
				if !ok {
					t.Fatalf("DiagnosticFromError(%v) did not find metadata", err)
				}
				if diagnostic.Code != shared.DiagnosticStateNotReady || diagnostic.Parameter != "" {
					t.Fatalf("diagnostic = %+v, want state_not_ready with empty parameter", diagnostic)
				}
			})

			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr = %q, want substring %q", stderr, want)
				}
			}
			if test.localizationsBody == `{"data":[]}` && appUpdateChecked {
				t.Fatal("did not expect app update lookup after an empty localization response")
			}
		})
	}
}

func TestSubmissionPreflightWrapPreservesReviewAndPublishValidationErrors(t *testing.T) {
	base := shared.WithDiagnostic(
		shared.NewValidationError(fmt.Errorf("submit preflight failed")),
		shared.DiagnosticStateNotReady,
		"",
	)

	for _, prefix := range []string{"review submit", "publish appstore"} {
		t.Run(prefix, func(t *testing.T) {
			err := submissionPreflightWrap(prefix, base)
			if got, want := err.Error(), prefix+": submit preflight failed"; got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
			if !shared.IsValidationError(err) {
				t.Fatalf("error = %v, want validation marker", err)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok || diagnostic.Code != shared.DiagnosticStateNotReady || diagnostic.Parameter != "" {
				t.Fatalf("diagnostic = %+v, ok=%t, want state_not_ready with empty parameter", diagnostic, ok)
			}
		})
	}
}
