package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsListAllowsIndependentVersionAndBuildNumberFilters(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=1.2.3", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}},
					{"type":"preReleaseVersions","id":"prv-2"}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-1,prv-2", "filter[version]=77", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-77","attributes":{"version":"77"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--version", "1.2.3", "--build-number", "77"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-77"`) {
		t.Fatalf("expected filtered build in output, got %q", stdout)
	}
}

func TestBuildsListVersionLookupPaginatesAndUsesAllPreReleaseVersions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	const nextPreReleaseURL = "https://api.appstoreconnect.apple.com/v1/preReleaseVersions?cursor=AQ"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=2.0.0,2.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"2.0.0","platform":"IOS"}}],"links":{"next":"`+nextPreReleaseURL+`"}}`))
	reg.Register(asctest.GET("/v1/preReleaseVersions", "cursor=AQ"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-2","attributes":{"version":"2.0.0","platform":"MAC_OS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion]=prv-1,prv-2", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-2"}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--version", "2.0.0"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-2"`) {
		t.Fatalf("expected filtered build in output, got %q", stdout)
	}
}

func TestBuildsListNextWithPaginateStillFollowsCursor(t *testing.T) {
	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?cursor=AQ&filter%5Bapp%5D=123456789&include=preReleaseVersion"
	captured := buildsListQuerySurfaceStub(t)

	stdout, stderr, err := runBuildsListQuerySurface(
		t,
		"builds", "list",
		"--next", nextURL,
		"--paginate",
	)
	if err != nil {
		t.Fatalf("run error: %v (stderr=%q)", err, stderr)
	}
	if captured.path != "/v1/builds" {
		t.Fatalf("expected /v1/builds path, got %q", captured.path)
	}
	for param, want := range map[string]string{
		"cursor":      "AQ",
		"filter[app]": "123456789",
		"include":     "preReleaseVersion",
	} {
		if got := captured.query.Get(param); got != want {
			t.Fatalf("expected %s=%q on the followed next URL, got %q", param, want, got)
		}
	}
	if !strings.Contains(stdout, `"id":"build-1"`) {
		t.Fatalf("expected next-page build in output, got %q", stdout)
	}
}

func TestBuildsListVersionFilterNoResultsReturnsEmptyData(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=9.9.9", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"self":"https://api.appstoreconnect.apple.com/v1/builds"}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--version", "9.9.9"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected empty data output, got %q", stdout)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected only pre-release lookup request, got %d", requestCount)
	}
}

func TestBuildsListBuildNumberMapsToVersionFilter(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[version]=77", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-77","attributes":{"version":"77"}}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--build-number", "77"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"build-77"`) {
		t.Fatalf("expected build id in output, got %q", stdout)
	}
}
