package submit

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func TestSubmitCommandShape(t *testing.T) {
	cmd := SubmitCommand()
	if cmd == nil {
		t.Fatal("expected submit command")
	}
	if cmd.Name != "submit" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 4 {
		t.Fatalf("expected 4 submit subcommands, got %d", len(cmd.Subcommands))
	}
}

func TestSubmitCreateCommand_MissingConfirm(t *testing.T) {
	cmd := SubmitCreateCommand()
	if err := cmd.FlagSet.Parse([]string{"--build", "BUILD_ID", "--version", "1.0.0", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestSubmitCreateCommand_MutuallyExclusiveVersionFlags(t *testing.T) {
	cmd := SubmitCreateCommand()
	args := []string{
		"--confirm",
		"--build", "BUILD_ID",
		"--app", "123",
		"--version", "1.0.0",
		"--version-id", "VERSION_ID",
	}
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for mutually exclusive flags, got %v", err)
	}
}

func TestRunSubmitCreateReadinessPreflight_PrintsNonBlockingPricingAndAvailabilityWarnings(t *testing.T) {
	tests := []struct {
		name   string
		checks []validation.CheckResult
	}{
		{
			name: "pricing unverified",
			checks: []validation.CheckResult{{
				ID:       "pricing.schedule.unverified",
				Severity: validation.SeverityWarning,
				Message:  "could not verify app price schedule",
			}},
		},
		{
			name: "availability unverified",
			checks: []validation.CheckResult{{
				ID:       "availability.unverified",
				Severity: validation.SeverityWarning,
				Message:  "could not verify app availability",
			}},
		},
		{
			name: "both warnings",
			checks: []validation.CheckResult{
				{
					ID:       "pricing.schedule.unverified",
					Severity: validation.SeverityWarning,
					Message:  "could not verify app price schedule",
				},
				{
					ID:       "availability.unverified",
					Severity: validation.SeverityWarning,
					Message:  "could not verify app availability",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalBuilder := submitReadinessReportBuilder
			t.Cleanup(func() {
				submitReadinessReportBuilder = originalBuilder
			})

			var gotOpts validatecli.ReadinessOptions
			submitReadinessReportBuilder = func(ctx context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
				gotOpts = opts
				return validation.Report{
					Summary: validation.Summary{Warnings: len(tt.checks)},
					Checks:  tt.checks,
				}, nil
			}

			var err error
			stderr := captureSubmitStderr(t, func() {
				err = runSubmitCreateReadinessPreflight(context.Background(), nil, "app-123", "version-123", "IOS", "")
			})
			if err != nil {
				t.Fatalf("expected warning-only readiness report to pass, got %v", err)
			}
			if gotOpts.AppID != "app-123" || gotOpts.VersionID != "version-123" || gotOpts.Platform != "IOS" {
				t.Fatalf("unexpected readiness options: %+v", gotOpts)
			}
			for _, check := range tt.checks {
				want := fmt.Sprintf("Warning: %s: %s", submitCreateReadinessCheckLabel(check), check.Message)
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected warning %q, got %q", want, stderr)
				}
			}
		})
	}
}

func TestRunSubmitCreateReadinessPreflight_PrintsPrivacyPublishStateAdvisory(t *testing.T) {
	originalBuilder := submitReadinessReportBuilder
	t.Cleanup(func() {
		submitReadinessReportBuilder = originalBuilder
	})

	submitReadinessReportBuilder = func(ctx context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			Summary: validation.Summary{Infos: 1},
			Checks: []validation.CheckResult{
				{
					ID:           "privacy.publish_state.unverified",
					Severity:     validation.SeverityInfo,
					ResourceType: "appPrivacy",
					ResourceID:   "app-123",
					Message:      "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
					Remediation:  "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/app-123/appPrivacy",
				},
			},
		}, nil
	}

	var err error
	stderr := captureSubmitStderr(t, func() {
		err = runSubmitCreateReadinessPreflight(context.Background(), nil, "app-123", "version-123", "IOS", "")
	})
	if err != nil {
		t.Fatalf("expected advisory-only readiness report to pass, got %v", err)
	}
	if !strings.Contains(stderr, "Advisory: App Privacy: App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission") {
		t.Fatalf("expected App Privacy advisory in stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "Hint: Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/app-123/appPrivacy") {
		t.Fatalf("expected App Privacy hint in stderr, got %q", stderr)
	}
	if strings.Contains(strings.ToLower(stderr), "asc web") {
		t.Fatalf("did not expect private/web command references in stderr, got %q", stderr)
	}
}

func TestRunSubmitCreateReadinessPreflight_DoesNotSkipOtherBlockingChecks(t *testing.T) {
	originalBuilder := submitReadinessReportBuilder
	t.Cleanup(func() {
		submitReadinessReportBuilder = originalBuilder
	})

	submitReadinessReportBuilder = func(ctx context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			Summary: validation.Summary{Errors: 1, Warnings: 1, Blocking: 1},
			Checks: []validation.CheckResult{
				{
					ID:       "pricing.schedule.unverified",
					Severity: validation.SeverityWarning,
					Message:  "could not verify app price schedule",
				},
				{
					ID:       "screenshots.required.any",
					Severity: validation.SeverityError,
					Message:  "at least one required screenshot set is missing",
				},
			},
		}, nil
	}

	var err error
	stderr := captureSubmitStderr(t, func() {
		err = runSubmitCreateReadinessPreflight(context.Background(), nil, "app-123", "version-123", "IOS", "")
	})
	if err == nil {
		t.Fatal("expected blocking readiness issues to fail submit preflight")
	}
	if !strings.Contains(err.Error(), "submit preflight failed") {
		t.Fatalf("expected submit preflight failure, got %v", err)
	}
	if !strings.Contains(stderr, "Screenshots: at least one required screenshot set is missing") {
		t.Fatalf("expected blocking screenshot issue in stderr, got %q", stderr)
	}
}

func TestRunSubmitCreateReadinessPreflight_PropagatesUnexpectedFetchErrors(t *testing.T) {
	originalBuilder := submitReadinessReportBuilder
	t.Cleanup(func() {
		submitReadinessReportBuilder = originalBuilder
	})

	submitReadinessReportBuilder = func(ctx context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{}, fmt.Errorf("failed to fetch app price schedule: %w", &asc.APIError{
			Code:       "INTERNAL_ERROR",
			Title:      "Server Error",
			StatusCode: http.StatusInternalServerError,
		})
	}

	err := runSubmitCreateReadinessPreflight(context.Background(), nil, "app-123", "version-123", "IOS", "")
	if err == nil {
		t.Fatal("expected unexpected readiness fetch error to propagate")
	}
	if !strings.Contains(err.Error(), "failed to run readiness preflight") {
		t.Fatalf("expected wrapped readiness preflight error, got %v", err)
	}
}

func TestSubmitStatusCommandValidation(t *testing.T) {
	t.Run("missing id and version-id", func(t *testing.T) {
		cmd := SubmitStatusCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("mutually exclusive id and version-id", func(t *testing.T) {
		cmd := SubmitStatusCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "S", "--version-id", "V"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestSubmitStatusCommand_ByIDUsesReviewSubmissionEndpoint(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {
						"state": "IN_REVIEW",
						"submittedDate": "2026-03-16T10:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {
								"type": "appStoreVersions",
								"id": "version-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "review-submission-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "review-submission-123" {
		t.Fatalf("expected review submission ID, got %q", result.ID)
	}
	if result.VersionID != "version-123" {
		t.Fatalf("expected version ID version-123, got %q", result.VersionID)
	}
	if result.VersionString != "1.2.3" {
		t.Fatalf("expected version string 1.2.3, got %q", result.VersionString)
	}
	if result.Platform != "IOS" {
		t.Fatalf("expected platform IOS, got %q", result.Platform)
	}
	if result.State != "IN_REVIEW" {
		t.Fatalf("expected review submission state IN_REVIEW, got %q", result.State)
	}
	if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T10:00:00Z" {
		t.Fatalf("expected submittedDate to be surfaced as createdDate, got %+v", result.CreatedDate)
	}

	wantRequests := []string{
		"GET /v1/reviewSubmissions/review-submission-123",
		"GET /v1/appStoreVersions/version-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByIDIgnoresInaccessibleItemLookup(t *testing.T) {
	setupSubmitAuth(t)

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "forbidden items lookup",
			statusCode: http.StatusForbidden,
			body:       `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`,
		},
		{
			name:       "missing items lookup",
			statusCode: http.StatusNotFound,
			body:       `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			requests := make([]string, 0, 3)
			http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests = append(requests, req.Method+" "+req.URL.RequestURI())

				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
					return submitJSONResponse(http.StatusOK, `{
						"data": {
							"type": "reviewSubmissions",
							"id": "review-submission-123",
							"attributes": {
								"state": "IN_REVIEW",
								"submittedDate": "2026-03-16T10:00:00Z"
							}
						}
					}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123/items":
					return submitJSONResponse(test.statusCode, test.body)
				default:
					return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
			})

			cmd := SubmitStatusCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{"--id", "review-submission-123", "--output", "json"}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			stdout, err := captureSubmitCommandOutput(t, func() error {
				return cmd.Exec(context.Background(), nil)
			})
			if err != nil {
				t.Fatalf("expected command to succeed, got %v", err)
			}

			var result asc.AppStoreVersionSubmissionStatusResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
			}
			if result.ID != "review-submission-123" {
				t.Fatalf("expected review submission ID, got %q", result.ID)
			}
			if result.State != "IN_REVIEW" {
				t.Fatalf("expected review submission state IN_REVIEW, got %q", result.State)
			}
			if result.VersionID != "" {
				t.Fatalf("expected empty version ID when items lookup is inaccessible, got %q", result.VersionID)
			}
			if result.VersionString != "" {
				t.Fatalf("expected empty version string when items lookup is inaccessible, got %q", result.VersionString)
			}
			if result.Platform != "" {
				t.Fatalf("expected empty platform when items lookup is inaccessible, got %q", result.Platform)
			}
			if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T10:00:00Z" {
				t.Fatalf("expected submittedDate to remain available, got %+v", result.CreatedDate)
			}

			wantRequests := []string{
				"GET /v1/reviewSubmissions/review-submission-123",
				"GET /v1/reviewSubmissions/review-submission-123/items?fields%5BreviewSubmissionItems%5D=appStoreVersion&limit=200",
			}
			if !reflect.DeepEqual(requests, wantRequests) {
				t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
			}
		})
	}
}

func TestSubmitStatusCommand_ByVersionIDUsesReviewSubmissionsForCurrentSubmission(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "app-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			if got := req.URL.Query().Get("include"); got != "appStoreVersionForReview" {
				return nil, fmt.Errorf("expected include=appStoreVersionForReview, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				return nil, fmt.Errorf("expected limit=200, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "review-submission-other",
						"attributes": {
							"state": "READY_FOR_REVIEW"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-other"
								}
							}
						}
					},
					{
						"type": "reviewSubmissions",
						"id": "review-submission-123",
						"attributes": {
							"state": "IN_REVIEW",
							"submittedDate": "2026-03-16T11:00:00Z"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "review-submission-123" {
		t.Fatalf("expected review submission ID review-submission-123, got %q", result.ID)
	}
	if result.VersionID != "version-123" {
		t.Fatalf("expected version ID version-123, got %q", result.VersionID)
	}
	if result.State != "IN_REVIEW" {
		t.Fatalf("expected review submission state IN_REVIEW, got %q", result.State)
	}
	if result.VersionString != "1.2.3" || result.Platform != "IOS" {
		t.Fatalf("unexpected version details: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByVersionIDFallsBackToLegacyRelationshipAndVersionState(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "app-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersionSubmissions",
					"id": "legacy-submission-123",
					"attributes": {
						"createdDate": "2026-03-16T09:00:00Z"
					},
					"relationships": {
						"appStoreVersion": {
							"data": {
								"type": "appStoreVersions",
								"id": "version-123"
							}
						}
					}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "legacy-submission-123" {
		t.Fatalf("expected legacy submission ID fallback, got %q", result.ID)
	}
	if result.State != "WAITING_FOR_REVIEW" {
		t.Fatalf("expected version state fallback WAITING_FOR_REVIEW, got %q", result.State)
	}
	if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T09:00:00Z" {
		t.Fatalf("expected legacy created date, got %+v", result.CreatedDate)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByVersionIDFallsBackWhenReviewSubmissionListingIsForbidden(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {
						"versionString": "1.2.3",
						"platform": "IOS",
						"appStoreState": "WAITING_FOR_REVIEW"
					},
					"relationships": {
						"app": {
							"data": {
								"type": "apps",
								"id": "app-123"
							}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersionSubmissions",
					"id": "legacy-submission-123",
					"attributes": {
						"createdDate": "2026-03-16T09:00:00Z"
					},
					"relationships": {
						"appStoreVersion": {
							"data": {
								"type": "appStoreVersions",
								"id": "version-123"
							}
						}
					}
				}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionStatusResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "legacy-submission-123" {
		t.Fatalf("expected legacy submission ID fallback, got %q", result.ID)
	}
	if result.State != "WAITING_FOR_REVIEW" {
		t.Fatalf("expected version state fallback WAITING_FOR_REVIEW, got %q", result.State)
	}
	if result.CreatedDate == nil || *result.CreatedDate != "2026-03-16T09:00:00Z" {
		t.Fatalf("expected legacy created date, got %+v", result.CreatedDate)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitStatusCommand_ByIDNotFoundSuggestsVersionIDFallback(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/missing-submission" {
			return submitJSONResponse(http.StatusNotFound, `{
				"errors": [{
					"status": "404",
					"code": "NOT_FOUND",
					"title": "Not Found"
				}]
			}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitStatusCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "missing-submission"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	_, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `retry with --version-id to inspect the App Store version state`) {
		t.Fatalf("expected fallback hint in error, got %v", err)
	}
}

func TestSubmitCancelCommandValidation(t *testing.T) {
	t.Run("missing confirm", func(t *testing.T) {
		cmd := SubmitCancelCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "S"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("mutually exclusive id and version-id", func(t *testing.T) {
		cmd := SubmitCancelCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm", "--id", "S", "--version-id", "V"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestCommandWrapper(t *testing.T) {
	if got := SubmitCommand(); got == nil {
		t.Fatal("expected Command wrapper to return submit command")
	}
}

type submitRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn submitRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func setupSubmitAuth(t *testing.T) {
	t.Helper()

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeSubmitECDSAPEM(t, keyPath)

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_APP_ID", "")
}

func writeSubmitECDSAPEM(t *testing.T, path string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key error: %v", err)
	}
	data := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if data == nil {
		t.Fatal("failed to encode PEM")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write key file error: %v", err)
	}
}

func submitJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestSubmitCancelCommand_ByIDUsesReviewSubmissionEndpoint(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 1)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)

		if req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123" {
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-123"}}`)
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "review-submission-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	wantRequests := []string{"PATCH /v1/reviewSubmissions/review-submission-123"}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDAttemptsReviewCancelThenFallsBackToLegacyDelete(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 6)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)

		switch {
		// Modern: resolve app ID from version — return version with app relationship
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123" && req.URL.Query().Get("include") == "app":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions", "id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		// Modern: find review submissions for app — return empty (no active submission)
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/reviewSubmissions"):
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		// Legacy: version submission lookup
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		// Modern cancel attempt (fails — not a modern submission)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		// Legacy delete (succeeds)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNoContent, "")
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	// Should try modern API first, then fall back to legacy
	foundLegacyGet := false
	foundLegacyDelete := false
	for _, r := range requests {
		if r == "GET /v1/appStoreVersions/version-123/appStoreVersionSubmission" {
			foundLegacyGet = true
		}
		if r == "DELETE /v1/appStoreVersionSubmissions/legacy-submission-123" {
			foundLegacyDelete = true
		}
	}
	if !foundLegacyGet || !foundLegacyDelete {
		t.Fatalf("expected legacy fallback flow; requests: %v", requests)
	}
}

func TestSubmitCancelCommand_ByVersionIDIgnoresStaleEnvAppIDForModernLookup(t *testing.T) {
	setupSubmitAuth(t)
	t.Setenv("ASC_APP_ID", "wrong-app")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {"state": "READY_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-123"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/wrong-app/reviewSubmissions":
			t.Fatalf("modern lookup should not use stale ASC_APP_ID; request: %s", req.URL.RequestURI())
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			t.Fatalf("legacy lookup should not be used when modern lookup succeeds")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/review-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDExplicitAppMismatchFailsFast(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-actual"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-actual/reviewSubmissions":
			t.Fatalf("did not expect lookup against version-owned app when explicit --app mismatches")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-expected/reviewSubmissions":
			t.Fatalf("did not expect modern lookup to continue after explicit --app mismatch")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback after explicit --app mismatch")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--app", "app-expected", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected explicit --app mismatch to fail, got nil")
	}
	if !strings.Contains(err.Error(), `version "version-123" belongs to app "app-actual", not "app-expected"`) {
		t.Fatalf("expected mismatch error, got %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDVersionLookupErrorFallsBackToLegacy(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusInternalServerError, `{
				"errors": [{
					"status": "500",
					"code": "INTERNAL_ERROR",
					"title": "Server Error"
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNoContent, "")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-lookup-error", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err == nil {
		var result asc.AppStoreVersionSubmissionCancelResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
		}
		if result.ID != "legacy-submission-123" || !result.Cancelled {
			t.Fatalf("unexpected result: %+v", result)
		}
	} else {
		t.Fatalf("expected legacy fallback success, got %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-lookup-error?include=app",
		"GET /v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-123",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDVersionLookupErrorFallsBackToExplicitApp(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusInternalServerError, `{
				"errors": [{
					"status": "500",
					"code": "INTERNAL_ERROR",
					"title": "Server Error"
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {"state": "READY_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-lookup-error"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-submission-123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-lookup-error/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback when explicit --app modern lookup succeeds")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-lookup-error", "--app", "app-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-lookup-error?include=app",
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/review-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDModernLookupErrorFallsBackToLegacy(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 5)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusInternalServerError, `{
				"errors": [{
					"status": "500",
					"code": "INTERNAL_ERROR",
					"title": "Server Error"
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNoContent, "")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err == nil {
		var result asc.AppStoreVersionSubmissionCancelResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
		}
		if result.ID != "legacy-submission-123" || !result.Cancelled {
			t.Fatalf("unexpected result: %+v", result)
		}
	} else {
		t.Fatalf("expected legacy fallback success, got %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-123",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDModernLookupTimeoutRefreshesLegacyFallbackContext(t *testing.T) {
	setupSubmitAuth(t)
	t.Setenv("ASC_TIMEOUT", "40ms")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 5)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			<-req.Context().Done()
			return nil, req.Context().Err()
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-submission-123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appStoreVersionSubmissions/legacy-submission-123":
			return submitJSONResponse(http.StatusNoContent, "")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected legacy fallback success after timeout refresh, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "legacy-submission-123" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
		"PATCH /v1/reviewSubmissions/legacy-submission-123",
		"DELETE /v1/appStoreVersionSubmissions/legacy-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDTreatsCancelingModernSubmissionAsSuccess(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 2)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {
						"state": "CANCELING",
						"submittedDate": "2026-03-15T11:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-123"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			t.Fatalf("did not expect cancel attempt for already CANCELING submission")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback for matched CANCELING submission")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "review-submission-123" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByVersionIDModernConflictSurfacesModernError(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {"state": "WAITING_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-123"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback after matched modern submission conflict")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `submission review-submission-123 is no longer cancellable`) {
		t.Fatalf("expected modern non-cancellable error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Resource is not in cancellable state") {
		t.Fatalf("expected original ASC conflict detail to be preserved, got %v", err)
	}
}

func TestSubmitCancelCommand_ByVersionIDModernConflictRefreshesCancelingStateToSuccess(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {"state": "WAITING_FOR_REVIEW"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-123"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusConflict, `{"errors":[{"status":"409","code":"CONFLICT","title":"Resource state is invalid.","detail":"Resource is not in cancellable state"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "review-submission-123",
					"attributes": {"state": "CANCELING"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-123"}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			t.Fatalf("did not expect legacy fallback after refreshed CANCELING state")
		}

		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm", "--output", "json"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, err := captureSubmitCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if err != nil {
		t.Fatalf("expected command to succeed after refreshed CANCELING state, got %v", err)
	}

	var result asc.AppStoreVersionSubmissionCancelResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.ID != "review-submission-123" || !result.Cancelled {
		t.Fatalf("unexpected result: %+v", result)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/review-submission-123",
		"GET /v1/reviewSubmissions/review-submission-123",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestSubmitCancelCommand_ByIDNotFoundReportsReviewSubmissionError(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/missing-review-id" {
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--id", "missing-review-id", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `no review submission found for ID "missing-review-id"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitCancelCommand_ByVersionIDNotFoundReportsLegacySubmissionError(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		// Modern: resolve app ID from version — return 404 (no app relationship)
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/missing-version":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		// Legacy: version submission lookup — return 404
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/missing-version/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "missing-version", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `no active submission found for version "missing-version"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitCancelCommand_ByVersionIDLegacyForbiddenSurfacesError(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		path := req.URL.Path
		switch {
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/version-forbidden":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-forbidden",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		case req.Method == http.MethodGet && path == "/v1/appStoreVersions/version-forbidden/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusForbidden, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-forbidden", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if strings.Contains(err.Error(), `no active submission found for version "version-forbidden"`) {
		t.Fatalf("expected forbidden error to surface, got %v", err)
	}
	if !strings.Contains(strings.ToUpper(err.Error()), "FORBIDDEN") {
		t.Fatalf("expected forbidden error to be preserved, got %v", err)
	}
}

func TestSubmitCancelCommand_ByVersionIDIgnoresHistoricalCompleteReviewSubmission(t *testing.T) {
	setupSubmitAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make([]string, 0, 3)
	http.DefaultTransport = submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123":
			if got := req.URL.Query().Get("include"); got != "app" {
				return nil, fmt.Errorf("expected include=app, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "appStoreVersions",
					"id": "version-123",
					"attributes": {"platform": "IOS", "versionString": "1.0"},
					"relationships": {"app": {"data": {"type": "apps", "id": "app-1"}}}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "historical-submission",
					"attributes": {
						"state": "COMPLETE",
						"submittedDate": "2026-03-15T11:00:00Z"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-123"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-123/appStoreVersionSubmission":
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	})

	cmd := SubmitCancelCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--version-id", "version-123", "--confirm"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), `no active submission found for version "version-123"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	wantRequests := []string{
		"GET /v1/appStoreVersions/version-123?include=app",
		"GET /v1/apps/app-1/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/appStoreVersions/version-123/appStoreVersionSubmission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestIsAppUpdate_IncludesReleasedAndRemovedStatesFilters(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}

		query := req.URL.Query()
		if got := query.Get("filter[platform]"); got != "IOS" {
			return nil, fmt.Errorf("unexpected filter[platform]: got %q want %q", got, "IOS")
		}
		if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
			return nil, fmt.Errorf("unexpected filter[appStoreState]: %q", got)
		}
		if got := query.Get("limit"); got != "1" {
			return nil, fmt.Errorf("unexpected limit: got %q want %q", got, "1")
		}

		return submitJSONResponse(http.StatusOK, `{
			"data": [
				{
					"type": "appStoreVersions",
					"id": "version-1",
					"attributes": {}
				}
			]
		}`)
	}))

	isUpdate, err := isAppUpdate(context.Background(), client, "app-123", "IOS")
	if err != nil {
		t.Fatalf("isAppUpdate() error = %v", err)
	}
	if !isUpdate {
		t.Fatal("isAppUpdate() = false, want true when released/removed versions exist")
	}
}

func TestIsAppUpdate_EmptyPlatformSkipsPlatformFilter(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}

		query := req.URL.Query()
		if got := query.Get("filter[platform]"); got != "" {
			return nil, fmt.Errorf("did not expect filter[platform], got %q", got)
		}
		if got := query.Get("filter[appStoreState]"); got != "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" {
			return nil, fmt.Errorf("unexpected filter[appStoreState]: %q", got)
		}
		if got := query.Get("limit"); got != "1" {
			return nil, fmt.Errorf("unexpected limit: got %q want %q", got, "1")
		}

		return submitJSONResponse(http.StatusOK, `{"data":[]}`)
	}))

	isUpdate, err := isAppUpdate(context.Background(), client, "app-123", "   ")
	if err != nil {
		t.Fatalf("isAppUpdate() error = %v", err)
	}
	if isUpdate {
		t.Fatal("isAppUpdate() = true, want false when no versions are returned")
	}
}

func TestIsAppUpdate_PropagatesClientErrors(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("unexpected method: %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		return submitJSONResponse(http.StatusInternalServerError, `{
			"errors": [{
				"status": "500",
				"code": "INTERNAL_ERROR",
				"title": "Internal Server Error"
			}]
		}`)
	}))

	_, err := isAppUpdate(context.Background(), client, "app-123", "IOS")
	if err == nil {
		t.Fatal("isAppUpdate() error = nil, want non-nil")
	}
}

func TestFindReviewSubmissionForVersion_FallsBackToSubmissionItems(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "review-submission-123",
						"attributes": {
							"state": "READY_FOR_REVIEW"
						}
					}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/review-submission-123/items":
			if got := req.URL.Query().Get("fields[reviewSubmissionItems]"); got != "appStoreVersion" {
				return nil, fmt.Errorf("expected review submission items fields query, got %q", got)
			}
			if got := req.URL.Query().Get("limit"); got != "200" {
				return nil, fmt.Errorf("expected limit=200, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissionItems",
						"id": "item-1",
						"relationships": {
							"appStoreVersion": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err != nil {
		t.Fatalf("findReviewSubmissionForVersion() error: %v", err)
	}
	if submission == nil {
		t.Fatal("expected review submission match, got nil")
	}
	if submission.ID != "review-submission-123" {
		t.Fatalf("expected review submission ID review-submission-123, got %q", submission.ID)
	}

	wantRequests := []string{
		"GET /v1/apps/app-123/reviewSubmissions?include=appStoreVersionForReview&limit=200",
		"GET /v1/reviewSubmissions/review-submission-123/items?fields%5BreviewSubmissionItems%5D=appStoreVersion&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
}

func TestFindReviewSubmissionForVersion_ContinuesAfterPerSubmissionLookupErrors(t *testing.T) {
	requests := make([]string, 0, 3)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "broken-submission",
						"attributes": {
							"state": "COMPLETE"
						}
					},
					{
						"type": "reviewSubmissions",
						"id": "current-submission",
						"attributes": {
							"state": "WAITING_FOR_REVIEW"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/broken-submission/items":
			return submitJSONResponse(http.StatusForbidden, `{
				"errors": [{
					"status": "403",
					"code": "FORBIDDEN",
					"title": "Forbidden"
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err != nil {
		t.Fatalf("findReviewSubmissionForVersion() error: %v", err)
	}
	if submission == nil {
		t.Fatal("expected review submission match, got nil")
	}
	if submission.ID != "current-submission" {
		t.Fatalf("expected current-submission, got %q", submission.ID)
	}
}

func TestFindReviewSubmissionForVersion_PrefersCurrentSubmissionOverHistoricalMatch(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "historical-submission",
						"attributes": {
							"state": "COMPLETE",
							"submittedDate": "2026-03-15T11:00:00Z"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					},
					{
						"type": "reviewSubmissions",
						"id": "current-submission",
						"attributes": {
							"state": "IN_REVIEW",
							"submittedDate": "2026-03-16T11:00:00Z"
						},
						"relationships": {
							"appStoreVersionForReview": {
								"data": {
									"type": "appStoreVersions",
									"id": "version-123"
								}
							}
						}
					}
				]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err != nil {
		t.Fatalf("findReviewSubmissionForVersion() error: %v", err)
	}
	if submission == nil {
		t.Fatal("expected review submission match, got nil")
	}
	if submission.ID != "current-submission" {
		t.Fatalf("expected current-submission, got %q", submission.ID)
	}
}

func TestFindReviewSubmissionForVersion_PropagatesUnexpectedLookupErrors(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-123/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "broken-submission",
						"attributes": {
							"state": "WAITING_FOR_REVIEW"
						}
					}
				]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/broken-submission/items":
			return submitJSONResponse(http.StatusInternalServerError, `{
				"errors": [{
					"status": "500",
					"code": "INTERNAL_ERROR",
					"title": "Server Error"
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	submission, err := findReviewSubmissionForVersion(context.Background(), client, "app-123", "version-123")
	if err == nil {
		t.Fatal("expected lookup error, got nil")
	}
	if submission != nil {
		t.Fatalf("expected nil submission on unexpected lookup error, got %#v", submission)
	}
	if !strings.Contains(err.Error(), "Server Error") {
		t.Fatalf("expected server error to propagate, got %v", err)
	}
}

func TestExtractExistingSubmissionID(t *testing.T) {
	t.Run("returns submission ID from associated error", func(t *testing.T) {
		err := &asc.APIError{
			Code:   "ENTITY_ERROR",
			Title:  "The request entity is not valid.",
			Detail: "An attribute value is not valid.",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 883340862 was already added to another reviewSubmission with id fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a",
					},
				},
			},
		}
		got := extractExistingSubmissionID(err)
		want := "fb5dad8e-bd5f-4d96-bc2f-561cf74a7e7a"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("returns empty for non-APIError", func(t *testing.T) {
		err := fmt.Errorf("some random error")
		if got := extractExistingSubmissionID(err); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for APIError without matching detail", func(t *testing.T) {
		err := &asc.APIError{
			Code:   "ENTITY_ERROR",
			Title:  "Something else went wrong.",
			Detail: "Unrelated problem.",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{Code: "OTHER_ERROR", Detail: "something unrelated"},
				},
			},
		}
		if got := extractExistingSubmissionID(err); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("returns empty for APIError with no associated errors", func(t *testing.T) {
		err := &asc.APIError{
			Code:  "ENTITY_ERROR",
			Title: "Something went wrong.",
		}
		if got := extractExistingSubmissionID(err); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("works with wrapped APIError", func(t *testing.T) {
		apiErr := &asc.APIError{
			Code: "ENTITY_ERROR",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 999 was already added to another reviewSubmission with id aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
					},
				},
			},
		}
		wrapped := fmt.Errorf("add item failed: %w", apiErr)
		got := extractExistingSubmissionID(wrapped)
		want := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("handles uppercase UUID", func(t *testing.T) {
		err := &asc.APIError{
			Code: "ENTITY_ERROR",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 123 was Already Added to another reviewSubmission with id FB5DAD8E-BD5F-4D96-BC2F-561CF74A7E7A",
					},
				},
			},
		}
		got := extractExistingSubmissionID(err)
		want := "FB5DAD8E-BD5F-4D96-BC2F-561CF74A7E7A"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("handles non-UUID identifier", func(t *testing.T) {
		err := &asc.APIError{
			Code: "ENTITY_ERROR",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 123 was already added to another reviewSubmission with id some-opaque-id-12345",
					},
				},
			},
		}
		got := extractExistingSubmissionID(err)
		want := "some-opaque-id-12345"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})
}

func TestExtractSubmissionConflict(t *testing.T) {
	t.Run("captures still-in-progress submission ID", func(t *testing.T) {
		err := &asc.APIError{
			Code:   "ENTITY_ERROR",
			Title:  "The request entity is not valid.",
			Detail: "An attribute value is not valid.",
			AssociatedErrors: map[string][]asc.APIAssociatedError{
				"/v1/reviewSubmissionItems": {
					{
						Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
						Detail: "appStoreVersions with id 883340862 is already in another reviewSubmission with id active-submission-1 still in progress",
					},
				},
			},
		}

		conflict := extractSubmissionConflict(err)
		if conflict.Kind != submissionConflictStillInProgress {
			t.Fatalf("expected still-in-progress conflict kind, got %q", conflict.Kind)
		}
		if conflict.SubmissionID != "active-submission-1" {
			t.Fatalf("expected submission ID active-submission-1, got %q", conflict.SubmissionID)
		}
	})

	t.Run("returns no conflict without associated errors", func(t *testing.T) {
		conflict := extractSubmissionConflict(&asc.APIError{Code: "ENTITY_ERROR"})
		if conflict.Kind != submissionConflictNone {
			t.Fatalf("expected no conflict, got %q", conflict.Kind)
		}
		if conflict.SubmissionID != "" {
			t.Fatalf("expected empty submission ID, got %q", conflict.SubmissionID)
		}
	})
}

func TestAddVersionToSubmissionOrRecover_ExhaustsRetriesForRecentlyCanceledSubmission(t *testing.T) {
	const staleSubmissionID = "stale-1"

	attempts := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		attempts++
		return submitJSONResponse(http.StatusConflict, submitAlreadyAddedConflictBody(staleSubmissionID))
	}))

	originalDelays := submitCreateRecentlyCanceledRetryDelays
	submitCreateRecentlyCanceledRetryDelays = []time.Duration{time.Millisecond, time.Millisecond}
	t.Cleanup(func() {
		submitCreateRecentlyCanceledRetryDelays = originalDelays
	})

	resolvedID, err := addVersionToSubmissionOrRecover(
		context.Background(),
		client,
		"new-sub-1",
		"version-1",
		map[string]struct{}{staleSubmissionID: {}},
	)
	if err == nil {
		t.Fatal("expected retry exhaustion error")
	}
	if resolvedID != "" {
		t.Fatalf("expected empty resolved submission ID on failure, got %q", resolvedID)
	}
	if !strings.Contains(err.Error(), "still attached to recently canceled review submission stale-1 after 2 retries") {
		t.Fatalf("expected retry exhaustion message, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 add-item attempts (initial + 2 retries), got %d", attempts)
	}
}

func TestAddVersionToSubmissionOrRecover_RetriesStillInProgressConflictForRecentlyCanceledSubmission(t *testing.T) {
	const staleSubmissionID = "stale-1"

	attempts := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		attempts++
		if attempts < 3 {
			return submitJSONResponse(http.StatusConflict, submitStillInProgressConflictBody(staleSubmissionID))
		}
		return submitJSONResponse(http.StatusCreated, `{
			"data": {
				"type": "reviewSubmissionItems",
				"id": "item-123",
				"attributes": {
					"state": "READY_FOR_REVIEW"
				}
			}
		}`)
	}))

	originalDelays := submitCreateRecentlyCanceledRetryDelays
	submitCreateRecentlyCanceledRetryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() {
		submitCreateRecentlyCanceledRetryDelays = originalDelays
	})

	resolvedID, err := addVersionToSubmissionOrRecover(
		context.Background(),
		client,
		"new-sub-1",
		"version-1",
		map[string]struct{}{staleSubmissionID: {}},
	)
	if err != nil {
		t.Fatalf("expected retry recovery, got %v", err)
	}
	if resolvedID != "new-sub-1" {
		t.Fatalf("expected new submission ID after retry recovery, got %q", resolvedID)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 add-item attempts (2 conflicts + success), got %d", attempts)
	}
}

func TestAddVersionToSubmissionOrRecover_ReturnsContextErrorWhileWaitingForDetach(t *testing.T) {
	const staleSubmissionID = "stale-1"

	attempts := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/reviewSubmissionItems" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		attempts++
		return submitJSONResponse(http.StatusConflict, submitAlreadyAddedConflictBody(staleSubmissionID))
	}))

	originalDelays := submitCreateRecentlyCanceledRetryDelays
	submitCreateRecentlyCanceledRetryDelays = []time.Duration{100 * time.Millisecond}
	t.Cleanup(func() {
		submitCreateRecentlyCanceledRetryDelays = originalDelays
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	resolvedID, err := addVersionToSubmissionOrRecover(
		ctx,
		client,
		"new-sub-1",
		"version-1",
		map[string]struct{}{staleSubmissionID: {}},
	)
	if err == nil {
		t.Fatal("expected context cancellation while waiting to retry")
	}
	if resolvedID != "" {
		t.Fatalf("expected empty resolved submission ID on failure, got %q", resolvedID)
	}
	if !strings.Contains(err.Error(), "waiting for recently canceled review submission stale-1 to clear") {
		t.Fatalf("expected wait/cancellation error message, got: %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped context deadline exceeded error, got: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one add-item attempt before context cancellation, got %d", attempts)
	}
}

func TestCleanupEmptyReviewSubmissionWarnsOnUnexpectedCancelError(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/empty-sub-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return submitJSONResponse(http.StatusInternalServerError, `{
			"errors": [{
				"status": "500",
				"code": "INTERNAL_ERROR",
				"title": "Internal Server Error"
			}]
		}`)
	}))

	stderr := captureSubmitStderr(t, func() {
		cleanupEmptyReviewSubmission(context.Background(), client, "empty-sub-1")
	})
	if !strings.Contains(stderr, "Warning: failed to cancel empty submission empty-sub-1:") {
		t.Fatalf("expected cleanup warning, got %q", stderr)
	}
}

func TestCleanupEmptyReviewSubmissionIgnoresExpectedNonCancellableState(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/empty-sub-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return submitJSONResponse(http.StatusConflict, `{
			"errors": [{
				"status": "409",
				"code": "CONFLICT",
				"title": "Resource state is invalid.",
				"detail": "Resource is not in cancellable state"
			}]
		}`)
	}))

	stderr := captureSubmitStderr(t, func() {
		cleanupEmptyReviewSubmission(context.Background(), client, "empty-sub-1")
	})
	if stderr != "" {
		t.Fatalf("expected no cleanup warning for expected non-cancellable state, got %q", stderr)
	}
}

func TestCleanupEmptyReviewSubmissionWarnsOnGenericConflict(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPatch || req.URL.Path != "/v1/reviewSubmissions/empty-sub-1" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return submitJSONResponse(http.StatusConflict, `{
			"errors": [{
				"status": "409",
				"code": "CONFLICT",
				"title": "Conflict",
				"detail": "Another operation is already in progress"
			}]
		}`)
	}))

	stderr := captureSubmitStderr(t, func() {
		cleanupEmptyReviewSubmission(context.Background(), client, "empty-sub-1")
	})
	if !strings.Contains(stderr, "Warning: failed to cancel empty submission empty-sub-1:") {
		t.Fatalf("expected cleanup warning for generic conflict, got %q", stderr)
	}
}

func TestPrepareReviewSubmissionForCreateWarnsOnGenericConflict(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "stale-sub-1",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-sub-1":
			return submitJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "CONFLICT",
					"title": "Conflict",
					"detail": "Another operation is already in progress"
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1")
		if got.reuseSubmissionID != "" {
			t.Fatalf("expected no reusable submission, got %#v", got)
		}
		if got.canceledSubmissionIDs != nil {
			t.Fatalf("expected no canceled submissions, got %#v", got.canceledSubmissionIDs)
		}
	})
	if !strings.Contains(stderr, "Warning: failed to cancel stale submission stale-sub-1:") {
		t.Fatalf("expected stale submission warning for generic conflict, got %q", stderr)
	}
	if strings.Contains(stderr, "Skipped stale submission stale-sub-1") {
		t.Fatalf("did not expect stale submission skip message, got %q", stderr)
	}
}

func TestPrepareReviewSubmissionForCreateDoesNotReuseSubmissionThatBecameCanceling(t *testing.T) {
	requests := make([]string, 0, 3)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "stale-sub-1",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					}
				}]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-sub-1":
			return submitJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "CONFLICT",
					"title": "Resource state is invalid.",
					"detail": "Resource is not in cancellable state"
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/stale-sub-1":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "stale-sub-1",
					"attributes": {
						"state": "CANCELING",
						"platform": "IOS"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/stale-sub-1/items":
			t.Fatalf("did not expect item lookup once refreshed submission includes the version relationship")
			return nil, fmt.Errorf("unexpected request after fatal: %s %s", req.Method, req.URL.RequestURI())
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1")
		if got.reuseSubmissionID != "" {
			t.Fatalf("expected no reusable submission after refreshed CANCELING state, got %#v", got)
		}
		if got.canceledSubmissionIDs != nil {
			t.Fatalf("expected no canceled submissions, got %#v", got.canceledSubmissionIDs)
		}
	})

	wantRequests := []string{
		"GET /v1/apps/app-1/reviewSubmissions?filter%5Bplatform%5D=IOS&filter%5Bstate%5D=READY_FOR_REVIEW&include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/stale-sub-1",
		"GET /v1/reviewSubmissions/stale-sub-1",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
	if !strings.Contains(stderr, "Skipped stale submission stale-sub-1: already transitioned to a non-cancellable state") {
		t.Fatalf("expected stale submission skip message, got %q", stderr)
	}
	if strings.Contains(stderr, "Reusing existing review submission stale-sub-1") {
		t.Fatalf("did not expect reuse message, got %q", stderr)
	}
}

func TestPrepareReviewSubmissionForCreateCancelsMixedTargetVersionSubmission(t *testing.T) {
	requests := make([]string, 0, 4)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "mixed-submission",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/mixed-submission/items":
			if got := req.URL.Query().Get("limit"); got != "200" {
				return nil, fmt.Errorf("expected limit=200, got %q", got)
			}
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissionItems",
						"id": "version-item",
						"relationships": {
							"appStoreVersion": {
								"data": {"type": "appStoreVersions", "id": "version-1"}
							}
						}
					},
					{
						"type": "reviewSubmissionItems",
						"id": "other-item"
					}
				]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/mixed-submission":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"mixed-submission"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1")
		if got.reuseSubmissionID != "" {
			t.Fatalf("expected mixed-item submission not to be reused, got %#v", got)
		}
		if got.reuseSubmissionHasVersion {
			t.Fatalf("expected mixed-item submission not to be marked as reusable target version, got %#v", got)
		}
		if _, ok := got.canceledSubmissionIDs["mixed-submission"]; !ok {
			t.Fatalf("expected mixed-item submission to be canceled, got %#v", got.canceledSubmissionIDs)
		}
	})

	wantRequests := []string{
		"GET /v1/apps/app-1/reviewSubmissions?filter%5Bplatform%5D=IOS&filter%5Bstate%5D=READY_FOR_REVIEW&include=appStoreVersionForReview&limit=200",
		"GET /v1/reviewSubmissions/mixed-submission/items?limit=200",
		"PATCH /v1/reviewSubmissions/mixed-submission",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
	if strings.Contains(stderr, "Reusing existing review submission mixed-submission") {
		t.Fatalf("did not expect reuse message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Canceled stale review submission mixed-submission") {
		t.Fatalf("expected stale submission cancellation message, got %q", stderr)
	}
}

func TestPrepareReviewSubmissionForCreateTreatsEmptyItemsAsMissingVersion(t *testing.T) {
	requests := make([]string, 0, 3)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "empty-items-submission",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/empty-items-submission/items":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1")
		if got.reuseSubmissionID != "empty-items-submission" {
			t.Fatalf("expected empty-items submission to be reused, got %#v", got)
		}
		if got.reuseSubmissionHasVersion {
			t.Fatalf("expected empty-items submission to require re-attaching the version, got %#v", got)
		}
		if got.canceledSubmissionIDs != nil {
			t.Fatalf("did not expect canceled submissions when reusable empty submission exists, got %#v", got.canceledSubmissionIDs)
		}
	})

	wantRequests := []string{
		"GET /v1/apps/app-1/reviewSubmissions?filter%5Bplatform%5D=IOS&filter%5Bstate%5D=READY_FOR_REVIEW&include=appStoreVersionForReview&limit=200",
		"GET /v1/reviewSubmissions/empty-items-submission/items?limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
	if !strings.Contains(stderr, "Reusing existing review submission empty-items-submission") {
		t.Fatalf("expected reuse message for empty-items submission, got %q", stderr)
	}
}

func TestPrepareReviewSubmissionForCreatePreservesCanceledIDsWhenReusingAfterConflict(t *testing.T) {
	requests := make([]string, 0, 6)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [
					{
						"type": "reviewSubmissions",
						"id": "stale-sub-1",
						"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
					},
					{
						"type": "reviewSubmissions",
						"id": "reusable-empty-sub",
						"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
					}
				]
			}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/stale-sub-1":
			return submitJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"stale-sub-1"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/reusable-empty-sub":
			return submitJSONResponse(http.StatusConflict, `{
				"errors": [{
					"status": "409",
					"code": "CONFLICT",
					"title": "Resource state is invalid.",
					"detail": "Resource is not in cancellable state"
				}]
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/reusable-empty-sub":
			return submitJSONResponse(http.StatusOK, `{
				"data": {
					"type": "reviewSubmissions",
					"id": "reusable-empty-sub",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/reusable-empty-sub/items":
			return submitJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1")
		if got.reuseSubmissionID != "reusable-empty-sub" {
			t.Fatalf("expected reusable submission after cancel conflict, got %#v", got)
		}
		if got.reuseSubmissionHasVersion {
			t.Fatalf("expected empty reusable submission to require re-adding the version, got %#v", got)
		}
		if _, ok := got.canceledSubmissionIDs["stale-sub-1"]; !ok {
			t.Fatalf("expected earlier canceled submission ID to be preserved, got %#v", got.canceledSubmissionIDs)
		}
	})

	wantRequests := []string{
		"GET /v1/apps/app-1/reviewSubmissions?filter%5Bplatform%5D=IOS&filter%5Bstate%5D=READY_FOR_REVIEW&include=appStoreVersionForReview&limit=200",
		"PATCH /v1/reviewSubmissions/stale-sub-1",
		"PATCH /v1/reviewSubmissions/reusable-empty-sub",
		"GET /v1/reviewSubmissions/reusable-empty-sub",
		"GET /v1/reviewSubmissions/reusable-empty-sub/items?limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
	if !strings.Contains(stderr, "Canceled stale review submission stale-sub-1") {
		t.Fatalf("expected stale submission cancellation message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Reusing existing empty review submission reusable-empty-sub") {
		t.Fatalf("expected empty reusable submission message, got %q", stderr)
	}
}

func TestPrepareReviewSubmissionForCreatePaginatesReadyForReviewLookups(t *testing.T) {
	requests := make([]string, 0, 5)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions" && req.URL.Query().Get("cursor") == "":
			return submitJSONResponse(http.StatusOK, `{
				"data": [],
				"links": {
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=page-2"
				}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions" && req.URL.Query().Get("cursor") == "page-2":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "existing-submission",
					"attributes": {
						"state": "READY_FOR_REVIEW",
						"platform": "IOS"
					},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/existing-submission/items":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "version-item",
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}]
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1")
		if got.reuseSubmissionID != "existing-submission" {
			t.Fatalf("expected paginated submission to be reused, got %#v", got)
		}
		if !got.reuseSubmissionHasVersion {
			t.Fatalf("expected reused paginated submission to already carry the target version, got %#v", got)
		}
		if got.canceledSubmissionIDs != nil {
			t.Fatalf("did not expect canceled submissions when paginated reusable submission exists, got %#v", got.canceledSubmissionIDs)
		}
	})

	if !strings.Contains(strings.Join(requests, "\n"), "GET /v1/apps/app-1/reviewSubmissions?cursor=page-2") {
		t.Fatalf("expected second review-submissions page lookup, got %v", requests)
	}
	if !strings.Contains(stderr, "Reusing existing review submission existing-submission") {
		t.Fatalf("expected reuse message for paginated submission, got %q", stderr)
	}
}

func TestPrintSubmissionErrorHintsUsesExistingRunnableCommands(t *testing.T) {
	stderr := captureSubmitStderr(t, func() {
		printSubmissionErrorHints(errors.New("ageRatingDeclaration contentRightsDeclaration usesNonExemptEncryption appDataUsage primaryCategory"), submissionErrorHintContext{
			AppID: "app-1",
		})
	})

	for _, want := range []string{
		"Hint: Review current age rating: asc age-rating view --app app-1",
		"Hint: Review age-rating update flags: asc age-rating edit --help",
		"Hint: If your app does not use third-party content: asc apps update --id app-1 --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT",
		"Hint: If your app uses third-party content: asc apps update --id app-1 --content-rights USES_THIRD_PARTY_CONTENT",
		"Hint: Set Uses Non-Exempt Encryption for the attached build in App Store Connect, then retry submission.",
		"Hint: Complete App Privacy at: https://appstoreconnect.apple.com/apps/app-1/appPrivacy",
		"Hint: List available categories: asc categories list",
		"Hint: Review category update flags: asc app-setup categories set --help",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected hint %q in stderr, got %q", want, stderr)
		}
	}

	for _, unwanted := range []string{
		"--all-none",
		"content-rights set",
		"--uses-third-party-content",
		"builds update",
		"--primary SPORTS",
		"...",
		"|",
	} {
		if strings.Contains(stderr, unwanted) {
			t.Fatalf("did not expect %q in stderr, got %q", unwanted, stderr)
		}
	}
}

func TestPrintSubmissionErrorHintsUsesAssociatedErrorsForSubmissionStateConflicts(t *testing.T) {
	err := &asc.APIError{
		Code:   "ENTITY_ERROR",
		Title:  "The request entity is not valid.",
		Detail: "This resource cannot be reviewed, please check associated errors to see why.",
		AssociatedErrors: map[string][]asc.APIAssociatedError{
			"/v1/reviewSubmissionItems": {
				{
					Code:   "ENTITY_ERROR.RELATIONSHIP.INVALID",
					Detail: "appStoreVersions with id version-1 is already in another reviewSubmission with id active-submission-1 still in progress",
				},
			},
			"/v1/appStoreVersions/version-1": {
				{
					Code:   "STATE_ERROR.ENTITY_INVALID",
					Detail: "appStoreVersions with id version-1 is not ready to be submitted for review",
				},
			},
		},
	}

	stderr := captureSubmitStderr(t, func() {
		printSubmissionErrorHints(err, submissionErrorHintContext{
			AppID:         "app-1",
			Platform:      "MAC_OS",
			VersionID:     "version-1",
			VersionString: "1.0",
		})
	})

	for _, want := range []string{
		"Hint: Check the active submission: asc submit status --id active-submission-1",
		"Hint: Inspect the active submission payload: asc review submissions-get --id active-submission-1",
		"Hint: Re-run readiness validation: asc validate --app app-1 --version-id version-1",
		"Hint: Re-run submit preflight: asc submit preflight --app app-1 --version 1.0 --platform MAC_OS",
		"Hint: Review the release dashboard: asc status --app app-1 --include submission,appstore,review",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected hint %q in stderr, got %q", want, stderr)
		}
	}
}

func newSubmitTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeSubmitECDSAPEM(t, keyPath)

	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func captureSubmitStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stderr = stderrW
	defer func() {
		os.Stderr = oldStderr
	}()

	fn()

	if err := stderrW.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	data, err := io.ReadAll(stderrR)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if err := stderrR.Close(); err != nil {
		t.Fatalf("close stderr reader: %v", err)
	}
	return string(data)
}

func submitAlreadyAddedConflictBody(existingSubmissionID string) string {
	return fmt.Sprintf(`{
		"errors": [{
			"status": "409",
			"code": "ENTITY_ERROR",
			"title": "The request entity is not valid.",
			"detail": "An attribute value is not valid.",
			"meta": {
				"associatedErrors": {
					"/v1/reviewSubmissionItems": [{
						"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
						"detail": "appStoreVersions with id version-1 was already added to another reviewSubmission with id %s"
					}]
				}
			}
		}]
	}`, existingSubmissionID)
}

func submitStillInProgressConflictBody(existingSubmissionID string) string {
	return fmt.Sprintf(`{
		"errors": [{
			"status": "409",
			"code": "ENTITY_ERROR",
			"title": "The request entity is not valid.",
			"detail": "This resource cannot be reviewed, please check associated errors to see why.",
			"meta": {
				"associatedErrors": {
					"/v1/reviewSubmissionItems": [{
						"code": "ENTITY_ERROR.RELATIONSHIP.INVALID",
						"detail": "appStoreVersions with id version-1 is already in another reviewSubmission with id %s still in progress"
					}]
				}
			}
		}]
	}`, existingSubmissionID)
}

func captureSubmitCommandOutput(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		_, _ = io.Copy(io.Discard, rErr)
		_ = rErr.Close()
	}()

	runErr := fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, runErr
}
