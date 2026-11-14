package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsListMatchesEquivalentMarketingVersionForPlatform(t *testing.T) {
	setupAuth(t)
	shared.ResetEquivalentVersionNotesForTest()
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=MAC_OS", "filter[version]=98.76.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=MAC_OS", "filter[version]=98.76", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-equivalent","attributes":{"version":"98.76","platform":"MAC_OS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=MAC_OS", "filter[preReleaseVersion]=prv-equivalent", "include=preReleaseVersion"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-equivalent"}]}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--version", "98.76.0", "--platform", "MAC_OS"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	if !strings.Contains(stdout, `"id":"build-equivalent"`) {
		t.Fatalf("expected equivalent-version build, got %q", stdout)
	}
	if !strings.Contains(stderr, `matched version "98.76" for requested "98.76.0"`) {
		t.Fatalf("expected equivalent-version note, got %q", stderr)
	}
}

func TestBuildsCountMatchesEquivalentMarketingVersionForPlatform(t *testing.T) {
	setupAuth(t)
	shared.ResetEquivalentVersionNotesForTest()
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=MAC_OS", "filter[version]=87.65.0", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=MAC_OS", "filter[version]=87.65", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"preReleaseVersions","id":"prv-count-equivalent","attributes":{"version":"87.65","platform":"MAC_OS"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "filter[preReleaseVersion.platform]=MAC_OS", "filter[preReleaseVersion]=prv-count-equivalent", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"meta":{"paging":{"total":4,"limit":1}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "count", "--app", "123456789", "--version", "87.65.0", "--platform", "MAC_OS"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
	var result struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if result.Total != 4 {
		t.Fatalf("total = %d, want 4", result.Total)
	}
	if !strings.Contains(stderr, `matched version "87.65" for requested "87.65.0"`) {
		t.Fatalf("expected equivalent-version note, got %q", stderr)
	}
}
