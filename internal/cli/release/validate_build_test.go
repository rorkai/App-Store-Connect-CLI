package release

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

// TestExecuteStageRejectsBuildOwnedByAnotherAppBeforeMutating proves --build is
// checked before the pipeline mutates anything. attach_build runs after the
// metadata and routing coverage steps, so an unusable build used to be
// discovered only once those mutations had already been applied.
func TestExecuteStageRejectsBuildOwnedByAnotherAppBeforeMutating(t *testing.T) {
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

	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata must not be applied for a build that belongs to another app")
		return metadata.PushPlanResult{}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness must not run for a build that belongs to another app")
		return validation.Report{}, nil
	}

	var mutations []string
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			mutations = append(mutations, req.Method+" "+req.URL.Path)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_OTHER/relationships/app":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"APP_OTHER"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"}}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_OTHER",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		Confirm:        true,
		CheckpointFile: filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err == nil {
		t.Fatal("executeStage() error = nil, want a rejected build")
	}
	if !strings.Contains(err.Error(), "BUILD_OTHER") || !strings.Contains(err.Error(), "APP_OTHER") {
		t.Fatalf("expected an error naming the build and its app, got %v", err)
	}
	if result.FailedStep != stepValidateBuild {
		t.Fatalf("expected the %s step to fail, got %q", stepValidateBuild, result.FailedStep)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected the pipeline to stop at the build check, got %#v", result.Steps)
	}
	if len(mutations) != 0 {
		t.Fatalf("expected no mutating requests, got %v", mutations)
	}
}

// TestExecuteStageRejectsBuildPlatformMismatchBeforeMutating proves the build
// preflight checks the selected build's platform before the version, metadata,
// or attachment steps can mutate App Store Connect.
func TestExecuteStageRejectsBuildPlatformMismatchBeforeMutating(t *testing.T) {
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
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataCalled = true
		return metadata.PushPlanResult{VersionID: "VERSION_123"}, nil
	}
	readinessCalled := false
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	var requestOrder []string
	var mutations []string
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestOrder = append(requestOrder, req.Method+" "+req.URL.Path)
		if req.Method != http.MethodGet {
			mutations = append(mutations, req.Method+" "+req.URL.Path)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/relationships/app":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/preReleaseVersion":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"2.4.0","platform":"IOS"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"MAC_OS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/VERSION_123/relationships/build":
			return releaseJSONResponse(http.StatusNoContent, "")
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
		Platform:       "MAC_OS",
		Timeout:        releaseRunTimeout,
		Confirm:        true,
		CheckpointFile: filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err == nil {
		t.Fatal("executeStage() error = nil, want a rejected platform mismatch")
	}
	if !strings.Contains(err.Error(), "BUILD_123") || !strings.Contains(err.Error(), "IOS") || !strings.Contains(err.Error(), "MAC_OS") {
		t.Fatalf("expected an error naming the build and both platforms, got %v", err)
	}
	if result.FailedStep != stepValidateBuild {
		t.Fatalf("expected the %s step to fail, got %q", stepValidateBuild, result.FailedStep)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected the pipeline to stop at the build check, got %#v", result.Steps)
	}
	if metadataCalled {
		t.Fatal("metadata must not run for a build on another platform")
	}
	if readinessCalled {
		t.Fatal("readiness must not run for a build on another platform")
	}
	if len(mutations) != 0 {
		t.Fatalf("expected no mutating requests, got %v (requests: %v)", mutations, requestOrder)
	}
	if len(requestOrder) < 2 || requestOrder[0] != "GET /v1/builds/BUILD_123/relationships/app" || requestOrder[1] != "GET /v1/builds/BUILD_123/preReleaseVersion" {
		t.Fatalf("expected app ownership then platform preflight, got %v", requestOrder)
	}
}

// TestExecuteStageDryRunRejectsBuildPlatformMismatchBeforeMutating proves a
// dry-run uses the same platform preflight rather than presenting a plan for a
// build that cannot be attached to the requested platform.
func TestExecuteStageDryRunRejectsBuildPlatformMismatchBeforeMutating(t *testing.T) {
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
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataCalled = true
		return metadata.PushPlanResult{VersionID: "VERSION_123"}, nil
	}
	readinessCalled := false
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	var requestOrder []string
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestOrder = append(requestOrder, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/relationships/app":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/preReleaseVersion":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"2.4.0","platform":"IOS"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	checkpointPath := filepath.Join(t.TempDir(), "stage-checkpoint.json")
	result, err := executeStage(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "MAC_OS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		CheckpointFile: checkpointPath,
	})
	if err == nil {
		t.Fatal("executeStage() error = nil, want a rejected platform mismatch")
	}
	if !strings.Contains(err.Error(), "BUILD_123") || !strings.Contains(err.Error(), "IOS") || !strings.Contains(err.Error(), "MAC_OS") {
		t.Fatalf("expected an error naming the build and both platforms, got %v", err)
	}
	if result.Status != "error" || result.FailedStep != stepValidateBuild {
		t.Fatalf("expected the dry-run to fail at the build check, got status %q step %q", result.Status, result.FailedStep)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected the dry-run to stop at the build check, got %#v", result.Steps)
	}
	if metadataCalled {
		t.Fatal("metadata must not be planned for a build on another platform")
	}
	if readinessCalled {
		t.Fatal("readiness must not run for a build on another platform")
	}
	if _, statErr := os.Stat(checkpointPath); !os.IsNotExist(statErr) {
		t.Fatalf("dry-run should not create a checkpoint, stat error = %v", statErr)
	}
	if len(requestOrder) != 2 || requestOrder[0] != "GET /v1/builds/BUILD_123/relationships/app" || requestOrder[1] != "GET /v1/builds/BUILD_123/preReleaseVersion" {
		t.Fatalf("expected only the ownership and platform reads, got %v", requestOrder)
	}
}

// TestExecuteStageRejectsBuildPlatformLookupErrorBeforeMutating proves an
// error while resolving the build platform is reported by validate_build and
// cannot fall through to version or metadata mutations.
func TestExecuteStageRejectsBuildPlatformLookupErrorBeforeMutating(t *testing.T) {
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
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataCalled = true
		return metadata.PushPlanResult{VersionID: "VERSION_123"}, nil
	}
	readinessCalled := false
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	var requestOrder []string
	var mutations []string
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestOrder = append(requestOrder, req.Method+" "+req.URL.Path)
		if req.Method != http.MethodGet {
			mutations = append(mutations, req.Method+" "+req.URL.Path)
		}
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/relationships/app":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/preReleaseVersion":
			return releaseJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"BAD_REQUEST","title":"Bad Request"}]}`)
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
		Platform:       "MAC_OS",
		Timeout:        releaseRunTimeout,
		Confirm:        true,
		CheckpointFile: filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err == nil {
		t.Fatal("executeStage() error = nil, want a platform lookup error")
	}
	if !strings.Contains(err.Error(), "validate build") || !strings.Contains(err.Error(), "BUILD_123") {
		t.Fatalf("expected a validate-build platform lookup error, got %v", err)
	}
	if result.FailedStep != stepValidateBuild || len(result.Steps) != 1 {
		t.Fatalf("expected the pipeline to stop at the build check, got status %q step %q steps %#v", result.Status, result.FailedStep, result.Steps)
	}
	if metadataCalled {
		t.Fatal("metadata must not run when build platform lookup fails")
	}
	if readinessCalled {
		t.Fatal("readiness must not run when build platform lookup fails")
	}
	if len(mutations) != 0 {
		t.Fatalf("expected no mutating requests, got %v (requests: %v)", mutations, requestOrder)
	}
	if len(requestOrder) != 2 || requestOrder[0] != "GET /v1/builds/BUILD_123/relationships/app" || requestOrder[1] != "GET /v1/builds/BUILD_123/preReleaseVersion" {
		t.Fatalf("expected app ownership then platform reads, got %v", requestOrder)
	}
}

// TestExecuteStageDryRunRejectsUnknownBuild proves the preview fails on a build
// that does not exist instead of reporting a clean plan for it.
func TestExecuteStageDryRunRejectsUnknownBuild(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata must not be planned for an unknown build")
		return metadata.PushPlanResult{}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_TYPO/relationships/app" {
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_TYPO",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		CheckpointFile: filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err == nil {
		t.Fatal("executeStage() error = nil, want an unusable build")
	}
	if !strings.Contains(err.Error(), "BUILD_TYPO") {
		t.Fatalf("expected an error naming the build, got %v", err)
	}
	if result.Status != "error" || result.FailedStep != stepValidateBuild {
		t.Fatalf("expected the dry-run to fail at the build check, got status %q step %q", result.Status, result.FailedStep)
	}
}

// TestExecuteStageValidatesBuildOnceBeforeAttaching proves the precondition read
// resolves the build the operator asked for and does not replace the
// attachment check that follows it.
func TestExecuteStageValidatesBuildOnceBeforeAttaching(t *testing.T) {
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

	metadataApplied := false
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataApplied = true
		return metadata.PushPlanResult{VersionID: "VERSION_123"}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	buildLookups := 0
	requestOrder := make([]string, 0, 4)
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestOrder = append(requestOrder, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/relationships/app":
			buildLookups++
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds/BUILD_123/preReleaseVersion":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"2.4.0","platform":"IOS"}}}`)
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
		CheckpointFile: filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeStage error: %v", err)
	}
	if !metadataApplied {
		t.Fatal("expected the pipeline to continue past the build check")
	}
	if buildLookups != 1 {
		t.Fatalf("expected exactly one build precondition read, got %d (%v)", buildLookups, requestOrder)
	}
	if len(result.Steps) != 5 || result.Steps[0].Name != stepValidateBuild {
		t.Fatalf("expected the build check to be reported first, got %#v", result.Steps)
	}
	details, ok := result.Steps[0].Details.(map[string]any)
	if !ok || details["buildPlatform"] != "IOS" {
		t.Fatalf("expected build platform in validation details, got %#v", result.Steps[0].Details)
	}
	if requestOrder[0] != "GET /v1/builds/BUILD_123/relationships/app" {
		t.Fatalf("expected the build to be resolved before anything else, got %v", requestOrder)
	}
}
