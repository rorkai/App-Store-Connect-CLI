package publish

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestPublishLocalBuildNumberFloorPreflight(t *testing.T) {
	for _, factory := range []struct {
		name    string
		command func(*shared.Deps) *ffcli.Command
	}{
		{"testflight", newPublishTestFlightCommand}, {"appstore", newPublishAppStoreCommand},
	} {
		t.Run(factory.name, func(t *testing.T) {
			if factory.command(newPublishTestDeps()).FlagSet.Lookup("local-build-number-floor") == nil {
				t.Fatal("publish does not support the opt-in local build-number floor")
			}
			for _, tc := range []struct {
				name string
				args []string
				want string
			}{
				{"prebuilt", []string{"--ipa", "Demo.ipa", "--local-build-number-floor"}, "--local-build-number-floor requires --workspace or --project"},
				{"workspace", []string{"--workspace", "Demo.xcworkspace", "--scheme", "Demo", "--local-build-number-floor"}, "--local-build-number-floor requires --project; workspace target selection is unsupported"},
				{"explicit number", []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--local-build-number-floor", "--build-number", "9"}, "--local-build-number-floor cannot be combined with --build-number"},
				{"target without policy", []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--target", "Demo"}, "--target requires --local-build-number-floor"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					stubPublishCommandSeams(t)
					publishTestClientWithTimeout = func(time.Duration) (*asc.Client, error) {
						t.Fatal("client created during invalid preflight")
						return nil, nil
					}
					runPublishArchiveFn = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
						t.Fatal("archive ran during invalid preflight")
						return nil, nil
					}
					args := append([]string{"--app", "123456789", "--version", "1.2.3"}, tc.args...)
					stdout, stderr, err := runPublishCommand(t, factory.command, args)
					if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, tc.want) {
						t.Fatalf("stdout=%q stderr=%q error=%v; want %q", stdout, stderr, err, tc.want)
					}
				})
			}
		})
	}
}

func TestPublishBuildNumberFloorCombinesRemoteHistory(t *testing.T) {
	for _, tc := range []struct {
		name, local, processed, upload, explicit, want string
		initial                                        int
	}{
		{"local above history", "42", "12", "20", "", "43", 1},
		{"older processed above local", "42", "88", "20", "", "89", 1},
		{"upload above local", "42", "12", "101", "", "102", 1},
		{"initial above local", "42", "", "", "", "100", 100},
		{"no history", "42", "", "", "", "43", 1},
		{"dotted numbers", "1.2.10", "1.2.3", "", "", "1.2.11", 1},
		{"policy absent", "", "12", "20", "", "21", 1},
		{"explicit unchanged", "", "", "", "5", "5", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := 0
			swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet {
					t.Fatalf("unexpected mutation: %s", req.Method)
				}
				body := `{"data":[],"links":{}}`
				switch req.URL.Path {
				case "/v1/preReleaseVersions":
					if req.URL.Query().Get("filter[platform]") != "IOS" || req.URL.Query().Get("filter[version]") != "1.2.3" {
						t.Fatalf("wrong version scope: %s", req.URL)
					}
					body = `{"data":[{"type":"preReleaseVersions","id":"prv-1","attributes":{"version":"1.2.3","platform":"IOS"}}]}`
				case "/v1/builds":
					if tc.processed != "" {
						number := tc.processed
						if tc.name == "older processed above local" && req.URL.Query().Get("limit") == "1" {
							number = "12"
						}
						body = fmt.Sprintf(`{"data":[{"type":"builds","id":"build-1","attributes":{"version":%q,"uploadedDate":"2026-02-01T00:00:00Z"}}],"links":{}}`, number)
					}
				case "/v1/apps/123456789/buildUploads":
					if tc.upload != "" {
						body = fmt.Sprintf(`{"data":[{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleVersion":%q}}],"links":{}}`, tc.upload)
					}
				case "/v1/apps/123456789":
					body = `{"data":{"type":"apps","id":"123456789"}}`
				default:
					t.Fatalf("unexpected request: %s", req.URL)
				}
				return publishCommandJSONResponse(http.StatusOK, body)
			}))
			client := newPublishCommandTestClient(t)
			values := &publishLocalBuildFlagValues{initialBuildNumber: &tc.initial}
			if tc.local != "" {
				values.resolvedFloor = &localxcode.ArchiveBuildNumberFloor{BuildNumber: tc.local}
			}
			got, err := resolvePublishBuildNumber(context.Background(), newPublishTestDeps(), client, "123456789", "1.2.3", "IOS", values, tc.explicit)
			if err != nil || got != tc.want {
				t.Fatalf("number=%q error=%v, want %q", got, err, tc.want)
			}
			if tc.explicit != "" && requests != 0 {
				t.Fatalf("explicit number made %d requests", requests)
			}
		})
	}
}

func TestPublishLocalFloorStopsMismatchedArtifacts(t *testing.T) {
	for _, stage := range []string{"archive", "export"} {
		for _, mismatch := range []string{"bundle", "build", "none"} {
			if stage == "export" && mismatch == "none" {
				continue
			}
			t.Run(stage+"/"+mismatch, func(t *testing.T) {
				stubPublishCommandSeams(t)
				stages := stubPublishArchiveAndExport()
				archive, export := runPublishArchiveFn, runPublishExportFn
				runPublishArchiveFn = func(ctx context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
					result, err := archive(ctx, opts)
					if stage == "archive" && mismatch == "bundle" {
						result.BundleID = "com.example.other"
					}
					if stage == "archive" && mismatch == "build" {
						result.BuildNumber = "7"
					}
					return result, err
				}
				runPublishExportFn = func(ctx context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
					result, err := export(ctx, opts)
					if stage == "export" && mismatch == "bundle" {
						result.BundleID = "com.example.other"
					}
					if stage == "export" && mismatch == "build" {
						result.BuildNumber = "7"
					}
					return result, err
				}
				stop := errors.New("stop before upload")
				uploads := 0
				uploadBuildAndWaitForIDFn = func(context.Context, *shared.Deps, *asc.Client, string, string, os.FileInfo, string, string, asc.Platform, time.Duration, time.Duration, bool) (*publishUploadResult, error) {
					uploads++
					return nil, stop
				}
				config := publishLocalBuildConfig{Scheme: "Demo", Configuration: "Release", ExportOptionsPath: "ExportOptions.plist", IPAPath: filepath.Join(t.TempDir(), "Demo.ipa"), BuildNumberFloor: &localxcode.ArchiveBuildNumberFloor{Target: "Demo", Configuration: "Release", BundleID: "com.example.demo", BuildNumber: "41"}}
				_, err := runPublishLocalBuild(context.Background(), newPublishTestDeps(), newPublishCommandTestClient(t), "123456789", "IOS", "1.2.3", "42", time.Second, time.Minute, false, config)
				if mismatch == "none" {
					if !errors.Is(err, stop) || uploads != 1 {
						t.Fatalf("valid artifact did not reach upload boundary: calls=%d error=%v", uploads, err)
					}
				} else {
					if err == nil || !strings.Contains(err.Error(), stage+" local build: app identity does not match") || uploads != 0 {
						t.Fatalf("mismatch: uploads=%d error=%v", uploads, err)
					}
					if stage == "archive" && stages.exportCalls != 0 {
						t.Fatal("mismatched archive was exported")
					}
				}
			})
		}
	}
}
