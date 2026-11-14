package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsListPlatformFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=TV_OS", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-tvos-1","attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--platform", "TV_OS"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-tvos-1"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
}

func TestBuildsListPlatformFilterWithVersionLookup(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=IOS", "filter[version]=1.2.3", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"preReleaseVersions","id":"prv-ios","attributes":{"version":"1.2.3","platform":"IOS"}},
					{"type":"preReleaseVersions","id":"prv-vision","attributes":{"version":"1.2.3","platform":"VISION_OS"}}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=IOS", "filter[preReleaseVersion]=prv-ios,prv-vision", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-ios-1"}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--version", "1.2.3", "--platform", "ios"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-ios-1"`) {
		t.Fatalf("expected build output, got %q", stdout)
	}
}
