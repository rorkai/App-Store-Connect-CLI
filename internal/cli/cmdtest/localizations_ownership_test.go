package cmdtest

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
					return locUpdateJSONResponse(`{"data":{"type":"appInfos","id":"info-b","relationships":{"app":{"data":{"type":"apps","id":"100000002"}}}}}`)
				case "GET /v1/appInfos/info-b/appInfoLocalizations":
					return locUpdateJSONResponse(`{"data":[{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Old"}}]}`)
				case "PATCH /v1/appInfoLocalizations/loc-b":
					return locUpdateJSONResponse(`{"data":{"type":"appInfoLocalizations","id":"loc-b","attributes":{"locale":"en-US","subtitle":"Updated"}}}`)
				}
				return nil, fmt.Errorf("unexpected request %s", request)
			})
			args := []string{"localizations", operation, "--type", "app-info", "--app", "100000001", "--app-info", "info-b", "--locale", "en-US", "--output", "json"}
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
			if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, `belongs to app "100000002", not "100000001"`) {
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
						if bundleID == "com.example.missing" {
							if req.URL.Query().Get("filter[name]") == "com.example.missing" {
								return locUpdateJSONResponse(`{"data":[{"type":"apps","id":"name-collision","attributes":{"bundleId":"com.other.app","name":"com.example.missing"}}]}`)
							}
							return locUpdateJSONResponse(`{"data":[{"type":"apps","id":"name-collision","attributes":{"bundleId":"com.other.app","name":"com.example.missing"}}]}`)
						}
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
		name         string
		args         []string
		matching     bool
		envApp       string
		wantError    string
		wantRequests int
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
		{
			name:         "configured bundle miss localizations upload",
			args:         []string{"localizations", "upload", "--version", "ver-1", "--path", inputPath},
			envApp:       "com.example.missing",
			wantError:    `no app found for bundle ID "com.example.missing" (exact match required)`,
			wantRequests: 1,
		},
		{
			name:         "configured bundle miss app setup upload",
			args:         []string{"app-setup", "localizations", "upload", "--version", "ver-1", "--path", inputPath},
			envApp:       "com.example.missing",
			wantError:    `no app found for bundle ID "com.example.missing" (exact match required)`,
			wantRequests: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupLocUpdateAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_APP_ID", test.envApp)
			var requests []string
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				request := req.Method + " " + req.URL.RequestURI()
				requests = append(requests, request)
				if req.Method == http.MethodGet && req.URL.Path == "/v1/apps" && req.URL.Query().Get("filter[bundleId]") == "com.example.missing" {
					return locUpdateJSONResponse(`{"data":[{"type":"apps","id":"name-collision","attributes":{"bundleId":"com.other.app","name":"com.example.missing"}}]}`)
				}
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
				wantError := test.wantError
				if wantError == "" {
					wantError = `belongs to app "9876543210", not "1234567890"`
				}
				if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, wantError) {
					t.Fatalf("exit=%d stdout=%q stderr=%q, want %q", code, stdout, stderr, wantError)
				}
				wantRequests := test.wantRequests
				if wantRequests == 0 {
					wantRequests = 1
				}
				if len(requests) != wantRequests || strings.Contains(strings.Join(requests, "\n"), "POST ") || strings.Contains(strings.Join(requests, "\n"), "PATCH ") {
					t.Fatalf("mismatch made requests=%v, want one ownership lookup and no write", requests)
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

func TestVersionLocalizationUploadsBindPaginationToVerifiedVersion(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "en-US.strings")
	if err := os.WriteFile(inputPath, []byte("\"description\" = \"Updated\";\n\"whatsNew\" = \"Notes\";\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "localizations", args: []string{"localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath}},
		{name: "app setup", args: []string{"app-setup", "localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath}},
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
					return locUpdateJSONResponse(`{"data":{"type":"appStoreVersions","id":"ver-1","relationships":{"app":{"data":{"type":"apps","id":"1234567890"}}}}}`)
				case "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
					if req.URL.Query().Get("cursor") == "" {
						return locUpdateJSONResponse(`{"data":[],"links":{"next":"/v1/appStoreVersions/ver-1/appStoreVersionLocalizations?cursor=page-2"}}`)
					}
					return locUpdateJSONResponse(`{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Updated","whatsNew":"Notes"}}],"links":{"next":""}}`)
				}
				return nil, fmt.Errorf("unexpected request %s", request)
			})
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(test.args, "test") })
			if code != cmd.ExitSuccess || stderr != "" {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(requests) != 3 || requests[1] != "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations?limit=200" || requests[2] != "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations?cursor=page-2" {
				t.Fatalf("requests=%v, want verified version and two same-resource pages", requests)
			}
		})
	}
}

func TestVersionLocalizationUploadsRejectPaginationDriftBeforeWrite(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "en-US.strings")
	if err := os.WriteFile(inputPath, []byte("\"description\" = \"Updated\";\n\"whatsNew\" = \"Notes\";\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		args []string
	}{
		{name: "localizations", args: []string{"localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath}},
		{name: "app setup", args: []string{"app-setup", "localizations", "upload", "--app", "1234567890", "--version", "ver-1", "--path", inputPath}},
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
					return locUpdateJSONResponse(`{"data":{"type":"appStoreVersions","id":"ver-1","relationships":{"app":{"data":{"type":"apps","id":"1234567890"}}}}}`)
				case "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
					return locUpdateJSONResponse(`{"data":[],"links":{"next":"/v1/appStoreVersions/ver-2/appStoreVersionLocalizations?cursor=drift"}}`)
				}
				return nil, fmt.Errorf("unexpected request %s", request)
			})
			var code int
			stdout, stderr := captureOutput(t, func() { code = cmd.Run(test.args, "test") })
			if code != cmd.ExitError || stdout != "" || !strings.Contains(stderr, "does not match expected resource path") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if len(requests) != 2 || strings.Contains(strings.Join(requests, "\n"), "POST ") || strings.Contains(strings.Join(requests, "\n"), "PATCH ") {
				t.Fatalf("requests=%v, want no downstream write or drift fetch", requests)
			}
		})
	}
}

func TestVersionLocalizationCreateConflictRejectsPaginationDrift(t *testing.T) {
	setupLocUpdateAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	var requests []string
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		request := req.Method + " " + req.URL.RequestURI()
		requests = append(requests, request)
		switch req.Method + " " + req.URL.Path {
		case "GET /v1/appStoreVersions/ver-1":
			return locUpdateJSONResponse(`{"data":{"type":"appStoreVersions","id":"ver-1","relationships":{"app":{"data":{"type":"apps","id":"1234567890"}}}}}`)
		case "POST /v1/appStoreVersionLocalizations":
			return &http.Response{Status: "409 Conflict", StatusCode: http.StatusConflict, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(localizationDuplicate409))}, nil
		case "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			return locUpdateJSONResponse(`{"data":[],"links":{"next":"/v1/appStoreVersions/ver-2/appStoreVersionLocalizations?cursor=drift"}}`)
		}
		return nil, fmt.Errorf("unexpected request %s", request)
	})
	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"localizations", "create", "--app", "1234567890", "--version", "ver-1", "--locale", "en-US", "--description", "Updated", "--if-exists", "skip", "--output", "json"}, "test")
	})
	if code != cmd.ExitConflict || stdout != "" || !strings.Contains(stderr, "does not match expected resource path") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(requests) != 3 || strings.Contains(strings.Join(requests, "\n"), "PATCH ") || strings.Contains(strings.Join(requests, "\n"), "ver-2") {
		t.Fatalf("requests=%v, want expected POST conflict then one scoped page only", requests)
	}
}

func TestVersionLocalizationUploadRefusesRedirectBeforeWrite(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "en-US.strings")
	if err := os.WriteFile(inputPath, []byte("\"description\" = \"Updated\";\n\"whatsNew\" = \"Fixture notes\";\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/v1/appStoreVersions/ver-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
		case "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
			http.Redirect(w, r, "/v1/appStoreVersions/other/appStoreVersionLocalizations", http.StatusTemporaryRedirect)
		case "/v1/appStoreVersions/other/appStoreVersionLocalizations":
			http.Error(w, "unexpected redirected version localization read", http.StatusInternalServerError)
		default:
			http.Error(w, "unexpected version localization request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	stdout, _, code := runWithDeps(t, stubServerClient(t, server), "localizations", "upload", "--app", "123456789", "--version", "ver-1", "--path", inputPath)
	if code != cmd.ExitError || stdout != "" {
		t.Fatalf("redirected version localization upload exit=%d stdout=%q, want error and no output", code, stdout)
	}
	mu.Lock()
	got := append([]string(nil), requests...)
	mu.Unlock()
	if !slices.Equal(got, []string{"GET /v1/appStoreVersions/ver-1", "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations"}) {
		t.Fatalf("version localization requests=%v, want scoped version proof and one refused collection read with no write", got)
	}
}

func TestVersionLocalizationUpdateRefusesRedirectBeforeWrite(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "version ID collection",
			args: []string{"localizations", "update", "--app", "123456789", "--version", "ver-1", "--locale", "en-US", "--description", "Updated"},
			want: []string{
				"GET /v1/appStoreVersions/ver-1",
				"GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations",
			},
		},
		{
			name: "version string resolution",
			args: []string{"localizations", "update", "--app", "123456789", "--version", "1.2.3", "--platform", "IOS", "--locale", "en-US", "--description", "Updated"},
			want: []string{"GET /v1/apps/123456789/appStoreVersions"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupLocUpdateAuth(t)
			var mu sync.Mutex
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				mu.Unlock()
				switch r.Method + " " + r.URL.Path {
				case "GET /v1/appStoreVersions/ver-1":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`)
				case "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
					http.Redirect(w, r, "/fixture/redirect-sentinel", http.StatusTemporaryRedirect)
				case "GET /v1/apps/123456789/appStoreVersions":
					http.Redirect(w, r, "/fixture/redirect-sentinel", http.StatusTemporaryRedirect)
				case "GET /fixture/redirect-sentinel":
					http.Error(w, "unexpected redirected read", http.StatusInternalServerError)
				default:
					http.Error(w, "unexpected request", http.StatusInternalServerError)
				}
			}))
			t.Cleanup(server.Close)

			stdout, _, code := runWithDeps(t, stubServerClient(t, server), tt.args...)
			if code != cmd.ExitError || stdout != "" {
				t.Fatalf("redirected version localization update exit=%d stdout=%q, want error and no output", code, stdout)
			}
			mu.Lock()
			got := append([]string(nil), requests...)
			mu.Unlock()
			if !slices.Equal(got, tt.want) {
				t.Fatalf("version localization update requests=%v, want scoped read refusal and no follow-up or write", got)
			}
			for _, request := range got {
				if strings.HasPrefix(request, "PATCH ") || strings.HasPrefix(request, "POST ") {
					t.Fatalf("version localization update sent a write after redirect: %v", got)
				}
			}
		})
	}
}
