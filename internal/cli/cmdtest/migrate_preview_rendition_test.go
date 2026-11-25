package cmdtest

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMetadataPreviewRenditionRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	for _, scenario := range []string{"unchanged", "edited", "oversized", "download-failure", "stale-membership", "other-version", "duplicate-content", "playlist", "playlist-missing", "legacy-playlist"} {
		t.Run(scenario, func(t *testing.T) {
			setupAuth(t)
			t.Chdir(t.TempDir())
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			root := t.TempDir()
			installValidStoreAssetProbe(t)
			playlist := strings.Contains(scenario, "playlist")
			if playlist {
				bin := t.TempDir()
				if err := os.WriteFile(filepath.Join(bin, "ffprobe"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", bin)
			}
			mutations, importRequests := 0, 0
			phase, pushMode, setReads := "export", "", 0
			var previews []map[string]any
			var order []map[string]string
			media := map[string][]byte{}
			for i := 1; i <= 3; i++ {
				name := fmt.Sprintf("preview-%d.mp4", i)
				id := fmt.Sprintf("VIDEO_%d", i)
				mediaName := name
				media["/"+name] = []byte(fmt.Sprintf("delivered rendition %d", i))
				if playlist {
					mediaName = fmt.Sprintf("preview-%d.m3u8", i)
					media["/"+mediaName] = []byte(fmt.Sprintf("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=%d\nvideo.m3u8\n", i))
				}
				original := md5.Sum([]byte(fmt.Sprintf("original upload %d", i)))
				if scenario == "duplicate-content" && i == 2 {
					original = md5.Sum(media["/"+name])
				}
				previews = append(previews, map[string]any{"type": "appPreviews", "id": id, "attributes": map[string]any{"fileName": name, "fileSize": 1000 + i, "sourceFileChecksum": fmt.Sprintf("%x", original), "videoUrl": "https://media.example/" + mediaName}})
				order = append(order, map[string]string{"type": "appPreviews", "id": id})
			}
			previewJSON, err := json.Marshal(map[string]any{"data": previews})
			if err != nil {
				t.Fatal(err)
			}
			orderJSON, err := json.Marshal(map[string]any{"data": order})
			if err != nil {
				t.Fatal(err)
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if phase == "import" {
					importRequests++
				}
				if req.Method != http.MethodGet {
					mutations++
					return nil, fmt.Errorf("unexpected mutation %s %s", req.Method, req.URL.Path)
				}
				if strings.HasPrefix(req.URL.Path, "/other/") {
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("different target content")), Header: http.Header{}}, nil
				}
				if body, ok := media[req.URL.Path]; ok {
					if phase == "import" && scenario == "oversized" {
						return &http.Response{StatusCode: http.StatusOK, ContentLength: 500_000_001, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
					}
					if phase == "import" && scenario == "download-failure" {
						return migrateJSONResponse(http.StatusServiceUnavailable, `{}`), nil
					}
					return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
				}
				switch req.URL.Path {
				case "/v1/apps/1000000001/appInfos":
					return migrateJSONResponse(200, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/apps/1000000001/appStoreVersions":
					if phase == "import" && (scenario == "other-version" || scenario == "playlist-missing") {
						return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"OTHER_VERSION","attributes":{"versionString":"2.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
					}
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`), nil
				case "/v1/appStoreVersions/VERSION_ID":
					return migrateJSONResponse(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"1000000001"}}}}}`), nil
				case "/v1/appStoreVersions/OTHER_VERSION/appStoreVersionLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"OTHER_LOC","attributes":{"locale":"en-US"}}]}`), nil
				case "/v1/appStoreVersionLocalizations/OTHER_LOC/appPreviewSets":
					return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"OTHER_SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
				case "/v1/appPreviewSets/OTHER_SET/appPreviews":
					if scenario == "playlist-missing" {
						other := strings.ReplaceAll(string(previewJSON), "VIDEO_", "OTHER_VIDEO_")
						return migrateJSONResponse(200, strings.ReplaceAll(other, `"sourceFileChecksum":"`, `"sourceFileChecksum":"0`)), nil
					}
					other := strings.ReplaceAll(string(previewJSON), "VIDEO_", "OTHER_VIDEO_")
					other = strings.ReplaceAll(other, "media.example/", "media.example/other/")
					return migrateJSONResponse(200, other), nil
				case "/v1/appPreviewSets/OTHER_SET/relationships/appPreviews":
					return migrateJSONResponse(200, strings.ReplaceAll(string(orderJSON), "VIDEO_", "OTHER_VIDEO_")), nil
				case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
					return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
				case "/v1/appStoreVersionLocalizations/LOC/appPreviewSets":
					return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
				case "/v1/appPreviewSets/SET/appPreviews":
					setReads++
					if phase == "import" && scenario == "stale-membership" && pushMode == "--confirm" && setReads > 1 {
						return migrateJSONResponse(200, `{"data":[]}`), nil
					}
					return migrateJSONResponse(200, string(previewJSON)), nil
				case "/v1/appPreviewSets/SET/relationships/appPreviews":
					return migrateJSONResponse(200, string(orderJSON)), nil
				default:
					return nil, fmt.Errorf("unexpected request %s", req.URL.Path)
				}
			}))
			var runErr error
			captureOutput(t, func() {
				runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "pull", "--app", "1000000001", "--version", "1.0", "--dir", root, "--include", "previews", "--output", "json"})
			})
			if runErr != nil {
				t.Fatalf("export: %v", runErr)
			}
			if scenario == "edited" {
				path := filepath.Join(root, "app_previews", "en-US", "iphone_65", "preview-1.mp4")
				if err := os.WriteFile(path, []byte("edited local preview"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			deviceDir := filepath.Join(root, "app_previews", "en-US", "iphone_65")
			if playlist {
				for i := 1; i <= 3; i++ {
					stem := filepath.Join(deviceDir, fmt.Sprintf("preview-%d.mp4", i))
					got, err := os.ReadFile(stem + ".m3u8")
					if err != nil || !bytes.Equal(got, media[fmt.Sprintf("/preview-%d.m3u8", i)]) {
						t.Fatalf("playlist %d not saved as .m3u8: %v", i, err)
					}
					if _, err := os.Stat(stem); !os.IsNotExist(err) {
						t.Fatalf("playlist %d saved under a video extension: %v", i, err)
					}
					var ref struct {
						ID, FileName, SourceFileChecksum string
						FileSize                         int
					}
					data, err := os.ReadFile(stem + ".preview.json")
					if err != nil || json.Unmarshal(data, &ref) != nil {
						t.Fatalf("preview reference %d: %v %s", i, err, data)
					}
					attrs := previews[i-1]["attributes"].(map[string]any)
					if ref.ID != previews[i-1]["id"] || ref.FileName != attrs["fileName"] || ref.SourceFileChecksum != attrs["sourceFileChecksum"] || ref.FileSize != 1000+i {
						t.Fatalf("preview reference %d identity: %s", i, data)
					}
					if scenario == "legacy-playlist" {
						if err := os.Rename(stem+".m3u8", stem); err != nil {
							t.Fatal(err)
						}
						if err := os.Remove(stem + ".preview.json"); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if scenario == "duplicate-content" {
				path := filepath.Join(root, "app_previews", "en-US", "iphone_65", "preview-1.mp4")
				if err := os.WriteFile(path, media["/preview-2.mp4"], 0o600); err != nil {
					t.Fatal(err)
				}
			}
			phase = "import"
			for _, mode := range []string{"--dry-run", "--confirm"} {
				pushMode, setReads = mode, 0
				version := "1.0"
				if scenario == "other-version" || scenario == "playlist-missing" {
					version = "2.0"
				}
				stdout, _ := captureOutput(t, func() {
					runErr = RootCommand("test").ParseAndRun(context.Background(), []string{"metadata", "push", "--app", "1000000001", "--version", version, "--dir", root, "--include", "previews", mode, "--output", "json"})
				})
				wantError := ""
				switch scenario {
				case "edited", "other-version":
					wantError = "exceed three videos"
				case "duplicate-content":
					wantError = "duplicate preview content"
				case "playlist-missing":
					wantError = "preview-1.mp4.m3u8 is an App Store Connect streaming playlist, not a video, and no preview in en-US/IPHONE_65 matches it; App Store Connect does not provide original preview videos, so replace it with the original video file or skip previews (migrate import --skip-previews, or omit previews from metadata push --include)"
				case "oversized":
					wantError = "exceeds 500000000 bytes"
				case "download-failure":
					wantError = "HTTP 503"
				case "stale-membership":
					if mode == "--confirm" {
						wantError = "since preflight"
					}
				}
				if wantError != "" {
					if runErr == nil || !strings.Contains(runErr.Error()+stdout, wantError) {
						t.Fatalf("expected %q before mutations: %v; %s", wantError, runErr, stdout)
					}
				} else {
					if runErr != nil {
						t.Fatalf("unchanged full set %s: %v; %s", mode, runErr, stdout)
					}
					var result struct {
						Adds, Updates, Deletes []json.RawMessage
						AssetResults           []struct{ ID, Status string }
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatal(err)
					}
					if len(result.Adds)+len(result.Updates)+len(result.Deletes) != 0 {
						t.Fatalf("unchanged rendition planned changes: %s", stdout)
					}
					if mode == "--confirm" {
						if len(result.AssetResults) != 3 {
							t.Fatalf("expected three existing preview receipts: %s", stdout)
						}
						for i, receipt := range result.AssetResults {
							if receipt.Status != "skipped" || receipt.ID != fmt.Sprintf("VIDEO_%d", i+1) {
								t.Fatalf("wrong preview identity: %s", stdout)
							}
						}
					}
				}
			}
			if scenario == "duplicate-content" && importRequests != 0 {
				t.Fatalf("duplicate content must fail local validation, sent %d requests", importRequests)
			}
			if mutations != 0 {
				t.Fatalf("unexpected remote mutations: %d", mutations)
			}
		})
	}
}
