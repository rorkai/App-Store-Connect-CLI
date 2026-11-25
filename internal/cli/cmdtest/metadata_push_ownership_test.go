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

func TestMetadataCommandsRejectAppInfoOwnedByAnotherApp(t *testing.T) {
	for _, command := range []string{"push", "pull", "keywords audit"} {
		t.Run(command, func(t *testing.T) {
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
				case "GET /v1/apps/100000001/appStoreVersions":
					return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersions","id":"version-a","attributes":{"versionString":"2.4.0","platform":"IOS"}}]}`)
				case "GET /v1/appInfos/info-b":
					if req.URL.Query().Get("include") != "app" {
						return nil, fmt.Errorf("expected ownership linkage in %s", req.URL.RequestURI())
					}
					return locUpdateJSONResponse(`{"data":{"type":"appInfos","id":"info-b","relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`)
				case "GET /v1/appInfos/info-b/appInfoLocalizations":
					return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Old"}}]}`)
				case "PATCH /v1/appInfoLocalizations/loc-b":
					return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Updated"}}}`)
				default:
					return nil, fmt.Errorf("unexpected request %s %s", req.Method, req.URL.RequestURI())
				}
			})

			args := append([]string{"metadata"}, strings.Fields(command)...)
			args = append(args, "--app", "100000001", "--app-info", "info-b", "--version", "2.4.0", "--platform", "IOS", "--output", "json")
			if command != "keywords audit" {
				args = append(args, "--dir", dir)
			}
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(args, "6.5.3")
			})
			if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, `belongs to app "100000002", not "100000001"`) {
				t.Errorf("cross-app metadata %s: exit=%d stdout=%q stderr=%q", command, code, stdout, stderr)
			}
			if len(requests) != 2 || requests[0] != "GET /v1/apps/100000001/appStoreVersions" || requests[1] != "GET /v1/appInfos/info-b" {
				t.Errorf("requests=%q, want only version resolution and App Info ownership lookup", requests)
			}
		})
	}
}
