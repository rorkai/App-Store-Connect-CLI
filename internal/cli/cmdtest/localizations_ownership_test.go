package cmdtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
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

func TestVersionLocalizationWritesCheckExplicitApp(t *testing.T) {
	const lookup = "GET /v1/appStoreVersions/ver-1?include=app"
	const list = "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations?limit=200"
	const created = "POST /v1/appStoreVersionLocalizations"
	const updated = "PATCH /v1/appStoreVersionLocalizations/loc-1"
	for _, operation := range []string{"create", "update"} {
		for _, tt := range []struct {
			name         string
			app          string
			relationship string
			wantError    string
		}{
			{"matching", "app-1", `,"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}`, ""},
			{"matching app self-link", "https://api.appstoreconnect.apple.com/v1/apps/app-1", `,"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}`, ""},
			{"mismatched", "app-1", `,"relationships":{"app":{"data":{"type":"apps","id":"app-2"}}}`, `belongs to app "app-2", not "app-1"`},
			{"missing", "app-1", "", `app relationship missing for app store version "ver-1"`},
		} {
			t.Run(operation+"/"+tt.name, func(t *testing.T) {
				setupLocUpdateAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
				var requests []string
				stubTransport(t, func(req *http.Request) (*http.Response, error) {
					request := req.Method + " " + req.URL.RequestURI()
					requests = append(requests, request)
					switch request {
					case lookup:
						return locUpdateJSONResponse(`{"data":{"type":"appStoreVersions","id":"ver-1"` + tt.relationship + `}}`)
					case list:
						return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
					case created, updated:
						return locUpdateJSONResponse(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"New"}}}`)
					default:
						return nil, fmt.Errorf("unexpected request %s", request)
					}
				})
				args := []string{"localizations", operation, "--app", tt.app, "--version", "ver-1", "--locale", "en-US", "--whats-new", "New", "--description", "Description", "--keywords", "focus", "--support-url", "https://example.com/support", "--output", "json"}
				var code int
				stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
				wantRequests := []string{lookup}
				if tt.wantError != "" {
					if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, tt.wantError) {
						t.Errorf("exit=%d stdout=%q stderr=%q, want ownership failure %q", code, stdout, stderr, tt.wantError)
					}
				} else {
					if operation == "create" {
						wantRequests = append(wantRequests, created)
					} else {
						wantRequests = append(wantRequests, list, updated)
					}
					var receipt struct {
						Data struct{ ID string }
					}
					if err := json.Unmarshal([]byte(stdout), &receipt); err != nil || receipt.Data.ID != "loc-1" || code != cmd.ExitSuccess || stderr != "" {
						t.Errorf("exit=%d stdout=%q stderr=%q, want localization receipt", code, stdout, stderr)
					}
				}
				if !slices.Equal(requests, wantRequests) {
					t.Errorf("requests=%v, want %v", requests, wantRequests)
				}
			})
		}
	}
}
