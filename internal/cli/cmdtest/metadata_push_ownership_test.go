package cmdtest

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestMetadataPushRejectsAppInfoOwnedByAnotherApp(t *testing.T) {
	setupLocUpdateAuth(t)
	dir := t.TempDir()
	appInfoDir := filepath.Join(dir, "app-info")
	if err := os.Mkdir(appInfoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appInfoDir, "en-US.json"), []byte(`{"subtitle":"Updated"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var requests []string
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/apps/app-a/appStoreVersions":
			return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-a","attributes":{"versionString":"2.4.0","platform":"IOS"}}]}`)
		case "GET /v1/appInfos/info-b":
			if req.URL.Query().Get("include") != "app" {
				return nil, fmt.Errorf("expected ownership linkage in %s", req.URL.RequestURI())
			}
			return locUpdateJSONResponse(`{"data":{"type":"appInfos","id":"info-b","relationships":{"app":{"data":{"type":"apps","id":"app-b"}}}}}`)
		case "GET /v1/appInfos/info-b/appInfoLocalizations":
			return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Old"}}]}`)
		case "PATCH /v1/appInfoLocalizations/loc-b":
			return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Updated"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL.RequestURI())
		}
	})

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"metadata", "push", "--app", "app-a", "--app-info", "info-b", "--version", "2.4.0", "--platform", "IOS", "--dir", dir, "--output", "json"}, "6.5.3")
	})
	if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, `belongs to app "app-b", not "app-a"`) {
		t.Errorf("cross-app metadata push: exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(requests) != 2 || requests[0] != "GET /v1/apps/app-a/appStoreVersions" || requests[1] != "GET /v1/appInfos/info-b" {
		t.Errorf("requests=%q, want only version resolution and App Info ownership lookup", requests)
	}
}
