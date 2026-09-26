package reviews

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type reviewRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn reviewRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func reviewJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func setupReviewTestAuth(t *testing.T) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("encode pem: nil")
	}

	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "KEY_ID")
	t.Setenv("ASC_ISSUER_ID", "ISSUER_ID")
	t.Setenv("ASC_PRIVATE_KEY", string(pemBytes))
}

func TestBuildReviewStatusResultMissingVersion(t *testing.T) {
	result := buildReviewStatusResult(reviewSnapshot{AppID: "123456789"})

	if result.ReviewState != "NO_VERSION" {
		t.Fatalf("expected NO_VERSION state, got %q", result.ReviewState)
	}
	if result.NextAction == "" {
		t.Fatal("expected next action for missing version")
	}
	if len(result.Blockers) != 1 {
		t.Fatalf("expected one blocker, got %d", len(result.Blockers))
	}
}

func TestBuildReviewStatusResultExplainsRemovedOnlyCompletedSubmission(t *testing.T) {
	snapshot := reviewSnapshot{
		AppID: "123456789",
		Version: &reviewVersionContext{
			ID:       "ver-1",
			Version:  "1.2.3",
			Platform: "IOS",
			State:    "DEVELOPER_REJECTED",
		},
		ReviewDetailID: "detail-1",
		LatestSubmission: &reviewSubmissionContext{
			ID:    "review-sub-1",
			State: "COMPLETE",
		},
		SubmissionItems: &reviewSubmissionItemsContext{
			TotalCount:   1,
			RemovedCount: 1,
			ActiveCount:  0,
		},
	}

	result := buildReviewStatusResult(snapshot)

	if result.ReviewState != "COMPLETE" {
		t.Fatalf("expected COMPLETE state, got %q", result.ReviewState)
	}
	if result.NextAction != "Create a fresh review submission for the current version." {
		t.Fatalf("expected stale submission next action, got %q", result.NextAction)
	}
	if !slices.Contains(result.Blockers, staleReviewSubmissionBlocker()) {
		t.Fatalf("expected stale submission blocker in %v", result.Blockers)
	}
}

func TestBuildReviewDoctorResultAddsSyntheticUnresolvedIssuesBlocker(t *testing.T) {
	snapshot := reviewSnapshot{
		AppID: "123456789",
		Version: &reviewVersionContext{
			ID:       "ver-1",
			Version:  "1.2.3",
			Platform: "IOS",
			State:    "WAITING_FOR_REVIEW",
		},
		LatestSubmission: &reviewSubmissionContext{
			ID:    "review-sub-1",
			State: "UNRESOLVED_ISSUES",
		},
	}
	report := validation.Report{
		Summary: validation.Summary{Errors: 1, Blocking: 1},
		Checks: []validation.CheckResult{
			{
				ID:          "review.details.missing",
				Severity:    validation.SeverityError,
				Message:     "Review details are missing",
				Remediation: "Create review details.",
			},
		},
	}

	result := buildReviewDoctorResult(snapshot, report)

	if len(result.BlockingChecks) < 2 {
		t.Fatalf("expected synthetic blocker plus readiness blocker, got %d", len(result.BlockingChecks))
	}
	if result.BlockingChecks[0].ID != "review.details.missing" && result.BlockingChecks[0].ID != "review.submission.unresolved_issues" {
		t.Fatalf("expected known blocker ID, got %q", result.BlockingChecks[0].ID)
	}
	if result.Summary.Blocking < 2 {
		t.Fatalf("expected blocking summary to include synthetic unresolved issues blocker, got %+v", result.Summary)
	}
	if result.NextAction == "" {
		t.Fatal("expected next action")
	}
}

func TestBuildReviewDoctorResultAddsRemovedItemsOnlyBlocker(t *testing.T) {
	snapshot := reviewSnapshot{
		AppID: "123456789",
		Version: &reviewVersionContext{
			ID:       "ver-1",
			Version:  "1.2.3",
			Platform: "IOS",
			State:    "DEVELOPER_REJECTED",
		},
		LatestSubmission: &reviewSubmissionContext{
			ID:    "review-sub-1",
			State: "COMPLETE",
		},
		SubmissionItems: &reviewSubmissionItemsContext{
			TotalCount:   1,
			RemovedCount: 1,
			ActiveCount:  0,
		},
	}

	result := buildReviewDoctorResult(snapshot, validation.Report{})

	if result.NextAction != "Create a fresh review submission for the current version." {
		t.Fatalf("expected stale submission next action, got %q", result.NextAction)
	}
	if len(result.BlockingChecks) != 1 {
		t.Fatalf("expected one synthetic blocker, got %d", len(result.BlockingChecks))
	}
	if result.BlockingChecks[0].ID != "review.submission.removed_items_only" {
		t.Fatalf("expected removed-items-only blocker, got %q", result.BlockingChecks[0].ID)
	}
	if result.Summary.Blocking != 1 || result.Summary.Errors != 1 {
		t.Fatalf("expected summary to include synthetic blocker, got %+v", result.Summary)
	}
}

func TestBuildReviewDoctorResultDisclosesWebOnlyCoverageWithoutChangingCheckSemantics(t *testing.T) {
	snapshot := reviewSnapshot{
		AppID: "123456789",
		Version: &reviewVersionContext{
			ID:       "ver-1",
			Version:  "1.2.3",
			Platform: "IOS",
			State:    "PREPARE_FOR_SUBMISSION",
		},
	}

	result := buildReviewDoctorResult(snapshot, validation.Report{})

	if result.Summary != (validation.Summary{}) {
		t.Fatalf("expected coverage warning not to change summary, got %+v", result.Summary)
	}
	if len(result.BlockingChecks) != 0 {
		t.Fatalf("expected coverage warning not to add blockers, got %+v", result.BlockingChecks)
	}
	if len(result.WarningChecks) != 0 {
		t.Fatalf("expected coverage warning not to add app-specific warning checks, got %+v", result.WarningChecks)
	}
	if result.NextAction != "No public-API submission blockers detected. Run `asc web apps declarations list --app \"123456789\"` before submission." {
		t.Fatalf("expected next action to disclose the public-API boundary, got %q", result.NextAction)
	}

	output, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal review doctor result: %v", err)
	}
	var payload struct {
		CoverageWarnings []struct {
			ID          string `json:"id"`
			Status      string `json:"status"`
			Message     string `json:"message"`
			Remediation string `json:"remediation"`
		} `json:"coverageWarnings"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("unmarshal review doctor result: %v", err)
	}
	if len(payload.CoverageWarnings) != 1 {
		t.Fatalf("expected one coverage warning, got %s", output)
	}
	warning := payload.CoverageWarnings[0]
	if warning.ID != "review.coverage.app_store_regulations_and_permits" {
		t.Fatalf("unexpected coverage warning ID %q", warning.ID)
	}
	if warning.Status != "NOT_CHECKED" {
		t.Fatalf("expected NOT_CHECKED status, got %q", warning.Status)
	}
	if !strings.Contains(warning.Message, "personal-service declaration") {
		t.Fatalf("expected personal-service declaration disclosure, got %q", warning.Message)
	}
	if !strings.Contains(warning.Remediation, `asc web apps declarations list --app "123456789"`) {
		t.Fatalf("expected declarations list remediation with app selector, got %q", warning.Remediation)
	}
	if !strings.Contains(warning.Remediation, `asc web apps medical-device set --app "123456789" --declared false`) {
		t.Fatalf("expected medical-device set remediation with app selector, got %q", warning.Remediation)
	}
}

func TestRenderReviewDoctorDisclosesWebOnlyCoverage(t *testing.T) {
	result := buildReviewDoctorResult(reviewSnapshot{
		AppID: "123456789",
		Version: &reviewVersionContext{
			ID:       "ver-1",
			Version:  "1.2.3",
			Platform: "IOS",
			State:    "PREPARE_FOR_SUBMISSION",
		},
	}, validation.Report{})

	for _, test := range []struct {
		name     string
		markdown bool
	}{
		{name: "table"},
		{name: "markdown", markdown: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := captureReviewStdout(t, func() {
				renderReviewDoctor(result, test.markdown)
			})
			for _, want := range []string{
				"Coverage Warnings",
				"review.coverage.app_store_regulations_and_permits",
				"NOT_CHECKED",
				"personal-service declaration",
				"asc web apps declarations list --app \"123456789\"",
			} {
				if !strings.Contains(strings.ToLower(output), strings.ToLower(want)) {
					t.Fatalf("expected %q in output:\n%s", want, output)
				}
			}
		})
	}
}

func captureReviewStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = oldStdout
		_ = reader.Close()
		_ = writer.Close()
	})

	done := make(chan []byte, 1)
	go func() {
		output, _ := io.ReadAll(reader)
		done <- output
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	os.Stdout = oldStdout

	return string(<-done)
}

func TestBuildReviewOverviewResultsExposeReviewDetailConfiguredState(t *testing.T) {
	snapshot := reviewSnapshot{
		AppID: "123456789",
		Version: &reviewVersionContext{
			ID:       "ver-1",
			Version:  "1.2.3",
			Platform: "IOS",
			State:    "PREPARE_FOR_SUBMISSION",
		},
	}

	statusResult := buildReviewStatusResult(snapshot)
	if statusResult.ReviewDetailConfigured {
		t.Fatal("expected status result reviewDetailConfigured=false")
	}

	doctorResult := buildReviewDoctorResult(snapshot, validation.Report{})
	if doctorResult.ReviewDetailConfigured {
		t.Fatal("expected doctor result reviewDetailConfigured=false")
	}
}

func TestAccumulateReviewSubmissionItemsIgnoresUnrelatedSubmissionItems(t *testing.T) {
	summary := reviewSubmissionItemsContext{}
	items := []asc.ReviewSubmissionItemResource{
		{
			ID: "item-removed-version",
			Attributes: asc.ReviewSubmissionItemAttributes{
				State: "REMOVED",
			},
			Relationships: &asc.ReviewSubmissionItemRelationships{
				AppStoreVersion: &asc.Relationship{
					Data: asc.ResourceData{ID: "ver-1", Type: asc.ResourceTypeAppStoreVersions},
				},
			},
		},
		{
			ID: "item-active-background",
			Attributes: asc.ReviewSubmissionItemAttributes{
				State: "APPROVED",
			},
			Relationships: &asc.ReviewSubmissionItemRelationships{
				BackgroundAssetVersion: &asc.Relationship{
					Data: asc.ResourceData{ID: "bg-1", Type: asc.ResourceTypeBackgroundAssetVersions},
				},
			},
		},
		{
			ID: "item-other-version",
			Attributes: asc.ReviewSubmissionItemAttributes{
				State: "APPROVED",
			},
			Relationships: &asc.ReviewSubmissionItemRelationships{
				AppStoreVersion: &asc.Relationship{
					Data: asc.ResourceData{ID: "ver-2", Type: asc.ResourceTypeAppStoreVersions},
				},
			},
		},
	}

	accumulateReviewSubmissionItems(&summary, items, "ver-1")

	if summary.TotalCount != 1 {
		t.Fatalf("expected only selected version item to count, got total=%d", summary.TotalCount)
	}
	if summary.RemovedCount != 1 {
		t.Fatalf("expected removed count 1, got %d", summary.RemovedCount)
	}
	if summary.ActiveCount != 0 {
		t.Fatalf("expected no active selected-version items, got %d", summary.ActiveCount)
	}
}

func TestSummarizeReviewSubmissionItemsStopsOnRepeatedNextURL(t *testing.T) {
	setupReviewTestAuth(t)

	requestCount := 0
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, []byte(os.Getenv("ASC_PRIVATE_KEY")), 0o600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient(
		"KEY_ID",
		"ISSUER_ID",
		keyPath,
		&http.Client{Transport: reviewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if requestCount > 2 {
				return nil, fmt.Errorf("unexpected request after repeated pagination URL: %d", requestCount)
			}

			return reviewJSONResponse(http.StatusOK, fmt.Sprintf(`{
				"data":[{
					"type":"reviewSubmissionItems",
					"id":"item-%d",
					"attributes":{"state":"REMOVED"},
					"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}
				}],
				"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":"/v1/reviewSubmissions/submission-1/items?page=2"}
			}`, requestCount))
		})},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}

	_, err = summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "ver-1")
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected ErrRepeatedPaginationURL, got %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected one initial request and one continuation request, got %d", requestCount)
	}
}

func TestSummarizeReviewSubmissionItemsTreatsWhitespaceNextURLAsEnd(t *testing.T) {
	setupReviewTestAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")

	requestCount := 0
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, []byte(os.Getenv("ASC_PRIVATE_KEY")), 0o600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient(
		"KEY_ID",
		"ISSUER_ID",
		keyPath,
		&http.Client{Transport: reviewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if requestCount > 1 {
				return nil, fmt.Errorf("unexpected request for whitespace-only next URL")
			}
			return reviewJSONResponse(http.StatusOK, `{
				"data":[{
					"type":"reviewSubmissionItems",
					"id":"item-1",
					"attributes":{"state":"REMOVED"},
					"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}
				}],
				"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":" \t\n "}
			}`)
		})},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}

	summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "ver-1")
	if err != nil {
		t.Fatalf("summarizeReviewSubmissionItems() error = %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want 1", requestCount)
	}
	if summary.TotalCount != 1 || summary.RemovedCount != 1 {
		t.Fatalf("summary = %+v, want one removed item", summary)
	}
}

func TestSummarizeReviewSubmissionItemsTrimsPaddedNextURL(t *testing.T) {
	setupReviewTestAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")

	requestCount := 0
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	if err := os.WriteFile(keyPath, []byte(os.Getenv("ASC_PRIVATE_KEY")), 0o600); err != nil {
		t.Fatalf("write test key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient(
		"KEY_ID",
		"ISSUER_ID",
		keyPath,
		&http.Client{Transport: reviewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			requestCount++
			if req.URL.Path != "/v1/reviewSubmissions/submission-1/items" {
				return nil, fmt.Errorf("unexpected request path %q", req.URL.Path)
			}
			if requestCount == 1 {
				return reviewJSONResponse(http.StatusOK, `{
					"data":[{
						"type":"reviewSubmissionItems",
						"id":"item-1",
						"attributes":{"state":"REMOVED"},
						"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}
					}],
					"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":" \t/v1/reviewSubmissions/submission-1/items?page=2 \n "}
				}`)
			}
			if req.URL.Query().Get("page") != "2" {
				return nil, fmt.Errorf("continuation query = %q, want page=2", req.URL.RawQuery)
			}
			return reviewJSONResponse(http.StatusOK, `{
				"data":[{
					"type":"reviewSubmissionItems",
					"id":"item-2",
					"attributes":{"state":"APPROVED"},
					"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"ver-1"}}}
				}],
				"links":{"self":"/v1/reviewSubmissions/submission-1/items","next":""}
			}`)
		})},
	)
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}

	summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "ver-1")
	if err != nil {
		t.Fatalf("summarizeReviewSubmissionItems() error = %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if summary.TotalCount != 2 || summary.RemovedCount != 1 || summary.ActiveCount != 1 {
		t.Fatalf("summary = %+v, want one removed and one active item", summary)
	}
}

func TestSelectRelevantReviewSubmissionPrefersActiveSubmissionWithoutSubmittedDate(t *testing.T) {
	submissions := []asc.ReviewSubmissionResource{
		{
			ID: "review-sub-complete",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmissionState: asc.ReviewSubmissionStateComplete,
				SubmittedDate:   "2026-03-16T10:00:00Z",
			},
			Relationships: &asc.ReviewSubmissionRelationships{
				AppStoreVersionForReview: &asc.Relationship{
					Data: asc.ResourceData{ID: "ver-1", Type: asc.ResourceTypeAppStoreVersions},
				},
			},
		},
		{
			ID: "review-sub-ready",
			Attributes: asc.ReviewSubmissionAttributes{
				SubmissionState: asc.ReviewSubmissionStateReadyForReview,
				SubmittedDate:   "",
			},
			Relationships: &asc.ReviewSubmissionRelationships{
				AppStoreVersionForReview: &asc.Relationship{
					Data: asc.ResourceData{ID: "ver-1", Type: asc.ResourceTypeAppStoreVersions},
				},
			},
		},
	}

	selected := selectRelevantReviewSubmission(submissions, "ver-1")
	if selected == nil {
		t.Fatal("expected selected submission, got nil")
		return
	}
	if selected.ID != "review-sub-ready" {
		t.Fatalf("expected active ready-for-review submission to win, got %q", selected.ID)
	}
	if selected.State != string(asc.ReviewSubmissionStateReadyForReview) {
		t.Fatalf("expected READY_FOR_REVIEW state, got %q", selected.State)
	}
}

func TestReviewDoctorUsesTimedContextForReadinessReport(t *testing.T) {
	setupReviewTestAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	origBuilder := reviewReadinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		reviewReadinessReportBuilder = origBuilder
		http.DefaultTransport = origTransport
	})

	builderCalled := false
	reviewReadinessReportBuilder = func(ctx context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
		builderCalled = true
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected readiness report builder to receive timeout-bound context")
		}
		return validation.Report{
			AppID:     opts.AppID,
			VersionID: opts.VersionID,
		}, nil
	}

	http.DefaultTransport = reviewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appStoreVersions/ver-1":
			if req.URL.Query().Get("include") != "app" {
				t.Fatalf("expected include=app, got %q", req.URL.Query().Get("include"))
			}
			return reviewJSONResponse(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"ver-1",
					"attributes":{
						"platform":"IOS",
						"versionString":"1.2.3",
						"appVersionState":"PREPARE_FOR_SUBMISSION"
					},
					"relationships":{
						"app":{"data":{"type":"apps","id":"123456789"}}
					}
				}
			}`)
		case "/v1/appStoreVersions/ver-1/appStoreReviewDetail":
			return reviewJSONResponse(http.StatusNotFound, `{
				"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]
			}`)
		case "/v1/apps/123456789/reviewSubmissions":
			return reviewJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/123456789/reviewSubmissions","next":""}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	cmd := ReviewDoctorCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--app", "123456789", "--version-id", "ver-1"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("exec doctor command: %v", err)
	}
	if !builderCalled {
		t.Fatal("expected readiness report builder to be called")
	}
}
