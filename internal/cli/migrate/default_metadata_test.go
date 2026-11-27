package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestImportPreservesDefaultMetadata(t *testing.T) {
	for _, mode := range []string{"offline", "remote preview", "apply", "locale overrides", "app info denied", "locale lookup denied", "no app info", "no remote locales"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			files := map[string]string{"default/release_notes.txt": "Shared notes", "default/subtitle.txt": "Shared subtitle"}
			if mode == "locale overrides" {
				files["en-US/release_notes.txt"] = "Local notes"
				files["en-US/subtitle.txt"] = "Local subtitle"
			}
			for name, value := range files {
				path := filepath.Join(root, "metadata", name)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			requests := 0
			writes := map[string]map[string]any{}
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = migrateUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if mode == "offline" {
					t.Fatalf("offline import made request %s", req.URL.Path)
				}
				if req.Method == http.MethodPatch {
					if mode == "remote preview" {
						t.Fatal("preview mutated metadata")
					}
					var body struct {
						Data struct {
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					writes[req.URL.Path] = body.Data.Attributes
					return migrateJSONResponse(200, `{"data":{"id":"updated"}}`)
				}
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected method %s %s", req.Method, req.URL.Path)
				}
				switch req.URL.Path {
				case "/v1/appStoreVersions/version-1":
					return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"version-1","relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`)
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					if mode == "no remote locales" {
						return migrateJSONResponse(200, `{"data":[]}`)
					}
					if req.URL.Query().Get("page") == "2" {
						return migrateJSONResponse(200, `{"data":[{"id":"version-de","attributes":{"locale":"de-DE"}}]}`)
					}
					return migrateJSONResponse(200, `{"data":[{"id":"version-en","attributes":{"locale":"en-US"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1/appStoreVersionLocalizations?page=2"}}`)
				case "/v1/apps/app-1/appInfos":
					if mode == "app info denied" {
						return migrateJSONResponse(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`)
					}
					if mode == "no app info" {
						return migrateJSONResponse(200, `{"data":[]}`)
					}
					return migrateJSONResponse(200, `{"data":[{"id":"info-1","attributes":{"appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
				case "/v1/appInfos/info-1/appInfoLocalizations":
					if mode == "locale lookup denied" {
						return migrateJSONResponse(403, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"Denied"}]}`)
					}
					if mode == "no remote locales" {
						return migrateJSONResponse(200, `{"data":[]}`)
					}
					if req.URL.Query().Get("page") == "2" {
						return migrateJSONResponse(200, `{"data":[{"id":"info-fr","attributes":{"locale":"fr-FR"}}]}`)
					}
					return migrateJSONResponse(200, `{"data":[{"id":"info-en","attributes":{"locale":"en-US"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appInfos/info-1/appInfoLocalizations?page=2"}}`)
				default:
					t.Fatalf("unexpected request %s", req.URL.Path)
					return nil, nil
				}
			})
			d := migrateTestDeps()
			d.Client = func() (*asc.Client, error) { return newMigrateUploadTestClient(t), nil }
			cmd := newMigrateImportCommand(d)
			args := []string{"--app", "app-1", "--version-id", "version-1", "--output", "json"}
			if mode != "offline" && mode != "remote preview" {
				args = append(args, "--confirm")
			} else {
				args = append(args, "--dry-run")
				if mode == "remote preview" {
					args = append(args, "--check-remote")
				}
			}
			err := cmd.ParseAndRun(context.Background(), args)
			if mode == "app info denied" || mode == "locale lookup denied" || mode == "no app info" {
				wantError := map[string]string{"app info denied": "failed to get app info", "locale lookup denied": "failed to fetch app info localizations", "no app info": "no app info found for app"}[mode]
				if err == nil || !strings.Contains(err.Error(), wantError) || len(writes) != 0 {
					t.Fatalf("lookup failure: err=%v want=%q writes=%#v", err, wantError, writes)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode == "no remote locales" && len(writes) != 0 {
				t.Fatalf("empty inventories caused writes: %#v", writes)
			}
			var result struct {
				Locales  []string `json:"locales"`
				Defaults struct {
					Version asc.AppStoreVersionLocalizationAttributes `json:"version"`
					AppInfo asc.AppInfoLocalizationAttributes         `json:"appInfo"`
				} `json:"defaults"`
			}
			if err := json.Unmarshal(d.IO.Out.(*bytes.Buffer).Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Defaults.Version.WhatsNew != "Shared notes" || result.Defaults.AppInfo.Subtitle != "Shared subtitle" {
				t.Fatalf("defaults lost: %s", d.IO.Out)
			}
			if mode == "offline" && (len(result.Locales) != 0 || result.Defaults.Version.Locale != "" || result.Defaults.AppInfo.Locale != "") {
				t.Fatalf("offline defaults contain a target locale: %s", d.IO.Out)
			}
			if mode == "offline" && requests != 0 {
				t.Fatalf("requests=%d", requests)
			}
			if mode == "apply" || mode == "locale overrides" {
				notes, subtitle := "Shared notes", "Shared subtitle"
				if mode == "locale overrides" {
					notes, subtitle = "Local notes", "Local subtitle"
				}
				want := map[string]map[string]any{"/v1/appStoreVersionLocalizations/version-en": {"whatsNew": notes}, "/v1/appStoreVersionLocalizations/version-de": {"whatsNew": "Shared notes"}, "/v1/appInfoLocalizations/info-en": {"subtitle": subtitle}, "/v1/appInfoLocalizations/info-fr": {"subtitle": "Shared subtitle"}}
				if !reflect.DeepEqual(writes, want) {
					t.Fatalf("writes=%#v, want %#v", writes, want)
				}
			}
		})
	}
}
