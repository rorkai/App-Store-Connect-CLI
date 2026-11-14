package cmdtest

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func runMigrateInspection(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	command := RootCommand("test")
	command.FlagSet.SetOutput(io.Discard)
	for _, group := range command.Subcommands {
		if group.Name == "migrate" {
			for _, sub := range group.Subcommands {
				if sub.Name == "import" {
					sub.FlagSet.Init(sub.FlagSet.Name(), flag.ContinueOnError)
					sub.FlagSet.SetOutput(io.Discard)
				}
			}
		}
	}
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = command.ParseAndRun(context.Background(), append([]string{"migrate", "import", "--skip-screenshots"}, args...))
	})
	return stdout, stderr, runErr
}

func TestMigrateImportRemoteInspectionRequiresDryRun(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		t.Run(fmt.Sprint(confirm), func(t *testing.T) {
			calls := 0
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { calls++; return nil, errors.New("must not create client") })
			t.Cleanup(restore)
			args := []string{"--check-remote", "--fastlane-dir", filepath.Join(t.TempDir(), "missing")}
			if confirm {
				args = append(args, "--confirm")
			}
			stdout, stderr, err := runMigrateInspection(t, args...)
			if rootcmd.ExitCodeFromError(err) != rootcmd.ExitUsage || err == nil || !strings.Contains(err.Error(), "--check-remote requires --dry-run") {
				t.Fatalf("error=%v exit=%d", err, rootcmd.ExitCodeFromError(err))
			}
			if stdout != "" || !strings.Contains(stderr, "Error: --check-remote requires --dry-run") {
				t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
			}
			if calls != 0 {
				t.Fatalf("client calls=%d", calls)
			}
		})
	}
}

func TestMigrateImportRemoteInspectionDefaultStaysOffline(t *testing.T) {
	root := t.TempDir()
	clipDir := filepath.Join(root, "metadata", "en-US", "app_clip")
	if err := os.MkdirAll(clipDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(clipDir, "subtitle.txt"), "Clip subtitle")
	writePNGForMigrate(t, filepath.Join(clipDir, "header_image.png"), 1800, 1200)
	calls := 0
	restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { calls++; return nil, errors.New("offline must not create client") })
	t.Cleanup(restore)
	stdout, stderr, err := runMigrateInspection(t, "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--dry-run", "--output", "json")
	if err != nil || stderr != "" {
		t.Fatalf("error=%v stderr=%q", err, stderr)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if _, present := result["remoteChecked"]; present {
		t.Fatal("offline plan claims remote inspection")
	}
	if _, present := result["assetResults"]; present {
		t.Fatal("offline plan claims asset operations")
	}
	if calls != 0 {
		t.Fatalf("client calls=%d", calls)
	}
}

func TestMigrateImportRemoteInspectionStoreAssets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ffprobe fixture")
	}
	for _, scenario := range []string{"unchanged", "changed", "missing experience", "missing preview locale", "missing preview set", "delivered reuse", "wrong owner", "read failure"} {
		formats := []string{"json"}
		if scenario == "unchanged" || scenario == "changed" {
			formats = append(formats, "table", "markdown")
		}
		for _, format := range formats {
			t.Run(scenario+"/"+format, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
				root := t.TempDir()
				clipDir := filepath.Join(root, "metadata", "en-US", "app_clip")
				previewDir := filepath.Join(root, "app_previews", "en-US", "iphone_65")
				for _, dir := range []string{clipDir, previewDir, filepath.Join(root, "metadata", "app_clip")} {
					if err := os.MkdirAll(dir, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				writeFile(t, filepath.Join(root, "metadata", "app_clip", "action.txt"), "PLAY")
				writeFile(t, filepath.Join(clipDir, "subtitle.txt"), "Clip subtitle")
				headerPath := filepath.Join(clipDir, "header_image.png")
				writePNGForMigrate(t, headerPath, 1800, 1200)
				header, err := os.ReadFile(headerPath)
				if err != nil {
					t.Fatal(err)
				}
				video := []byte("fixture preview")
				writeFile(t, filepath.Join(previewDir, "preview.mp4"), string(video))
				writeFile(t, filepath.Join(previewDir, "preview.poster_frame.txt"), "00:00:02.000")
				headerChecksum := fmt.Sprintf("%x", md5.Sum(header))
				videoChecksum := fmt.Sprintf("%x", md5.Sum(video))
				installValidStoreAssetProbe(t)
				requests, downloads, mutations := 0, 0, 0
				installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					if req.Method != http.MethodGet {
						mutations++
						return nil, fmt.Errorf("mutation attempted: %s", req.Method)
					}
					if req.URL.Host == "media.example" {
						downloads++
						body := header
						if req.URL.Path == "/preview.mp4" {
							body = video
						}
						return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(body)), Header: http.Header{}}, nil
					}
					switch req.URL.Path {
					case "/v1/appStoreVersions/VERSION_ID":
						owner := "APP_ID"
						if scenario == "wrong owner" {
							owner = "OTHER_APP"
						}
						return migrateJSONResponse(200, fmt.Sprintf(`{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":%q}}}}}`, owner)), nil
					case "/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience":
						if scenario == "missing experience" {
							return migrateJSONResponse(200, `{"data":null}`), nil
						}
						if scenario == "read failure" {
							return migrateJSONResponse(400, `{"errors":[{"status":"400","code":"READ_FAILED","detail":"asset inspection failed"}]}`), nil
						}
						action := "PLAY"
						if scenario == "changed" {
							action = "OPEN"
						}
						return migrateJSONResponse(200, fmt.Sprintf(`{"data":{"type":"appClipDefaultExperiences","id":"EXP","attributes":{"action":%q}}}`, action)), nil
					case "/v1/apps/APP_ID/appClips":
						return migrateJSONResponse(200, `{"data":[{"type":"appClips","id":"CLIP"}]}`), nil
					case "/v1/appClipDefaultExperiences/EXP/appClipDefaultExperienceLocalizations":
						subtitle := "Clip subtitle"
						if scenario == "changed" {
							subtitle = "Old subtitle"
						}
						return migrateJSONResponse(200, fmt.Sprintf(`{"data":[{"type":"appClipDefaultExperienceLocalizations","id":"CLIP_LOC","attributes":{"locale":"en-US","subtitle":%q}}]}`, subtitle)), nil
					case "/v1/appClipDefaultExperienceLocalizations/CLIP_LOC/appClipHeaderImage":
						checksum := headerChecksum
						asset := ""
						if scenario == "changed" {
							checksum = "different"
						}
						if scenario == "delivered reuse" {
							checksum = "original upload"
							asset = `,"imageAsset":{"templateUrl":"https://media.example/header.png","width":1800,"height":1200}`
						}
						return migrateJSONResponse(200, fmt.Sprintf(`{"data":{"type":"appClipHeaderImages","id":"HEADER","attributes":{"sourceFileChecksum":%q%s}}}`, checksum, asset)), nil
					case "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations":
						if scenario == "missing preview locale" {
							return migrateJSONResponse(200, `{"data":[]}`), nil
						}
						return migrateJSONResponse(200, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC","attributes":{"locale":"en-US"}}]}`), nil
					case "/v1/appStoreVersionLocalizations/LOC/appPreviewSets":
						if scenario == "missing preview set" {
							return migrateJSONResponse(200, `{"data":[]}`), nil
						}
						return migrateJSONResponse(200, `{"data":[{"type":"appPreviewSets","id":"SET","attributes":{"previewType":"IPHONE_65"}}]}`), nil
					case "/v1/appPreviewSets/SET/relationships/appPreviews":
						return migrateJSONResponse(200, `{"data":[{"type":"appPreviews","id":"VIDEO"}]}`), nil
					case "/v1/appPreviewSets/SET/appPreviews":
						frame := "00:00:02.000"
						if scenario == "changed" {
							frame = "00:00:01.000"
						}
						checksum := videoChecksum
						url := ""
						if scenario == "delivered reuse" {
							checksum = "original upload"
							url = `,"videoUrl":"https://media.example/preview.mp4"`
						}
						return migrateJSONResponse(200, fmt.Sprintf(`{"data":[{"type":"appPreviews","id":"VIDEO","attributes":{"fileName":"preview.mp4","sourceFileChecksum":%q,"previewFrameTimeCode":%q%s}}]}`, checksum, frame, url)), nil
					default:
						return nil, fmt.Errorf("unexpected request: %s", req.URL.Path)
					}
				}))
				stdout, stderr, runErr := runMigrateInspection(t, "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--dry-run", "--check-remote", "--output", format)
				if mutations != 0 {
					t.Fatalf("mutations=%d", mutations)
				}
				if scenario == "wrong owner" || scenario == "read failure" {
					if runErr == nil || strings.TrimSpace(stdout) != "" {
						t.Fatalf("error=%v stdout=%q", runErr, stdout)
					}
					if scenario == "read failure" && (requests < 2 || !strings.Contains(runErr.Error(), "asset inspection failed")) {
						t.Fatalf("not the remote read failure: %v; reads=%d", runErr, requests)
					}
					if scenario == "wrong owner" && requests != 1 {
						t.Fatalf("ownership failure allowed asset reads: %d requests", requests)
					}
					return
				}
				if runErr != nil || stderr != "" {
					t.Fatalf("error=%v stderr=%q", runErr, stderr)
				}
				if requests == 0 {
					t.Fatal("remote inspection made no reads")
				}
				if scenario == "delivered reuse" && downloads != 2 {
					t.Fatalf("media GETs=%d, want2", downloads)
				}
				want := []string{}
				switch scenario {
				case "changed":
					want = []string{"app_clip/update", "app_clip_header/replace", "app_clip_localization/update", "preview/update"}
				case "missing experience":
					want = []string{"app_clip/create", "app_clip_header/upload", "app_clip_localization/create"}
				case "missing preview locale":
					want = []string{"preview/upload", "preview_order/update", "preview_set/create", "version_localization/create"}
				case "missing preview set":
					want = []string{"preview/upload", "preview_order/update", "preview_set/create"}
				}
				if format == "json" {
					var result struct {
						RemoteChecked bool                   `json:"remoteChecked"`
						DryRun        bool                   `json:"dryRun"`
						AssetResults  []asc.StoreAssetResult `json:"assetResults"`
					}
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatal(err)
					}
					if !result.RemoteChecked || !result.DryRun {
						t.Fatalf("inspection flags=%+v", result)
					}
					got := []string{}
					for _, item := range result.AssetResults {
						got = append(got, item.Kind+"/"+item.Action)
						if item.Status != "planned" || item.ID != "" || item.PreviousID != "" || item.PreviousDeleted || item.Error != "" {
							t.Fatalf("planned operation looks applied: %+v", item)
						}
						if item.Kind != "app_clip" && item.Locale != "en-US" {
							t.Fatalf("missing planned locale: %+v", item)
						}
						if item.Kind != "version_localization" && item.Path == "" {
							t.Fatalf("missing planned path: %+v", item)
						}
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("changes=%v want%v", got, want)
					}
				} else if len(want) == 0 {
					if !strings.Contains(stdout, "No remote store asset changes") || strings.Contains(stdout, "local input") {
						t.Fatalf("no-change output=%s", stdout)
					}
				} else {
					if !strings.Contains(stdout, "Remote store asset changes") || !strings.Contains(stdout, "planned") || strings.Contains(stdout, "local input") {
						t.Fatalf("planned output=%s", stdout)
					}
				}
			})
		}
	}
}

func TestMigrateImportRemoteInspectionCreateLocales(t *testing.T) {
	for _, kind := range []string{"version", "app info"} {
		for _, exists := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/existing=%t", kind, exists), func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
				root := t.TempDir()
				dir := filepath.Join(root, "metadata", "nl")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				file := "description.txt"
				if kind == "app info" {
					file = "name.txt"
				}
				writeFile(t, filepath.Join(dir, file), "Dutch metadata")
				localizations := asctest.JSON(200, `{"data":[]}`)
				if exists {
					localizations = asctest.JSON(200, `{"data":[{"id":"EXISTING","attributes":{"locale":"nl"}}]}`)
				}
				reg := asctest.NewRegistry(t)
				reg.Install()
				reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
					asctest.JSON(200, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
				reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"), localizations)
				if kind == "app info" && exists {
					reg.Register(asctest.GET("/v1/apps/APP_ID/appInfos"),
						asctest.JSON(200, `{"data":[{"type":"appInfos","id":"INFO","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`))
					reg.Register(asctest.GET("/v1/appInfos/INFO/appInfoLocalizations", "limit=200"), localizations)
				}
				stdout, _, err := runMigrateInspection(t, "--app", "APP_ID", "--version-id", "VERSION_ID", "--fastlane-dir", root, "--dry-run", "--check-remote", "--output", "json")
				reads := len(reg.Requests())
				if reads < 2 {
					t.Fatalf("locale exemption evaluated before remote reads: %d", reads)
				}
				if exists {
					if err != nil || !strings.Contains(stdout, `"remoteChecked":true`) {
						t.Fatalf("existing locale error=%v stdout=%s", err, stdout)
					}
				} else if err == nil || !strings.Contains(err.Error(), `unsupported locale "nl"`) || stdout != "" {
					t.Fatalf("new locale error=%v stdout=%s", err, stdout)
				}
			})
		}
	}
}
