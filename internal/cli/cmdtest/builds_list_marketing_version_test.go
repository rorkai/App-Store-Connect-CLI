package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsListIncludesPreReleaseVersion(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "include=preReleaseVersion"), asctest.JSON(http.StatusOK, `{
				"data":[{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}
				}],
				"included":[{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}]
			}`))

	stdout, _, err := runCommand(t, []string{"builds", "list", "--app", "123456789"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	var capturedPath, capturedAppFilter, includeValue string
	for _, req := range reg.Requests() {
		capturedPath = req.URL.Path
		capturedAppFilter = req.URL.Query().Get("filter[app]")
		includeValue = req.URL.Query().Get("include")
	}
	if !strings.Contains(includeValue, "preReleaseVersion") {
		t.Fatalf("expected include=preReleaseVersion in API request, got %q", includeValue)
	}
	if capturedPath != "/v1/builds" {
		t.Fatalf("expected /v1/builds when include=preReleaseVersion is requested, got %q", capturedPath)
	}
	if capturedAppFilter != "123456789" {
		t.Fatalf("expected filter[app]=123456789 on /v1/builds request, got %q", capturedAppFilter)
	}
	if !strings.Contains(stdout, "1.2.3") {
		t.Fatalf("expected marketing version 1.2.3 in output, got %q", stdout)
	}
}

func TestBuildsListPaginateKeepsPreReleaseVersionOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_SPINNER_DISABLED", "1")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/builds?filter%5Bapp%5D=123456789&include=preReleaseVersion&limit=200&cursor=AQ"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "include=preReleaseVersion", "limit=200"), asctest.JSON(http.StatusOK, `{
				"data":[{
					"type":"builds",
					"id":"build-1",
					"attributes":{"version":"9","uploadedDate":"2026-03-13T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-1"}}}
				}],
				"included":[{
					"type":"preReleaseVersions",
					"id":"prv-1",
					"attributes":{"version":"1.2.3","platform":"TV_OS"}
				}],
				"links":{"next":"`+nextURL+`"}
			}`))
	reg.Register(asctest.GET("/v1/builds", "filter[app]=123456789", "include=preReleaseVersion", "limit=200", "cursor=AQ"), asctest.JSON(http.StatusOK, `{
				"data":[{
					"type":"builds",
					"id":"build-2",
					"attributes":{"version":"10","uploadedDate":"2026-03-14T00:00:00Z","processingState":"VALID","expired":false},
					"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":"prv-2"}}}
				}],
				"included":[{
					"type":"preReleaseVersions",
					"id":"prv-2",
					"attributes":{"version":"2.0.0","platform":"IOS"}
				}],
				"links":{"next":""}
			}`))

	stdout, stderr, err := runCommand(t, []string{"builds", "list", "--app", "123456789", "--paginate", "--output", "table"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "1.2.3") || !strings.Contains(stdout, "2.0.0") {
		t.Fatalf("expected paginated table output to contain both marketing versions, got %q", stdout)
	}
	if !strings.Contains(stdout, "TV_OS") || !strings.Contains(stdout, "IOS") {
		t.Fatalf("expected paginated table output to contain both platforms, got %q", stdout)
	}
}
