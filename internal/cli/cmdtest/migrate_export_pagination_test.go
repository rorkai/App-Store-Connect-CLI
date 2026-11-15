package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMigrateExportMetadataPagination(t *testing.T) {
	for _, scenario := range []string{"all pages", "version later error", "app info later error", "app info empty first page later error", "optional app info absent"} {
		t.Run(scenario, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			outputDir := t.TempDir()
			sentinel := filepath.Join(outputDir, "keep.txt")
			if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
				t.Fatal(err)
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					return nil, fmt.Errorf("unexpected mutation: %s", req.Method)
				}
				switch req.URL.Path {
				case "/v1/appStoreVersions/VERSION_ID":
					return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
				case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
					// Asset discovery already paginates independently. Limit distinguishes metadata discovery.
					if req.URL.Query().Get("page") == "2" {
						if scenario == "version later error" && req.URL.Query().Get("limit") == "200" {
							return migrateJSONResponse(400, `{"errors":[{"status":"400","code":"PAGE_FAILED","detail":"version later page failed"}]}`), nil
						}
						return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_FR","attributes":{"locale":"fr-FR","description":"French description"}}]}`), nil
					}
					next := "https://api.appstoreconnect.apple.com/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations?page=2"
					if req.URL.Query().Get("limit") == "200" {
						next += "&limit=200"
					}
					return migrateJSONResponse(200, fmt.Sprintf(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_EN","attributes":{"locale":"en-US","description":"English description"}}],"links":{"next":%q}}`, next)), nil
				case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
					return migrateJSONResponse(200, `{"data":null}`), nil
				case "/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail":
					return migrateJSONResponse(404, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`), nil
				case "/v1/appStoreVersionLocalizations/LOC_EN/appPreviewSets", "/v1/appStoreVersionLocalizations/LOC_FR/appPreviewSets":
					return migrateJSONResponse(200, `{"data":[]}`), nil
				case "/v1/apps/APP_ID/appInfos":
					if scenario == "optional app info absent" {
						return migrateJSONResponse(200, `{"data":[]}`), nil
					}
					return migrateJSONResponse(200, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/appInfos/INFO_ID/appInfoLocalizations":
					if req.URL.Query().Get("page") == "2" {
						if strings.Contains(scenario, "app info") {
							return migrateJSONResponse(400, `{"errors":[{"status":"400","code":"PAGE_FAILED","detail":"app info later page failed"}]}`), nil
						}
						return migrateJSONResponse(200, `{"data":[{"type":"appInfoLocalizations","id":"INFO_FR","attributes":{"locale":"fr-FR","name":"French name"}}]}`), nil
					}
					data := `[{"type":"appInfoLocalizations","id":"INFO_EN","attributes":{"locale":"en-US","name":"English name"}}]`
					if strings.Contains(scenario, "empty first page") {
						data = `[]`
					}
					return migrateJSONResponse(200, fmt.Sprintf(`{"data":%s,"links":{"next":"https://api.appstoreconnect.apple.com/v1/appInfos/INFO_ID/appInfoLocalizations?page=2"}}`, data)), nil
				default:
					return nil, fmt.Errorf("unexpected request: %s", req.URL)
				}
			}))
			command := RootCommand("test")
			command.FlagSet.SetOutput(io.Discard)
			var runErr error
			stdout, _ := captureOutput(t, func() {
				if err := command.Parse([]string{"migrate", "export", "--app", "APP_ID", "--version-id", "VERSION_ID", "--output-dir", outputDir, "--output", "json"}); err != nil {
					t.Fatal(err)
				}
				runErr = command.Run(context.Background())
			})
			if strings.Contains(scenario, "error") {
				if runErr == nil || !strings.Contains(runErr.Error(), "later page failed") {
					t.Fatalf("error = %v, want later-page failure", runErr)
				}
				if strings.TrimSpace(stdout) != "" {
					t.Fatalf("failure printed success receipt: %s", stdout)
				}
				if _, err := os.Stat(filepath.Join(outputDir, "metadata")); !os.IsNotExist(err) {
					t.Fatalf("pagination failure wrote metadata: %v", err)
				}
			} else {
				if runErr != nil {
					t.Fatal(runErr)
				}
				var result struct {
					Locales    []string `json:"locales"`
					TotalFiles int      `json:"totalFiles"`
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(result.Locales, []string{"en-US", "fr-FR"}) {
					t.Fatalf("locales = %v", result.Locales)
				}
				wantFiles := map[string]string{"en-US/description.txt": "English description", "fr-FR/description.txt": "French description"}
				if scenario != "optional app info absent" {
					wantFiles["en-US/name.txt"] = "English name"
					wantFiles["fr-FR/name.txt"] = "French name"
				}
				if result.TotalFiles != len(wantFiles) {
					t.Fatalf("totalFiles = %d, want %d", result.TotalFiles, len(wantFiles))
				}
				for path, want := range wantFiles {
					got, err := os.ReadFile(filepath.Join(outputDir, "metadata", path))
					if err != nil || strings.TrimSpace(string(got)) != want {
						t.Errorf("%s = %q, %v; want %q", path, got, err, want)
					}
				}
			}
			if got, err := os.ReadFile(sentinel); err != nil || string(got) != "keep" {
				t.Fatalf("unrelated file changed: %q %v", got, err)
			}
		})
	}
}
