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

func TestAppInfoLocalizationWritesRejectOtherApp(t *testing.T) {
	for _, operation := range []string{"update", "upload"} {
		t.Run(operation, func(t *testing.T) {
			setupLocUpdateAuth(t)
			var requests []string
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				request := req.Method + " " + req.URL.RequestURI()
				requests = append(requests, request)
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/appInfos/info-b":
					return locUpdateJSONResponse(`{"data":{"type":"appInfos","id":"info-b","relationships":{"app":{"data":{"type":"apps","id":"app-b"}}}}}`)
				case "GET /v1/appInfos/info-b/appInfoLocalizations":
					return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Old"}}]}`)
				case "PATCH /v1/appInfoLocalizations/loc-b":
					return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Updated"}}}`)
				}
				return nil, fmt.Errorf("unexpected request %s", request)
			})
			args := []string{"localizations", operation, "--type", "app-info", "--app", "app-a", "--app-info", "info-b", "--locale", "en-US", "--output", "json"}
			if operation == "update" {
				args = append(args, "--subtitle", "Updated")
			} else {
				path := filepath.Join(t.TempDir(), "en-US.strings")
				if err := os.WriteFile(path, []byte("\"subtitle\" = \"Updated\";\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--path", path)
			}
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(args, "6.5.3")
			})
			if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, `belongs to app "app-b", not "app-a"`) {
				t.Errorf("cross-app %s: exit=%d stdout=%q stderr=%q", operation, code, stdout, stderr)
			}
			if len(requests) != 1 || requests[0] != "GET /v1/appInfos/info-b?include=app" {
				t.Errorf("cross-app %s requests=%q, want only App Info ownership lookup", operation, requests)
			}
		})
	}
}
