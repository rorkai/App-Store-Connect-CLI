package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReleaseStageUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "MissingMetadataSource",
			args:    []string{"release", "stage", "--app", "APP_123", "--version", "1.2.3", "--build-id", "BUILD_123", "--dry-run"},
			wantErr: "exactly one of --metadata-dir or --copy-metadata-from is required",
		},
		{
			name:    "InvalidCopyFieldsValue",
			args:    []string{"release", "stage", "--app", "APP_123", "--version", "1.2.3", "--build-id", "BUILD_123", "--copy-metadata-from", "1.2.2", "--copy-fields", "description,notAField", "--dry-run"},
			wantErr: "--copy-fields must be one of",
		},
	})
}

func TestReleaseStage_DryRunCopyMetadataFromVersion(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/BUILD_123/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123"), asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123/preReleaseVersion"), asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.3","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_123/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, err := runCommand(t, []string{
		"release", "stage",
		"--app", "APP_123",
		"--version", "1.2.3",
		"--build-id", "BUILD_123",
		"--copy-metadata-from", "1.2.2",
		"--copy-fields", "description,keywords",
		"--exclude-fields", "keywords",
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"status":"dry-run"`) {
		t.Fatalf("expected dry-run status, got %q", stdout)
	}
	if !strings.Contains(stdout, `"copyMetadataFrom":"1.2.2"`) {
		t.Fatalf("expected copy metadata source in output, got %q", stdout)
	}
	if !strings.Contains(stdout, `"message":"would create app store version"`) {
		t.Fatalf("expected ensure version dry-run message, got %q", stdout)
	}
	if !strings.Contains(stdout, `"message":"metadata copy plan deferred until version exists"`) {
		t.Fatalf("expected deferred metadata copy message, got %q", stdout)
	}
	if !strings.Contains(stdout, `"name":"validate_build","status":"dry-run"`) {
		t.Fatalf("expected the build precondition check in the plan, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 4 {
		t.Fatalf("expected the build ownership, platform, and audience checks plus the version lookup, got %d requests", requestCount)
	}
}

// TestReleaseStage_ResolvesTrimmedCheckpointFile pins the CLI surface for
// --checkpoint-file: like every other path flag it is trimmed, so the reported
// and used checkpoint path never carries the operator's surrounding whitespace.
func TestReleaseStage_ResolvesTrimmedCheckpointFile(t *testing.T) {
	setupAuth(t)

	workDir := t.TempDir()
	t.Chdir(workDir)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/BUILD_123/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123"), asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123/preReleaseVersion"), asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.3","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_123/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, _, err := runCommand(t, []string{
		"release", "stage",
		"--app", "APP_123",
		"--version", "1.2.3",
		"--build-id", "BUILD_123",
		"--copy-metadata-from", "1.2.2",
		"--checkpoint-file", "  stage-checkpoint.json  ",
		"--dry-run",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var plan struct {
		CheckpointFile string `json:"checkpointFile"`
	}
	if err := json.Unmarshal([]byte(stdout), &plan); err != nil {
		t.Fatalf("parse plan output %q: %v", stdout, err)
	}
	if !filepath.IsAbs(plan.CheckpointFile) {
		t.Fatalf("expected an absolute checkpoint path, got %q", plan.CheckpointFile)
	}
	if filepath.Base(plan.CheckpointFile) != "stage-checkpoint.json" {
		t.Fatalf("expected the checkpoint file name to be trimmed, got %q", plan.CheckpointFile)
	}
}
