package release

import (
	"context"
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

func newCheckpointBindingClient(t *testing.T, handler releaseRoundTripFunc) *asc.Client {
	t.Helper()

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = handler
	return newReleaseTestClient(t)
}

func checkpointBindingOptions() runOptions {
	return runOptions{
		AppID:    "APP_123",
		Version:  "2.4.0",
		BuildID:  "BUILD_123",
		Platform: "IOS",
		Mode:     releaseModeStage,
	}
}

func TestVerifyResumedCheckpointBindingRejectsVersionOwnedByAnotherApp(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_B":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_B","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_B"}}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_B",
		Completed: map[string]bool{stepEnsureVersion: true},
	}
	err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, nil)
	if err == nil {
		t.Fatal("expected checkpoint verification to reject a version owned by another app")
	}
	if !strings.Contains(err.Error(), "belongs to app") {
		t.Fatalf("expected ownership error, got %v", err)
	}
}

func TestVerifyResumedCheckpointBindingRejectsVersionStringMismatch(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"9.9.9","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{stepEnsureVersion: true},
	}
	err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, nil)
	if err == nil {
		t.Fatal("expected checkpoint verification to reject a version string mismatch")
	}
	if !strings.Contains(err.Error(), "2.4.0") {
		t.Fatalf("expected error naming the requested version, got %v", err)
	}
}

func TestVerifyResumedCheckpointBindingRejectsCompletedStepsWithoutVersionBinding(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	checkpoint := runCheckpoint{
		Completed: map[string]bool{stepApplyMetadata: true},
	}
	err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, nil)
	if err == nil {
		t.Fatal("expected checkpoint verification to reject completed steps without a bound version")
	}
}

func TestVerifyResumedCheckpointBindingRejectsUnknownCompletedStep(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{"publish_everything": true},
	}
	err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, nil)
	if err == nil {
		t.Fatal("expected checkpoint verification to reject an unknown completed step")
	}
	if !strings.Contains(err.Error(), "publish_everything") {
		t.Fatalf("expected error naming the unknown step, got %v", err)
	}
}

func TestVerifyResumedCheckpointBindingDropsUnprovenAttachBuildCompletion(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_OTHER","attributes":{"version":"41"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{stepEnsureVersion: true, stepAttachBuild: true},
	}
	var messages []string
	if err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, func(message string) {
		messages = append(messages, message)
	}); err != nil {
		t.Fatalf("verifyResumedCheckpointBinding error: %v", err)
	}
	if checkpoint.Completed[stepAttachBuild] {
		t.Fatal("expected unproven attach_build completion to be discarded")
	}
	if !checkpoint.Completed[stepEnsureVersion] {
		t.Fatal("expected verified ensure_version completion to survive")
	}
	if len(messages) == 0 || !strings.Contains(strings.Join(messages, "\n"), stepAttachBuild) {
		t.Fatalf("expected a diagnostic naming attach_build, got %v", messages)
	}
}

// TestVerifyResumedCheckpointBindingDropsReadinessWithUnprovenAttachBuild
// proves that discarding an attach_build completion also discards a completed
// validate_readiness: readiness was checked against whatever build was
// attached at the time, so it must run again after the build is re-attached.
func TestVerifyResumedCheckpointBindingDropsReadinessWithUnprovenAttachBuild(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_OTHER","attributes":{"version":"41"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
		},
	}
	var messages []string
	if err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, func(message string) {
		messages = append(messages, message)
	}); err != nil {
		t.Fatalf("verifyResumedCheckpointBinding error: %v", err)
	}
	if checkpoint.Completed[stepAttachBuild] {
		t.Fatal("expected unproven attach_build completion to be discarded")
	}
	if checkpoint.Completed[stepValidateReadiness] {
		t.Fatal("expected dependent validate_readiness completion to be discarded")
	}
	if !checkpoint.Completed[stepEnsureVersion] {
		t.Fatal("expected verified ensure_version completion to survive")
	}
	if len(messages) == 0 || !strings.Contains(strings.Join(messages, "\n"), stepValidateReadiness) {
		t.Fatalf("expected a diagnostic naming validate_readiness, got %v", messages)
	}
}

func TestVerifyResumedCheckpointBindingKeepsProvenCheckpoint(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
		},
	}
	if err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, nil); err != nil {
		t.Fatalf("verifyResumedCheckpointBinding error: %v", err)
	}
	if len(checkpoint.Completed) != 2 {
		t.Fatalf("expected only remotely verifiable completions to survive, got %#v", checkpoint.Completed)
	}
	if checkpoint.Completed[stepApplyMetadata] || checkpoint.Completed[stepValidateReadiness] {
		t.Fatalf("expected local unprovable completions to be discarded, got %#v", checkpoint.Completed)
	}
	if !checkpoint.Completed[stepEnsureVersion] || !checkpoint.Completed[stepAttachBuild] {
		t.Fatalf("expected remotely verified completions to survive, got %#v", checkpoint.Completed)
	}
}

// TestExecuteStage_RejectsForgedCheckpointVersionBeforeMutation proves a modified
// checkpoint cannot substitute VersionID and have the pipeline act on it.
func TestExecuteStage_RejectsForgedCheckpointVersionBeforeMutation(t *testing.T) {
	d := releaseTestDeps()
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	t.Cleanup(func() {
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
	})

	metadataPushExecutor = func(context.Context, *shared.Deps, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata executor must not run for an unverifiable checkpoint")
		return metadata.PushPlanResult{}, nil
	}
	readinessReportBuilder = func(context.Context, *shared.Deps, validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness builder must not run for an unverifiable checkpoint")
		return validation.Report{}, nil
	}

	var mutations []string
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations = append(mutations, req.Method+" "+req.URL.Path)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_FORGED":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_FORGED","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_OTHER"}}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	d.Client = func() (*asc.Client, error) { return client, nil }

	checkpointPath := filepath.Join(t.TempDir(), "release-checkpoint.json")
	if err := saveCheckpoint(d, checkpointPath, runCheckpoint{
		AppID:       "APP_123",
		Version:     "2.4.0",
		BuildID:     "BUILD_123",
		MetadataDir: "./metadata/version/2.4.0",
		Platform:    "IOS",
		Mode:        releaseModeStage,
		VersionID:   "VERSION_FORGED",
		Completed: map[string]bool{
			stepEnsureVersion: true,
			stepApplyMetadata: true,
			stepAttachBuild:   true,
		},
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
		Confirm:        true,
		CheckpointFile: checkpointPath,
	})
	if err == nil {
		t.Fatal("expected executeStage to fail for a checkpoint version owned by another app")
	}
	if result.Status != "error" {
		t.Fatalf("expected error status, got %q", result.Status)
	}
	if len(mutations) != 0 {
		t.Fatalf("expected no mutating requests, got %v", mutations)
	}
}

// TestVerifyResumedCheckpointBindingDropsReadinessWithIncompletePrerequisite
// proves an unsigned checkpoint cannot claim validate_readiness while a
// prerequisite mutation step is missing. The pipeline would run that mutation
// and then skip readiness, submitting a version whose readiness was never
// validated against the state the mutation produced.
func TestVerifyResumedCheckpointBindingDropsReadinessWithIncompletePrerequisite(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepValidateReadiness: true,
		},
	}
	var messages []string
	if err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, func(message string) {
		messages = append(messages, message)
	}); err != nil {
		t.Fatalf("verifyResumedCheckpointBinding error: %v", err)
	}
	if checkpoint.Completed[stepValidateReadiness] {
		t.Fatal("expected validate_readiness to be discarded while attach_build is incomplete")
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, stepValidateReadiness) || !strings.Contains(joined, stepApplyMetadata) {
		t.Fatalf("expected diagnostics naming the unprovable local steps, got %v", messages)
	}
}

// TestVerifyResumedCheckpointBindingRerunsUnprovableLocalSteps proves an
// unsigned checkpoint cannot suppress operations whose effects cannot be
// authenticated from current App Store Connect state. Metadata may have
// changed locally after the checkpoint was written, and readiness is a
// point-in-time validation, so both steps must run on every resume.
func TestVerifyResumedCheckpointBindingRerunsUnprovableLocalSteps(t *testing.T) {
	d := releaseTestDeps()
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	checkpoint := runCheckpoint{
		VersionID: "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
		},
	}
	var messages []string
	if err := verifyResumedCheckpointBinding(context.Background(), d, client, checkpointBindingOptions(), &checkpoint, func(message string) {
		messages = append(messages, message)
	}); err != nil {
		t.Fatalf("verifyResumedCheckpointBinding error: %v", err)
	}
	if checkpoint.Completed[stepApplyMetadata] {
		t.Fatal("expected unprovable apply_metadata completion to be discarded")
	}
	if checkpoint.Completed[stepValidateReadiness] {
		t.Fatal("expected point-in-time validate_readiness completion to be discarded")
	}
	if !checkpoint.Completed[stepEnsureVersion] || !checkpoint.Completed[stepAttachBuild] {
		t.Fatalf("expected authenticated remote-state completions to survive, got %#v", checkpoint.Completed)
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, stepApplyMetadata) || !strings.Contains(joined, stepValidateReadiness) {
		t.Fatalf("expected diagnostics naming both rerun steps, got %v", messages)
	}
}

// TestExecuteStage_PersistsDiscardedCompletionsBeforeMutating proves discarded
// completions reach the checkpoint file before the pipeline mutates anything.
// Otherwise a checkpoint write that fails after a successful re-attachment
// leaves the stale validate_readiness flag on disk, and the next resume — where
// the attachment now matches --build — skips readiness for the new build.
func TestExecuteStage_PersistsDiscardedCompletionsBeforeMutating(t *testing.T) {
	d := releaseTestDeps()
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	t.Cleanup(func() {
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
	})

	metadataPushExecutor = func(context.Context, *shared.Deps, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{}, nil
	}
	readinessReportBuilder = func(context.Context, *shared.Deps, validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{}, nil
	}

	checkpointPath := filepath.Join(t.TempDir(), "release-checkpoint.json")
	var checkpointAtFirstMutation *runCheckpoint
	attached := false
	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		if resp, ok := releaseBuildAppLinkageResponse(req); ok {
			return resp, nil
		}
		if req.Method != http.MethodGet && checkpointAtFirstMutation == nil {
			persisted, loadErr := loadCheckpoint(checkpointPath)
			if loadErr != nil {
				return nil, fmt.Errorf("load checkpoint during mutation: %w", loadErr)
			}
			checkpointAtFirstMutation = persisted
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			if attached {
				return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
			}
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_OTHER","attributes":{"version":"41"}}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/VERSION_123/relationships/build":
			attached = true
			return releaseJSONResponse(http.StatusNoContent, ``)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	d.Client = func() (*asc.Client, error) { return client, nil }

	if err := saveCheckpoint(d, checkpointPath, runCheckpoint{
		AppID:       "APP_123",
		Version:     "2.4.0",
		BuildID:     "BUILD_123",
		MetadataDir: "./metadata/version/2.4.0",
		Platform:    "IOS",
		Mode:        releaseModeStage,
		VersionID:   "VERSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	if _, err := executeStage(context.Background(), d, runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		Confirm:        true,
		CheckpointFile: checkpointPath,
	}); err != nil {
		t.Fatalf("executeStage error: %v", err)
	}

	if checkpointAtFirstMutation == nil {
		t.Fatal("expected the pipeline to re-attach the build")
	}
	if checkpointAtFirstMutation.Completed[stepAttachBuild] {
		t.Fatal("expected the discarded attach_build flag to be persisted before the re-attachment")
	}
	if checkpointAtFirstMutation.Completed[stepValidateReadiness] {
		t.Fatal("expected the discarded validate_readiness flag to be persisted before the re-attachment")
	}
}
