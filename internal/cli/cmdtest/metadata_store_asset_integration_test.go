package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMetadataPreviewsPreserveLocalizationPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	for _, policy := range []string{"skip", "update", "unmanaged-version"} {
		t.Run(policy, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			root := t.TempDir()
			if policy == "unmanaged-version" {
				writeMetadataScopeFile(t, filepath.Join(root, "app-info", "ja.json"), `{"subtitle":null}`)
			} else {
				writeMetadataScopeFile(t, filepath.Join(root, "version", "1.2.3", "ja.json"), `{"description":"Planned JA description","whatsNew":"Planned JA release notes"}`)
			}
			writeMetadataScopeFile(t, filepath.Join(root, "app_previews", "ja", "iphone_65", "preview.mp4"), "fixture-video")
			installValidStoreAssetProbe(t)
			localeCreates, localePatches, setCreates := 0, 0, 0
			cleared, committed := false, false
			var uploaded string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.Method + " " + req.URL.Path {
				case "GET /v1/apps/app-1/appStoreVersions":
					return migrateJSONResponse(200, metadataPushVersionsList), nil
				case "GET /v1/apps/app-1/appInfos":
					if policy != "unmanaged-version" {
						t.Error("version-only metadata must not resolve app-info")
					}
					return migrateJSONResponse(200, metadataPushAppInfosList), nil
				case "GET /v1/appInfos/appinfo-1/appInfoLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appInfoLocalizations","id":"INFO_LOC","attributes":{"locale":"ja","name":"App","subtitle":"Clear me"}}]}`), nil
				case "PATCH /v1/appInfoLocalizations/INFO_LOC":
					body, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, err
					}
					cleared = strings.Contains(string(body), `"subtitle":null`)
					return migrateJSONResponse(200, `{"data":{"type":"appInfoLocalizations","id":"INFO_LOC","attributes":{"locale":"ja","name":"App"}}}`), nil
				case "GET /v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					if policy != "unmanaged-version" && localeCreates == 0 {
						return migrateJSONResponse(200, metadataPushEmptyList), nil
					}
					return migrateJSONResponse(200, metadataPushVersionLocalizationsRemote), nil
				case "POST /v1/appStoreVersionLocalizations":
					localeCreates++
					return migrateJSONResponse(409, metadataVersionLocaleDuplicate409), nil
				case "PATCH /v1/appStoreVersionLocalizations/loc-ja":
					localePatches++
					return migrateJSONResponse(200, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Planned JA description","whatsNew":"Planned JA release notes"}}}`), nil
				case "GET /v1/appStoreVersionLocalizations/loc-ja/appPreviewSets":
					return migrateJSONResponse(200, metadataPushEmptyList), nil
				case "POST /v1/appPreviewSets":
					setCreates++
					body, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, err
					}
					if !strings.Contains(string(body), `"id":"loc-ja"`) {
						t.Errorf("preview set must use recovered/existing locale: %s", body)
					}
					return migrateJSONResponse(201, `{"data":{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}}`), nil
				case "GET /v1/appPreviewSets/SET/appPreviews":
					return migrateJSONResponse(200, metadataPushEmptyList), nil
				case "POST /v1/appPreviews":
					return migrateJSONResponse(201, `{"data":{"type":"appPreviews","id":"VIDEO","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/video","length":13,"offset":0}]}}}`), nil
				case "PUT /video":
					body, err := io.ReadAll(req.Body)
					uploaded = string(body)
					return migrateJSONResponse(200, `{}`), err
				case "PATCH /v1/appPreviews/VIDEO":
					committed = true
					return migrateJSONResponse(200, `{"data":{"type":"appPreviews","id":"VIDEO","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
				case "GET /v1/appPreviews/VIDEO":
					return migrateJSONResponse(200, `{"data":{"type":"appPreviews","id":"VIDEO","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
				case "GET /v1/appPreviewSets/SET/relationships/appPreviews":
					return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"VIDEO"}]}`), nil
				default:
					t.Errorf("out-of-scope request %s %s", req.Method, req.URL.Path)
					return nil, fmt.Errorf("unexpected request")
				}
			}))
			args := []string{"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--dir", root, "--include", "localizations,previews", "--confirm", "--output", "json"}
			if policy == "unmanaged-version" {
				args = append(args, "--allow-deletes")
			} else {
				args = append(args, "--if-exists", policy)
			}
			var runErr error
			stdout, stderr := captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
			if runErr != nil {
				t.Fatalf("preview apply failed: %v; stderr=%s; receipt=%s", runErr, stderr, stdout)
			}
			wantCreates, wantPatches := 1, 0
			switch policy {
			case "unmanaged-version":
				wantCreates = 0
				if !cleared {
					t.Fatal("managed app-info clear did not send JSON null")
				}
			case "update":
				wantPatches = 1
			}
			if localeCreates != wantCreates || localePatches != wantPatches || setCreates != 1 || !committed || uploaded != "fixture-video" {
				t.Fatalf("creates=%d patches=%d sets=%d committed=%v uploaded=%q", localeCreates, localePatches, setCreates, committed, uploaded)
			}
			var result struct {
				Applied bool
				Skipped int
				Deletes []json.RawMessage
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if !result.Applied || len(result.Deletes) != 0 || (policy == "skip" && result.Skipped != 1) {
				t.Fatalf("unexpected result: %s", stdout)
			}
		})
	}
}

func TestMetadataPreviewsRejectDeletedLocalizationParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	for _, mode := range []string{"--dry-run", "--confirm"} {
		t.Run(mode, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			root := t.TempDir()
			writeMetadataScopeFile(t, filepath.Join(root, "version", "1.2.3", "en-US.json"), `{"description":"Keep"}`)
			writeMetadataScopeFile(t, filepath.Join(root, "app_previews", "ja", "iphone_65", "preview.mp4"), "fixture-video")
			installValidStoreAssetProbe(t)
			mutations := 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					mutations++
					return nil, fmt.Errorf("unexpected mutation %s %s", req.Method, req.URL.Path)
				}
				switch req.URL.Path {
				case "/v1/apps/app-1/appStoreVersions":
					return migrateJSONResponse(200, metadataPushVersionsList), nil
				case "/v1/appStoreVersions/version-1/appStoreVersionLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","description":"Keep"}},{"type":"appStoreVersionLocalizations","id":"loc-ja","attributes":{"locale":"ja","description":"Preview parent"}}]}`), nil
				case "/v1/appStoreVersionLocalizations/loc-ja/appPreviewSets":
					return migrateJSONResponse(200, metadataPushEmptyList), nil
				default:
					return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
				}
			}))
			var runErr error
			stdout, stderr := captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "app-1", "--version", "1.2.3", "--dir", root, "--include", "localizations,previews", "--allow-deletes", mode, "--output", "json"})
			})
			if runErr == nil || !strings.Contains(stderr, `version localization "ja" is required by local previews`) {
				t.Fatalf("expected contradictory-input diagnostic: err=%v stderr=%s stdout=%s", runErr, stderr, stdout)
			}
			if mutations != 0 {
				t.Fatalf("must reject before any mutation, got %d", mutations)
			}
		})
	}
}
