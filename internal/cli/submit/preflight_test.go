package submit

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
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

func TestPreflightResult_TallyCounts(t *testing.T) {
	result := &preflightResult{
		Checks: []checkResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: false},
			{Name: "c", Passed: true},
			{Name: "d", Passed: false},
			{Name: "info", Passed: true, Advisory: true},
		},
	}
	tallyCounts(result)
	if result.PassCount != 3 {
		t.Fatalf("expected 3 passes including advisories, got %d", result.PassCount)
	}
	if result.FailCount != 2 {
		t.Fatalf("expected 2 failures, got %d", result.FailCount)
	}
}

func TestPreflightResult_AllPass(t *testing.T) {
	result := &preflightResult{
		Checks: []checkResult{
			{Name: "a", Passed: true},
			{Name: "b", Passed: true},
			{Name: "info", Passed: true, Advisory: true},
		},
	}
	tallyCounts(result)
	if result.PassCount != 3 {
		t.Fatalf("expected 3 passes including advisories, got %d", result.PassCount)
	}
	if result.FailCount != 0 {
		t.Fatalf("expected 0 failures, got %d", result.FailCount)
	}
}

func TestPrivacyPublishStateAdvisoryCheck_SetsPassedWhenPresent(t *testing.T) {
	check, ok := privacyPublishStateAdvisoryCheck("app-1")
	if !ok {
		t.Fatal("expected advisory check for non-empty app ID")
	}
	if !check.Advisory {
		t.Fatalf("expected advisory flag, got %+v", check)
	}
	if !check.Passed {
		t.Fatalf("expected advisory check to serialize as passed, got %+v", check)
	}
}

func TestPrivacyPublishStateAdvisoryCheck_SkipsBlankAppID(t *testing.T) {
	if _, ok := privacyPublishStateAdvisoryCheck(" \t "); ok {
		t.Fatal("expected blank app ID to skip advisory check")
	}
}

func TestPreflightResultFromReport_MapsContentRightsCheckName(t *testing.T) {
	result := preflightResultFromReport("app-123", "1.0", validation.Report{
		Platform: "IOS",
		Checks: []validation.CheckResult{
			{
				ID:       "content_rights.missing",
				Severity: validation.SeverityError,
				Message:  "content rights declaration is not set",
			},
		},
	})

	if len(result.Checks) != 1 {
		t.Fatalf("expected one check, got %+v", result.Checks)
	}
	if result.Checks[0].Name != "Content rights" {
		t.Fatalf("expected content rights label, got %+v", result.Checks[0])
	}
}

func TestPreflightTextOutput(t *testing.T) {
	var buf bytes.Buffer
	printPreflightText(&buf, &preflightResult{
		AppID:    "123",
		Version:  "1.0",
		Platform: "IOS",
		Checks: []checkResult{
			{Name: "Version exists", Passed: true, Message: "Version 1.0 found"},
			{Name: "Build attached", Passed: false, Message: "No build", Hint: "Attach a build with `asc release stage ...`, or upload and submit with `asc publish appstore ... --submit`"},
			{Name: "App Privacy", Advisory: true, Message: "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission", Hint: "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/123/appPrivacy"},
		},
		PassCount: 1,
		FailCount: 1,
	})
	if !strings.Contains(buf.String(), "Preflight check for app 123 v1.0 (IOS)") {
		t.Fatalf("expected header in text output, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), "App Privacy publish state is not verifiable via the public App Store Connect API") {
		t.Fatalf("expected advisory in text output, got %q", buf.String())
	}
}

func TestPreflightTextOutput_AdvisoryOnlyDoesNotClaimReadyToSubmit(t *testing.T) {
	var buf bytes.Buffer
	printPreflightText(&buf, &preflightResult{
		AppID:    "123",
		Version:  "1.0",
		Platform: "IOS",
		Checks: []checkResult{
			{
				Name:     "App Privacy",
				Passed:   true,
				Advisory: true,
				Message:  "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Hint:     "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/123/appPrivacy",
			},
		},
	})

	output := buf.String()
	if strings.Contains(output, "Ready to submit") {
		t.Fatalf("did not expect advisory-only result to claim readiness, got %q", output)
	}
	if !strings.Contains(output, "Result: Required checks passed, but 1 advisory should be reviewed before submitting.") {
		t.Fatalf("expected advisory summary in text output, got %q", output)
	}
}
