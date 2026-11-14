package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewSubmitValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"review", "submit", "--version", "1.2.3", "--build-id", "build-1", "--confirm"},
			wantErr: "--app is required",
		},
		{
			name:    "missing build",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--confirm"},
			wantErr: "--build-id, --build-number, or --latest is required",
		},
		{
			name:    "missing version selector",
			args:    []string{"review", "submit", "--app", "app-1", "--build-id", "build-1", "--confirm"},
			wantErr: "--version or --version-id is required",
		},
		{
			name:    "conflicting version selectors",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--version-id", "version-1", "--build-id", "build-1", "--confirm"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name:    "missing confirm",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--build-id", "build-1"},
			wantErr: "--confirm is required unless --dry-run is set",
		},
		{
			name:    "invalid platform",
			args:    []string{"review", "submit", "--app", "app-1", "--version", "1.2.3", "--build-id", "build-1", "--platform", "watchos", "--confirm"},
			wantErr: "--platform must be one of",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestReviewSubmitLocalizationPreflightUsesReviewSubmitGuidance(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"version-1",
			"attributes":{"platform":"IOS","versionString":"1.2.3"},
			"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
		},
		"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version-id", "version-1",
			"--build-id", "build-1",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil {
		t.Fatal("expected localization preflight error, got nil")
	}
	if !strings.Contains(runErr.Error(), "review submit: submit preflight failed") {
		t.Fatalf("expected review submit error prefix, got %v", runErr)
	}
	if strings.Contains(runErr.Error(), "submit create:") {
		t.Fatalf("did not expect removed submit create prefix, got %v", runErr)
	}
	if !strings.Contains(stderr, "before retrying `asc review submit`") {
		t.Fatalf("expected retry guidance for review submit, got %q", stderr)
	}
	if strings.Contains(stderr, "submit create") {
		t.Fatalf("did not expect removed submit create guidance, got %q", stderr)
	}
}

func TestReviewSubmitPreflightRequiresWhatsNewWhenOnlyAppVersionStateIsLive(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"version-1",
			"attributes":{"platform":"IOS","versionString":"1.2.3"},
			"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
		},
		"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"one,two","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-0","attributes":{"platform":"IOS","versionString":"1.0.0","appVersionState":"READY_FOR_DISTRIBUTION"}}]}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"review", "submit",
			"--app", "app-1",
			"--version-id", "version-1",
			"--build-id", "build-1",
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "review submit: submit preflight failed") {
		t.Fatalf("expected submit preflight failure, got %v", runErr)
	}
	if !strings.Contains(stderr, "en-US: whatsNew") {
		t.Fatalf("expected whatsNew to be required for an app update, got %q", stderr)
	}
	var versionQueries []string
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/apps/app-1/appStoreVersions" {
			query := req.URL.Query()
			versionQueries = append(versionQueries, query.Get("filter[appStoreState]")+"|"+query.Get("filter[appVersionState]"))
		}
	}
	if got, want := strings.Join(versionQueries, ","), "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE|,|READY_FOR_DISTRIBUTION"; got != want {
		t.Fatalf("released-version queries = %q, want %q", got, want)
	}
}

func TestReviewSubmitSubscriptionPreflightUsesReviewSubmitGuidance(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"version-1",
			"attributes":{"platform":"IOS","versionString":"1.2.3"},
			"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
		},
		"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptionGroups","id":"group-1","attributes":{"referenceName":"Premium"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/subscriptionGroups/group-1/subscriptions", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"subscriptions","id":"sub-1","attributes":{"name":"Monthly","state":"READY_TO_SUBMIT"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

	stdout, stderr, err := runCommand(t, []string{
		"review", "submit",
		"--app", "app-1",
		"--version-id", "version-1",
		"--build-id", "build-1",
		"--dry-run",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if !strings.Contains(stdout, `"wouldSubmit":true`) {
		t.Fatalf("expected successful dry-run output, got %q", stdout)
	}
	if !strings.Contains(stderr, "before retrying `asc review submit`") {
		t.Fatalf("expected retry guidance for review submit, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc subscriptions versions list --subscription-id "SUB_ID"`) {
		t.Fatalf("expected subscription version discovery guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, `asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "VERSION_ID"`) {
		t.Fatalf("expected version review-item guidance, got %q", stderr)
	}
	if strings.Contains(stderr, "asc subscriptions review submit") {
		t.Fatalf("did not expect deprecated subscription submission guidance, got %q", stderr)
	}
	if strings.Contains(stderr, "submit create") {
		t.Fatalf("did not expect removed submit create guidance, got %q", stderr)
	}
}

func TestReviewSubmitDryRunPreviewsWithoutMutations(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"version-1",
			"attributes":{"platform":"IOS","versionString":"1.2.3"},
			"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
		},
		"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"review", "submit",
		"--app", "app-1",
		"--version-id", "version-1",
		"--build-id", "build-1",
		"--dry-run",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}

	var payload struct {
		AppID            string `json:"appId"`
		VersionID        string `json:"versionId"`
		BuildID          string `json:"buildId"`
		Platform         string `json:"platform"`
		DryRun           bool   `json:"dryRun"`
		WouldSubmit      bool   `json:"wouldSubmit"`
		AlreadySubmitted bool   `json:"alreadySubmitted"`
		BuildAttachment  struct {
			VersionID   string `json:"versionId"`
			BuildID     string `json:"buildId"`
			WouldAttach bool   `json:"wouldAttach"`
		} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr for clean dry-run, got %q", stderr)
	}
	if payload.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", payload.AppID)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.BuildID != "build-1" {
		t.Fatalf("expected buildId build-1, got %q", payload.BuildID)
	}
	if payload.Platform != "IOS" {
		t.Fatalf("expected platform IOS, got %q", payload.Platform)
	}
	if !payload.DryRun {
		t.Fatal("expected dryRun=true")
	}
	if !payload.WouldSubmit {
		t.Fatal("expected wouldSubmit=true")
	}
	if payload.AlreadySubmitted {
		t.Fatal("did not expect alreadySubmitted=true")
	}
	if !payload.BuildAttachment.WouldAttach {
		t.Fatal("expected buildAttachment.wouldAttach=true")
	}

	recordedRequests := strings.Join(reviewSubmitRequests(reg), "\n")
	if strings.Contains(recordedRequests, "POST /v1/reviewSubmissions") || strings.Contains(recordedRequests, "PATCH /v1/reviewSubmissions/") {
		t.Fatalf("expected dry-run to avoid mutations, got requests: %s", recordedRequests)
	}
}

func TestReviewSubmitUsesModernReviewSubmissionFlow(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=READY_FOR_REVIEW", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersions/version-1/relationships/build").JSONBody(`{"data":{"type":"builds","id":"build-1"}}`), asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.POST("/v1/reviewSubmissions").JSONBody(`{"data":{"type":"reviewSubmissions","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`))
	reg.Register(asctest.POST("/v1/reviewSubmissionItems").JSONBody(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"review-sub-1"}},"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-sub-1").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"submitted":true}}}`), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"WAITING_FOR_REVIEW","submittedDate":"2026-04-08T00:00:00Z"}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"review", "submit",
		"--app", "app-1",
		"--version", "1.2.3",
		"--build-id", "build-1",
		"--confirm",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		AppID           string `json:"appId"`
		Version         string `json:"version"`
		VersionID       string `json:"versionId"`
		BuildID         string `json:"buildId"`
		Platform        string `json:"platform"`
		SubmissionID    string `json:"submissionId"`
		BuildAttachment struct {
			Attached bool `json:"attached"`
		} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if payload.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", payload.AppID)
	}
	if payload.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", payload.Version)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.BuildID != "build-1" {
		t.Fatalf("expected buildId build-1, got %q", payload.BuildID)
	}
	if payload.Platform != "IOS" {
		t.Fatalf("expected platform IOS, got %q", payload.Platform)
	}
	if payload.SubmissionID != "review-sub-1" {
		t.Fatalf("expected submissionId review-sub-1, got %q", payload.SubmissionID)
	}
	if !payload.BuildAttachment.Attached {
		t.Fatal("expected build attachment to report attached=true")
	}

	recordedRequests := strings.Join(reviewSubmitRequests(reg), "\n")
	if strings.Contains(recordedRequests, "POST /v1/appStoreVersionSubmissions") {
		t.Fatalf("did not expect legacy endpoint usage, got requests: %s", recordedRequests)
	}
	if !strings.Contains(recordedRequests, "POST /v1/reviewSubmissions") {
		t.Fatalf("expected modern review submission create request, got requests: %s", recordedRequests)
	}
	if got := strings.Count(recordedRequests, "GET /v1/appStoreVersions/version-1/appStoreVersionSubmission"); got != 1 {
		t.Fatalf("expected exactly one existing submission lookup, got %d requests: %s", got, recordedRequests)
	}
}

func TestReviewSubmitAlreadySubmittedSkipsPreflightAndBuildAttachment(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"), asctest.JSON(http.StatusOK, `{
		"data":{
			"type":"appStoreVersions",
			"id":"version-1",
			"attributes":{"platform":"IOS","versionString":"1.2.3"},
			"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
		},
		"included":[{"type":"apps","id":"app-1","attributes":{"bundleId":"app-1","name":"App One"}}]
	}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"legacy-sub-1"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"review", "submit",
		"--app", "app-1",
		"--version-id", "version-1",
		"--build-id", "build-2",
		"--confirm",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		AppID            string    `json:"appId"`
		VersionID        string    `json:"versionId"`
		BuildID          string    `json:"buildId"`
		SubmissionID     string    `json:"submissionId"`
		AlreadySubmitted bool      `json:"alreadySubmitted"`
		BuildAttachment  *struct{} `json:"buildAttachment"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}

	if payload.AppID != "app-1" {
		t.Fatalf("expected appId app-1, got %q", payload.AppID)
	}
	if payload.VersionID != "version-1" {
		t.Fatalf("expected versionId version-1, got %q", payload.VersionID)
	}
	if payload.BuildID != "build-2" {
		t.Fatalf("expected buildId build-2, got %q", payload.BuildID)
	}
	if payload.SubmissionID != "legacy-sub-1" {
		t.Fatalf("expected submissionId legacy-sub-1, got %q", payload.SubmissionID)
	}
	if !payload.AlreadySubmitted {
		t.Fatal("expected alreadySubmitted=true")
	}
	if payload.BuildAttachment != nil {
		t.Fatalf("expected buildAttachment to be omitted when already submitted, got %s", stdout)
	}

	recordedRequests := strings.Join(reviewSubmitRequests(reg), "\n")
	if !strings.Contains(recordedRequests, "GET /v1/appStoreVersions/version-1/appStoreVersionSubmission") {
		t.Fatalf("expected existing submission lookup, got requests: %s", recordedRequests)
	}
	if strings.Contains(recordedRequests, "/relationships/build") {
		t.Fatalf("did not expect build mutation requests, got requests: %s", recordedRequests)
	}
}

// isReleasedVersionStateQuery reports whether an app store versions request is
// the released-version lookup that decides whether whatsNew is required. It
// queries the deprecated appStoreState and appVersionState separately.
func isReleasedVersionStateQuery(query url.Values) bool {
	return query.Get("filter[appStoreState]") == "READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE" ||
		(query.Get("filter[appStoreState]") == "" && query.Get("filter[appVersionState]") == "READY_FOR_DISTRIBUTION")
}

func TestReviewSubmitConflictAtFinalSubmitReportsCreatedSubmissionAndHints(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionSubmission"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description","keywords":"keyword","supportUrl":"https://example.com/support"}}]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[appVersionState]=READY_FOR_DISTRIBUTION", "filter[platform]=IOS", "limit=1"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/subscriptionGroups", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[platform]=IOS", "filter[state]=READY_FOR_REVIEW", "include=appStoreVersionForReview", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions"}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/build"), asctest.JSON(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`))
	reg.Register(asctest.PATCH("/v1/appStoreVersions/version-1/relationships/build").JSONBody(`{"data":{"type":"builds","id":"build-1"}}`), asctest.JSON(http.StatusNoContent, ""))
	reg.Register(asctest.POST("/v1/reviewSubmissions").JSONBody(`{"data":{"type":"reviewSubmissions","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`))
	reg.Register(asctest.POST("/v1/reviewSubmissionItems").JSONBody(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"review-sub-1"}},"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`), asctest.JSON(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"item-1"}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1", "include=app"), asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
	reg.Register(asctest.GET("/v1/reviewSubmissions/review-sub-1/items", "fields[reviewSubmissionItems]=appStoreVersion", "include=appStoreVersion", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"item-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}],"links":{"self":"/v1/reviewSubmissions/review-sub-1/items"}}`))
	reg.Register(asctest.PATCH("/v1/reviewSubmissions/review-sub-1").JSONBody(`{"data":{"type":"reviewSubmissions","id":"review-sub-1","attributes":{"submitted":true}}}`), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Submission is not complete.","meta":{"associatedErrors":{"/v1/ageRatingDeclarations/age-1":[{"code":"ENTITY_ERROR.ATTRIBUTE.REQUIRED","detail":"An attribute value is required."}],"/v1/apps/app-1/contentRightsDeclaration":[{"code":"ENTITY_ERROR.ATTRIBUTE.REQUIRED","detail":"An attribute value is required."}]}}}]}`))

	code, stdout, stderr := runASCForExitCode(
		t,
		"review", "submit",
		"--app", "app-1",
		"--version", "1.2.3",
		"--build-id", "build-1",
		"--confirm",
		"--output", "json",
	)

	if code != cmd.ExitConflict {
		t.Fatalf("expected exit code %d, got %d (stderr %q)", cmd.ExitConflict, code, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{
		"asc submit cancel --id review-sub-1 --confirm",
		"Hint: Review current age rating: asc age-rating view --app app-1",
		"Hint: If your app does not use third-party content: asc apps update --id app-1 --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT",
		"Hint: Check the app's review state and blockers with `asc review status --app APP_ID` and `asc review doctor --app APP_ID`.",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func reviewSubmitRequests(reg *asctest.Registry) []string {
	requests := make([]string, 0, len(reg.Requests()))
	for _, req := range reg.Requests() {
		requests = append(requests, req.Method+" "+req.URL.Path)
	}
	return requests
}
