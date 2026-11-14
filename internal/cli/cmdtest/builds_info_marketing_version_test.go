package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsInfoTableShowsMarketingVersionAndPlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1"), asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}
				}
			}`))
	reg.Register(asctest.GET("/v1/builds/build-1/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}
			}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--build-id", "build-1", "--output", "table"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg, "GET /v1/builds/build-1", "GET /v1/builds/build-1/preReleaseVersion")
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "1.2.3") || !strings.Contains(stdout, "TV_OS") {
		t.Fatalf("expected builds info table output to include marketing version and platform, got %q", stdout)
	}
}

func TestBuildsInfoJSONPreservesExistingRelationships(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds/build-1"), asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{
						"app":{"links":{"related":"https://example.com/apps/app-1"}}
					}
				}
			}`))
	reg.Register(asctest.GET("/v1/builds/build-1/preReleaseVersion"), asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}
			}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "info", "--build-id", "build-1", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	assertBuildsRequestOrder(t, reg, "GET /v1/builds/build-1", "GET /v1/builds/build-1/preReleaseVersion")
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got %v", err)
	}

	data, ok := payload["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected object data payload, got %#v", payload["data"])
	}
	relationships, ok := data["relationships"].(map[string]any)
	if !ok {
		t.Fatalf("expected relationships object, got %#v", data["relationships"])
	}
	if _, ok := relationships["app"]; !ok {
		t.Fatalf("expected existing app relationship to be preserved, got %#v", relationships)
	}
	if _, ok := relationships["preReleaseVersion"]; !ok {
		t.Fatalf("expected preReleaseVersion relationship to be attached, got %#v", relationships)
	}
}

func TestBuildsInfoBuildNumberRequiresUniqueMatch(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(buildsByNumberIOS42, asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"builds","id":"build-ios","attributes":{"version":"42","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false}},
				{"type":"builds","id":"build-macos","attributes":{"version":"42","uploadedDate":"2026-03-12T00:00:00Z","processingState":"VALID","expired":false}}
			]
		}`))

	stdout, stderr, runErr := runCommand(t, []string{"builds", "info", "--app", "123456789", "--build-number", "42", "--platform", "IOS", "--output", "json"})

	if runErr == nil {
		t.Fatal("expected unique build-number lookup error")
	}
	if !isUsageClassError(runErr) {
		t.Fatalf("expected ambiguity to be a usage error, got %v", runErr)
	}
	assertUsageDiagnosticFirstLine(t, stderr, `2 builds match build number "42" for platform IOS for app 123456789; pass --build-id with one of:`)
	if !strings.Contains(runErr.Error(), `2 builds match build number "42" for platform IOS for app 123456789; pass --build-id with one of:`) {
		t.Fatalf("expected ambiguity error, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "build-ios") || !strings.Contains(runErr.Error(), "build-macos") {
		t.Fatalf("expected candidate build IDs in the ambiguity error, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout on ambiguity error, got %q", stdout)
	}
}
