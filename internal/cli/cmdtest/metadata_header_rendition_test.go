package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetadataHeaderRenditionRoundTrip(t *testing.T) {
	for _, scenario := range []string{"unchanged", "edited", "stale-header", "download-failure", "new-header", "new-locale", "new-experience", "unavailable-delivery", "unavailable-dimensions", "unsafe-delivery"} {
		t.Run(scenario, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			fastlaneRoot := t.TempDir()
			root := filepath.Join(fastlaneRoot, "metadata")
			deliveredPath := filepath.Join(t.TempDir(), "header.png")
			writePNGForMigrate(t, deliveredPath, 1800, 1200)
			delivered, err := os.ReadFile(deliveredPath)
			if err != nil {
				t.Fatal(err)
			}
			phase, mode := "export", ""
			headerReads, localeReads, experienceReads, mutations := 0, 0, 0, 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
					return nil, fmt.Errorf("unexpected mutation %s %s", req.Method, req.URL.Path)
				}
				switch req.URL.Path {
				case "/v1/appStoreVersions/VERSION_ID":
					return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`), nil
				case "/v1/apps/APP_ID/appClips":
					return migrateJSONResponse(200, `{"data":[{"type":"appClips","id":"CLIP"}]}`), nil
				case "/v1/apps/APP_ID/appStoreVersions":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
					experienceReads++
					if phase == "import" && scenario == "new-experience" && experienceReads == 1 {
						return migrateJSONResponse(404, `{}`), nil
					}
					return migrateJSONResponse(200, `{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":"PLAY"}}}`), nil
				case "/v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
					localeReads++
					if phase == "import" && scenario == "new-locale" && localeReads == 1 {
						return migrateJSONResponse(200, `{"data":[]}`), nil
					}
					return migrateJSONResponse(200, `{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"LOC","attributes":{"locale":"en-US","subtitle":"Play now"}}]}`), nil
				case "/v1/appClipDefaultExperienceLocalizations/LOC/appClipHeaderImage":
					headerReads++
					if phase == "import" && scenario == "new-header" && headerReads == 1 {
						return migrateJSONResponse(404, `{}`), nil
					}
					if phase == "import" && scenario == "unavailable-dimensions" {
						return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"sourceFileChecksum":"original-upload-checksum","imageAsset":{"templateUrl":"https://media.example/header.png"}}}}`), nil
					}
					if phase == "import" && scenario == "unsafe-delivery" {
						return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"sourceFileChecksum":"original-upload-checksum","imageAsset":{"templateUrl":"file:///private/header.png","width":1800,"height":1200}}}}`), nil
					}
					if phase == "import" && scenario == "unavailable-delivery" {
						return migrateJSONResponse(200, `{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"sourceFileChecksum":"original-upload-checksum","imageAsset":{}}}}`), nil
					}
					id := "HEADER"
					if phase == "import" && scenario == "stale-header" && mode == "--confirm" && headerReads > 1 {
						id = "REPLACED_HEADER"
					}
					return migrateJSONResponse(200, fmt.Sprintf(`{"data":{"type":"appClipHeaderImages","id":%q,"attributes":{"sourceFileChecksum":"original-upload-checksum","imageAsset":{"templateUrl":"https://media.example/header.png","width":1800,"height":1200}}}}`, id)), nil
				case "/header.png":
					if phase == "import" && scenario == "download-failure" {
						return migrateJSONResponse(503, `{}`), nil
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(delivered)), Header: http.Header{}}, nil
				default:
					return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
				}
			}))
			var runErr error
			captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "app-clip", "--output", "json"})
			})
			if runErr != nil {
				t.Fatal(runErr)
			}
			if scenario == "edited" {
				img := image.NewNRGBA(image.Rect(0, 0, 1800, 1200))
				img.Set(0, 0, color.NRGBA{R: 255, A: 255})
				var content bytes.Buffer
				if err := png.Encode(&content, img); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "en-US", "app_clip", "header_image.png"), content.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			phase = "import"
			for _, nextMode := range []string{"--dry-run", "--confirm"} {
				mode, headerReads, localeReads, experienceReads = nextMode, 0, 0, 0
				stdout, stderr := captureOutput(t, func() {
					runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "APP_ID", "--version", "1.0", "--dir", root, "--include", "app-clip", mode, "--output", "json"})
				})
				wantError := ""
				if (scenario == "edited" || strings.HasPrefix(scenario, "unavailable-")) && mode == "--confirm" {
					wantError = "--allow-deletes is required"
				} else if (scenario == "stale-header" || strings.HasPrefix(scenario, "new-")) && mode == "--confirm" {
					wantError = "since preflight"
				} else if scenario == "unsafe-delivery" {
					wantError = "invalid delivery URL"
				} else if scenario == "download-failure" {
					wantError = "HTTP 503"
				}
				if wantError != "" {
					if runErr == nil || !strings.Contains(runErr.Error()+stdout+stderr, wantError) {
						t.Fatalf("want %q: err=%v stdout=%s stderr=%s", wantError, runErr, stdout, stderr)
					}
					continue
				}
				if runErr != nil {
					t.Fatalf("%s: %v; %s", mode, runErr, stderr)
				}
				var result struct {
					Adds, Updates, Deletes []json.RawMessage
					AssetResults           []struct{ ID, Status string }
				}
				if err := json.Unmarshal([]byte(stdout), &result); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(scenario, "new-") {
					if len(result.Adds) != map[string]int{"new-header": 1, "new-locale": 2, "new-experience": 3}[scenario] {
						t.Fatalf("absent header should plan upload: %s", stdout)
					}
				} else if scenario == "edited" || strings.HasPrefix(scenario, "unavailable-") {
					if len(result.Deletes) != 1 {
						t.Fatalf("edited header must remain a replacement: %s", stdout)
					}
				} else if len(result.Adds)+len(result.Updates)+len(result.Deletes) != 0 {
					t.Fatalf("unchanged delivered header must not plan replacement: %s", stdout)
				}
				if mode == "--confirm" && (len(result.AssetResults) != 1 || result.AssetResults[0].ID != "HEADER" || result.AssetResults[0].Status != "skipped") {
					t.Fatalf("unchanged header must reuse remote identity: %s", stdout)
				}
			}
			if scenario == "unchanged" {
				headerReads = 0
				stdout, stderr := captureOutput(t, func() {
					runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"migrate", "import", "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", fastlaneRoot, "--skip-screenshots", "--confirm", "--output", "json"})
				})
				if runErr != nil || !strings.Contains(stdout, `"status":"skipped"`) || !strings.Contains(stdout, `"id":"HEADER"`) {
					t.Fatalf("migration must reuse unchanged header: err=%v stdout=%s stderr=%s", runErr, stdout, stderr)
				}
			}
			if mutations != 0 {
				t.Fatalf("unexpected mutations: %d", mutations)
			}
		})
	}
}
