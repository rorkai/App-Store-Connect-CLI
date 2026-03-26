package release

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type releaseRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn releaseRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func releaseJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func newReleaseTestClient(t *testing.T) *asc.Client {
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

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func TestReleaseCommandShape(t *testing.T) {
	cmd := ReleaseCommand()
	if cmd == nil {
		t.Fatal("expected release command")
	}
	if cmd.Name != "release" {
		t.Fatalf("expected command name release, got %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(cmd.Subcommands))
	}
	if cmd.Subcommands[0].Name != "run" {
		t.Fatalf("expected subcommand run, got %q", cmd.Subcommands[0].Name)
	}
	if cmd.Subcommands[1].Name != "stage" {
		t.Fatalf("expected subcommand stage, got %q", cmd.Subcommands[1].Name)
	}
}

func TestReleaseRunCommand_MissingRequiredFlags(t *testing.T) {
	cmd := ReleaseRunCommand()
	if err := cmd.FlagSet.Parse([]string{"--dry-run"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestReleaseStageCommand_MissingRequiredFlags(t *testing.T) {
	cmd := ReleaseStageCommand()
	if err := cmd.FlagSet.Parse([]string{"--dry-run"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestDefaultCheckpointPathSanitizesValues(t *testing.T) {
	path := defaultCheckpointPath("app/123", "1.2.3-beta", "build#12", "IOS")
	want := filepath.Join(".asc", "release", "checkpoints", "app_123_1.2.3-beta_build_12_IOS.json")
	if path != want {
		t.Fatalf("unexpected checkpoint path: got %q want %q", path, want)
	}
}

func TestCheckpointModeMatches(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		desired     string
		wantMatched bool
	}{
		{name: "legacy run checkpoint", existing: "", desired: releaseModeRun, wantMatched: true},
		{name: "legacy stage mismatch", existing: "", desired: releaseModeStage, wantMatched: false},
		{name: "trimmed run mode", existing: "  run  ", desired: releaseModeRun, wantMatched: true},
		{name: "trimmed stage mode", existing: "\tstage\n", desired: releaseModeStage, wantMatched: true},
		{name: "mismatched mode", existing: "run", desired: releaseModeStage, wantMatched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkpointModeMatches(tt.existing, tt.desired)
			if got != tt.wantMatched {
				t.Fatalf("checkpointModeMatches(%q, %q) = %v, want %v", tt.existing, tt.desired, got, tt.wantMatched)
			}
		})
	}
}

func TestExecuteRun_ResumesCompletedCheckpoint(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
	})

	releaseClientFactory = func() (*asc.Client, error) { return nil, nil }
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata executor should not be called for completed checkpoint")
		return metadata.PushPlanResult{}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness builder should not be called for completed checkpoint")
		return validation.Report{}, nil
	}

	dir := t.TempDir()
	checkpointPath := filepath.Join(dir, "release-checkpoint.json")
	checkpoint := runCheckpoint{
		AppID:        "APP_123",
		Version:      "2.4.0",
		BuildID:      "BUILD_123",
		MetadataDir:  "./metadata/version/2.4.0",
		Platform:     "IOS",
		VersionID:    "VERSION_123",
		SubmissionID: "SUBMISSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
			stepSubmitReview:      true,
		},
	}
	if err := saveCheckpoint(checkpointPath, checkpoint); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: checkpointPath,
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("expected resumed result")
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected versionID from checkpoint, got %q", result.VersionID)
	}
	if result.SubmissionID != "SUBMISSION_123" {
		t.Fatalf("expected submissionID from checkpoint, got %q", result.SubmissionID)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 skipped steps, got %d", len(result.Steps))
	}
	for i, step := range result.Steps {
		if step.Status != "skipped" {
			t.Fatalf("expected step %d skipped, got %q", i, step.Status)
		}
	}
}

func TestExecuteRun_SuccessPath(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataCalled := false
	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataCalled = true
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessCalled := false
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 1, Blocking: 0},
			Checks: []validation.CheckResult{{
				ID:           "privacy.publish_state.unverified",
				Severity:     validation.SeverityInfo,
				ResourceType: "appPrivacy",
				ResourceID:   "APP_123",
				Message:      "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Remediation:  "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/APP_123/appPrivacy",
			}},
		}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/VERSION_123/relationships/build":
			return releaseJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/reviewSubmissions":
			return releaseJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"REV_SUB_123","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"ITEM_123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/REV_SUB_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"REV_SUB_123","attributes":{"state":"SUBMITTED","platform":"IOS","submittedDate":"2026-03-02T00:00:00Z"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected versionID VERSION_123, got %q", result.VersionID)
	}
	if result.SubmissionID != "REV_SUB_123" {
		t.Fatalf("expected submissionID REV_SUB_123, got %q", result.SubmissionID)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if !metadataCalled {
		t.Fatal("expected metadata step to be executed")
	}
	if !readinessCalled {
		t.Fatal("expected readiness checks to be executed")
	}
	if result.Steps[3].Message != "readiness checks passed with 1 advisory; App Privacy may still block submission" {
		t.Fatalf("expected readiness advisory message, got %q", result.Steps[3].Message)
	}
}

func TestExecuteStage_CopyMetadataSuccessPath(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origMetadataCopyExecutor := metadataCopyExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		metadataCopyExecutor = origMetadataCopyExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	copyCalled := false
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata dir executor should not be called for copy-metadata stage flow")
		return metadata.PushPlanResult{}, nil
	}
	metadataCopyExecutor = func(_ context.Context, _ *asc.Client, opts metadataCopyOptions) (*asc.AppStoreVersionMetadataCopySummary, error) {
		copyCalled = true
		if opts.AppID != "APP_123" {
			t.Fatalf("expected app id APP_123, got %q", opts.AppID)
		}
		if opts.Platform != "IOS" {
			t.Fatalf("expected platform IOS, got %q", opts.Platform)
		}
		if opts.SourceVersion != "2.3.2" {
			t.Fatalf("expected source version 2.3.2, got %q", opts.SourceVersion)
		}
		if opts.DestinationVersionID != "VERSION_123" {
			t.Fatalf("expected destination version VERSION_123, got %q", opts.DestinationVersionID)
		}
		if opts.DryRun {
			t.Fatal("expected live copy metadata execution")
		}
		if got, want := strings.Join(opts.SelectedFields, ","), "description,keywords"; got != want {
			t.Fatalf("expected selected fields %q, got %q", want, got)
		}
		return &asc.AppStoreVersionMetadataCopySummary{
			SourceVersion:      "2.3.2",
			SourceVersionID:    "SOURCE_VERSION_123",
			SelectedFields:     []string{"description", "keywords"},
			CopiedLocales:      2,
			CopiedFieldUpdates: 4,
		}, nil
	}

	readinessCalled := false
	readinessReportBuilder = func(_ context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		if opts.VersionID != "VERSION_123" {
			t.Fatalf("expected readiness version VERSION_123, got %q", opts.VersionID)
		}
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 1, Blocking: 0},
			Checks: []validation.CheckResult{{
				ID:           "privacy.publish_state.unverified",
				Severity:     validation.SeverityInfo,
				ResourceType: "appPrivacy",
				ResourceID:   "APP_123",
				Message:      "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Remediation:  "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/APP_123/appPrivacy",
			}},
		}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/VERSION_123/relationships/build":
			return releaseJSONResponse(http.StatusNoContent, "")
		case strings.HasPrefix(req.URL.Path, "/v1/reviewSubmissions"), strings.HasPrefix(req.URL.Path, "/v1/reviewSubmissionItems"):
			t.Fatalf("did not expect submission request for stage flow: %s %s", req.Method, req.URL.Path)
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:              "APP_123",
		Version:            "2.4.0",
		BuildID:            "BUILD_123",
		CopyMetadataFrom:   "2.3.2",
		SelectedCopyFields: []string{"description", "keywords"},
		Platform:           "IOS",
		Timeout:            releaseRunTimeout,
		DryRun:             false,
		Confirm:            true,
		StrictValidate:     false,
		CheckpointFile:     filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeStage error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected versionID VERSION_123, got %q", result.VersionID)
	}
	if result.SubmissionID != "" {
		t.Fatalf("expected empty submissionID, got %q", result.SubmissionID)
	}
	if len(result.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Name != stepEnsureVersion {
		t.Fatalf("expected first step %q, got %q", stepEnsureVersion, result.Steps[0].Name)
	}
	if result.Steps[1].Name != stepApplyMetadata {
		t.Fatalf("expected second step %q, got %q", stepApplyMetadata, result.Steps[1].Name)
	}
	if result.Steps[2].Name != stepAttachBuild {
		t.Fatalf("expected third step %q, got %q", stepAttachBuild, result.Steps[2].Name)
	}
	if result.Steps[3].Name != stepValidateReadiness {
		t.Fatalf("expected fourth step %q, got %q", stepValidateReadiness, result.Steps[3].Name)
	}
	if !copyCalled {
		t.Fatal("expected metadata copy executor to be called")
	}
	if !readinessCalled {
		t.Fatal("expected readiness checks to be called")
	}
	if result.Steps[3].Message != "readiness checks passed with 1 advisory; App Privacy may still block submission" {
		t.Fatalf("expected readiness advisory message, got %q", result.Steps[3].Message)
	}
}

func TestExecuteRun_IdempotentWhenSubmissionExists(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	requests := make([]string, 0, 4)
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"WAITING_FOR_REVIEW"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"SUBMISSION_123"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }
	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 0, Blocking: 0},
		}, nil
	}

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if result.SubmissionID != "SUBMISSION_123" {
		t.Fatalf("expected existing submission id, got %q", result.SubmissionID)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if result.Steps[2].Status != "skipped" {
		t.Fatalf("expected attach step skipped, got %q", result.Steps[2].Status)
	}
	if result.Steps[4].Status != "skipped" {
		t.Fatalf("expected submit step skipped, got %q", result.Steps[4].Status)
	}

	for _, req := range requests {
		if strings.HasPrefix(req, "POST /v1/reviewSubmissions") || strings.HasPrefix(req, "POST /v1/reviewSubmissionItems") {
			t.Fatalf("expected idempotent path without new submission creation, saw %q", req)
		}
	}
}

func TestExecuteRun_DryRunReadinessStepMarkedDryRun(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 0, Blocking: 0},
		}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		Confirm:        false,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if result.Steps[3].Name != stepValidateReadiness {
		t.Fatalf("expected step 4 to be %q, got %q", stepValidateReadiness, result.Steps[3].Name)
	}
	if result.Steps[3].Status != "dry-run" {
		t.Fatalf("expected readiness step dry-run status, got %q", result.Steps[3].Status)
	}
}

func TestExecuteRun_TimeoutCancelsPipeline(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(ctx context.Context, _ metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		<-ctx.Done()
		return metadata.PushPlanResult{}, ctx.Err()
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness should not run when metadata step times out")
		return validation.Report{}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions" {
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	_, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        20 * time.Millisecond,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
