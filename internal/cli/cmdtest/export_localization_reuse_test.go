package cmdtest

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExportReusesVersionLocalizationSnapshot(t *testing.T) {
	for _, command := range []string{"metadata", "migrate", "previews-only"} {
		for _, empty := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/empty=%t", command, empty), func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
				phase := 0
				pageRequests := 0
				var previewIDs []string
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					switch req.URL.Path {
					case "/v1/appStoreVersions/VERSION_ID":
						return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"123456789"}}}}}`), nil
					case "/v1/apps/123456789/appStoreVersions":
						return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"}}]}`), nil
					case "/v1/apps/123456789/appInfos":
						return migrateJSONResponse(200, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
					case "/v1/appInfos/INFO_ID/appInfoLocalizations":
						return migrateJSONResponse(200, `{"data":[]}`), nil
					case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
						return migrateJSONResponse(200, `{"data":null}`), nil
					case "/v1/appStoreVersions/VERSION_ID/appStoreReviewDetail":
						return migrateJSONResponse(404, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not found"}]}`), nil
					case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
						pageRequests++
						if empty {
							return migrateJSONResponse(200, `{"data":[]}`), nil
						}
						page := req.URL.Query().Get("page")
						locale, suffix, links := "en-US", "EN", `,"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations?page=2"}`
						if page == "2" {
							locale, suffix, links = "fr-FR", "FR", ""
						}
						return migrateJSONResponse(200, fmt.Sprintf(`{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_%d_%s","attributes":{"locale":%q,"description":"description"}}]%s}`, phase, suffix, locale, links)), nil
					default:
						if strings.HasPrefix(req.URL.Path, "/v1/appStoreVersionLocalizations/") && strings.HasSuffix(req.URL.Path, "/appPreviewSets") {
							previewIDs = append(previewIDs, strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/v1/appStoreVersionLocalizations/"), "/appPreviewSets"))
							return migrateJSONResponse(200, `{"data":[]}`), nil
						}
						return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
					}
				}))
				for phase = 0; phase < 2; phase++ {
					pageRequests, previewIDs = 0, nil
					outputDir := t.TempDir()
					args := []string{"migrate", "export", "--app", "123456789", "--version-id", "VERSION_ID", "--output-dir", outputDir, "--output", "json"}
					if command != "migrate" {
						include := "localizations,previews"
						if command == "previews-only" {
							include = "previews"
						}
						args = []string{"metadata", "pull", "--app", "123456789", "--version", "1.0", "--dir", outputDir, "--include", include, "--output", "json"}
					}
					var runErr error
					captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
					if runErr != nil {
						t.Fatal(runErr)
					}
					wantRequests := 2
					var wantIDs []string
					if empty {
						wantRequests = 1
					} else {
						wantIDs = []string{fmt.Sprintf("LOC_%d_EN", phase), fmt.Sprintf("LOC_%d_FR", phase)}
					}
					if pageRequests != wantRequests {
						t.Errorf("localization requests = %d, want %d", pageRequests, wantRequests)
					}
					if !reflect.DeepEqual(previewIDs, wantIDs) {
						t.Errorf("preview localization IDs = %v, want %v", previewIDs, wantIDs)
					}
				}
			})
		}
	}
}

func TestExportSnapshotPreservesAssetFailureChecks(t *testing.T) {
	for _, scenario := range []string{"clip failure", "missing ordered asset", "omitted ordered asset"} {
		t.Run(scenario, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			previewReads := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/apps/123456789/appStoreVersions":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"}}]}`), nil
				case "/v1/apps/123456789/appInfos":
					return migrateJSONResponse(200, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/appInfos/INFO_ID/appInfoLocalizations":
					return migrateJSONResponse(200, `{"data":[]}`), nil
				case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US","description":"description"}}]}`), nil
				case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
					if scenario == "clip failure" {
						return migrateJSONResponse(400, `{"errors":[{"status":"400","code":"CLIP_FAILED","detail":"clip failed"}]}`), nil
					}
					return migrateJSONResponse(200, `{"data":null}`), nil
				case "/v1/appStoreVersionLocalizations/LOC/appPreviewSets":
					previewReads++
					return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
				case "/v1/appPreviewSets/SET/appPreviews":
					return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"VIDEO","attributes":{"fileName":"preview.mp4","videoUrl":"https://media.example/video"}}]}`), nil
				case "/v1/appPreviewSets/SET/relationships/appPreviews":
					if scenario == "missing ordered asset" {
						return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"MISSING"}]}`), nil
					}
					return migrateJSONResponse(200, `{"data":[]}`), nil
				default:
					return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
				}
			}))
			outputDir := filepath.Join(t.TempDir(), "not-created")
			var runErr error
			stdout, _ := captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "123456789", "--version", "1.0", "--dir", outputDir, "--include", "localizations,app-clip,previews", "--output", "json"})
			})
			wantError := "clip failed"
			if scenario == "missing ordered asset" {
				wantError = "preview order references missing asset MISSING"
			}
			if scenario == "omitted ordered asset" {
				wantError = "preview set SET changed during export"
			}
			if runErr == nil || !strings.Contains(runErr.Error(), wantError) {
				t.Fatalf("error = %v, want %q", runErr, wantError)
			}
			if scenario == "clip failure" && previewReads != 0 {
				t.Fatalf("preview discovery preceded clip failure: %d reads", previewReads)
			}
			if stdout != "" {
				t.Fatalf("failure printed receipt: %s", stdout)
			}
			if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
				t.Fatalf("failed discovery touched output: %v", err)
			}
		})
	}
}
