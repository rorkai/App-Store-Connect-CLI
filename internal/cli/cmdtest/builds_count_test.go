package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsCountReturnsPagingTotal(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[],"meta":{"paging":{"total":42,"limit":1}}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "count", "--app", "123456789"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var out struct {
		AppID string `json:"appId"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.AppID != "123456789" {
		t.Fatalf("expected appId=123456789, got %q", out.AppID)
	}
	if out.Total != 42 {
		t.Fatalf("expected total=42, got %d", out.Total)
	}
}

func TestBuildsCountUsesVersionLookupAndFilters(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[platform]=IOS", "filter[version]=1.2.3", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[
					{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}},
					{"type":"preReleaseVersions","id":"prv-2"}
				],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/builds",
		"filter[app]=123456789",
		"filter[version]=77",
		"filter[preReleaseVersion.platform]=IOS",
		"filter[processingState]=PROCESSING,FAILED,INVALID,VALID",
		"filter[preReleaseVersion]=prv-1,prv-2",
		"limit=1",
	), asctest.JSON(http.StatusOK, `{"data":[],"meta":{"paging":{"total":3,"limit":1}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "count",
		"--app", "123456789",
		"--version", "1.2.3",
		"--build-number", "77",
		"--platform", "ios",
		"--processing-state", "all",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}

	var out struct {
		AppID string `json:"appId"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.AppID != "123456789" {
		t.Fatalf("expected appId=123456789, got %q", out.AppID)
	}
	if out.Total != 3 {
		t.Fatalf("expected total=3, got %d", out.Total)
	}
}

func TestBuildsCountVersionLookupNoMatchesReturnsZero(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/preReleaseVersions", "filter[app]=123456789", "filter[version]=9.9.9", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "count", "--app", "123456789", "--version", "9.9.9"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected only pre-release lookup request, got %d requests", requestCount)
	}

	var out struct {
		AppID string `json:"appId"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.AppID != "123456789" {
		t.Fatalf("expected appId=123456789, got %q", out.AppID)
	}
	if out.Total != 0 {
		t.Fatalf("expected total=0, got %d", out.Total)
	}
}

func TestBuildsCountFallsBackToPaginationWhenPagingTotalMissing(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_SPINNER_DISABLED", "1")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?filter%5Bapp%5D=123456789&limit=200&cursor=AQ"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=1"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"probe-build"}],"meta":{"paging":{"limit":1}}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-1"}],"links":{"next":"`+nextURL+`"}}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "limit=200", "cursor=AQ"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"builds","id":"build-2"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "count", "--app", "123456789"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestCount := len(reg.Requests()); requestCount != 3 {
		t.Fatalf("expected 3 requests, got %d", requestCount)
	}

	var out struct {
		AppID string `json:"appId"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout: %s", err, stdout)
	}
	if out.AppID != "123456789" {
		t.Fatalf("expected appId=123456789, got %q", out.AppID)
	}
	if out.Total != 2 {
		t.Fatalf("expected total=2, got %d", out.Total)
	}
}
