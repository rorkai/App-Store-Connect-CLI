package cmdtest

import (
	"cmp"
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

func TestVersionLocalizationWritesCheckAppOwnership(t *testing.T) {
	const lookup = "GET /v1/appStoreVersions/ver-1?include=app"
	const list = "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations?limit=200"
	const created = "POST /v1/appStoreVersionLocalizations"
	const updated = "PATCH /v1/appStoreVersionLocalizations/loc-1"
	const versionData = `"type":"appStoreVersions","id":"ver-1"`
	const matchingApp = `,"relationships":{"app":{"data":{"type":"apps","id":"1234567890"}}}`
	bundleApps := map[string]string{"com.example.owner": "1234567890", "com.example.other": "9876543210", "com.example.own": "1234567890"}
	// Apple's filter[bundleId] also returns apps whose bundle ID starts with the value.
	const prefixSiblings = `{"data":[{"type":"apps","id":"9876543210","attributes":{"bundleId":"com.example.owner"}},{"type":"apps","id":"1234567890","attributes":{"bundleId":"com.example.own"}}]}`
	for _, operation := range []string{"create", "update"} {
		for _, tt := range []struct {
			name      string
			app       string
			envApp    string
			data      string
			wantError string
		}{
			{"matching", "1234567890", "", versionData + matchingApp, ""},
			{"matching app self-link", "https://api.appstoreconnect.apple.com/v1/apps/1234567890", "", versionData + matchingApp, ""},
			{"mismatched", "1234567890", "", versionData + `,"relationships":{"app":{"data":{"type":"apps","id":"9876543210"}}}`, `belongs to app "9876543210", not "1234567890"`},
			{"missing", "1234567890", "", versionData, `app relationship missing for app store version "ver-1"`},
			{"matching environment app", "", "https://api.appstoreconnect.apple.com/v1/apps/1234567890", versionData + matchingApp, ""},
			{"mismatched environment app", "", "9876543210", versionData + matchingApp, `belongs to app "1234567890", not "9876543210"`},
			{"flag overrides environment app", "1234567890", "9876543210", versionData + matchingApp, ""},
			{"no app selected", "", "", versionData + matchingApp, ""},
			{"matching bundle ID", "com.example.owner", "", versionData + matchingApp, ""},
			{"matching environment bundle ID", "", "com.example.owner", versionData + matchingApp, ""},
			{"mismatched environment bundle ID", "", "com.example.other", versionData + matchingApp, `belongs to app "1234567890", not "9876543210"`},
			{"environment bundle ID that another app's bundle ID starts with", "", "com.example.own", versionData + matchingApp, ""},
			{"missing version ID", "1234567890", "", `"type":"appStoreVersions"` + matchingApp, `expected app store version "ver-1", got ""`},
			{"different version ID", "1234567890", "", `"type":"appStoreVersions","id":"ver-2"` + matchingApp, `expected app store version "ver-1", got "ver-2"`},
			{"wrong version type", "1234567890", "", `"type":"appInfos","id":"ver-1"` + matchingApp, `expected resource type "appStoreVersions", got "appInfos"`},
			{"wrong app relationship type", "1234567890", "", versionData + `,"relationships":{"app":{"data":{"type":"builds","id":"1234567890"}}}`, `expected app relationship type "apps", got "builds"`},
		} {
			t.Run(operation+"/"+tt.name, func(t *testing.T) {
				setupLocUpdateAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
				t.Setenv("ASC_APP_ID", tt.envApp)
				var requests []string
				stubTransport(t, func(req *http.Request) (*http.Response, error) {
					request := req.Method + " " + req.URL.RequestURI()
					requests = append(requests, request)
					if req.URL.Path == "/v1/apps" {
						if req.URL.Query().Get("filter[bundleId]") == "com.example.own" {
							return locUpdateJSONResponse(prefixSiblings)
						}
						bundleID := req.URL.Query().Get("filter[bundleId]")
						if appID := bundleApps[bundleID]; appID != "" {
							return locUpdateJSONResponse(`{"data":[{"type":"apps","id":"` + appID + `","attributes":{"bundleId":"` + bundleID + `"}}]}`)
						}
					}
					switch request {
					case lookup:
						return locUpdateJSONResponse(`{"data":{` + tt.data + `}}`)
					case list:
						return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
					case created, updated:
						return locUpdateJSONResponse(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","whatsNew":"New"}}}`)
					default:
						return nil, fmt.Errorf("unexpected request %s", request)
					}
				})
				args := []string{"localizations", operation, "--version", "ver-1", "--locale", "en-US", "--whats-new", "New", "--description", "Description", "--keywords", "focus", "--support-url", "https://example.com/support", "--output", "json"}
				if tt.app != "" {
					args = append(args, "--app", tt.app)
				}
				var code int
				stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "test") })
				var wantRequests []string
				selectedApp := cmp.Or(tt.app, tt.envApp)
				if _, isBundleID := bundleApps[selectedApp]; isBundleID {
					wantRequests = append(wantRequests, "GET /v1/apps?filter%5BbundleId%5D="+selectedApp+"&limit=200")
				}
				if selectedApp != "" {
					wantRequests = append(wantRequests, lookup)
				}
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

func TestVersionLocalizationUploadsCheckAppOwnership(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "en-US.strings")
	if err := os.WriteFile(inputPath, []byte("\"description\" = \"Updated\";\n\"whatsNew\" = \"Notes\";\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name     string
		args     []string
		matching bool
	}{
		{
			name: "localizations mismatch",
			args: []string{"localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath},
		},
		{
			name: "app setup mismatch",
			args: []string{"app-setup", "localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath},
		},
		{
			name:     "localizations matching upload",
			args:     []string{"localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath},
			matching: true,
		},
		{
			name:     "app setup matching upload",
			args:     []string{"app-setup", "localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath},
			matching: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupLocUpdateAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			var requests []string
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				request := req.Method + " " + req.URL.RequestURI()
				requests = append(requests, request)
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/appStoreVersions/ver-1":
					return locUpdateJSONResponse(`{"data":{"type":"appStoreVersions","id":"ver-1","relationships":{"app":{"data":{"type":"apps","id":"` + map[bool]string{false: "9876543210", true: "1234567890"}[test.matching] + `"}}}}}`)
				case "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
					return locUpdateJSONResponse(`{"data":[],"links":{"next":""}}`)
				case "POST /v1/appStoreVersionLocalizations":
					return locUpdateJSONResponse(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated","whatsNew":"Notes"}}}`)
				}
				return nil, fmt.Errorf("unexpected request %s", request)
			})

			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(test.args, "test") })
			if !test.matching {
				if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, `belongs to app "9876543210", not "1234567890"`) {
					t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				if len(requests) != 1 {
					t.Fatalf("mismatch made %d requests, want one ownership lookup: %v", len(requests), requests)
				}
				return
			}
			if code != cmd.ExitSuccess || stdout == "" {
				t.Fatalf("matching upload exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(requests) != 3 || requests[1] != "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations?limit=200" || requests[2] != "POST /v1/appStoreVersionLocalizations" {
				t.Fatalf("matching upload requests=%v", requests)
			}
		})
	}
}
