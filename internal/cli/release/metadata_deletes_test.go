package release

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// TestExecuteStagePropagatesMetadataDeleteFlags proves the operator's
// --allow-deletes and --confirm reach metadata.ExecutePush. Hardcoding them to
// false made a confirmed run abort on a delete plan the command could not
// authorize.
func TestExecuteStagePropagatesMetadataDeleteFlags(t *testing.T) {
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

	var pushOptions metadata.PushExecutionOptions
	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		pushOptions = opts
		return metadata.PushPlanResult{
			VersionID: "VERSION_123",
			Deletes:   []metadata.PlanItem{{Locale: "fr-FR", Field: "description"}},
		}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{Summary: validation.Summary{}}, nil
	}
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if resp, ok := releaseBuildAppLinkageResponse(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		Confirm:        true,
		AllowDeletes:   true,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeStage error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if !pushOptions.AllowDeletes {
		t.Fatal("executeStage did not propagate --allow-deletes to metadata push")
	}
	if !pushOptions.Confirm {
		t.Fatal("executeStage did not propagate --confirm to metadata push")
	}
}

// TestExecuteStageDryRunRejectsPlannedDeletesWithoutAllowDeletes proves the
// dry-run plan and the confirmed run agree: a plan containing deletes fails the
// preview with the same requirement the confirmed run enforces instead of
// reporting a green plan that then aborts under --confirm.
func TestExecuteStageDryRunRejectsPlannedDeletesWithoutAllowDeletes(t *testing.T) {
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
			VersionID: "VERSION_123",
			DryRun:    opts.DryRun,
			Deletes: []metadata.PlanItem{
				{Locale: "fr-FR", Field: "description"},
				{Locale: "fr-FR", Field: "keywords"},
			},
		}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness must not run after the metadata plan is rejected")
		return validation.Report{}, nil
	}
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if resp, ok := releaseBuildAppLinkageResponse(req); ok {
			return resp, nil
		}
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions" {
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"}}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err == nil {
		t.Fatal("expected the dry-run plan to fail on planned deletes without --allow-deletes")
	}
	if !strings.Contains(err.Error(), "--allow-deletes") {
		t.Fatalf("expected error naming --allow-deletes, got %v", err)
	}
	if !shared.IsValidationError(err) {
		t.Fatalf("expected the planned-deletes refusal to be a validation outcome, got %T", err)
	}
	if errors.Is(err, flag.ErrHelp) {
		t.Fatalf("mid-pipeline plan failure was classified as invalid command usage: %v", err)
	}
	if result.Status != "error" || result.FailedStep != stepApplyMetadata {
		t.Fatalf("expected apply_metadata failure, got status %q step %q", result.Status, result.FailedStep)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected three steps, got %#v", result.Steps)
	}
	metadataStep := result.Steps[2]
	if !strings.Contains(metadataStep.Remediation, "--allow-deletes") {
		t.Fatalf("expected an actionable remediation naming --allow-deletes, got %q", metadataStep.Remediation)
	}
	details, ok := metadataStep.Details.(map[string]any)
	if !ok {
		t.Fatalf("expected map details, got %#v", metadataStep.Details)
	}
	if details["deletes"] != 2 {
		t.Fatalf("expected the planned delete count in details, got %#v", details)
	}
}

// TestExecuteStageDryRunPlansDeletesWithAllowDeletes proves an authorized
// delete plan still previews cleanly.
func TestExecuteStageDryRunPlansDeletesWithAllowDeletes(t *testing.T) {
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

	var pushOptions metadata.PushExecutionOptions
	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		pushOptions = opts
		return metadata.PushPlanResult{
			VersionID: "VERSION_123",
			DryRun:    opts.DryRun,
			Deletes:   []metadata.PlanItem{{Locale: "fr-FR", Field: "description"}},
		}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{Summary: validation.Summary{}}, nil
	}
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if resp, ok := releaseBuildAppLinkageResponse(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		AllowDeletes:   true,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeStage error: %v", err)
	}
	if result.Status != "dry-run" {
		t.Fatalf("expected dry-run status, got %q", result.Status)
	}
	if !pushOptions.AllowDeletes || !pushOptions.DryRun {
		t.Fatalf("expected an authorized dry-run plan, got %#v", pushOptions)
	}
	if result.Steps[2].Name != stepApplyMetadata || result.Steps[2].Status != "dry-run" {
		t.Fatalf("expected metadata plan step, got %#v", result.Steps[2])
	}
}

// TestReleaseStageRejectsAllowDeletesWithoutMetadataDir proves the flag is
// never accepted and silently ignored: the copy pipeline has no delete
// operations to authorize.
func TestReleaseStageRejectsAllowDeletesWithoutMetadataDir(t *testing.T) {
	originalClientFactory := releaseClientFactory
	t.Cleanup(func() { releaseClientFactory = originalClientFactory })
	clientCalled := false
	releaseClientFactory = func() (*asc.Client, error) {
		clientCalled = true
		return nil, errors.New("client must not be created")
	}

	cmd := ReleaseStageCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "APP_123",
		"--version", "2.4.0",
		"--build-id", "BUILD_123",
		"--copy-metadata-from", "2.3.2",
		"--allow-deletes",
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var execErr error
	stderr := captureReleaseStderr(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", execErr)
	}
	if !strings.Contains(stderr, "--allow-deletes requires --metadata-dir") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if clientCalled {
		t.Fatal("release pipeline started despite an unusable --allow-deletes")
	}
}
