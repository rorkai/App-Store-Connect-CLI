package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const localizationsListVersionUUID = "3f0e8b8e-1c2d-4e5f-8a9b-0c1d2e3f4a5b"

func installLocalizationsVersionStringTransport(t *testing.T, versionsByQuery map[string]string, log *[]string) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/123456789/appStoreVersions":
			key := appStoreVersionsQueryKey(req.URL.Query())
			*log = append(*log, req.URL.Path+"?"+key)
			body, ok := versionsByQuery[key]
			if !ok {
				t.Fatalf("unexpected versions query %q", req.URL.RawQuery)
			}
			return jsonResponse(http.StatusOK, body)
		case "/v1/appStoreVersions/ver-1/appStoreVersionLocalizations",
			"/v1/appStoreVersions/" + localizationsListVersionUUID + "/appStoreVersionLocalizations":
			*log = append(*log, req.URL.Path)
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}],"links":{"next":""}}`)
		default:
			t.Fatalf("unexpected path: %s", req.URL.Path)
			return nil, nil
		}
	})
}

func TestLocalizationsListResolvesVersionStringWithApp(t *testing.T) {
	var log []string
	installLocalizationsVersionStringTransport(t, map[string]string{
		"filter[versionString]=1.0": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`,
	}, &log)

	stdout, stderr, runErr := runRootCommand(t, []string{"localizations", "list", "--app", "123456789", "--version", "1.0", "--output", "json"})
	if runErr != nil {
		t.Fatalf("Run() error = %v (stderr=%q)", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"loc-1"`) {
		t.Fatalf("expected localization envelope, got %q", stdout)
	}
	want := "/v1/apps/123456789/appStoreVersions?filter[versionString]=1.0,/v1/appStoreVersions/ver-1/appStoreVersionLocalizations"
	if got := strings.Join(log, ","); got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestLocalizationsListResolvesVersionStringWithPlatform(t *testing.T) {
	var log []string
	installLocalizationsVersionStringTransport(t, map[string]string{
		"filter[platform]=MAC_OS&filter[versionString]=1.0": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS","versionString":"1.0"}}],"links":{"next":""}}`,
	}, &log)

	_, stderr, runErr := runRootCommand(t, []string{"localizations", "list", "--app", "123456789", "--version", "1.0", "--platform", "mac_os"})
	if runErr != nil {
		t.Fatalf("Run() error = %v (stderr=%q)", runErr, stderr)
	}
	want := "/v1/apps/123456789/appStoreVersions?filter[platform]=MAC_OS&filter[versionString]=1.0,/v1/appStoreVersions/ver-1/appStoreVersionLocalizations"
	if got := strings.Join(log, ","); got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestLocalizationsListPrefersVersionIDWhenValueIsResourceID(t *testing.T) {
	var log []string
	installLocalizationsVersionStringTransport(t, map[string]string{}, &log)

	_, stderr, runErr := runRootCommand(t, []string{"localizations", "list", "--app", "123456789", "--version", localizationsListVersionUUID})
	if runErr != nil {
		t.Fatalf("Run() error = %v (stderr=%q)", runErr, stderr)
	}
	want := "/v1/appStoreVersions/" + localizationsListVersionUUID + "/appStoreVersionLocalizations"
	if got := strings.Join(log, ","); got != want {
		t.Fatalf("request sequence = %q, want %q", got, want)
	}
}

func TestLocalizationsListVersionStringErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		versions   map[string]string
		wantStderr string
		wantExit   int
	}{
		{
			name: "ambiguous across platforms",
			args: []string{"localizations", "list", "--app", "123456789", "--version", "1.0"},
			versions: map[string]string{
				"filter[versionString]=1.0": `{"data":[
					{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0"}},
					{"type":"appStoreVersions","id":"ver-2","attributes":{"platform":"MAC_OS","versionString":"1.0"}}
				],"links":{"next":""}}`,
			},
			wantStderr: "--platform",
			wantExit:   rootcmd.ExitUsage,
		},
		{
			name: "version string not found",
			args: []string{"localizations", "list", "--app", "123456789", "--version", "9.9"},
			versions: map[string]string{
				"filter[versionString]=9.9": `{"data":[],"links":{"next":""}}`,
				// The not-found diagnostic lists the app's existing versions.
				"": `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"READY_FOR_DISTRIBUTION"}}],"links":{"next":""}}`,
			},
			wantStderr: "app store version not found for version \"9.9\"\nExisting App Store versions for app \"123456789\" (newest first):\n  1.0  IOS  READY_FOR_DISTRIBUTION  ver-1\n",
			wantExit:   rootcmd.ExitNotFound,
		},
		{
			name:       "platform with version ID",
			args:       []string{"localizations", "list", "--app", "123456789", "--version", localizationsListVersionUUID, "--platform", "IOS"},
			versions:   map[string]string{},
			wantStderr: "--platform only applies when --version is omitted or is a version string with --app",
			wantExit:   rootcmd.ExitUsage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var log []string
			installLocalizationsVersionStringTransport(t, test.versions, &log)
			stdout, stderr, runErr := runRootCommand(t, test.args)
			if runErr == nil {
				t.Fatal("expected error")
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d (err=%v)", got, test.wantExit, runErr)
			}
			if !strings.Contains(stderr+runErr.Error(), test.wantStderr) {
				t.Fatalf("expected %q in stderr %q or error %v", test.wantStderr, stderr, runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
		})
	}
}

func TestLocalizationsMutationsResolveVersionStrings(t *testing.T) {
	for _, command := range []string{"create", "update"} {
		for _, scenario := range []struct {
			name       string
			ambiguous  bool
			fromEnv    bool
			resourceID bool
			paddedApp  bool
		}{{name: "resolved"}, {name: "padded app", paddedApp: true}, {name: "environment app", fromEnv: true}, {name: "resource ID with environment app", fromEnv: true, resourceID: true}, {name: "ambiguous", ambiguous: true}} {
			t.Run(command+"/"+scenario.name, func(t *testing.T) {
				ambiguous := scenario.ambiguous
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
				mutations := 0
				stubTransport(t, func(req *http.Request) (*http.Response, error) {
					switch req.Method + " " + req.URL.Path {
					case "GET /v1/apps/app-1/appStoreVersions":
						if scenario.resourceID && req.URL.Query().Get("filter[versionString]") != "" {
							t.Fatal("resource ID must skip version-string discovery")
						}
						if req.URL.Query().Get("filter[versionString]") == "" {
							return jsonResponse(http.StatusOK, `{"data":[]}`)
						}
						if req.URL.Query().Get("filter[versionString]") != "1.17.0" || req.URL.Query().Get("filter[platform]") != "MAC_OS" {
							t.Fatalf("unexpected version query: %s", req.URL.RawQuery)
						}
						if ambiguous {
							return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS"}},{"type":"appStoreVersions","id":"ver-2","attributes":{"platform":"MAC_OS"}}]}`)
						}
						return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS"}}]}`)
					case "GET /v1/appStoreVersions/ver-1":
						return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"ver-1","attributes":{"platform":"MAC_OS"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
					case "GET /v1/appStoreVersions/ver-1/appStoreVersionLocalizations":
						return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US"}}]}`)
					case "POST /v1/appStoreVersionLocalizations", "PATCH /v1/appStoreVersionLocalizations/loc-1":
						mutations++
						var body asc.AppStoreVersionLocalizationCreateRequest
						if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						if body.Data.Attributes.Description != "Description" {
							t.Fatalf("unexpected payload: %#v", body)
						}
						if command == "create" && (body.Data.Relationships == nil || body.Data.Relationships.AppStoreVersion == nil || body.Data.Relationships.AppStoreVersion.Data.ID != "ver-1") {
							t.Fatalf("wrong version relationship: %#v", body)
						}
						return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"Description"}}}`)
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				})
				args := []string{"localizations", command, "--locale", "en-US", "--description", "Description", "--output", "json"}
				if scenario.fromEnv {
					t.Setenv("ASC_APP_ID", "app-1")
				} else {
					appID := "app-1"
					if scenario.paddedApp {
						appID = " app-1 "
					}
					args = append(args, "--app", appID)
				}
				if scenario.resourceID {
					args = append(args, "--version", "ver-1")
				} else {
					args = append(args, "--version", "1.17.0", "--platform", "mac_os")
				}
				stdout, stderr, err := runRootCommand(t, args)
				if ambiguous {
					if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage || mutations != 0 || stdout != "" || !strings.Contains(stderr+err.Error(), "--version") {
						t.Fatalf("expected ambiguity before mutation: stdout=%q stderr=%q err=%v mutations=%d", stdout, stderr, err, mutations)
					}
				} else {
					if err != nil || mutations != 1 {
						t.Fatalf("err=%v stderr=%q mutations=%d", err, stderr, mutations)
					}
					var result struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil || result.Data.ID != "loc-1" {
						t.Fatalf("unexpected stdout: %q, err=%v", stdout, err)
					}
				}
			})
		}
	}
}
