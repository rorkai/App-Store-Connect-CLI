package cmdtest

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReleaseStageUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "MissingMetadataSource",
			args:    []string{"release", "stage", "--app", "APP_123", "--version", "1.2.3", "--build-id", "BUILD_123", "--dry-run"},
			wantErr: "exactly one of --metadata-dir or --copy-metadata-from is required",
			deps:    true,
		},
		{
			name:    "InvalidCopyFieldsValue",
			args:    []string{"release", "stage", "--app", "APP_123", "--version", "1.2.3", "--build-id", "BUILD_123", "--copy-metadata-from", "1.2.2", "--copy-fields", "description,notAField", "--dry-run"},
			wantErr: "--copy-fields must be one of",
			deps:    true,
		},
	})
}

func TestReleaseStage_DryRunCopyMetadataFromVersion(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/builds/BUILD_123/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123"), asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123/preReleaseVersion"), asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.3","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_123/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), []string{
		"release", "stage",
		"--app", "APP_123",
		"--version", "1.2.3",
		"--build-id", "BUILD_123",
		"--copy-metadata-from", "1.2.2",
		"--copy-fields", "description,keywords",
		"--exclude-fields", "keywords",
		"--dry-run",
	}...)
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
	reg.Register(asctest.GET("/v1/builds/BUILD_123/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123"), asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123/preReleaseVersion"), asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.3","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_123/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.3", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), []string{
		"release", "stage",
		"--app", "APP_123",
		"--version", "1.2.3",
		"--build-id", "BUILD_123",
		"--copy-metadata-from", "1.2.2",
		"--checkpoint-file", "  stage-checkpoint.json  ",
		"--dry-run",
	}...)
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

// TestReleaseStage_ExplainsRefusedVersionCreate: while another version of the
// platform is unreleased, App Store Connect refuses to create one with a 409
// that names neither. The stage names that version, as asc versions create
// does, in its error and in its JSON result.
func TestReleaseStage_ExplainsRefusedVersionCreate(t *testing.T) {
	setupAuth(t)
	t.Chdir(t.TempDir())

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/builds/BUILD_123/relationships/app"), asctest.JSON(http.StatusOK, `{"data":{"type":"apps","id":"APP_123"}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123"), asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`))
	reg.Register(asctest.GET("/v1/builds/BUILD_123/preReleaseVersion"), asctest.JSON(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"PRE_RELEASE_123","attributes":{"version":"1.2.4","platform":"IOS"}}}`))
	reg.Register(asctest.GET("/v1/apps/APP_123/appStoreVersions", "filter[platform]=IOS", "filter[versionString]=1.2.4", "limit=10"), asctest.JSON(http.StatusOK, `{"data":[]}`))
	reg.Register(asctest.POST("/v1/appStoreVersions"), asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.RELATIONSHIP.INVALID","title":"The provided entity includes a relationship with an invalid value","detail":"You cannot create a new version of the App in the current state."}]}`))
	reg.Register(asctest.GET("/v1/apps/APP_123/appStoreVersions", "filter[platform]=IOS", "limit=200"), asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"platform":"IOS","versionString":"1.2.3","appVersionState":"REJECTED","appStoreState":"REJECTED","createdDate":"2026-11-03T18:45:51-08:00"}}]}`))

	stdout, _, err := runRootWithDeps(t, stubClient(t, reg), []string{
		"release", "stage",
		"--app", "APP_123",
		"--version", "1.2.4",
		"--build-id", "BUILD_123",
		"--copy-metadata-from", "1.2.2",
		"--confirm",
	}...)
	if !errors.Is(err, asc.ErrConflict) {
		t.Fatalf("error = %v, want App Store Connect's 409", err)
	}
	want := strings.Join([]string{
		"IOS versions that are not live yet (an unreleased version usually blocks creating another):",
		"  1.2.3  IOS  REJECTED  VERSION_123",
		`To ship "1.2.4" from editable version 1.2.3, rename it: asc versions update --version-id "VERSION_123" --version "1.2.4"`,
	}, "\n")
	if !strings.HasSuffix(err.Error(), "You cannot create a new version of the App in the current state.\n"+want) {
		t.Fatalf("error =\n%s\nwant the refusal followed by\n%s", err.Error(), want)
	}
	var result struct {
		Status     string `json:"status"`
		FailedStep string `json:"failedStep"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not the stage result: %v\n%s", err, stdout)
	}
	if result.Status != "error" || result.FailedStep != "ensure_version" || !strings.HasSuffix(result.Error, want) {
		t.Fatalf("stage result = %+v, want ensure_version failed with the explanation", result)
	}
}
