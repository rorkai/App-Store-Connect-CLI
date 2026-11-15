package release

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// TestExecuteStageDryRunHonorsExistingCheckpoint proves --dry-run previews the
// run the checkpoint would actually produce: steps the confirmed run would skip
// are reported as skipped, and the checkpoint file is only read.
func TestExecuteStageDryRunHonorsExistingCheckpoint(t *testing.T) {
	d := releaseTestDeps()
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	t.Cleanup(func() {
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
	})

	metadataPushExecutor = func(context.Context, *shared.Deps, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{VersionID: "VERSION_123"}, nil
	}
	readinessReportBuilder = func(context.Context, *shared.Deps, validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := releaseBuildAppLinkageResponse(req); ok {
			return resp, nil
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	d.Client = func() (*asc.Client, error) { return client, nil }

	checkpointPath := filepath.Join(t.TempDir(), "stage-checkpoint.json")
	if err := saveCheckpoint(d, checkpointPath, runCheckpoint{
		AppID:       "APP_123",
		Version:     "2.4.0",
		BuildID:     "BUILD_123",
		MetadataDir: "./metadata/version/2.4.0",
		Platform:    "IOS",
		Mode:        releaseModeStage,
		VersionID:   "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion: true,
			stepAttachBuild:   true,
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}
	before, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatalf("read checkpoint: %v", err)
	}

	result, err := executeStage(context.Background(), d, runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		CheckpointFile: checkpointPath,
	})
	if err != nil {
		t.Fatalf("executeStage error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("expected the dry-run plan to report the checkpoint as resumed")
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected the checkpoint version in the plan, got %q", result.VersionID)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected five steps, got %#v", result.Steps)
	}
	for _, index := range []int{1, 3} {
		step := result.Steps[index]
		if !strings.Contains(step.Message, "already completed in checkpoint") {
			t.Fatalf("expected step %d to preview a checkpoint skip, got %#v", index, step)
		}
	}
	if result.Steps[2].Name != stepApplyMetadata || strings.Contains(result.Steps[2].Message, "checkpoint") {
		t.Fatalf("expected the metadata step to be planned again, got %#v", result.Steps[2])
	}

	after, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatalf("read checkpoint after dry-run: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("dry-run wrote to the checkpoint file:\nbefore %s\nafter  %s", before, after)
	}
}

// TestExecuteStageDryRunRejectsMismatchedCheckpoint proves --dry-run surfaces
// the same checkpoint mismatch that aborts the confirmed run, so a resume can
// be pre-flighted instead of failing only under --confirm.
func TestExecuteStageDryRunRejectsMismatchedCheckpoint(t *testing.T) {
	d := releaseTestDeps()
	clientCalled := false
	d.Client = func() (*asc.Client, error) {
		clientCalled = true
		return nil, errors.New("client must not be created")
	}

	checkpointPath := filepath.Join(t.TempDir(), "stage-checkpoint.json")
	if err := saveCheckpoint(d, checkpointPath, runCheckpoint{
		AppID:            "APP_123",
		Version:          "2.4.0",
		BuildID:          "BUILD_123",
		CopyMetadataFrom: "2.3.2",
		Platform:         "IOS",
		Mode:             releaseModeStage,
		VersionID:        "VERSION_123",
		Completed:        map[string]bool{stepEnsureVersion: true},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	result, err := executeStage(context.Background(), d, runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		CheckpointFile: checkpointPath,
	})
	if err == nil || !strings.Contains(err.Error(), "checkpoint does not match current run arguments") {
		t.Fatalf("executeStage() error = %v, want checkpoint mismatch", err)
	}
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if clientCalled {
		t.Fatal("dry-run created a client before rejecting the mismatched checkpoint")
	}
}
