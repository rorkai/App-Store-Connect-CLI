package publish

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestPublishCommandsCheckIPABundleBeforeReservation(t *testing.T) {
	for _, command := range []struct {
		name string
		new  func(*shared.Deps) *ffcli.Command
		args []string
	}{
		{"testflight", newPublishTestFlightCommand, []string{"--upload-only"}},
		{"appstore", newPublishAppStoreCommand, nil},
	} {
		for _, test := range []struct {
			name, appBundleID, wantError string
			missingPlist, missingBundle  bool
			metadataChanged              bool
			metadataAmbiguous            bool
			inferMetadata                bool
			wantValidation               bool
			appStatus                    int
			wantRequests                 []string
		}{
			{name: "mismatch", appBundleID: "com.example.other", wantValidation: true, wantError: `IPA bundle ID "com.example.app" does not match selected app "123" bundle ID "com.example.other"`, wantRequests: []string{"GET /v1/apps/123"}},
			{name: "match", appBundleID: "COM.EXAMPLE.APP", wantError: "stop at reservation", wantRequests: []string{"GET /v1/apps/123", "POST /v1/buildUploads"}},
			{name: "missing app bundle", wantError: `selected app "123" has no bundle ID`, wantRequests: []string{"GET /v1/apps/123"}},
			{name: "app read failure", appStatus: http.StatusForbidden, wantError: "app lookup refused", wantRequests: []string{"GET /v1/apps/123"}},
			{name: "missing IPA bundle", missingBundle: true, wantValidation: true, wantError: "IPA top-level app Info.plist is missing CFBundleIdentifier"},
			{name: "unreadable plist with overrides", missingPlist: true, wantError: "stop at reservation", wantRequests: []string{"POST /v1/buildUploads"}},
			{name: "inferred metadata becomes unreadable", metadataChanged: true, inferMetadata: true, wantError: "inspect IPA metadata before upload"},
			{name: "readable metadata becomes unreadable with overrides", metadataChanged: true, wantError: "inspect IPA metadata before upload"},
			{name: "unreadable metadata becomes ambiguous with overrides", missingPlist: true, metadataAmbiguous: true, wantError: "IPA has multiple top-level app Info.plist entries"},
		} {
			t.Run(command.name+"/"+test.name, func(t *testing.T) {
				t.Cleanup(overridePublishCommandTestHooks(t))
				publishTestClientWithTimeout = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
				lookupDeadline := time.Now().Add(10 * time.Second)
				var requests []string
				swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests = append(requests, req.Method+" "+req.URL.Path)
					switch req.Method + " " + req.URL.Path {
					case "GET /v1/apps/123":
						if deadline, ok := req.Context().Deadline(); !ok || !deadline.Equal(lookupDeadline) {
							t.Errorf("app lookup deadline=%v, want normal request deadline=%v", deadline, lookupDeadline)
						}
						if test.appStatus != 0 {
							return publishCommandJSONResponse(test.appStatus, `{"errors":[{"status":"403","code":"FORBIDDEN","title":"app lookup refused"}]}`)
						}
						return publishCommandJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"apps","id":"123","attributes":{"bundleId":%q}}}`, test.appBundleID))
					case "POST /v1/buildUploads":
						var body asc.BuildUploadCreateRequest
						if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
							t.Fatal(err)
						}
						wantVersion, wantBuild := "2.0", "99"
						if test.inferMetadata {
							wantVersion, wantBuild = "1.2.3", "42"
						}
						if body.Data.Attributes.CFBundleShortVersionString != wantVersion || body.Data.Attributes.CFBundleVersion != wantBuild || body.Data.Attributes.Platform != asc.PlatformIOS || body.Data.Relationships.App.Data.ID != "123" {
							t.Fatalf("unexpected reservation: %+v", body.Data)
						}
						return publishCommandJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","code":"INVALID","title":"stop at reservation"}]}`)
					default:
						t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
						return nil, nil
					}
				}))
				ipaPath := writePublishIPA(t, "iPhoneOS")
				if test.missingPlist {
					ipaPath = writePublishIPAFile(t, "Payload/Demo.app/Demo", []byte("app executable fixture"))
				} else if test.missingBundle {
					ipaPath = writePublishIPAFile(t, "Payload/Demo.app/Info.plist", []byte(`<?xml version="1.0"?><plist version="1.0"><dict><key>CFBundleVersion</key><string>42</string></dict></plist>`))
				}
				resolvePublishAppIDWithLookupFn = func(context.Context, *asc.Client, string) (string, error) {
					if test.metadataAmbiguous {
						info := []byte(`<plist><dict><key>CFBundleIdentifier</key><string>com.example.app</string></dict></plist>`)
						replacement := writePublishIPAFile(t, "Payload/Demo.app/Info.plist", info, "Payload/Other.app/Info.plist")
						data, err := os.ReadFile(replacement)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(ipaPath, data, 0o600); err != nil {
							t.Fatal(err)
						}
					}
					if test.metadataChanged {
						info, err := os.Stat(ipaPath)
						if err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(ipaPath, make([]byte, info.Size()), 0o600); err != nil {
							t.Fatal(err)
						}
						if err := os.Chtimes(ipaPath, info.ModTime(), info.ModTime()); err != nil {
							t.Fatal(err)
						}
					}
					return "123", nil
				}
				d := newPublishTestDeps()
				d.Timeout = func(ctx context.Context) (context.Context, context.CancelFunc) {
					return context.WithDeadline(ctx, lookupDeadline)
				}
				cmd := command.new(d)
				cmd.FlagSet.SetOutput(io.Discard)
				args := append([]string{"--app", "123", "--ipa", ipaPath, "--output", "json"}, command.args...)
				if !test.inferMetadata {
					args = append(args, "--version", "2.0", "--build-number", "99")
				}
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatal(err)
				}
				var runErr error
				stdout, stderr := capturePublishDepsOutput(t, d, func() error {
					runErr = cmd.Exec(context.Background(), nil)
					return nil
				})
				if runErr == nil || !strings.Contains(runErr.Error(), test.wantError) || stdout != "" || stderr != "" || !reflect.DeepEqual(requests, test.wantRequests) {
					t.Fatalf("err=%v, stdout=%q, stderr=%q, requests=%v; want error %q, requests=%v", runErr, stdout, stderr, requests, test.wantError, test.wantRequests)
				}
				if test.wantValidation {
					diagnostic, ok := shared.DiagnosticFromError(runErr)
					if !shared.IsValidationError(runErr) || !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--ipa" {
						t.Fatalf("expected --ipa invalid-input validation error, got %v (%+v)", runErr, diagnostic)
					}
				}
			})
		}
	}
}

func TestPublishCommandsSelectPrebuiltIPAPlatform(t *testing.T) {
	for _, command := range []struct {
		name string
		new  func(*shared.Deps) *ffcli.Command
		args []string
	}{
		{"testflight", newPublishTestFlightCommand, []string{"--upload-only"}},
		{"appstore", newPublishAppStoreCommand, nil},
		{"appstore dry run", newPublishAppStoreCommand, []string{"--dry-run"}},
	} {
		for _, test := range []struct {
			name, marker, platform, wantPlatform string
			args                                 []string
			wantVersion, wantBuild, wantError    string
			missingInfoPlist                     bool
			wantExtractionError                  string
		}{
			{name: "visionOS", marker: "XROS", wantPlatform: "VISION_OS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "tvOS", marker: "AppleTVOS", wantPlatform: "TV_OS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "detected macOS", marker: "MacOSX", wantError: "Error: --ipa contains a macOS app; use --pkg with --version and --build-number\n"},
			{name: "macOS mismatch", marker: "MacOSX", platform: "IOS", wantError: "Error: --platform IOS does not match IPA platform MAC_OS\n"},
			{name: "legacy default", wantPlatform: "IOS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "legacy explicit", platform: "VISION_OS", wantPlatform: "VISION_OS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "version overrides", marker: "XROS", wantPlatform: "VISION_OS", args: []string{"--version", "2.0", "--build-number", "99"}, wantVersion: "2.0", wantBuild: "99"},
			{name: "mismatch with overrides", marker: "XROS", platform: "IOS", args: []string{"--version", "2.0", "--build-number", "99"}, wantError: "Error: --platform IOS does not match IPA platform VISION_OS\n"},
			{name: "missing plist with overrides", missingInfoPlist: true, wantPlatform: "IOS", args: []string{"--version", "2.0", "--build-number", "99"}, wantVersion: "2.0", wantBuild: "99"},
			{name: "missing plist with explicit platform", missingInfoPlist: true, platform: "VISION_OS", wantPlatform: "VISION_OS", args: []string{"--version", "2.0", "--build-number", "99"}, wantVersion: "2.0", wantBuild: "99"},
			{name: "missing plist needs build number", missingInfoPlist: true, args: []string{"--version", "2.0"}, wantExtractionError: "--build-number required (failed to extract from IPA: missing Info.plist in IPA)"},
		} {
			if command.name == "appstore dry run" && test.wantError == "" && !test.missingInfoPlist {
				continue
			}
			t.Run(command.name+"/"+test.name, func(t *testing.T) {
				t.Cleanup(overridePublishCommandTestHooks(t))
				clientCalls := 0
				publishTestClientWithTimeout = func(time.Duration) (*asc.Client, error) {
					clientCalls++
					return newPublishCommandTestClient(t), nil
				}
				resolvePublishAppIDWithLookupFn = func(context.Context, *asc.Client, string) (string, error) { return "123", nil }
				requests := rejectPublishUploadOnlyHTTP(t)
				stop := errors.New("stopped before upload")
				var gotPlatform asc.Platform
				var gotVersion, gotBuild string
				uploadBuildAndWaitForIDFn = func(_ context.Context, _ *shared.Deps, _ *asc.Client, _, _ string, _ os.FileInfo, version, build string, platform asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
					gotPlatform, gotVersion, gotBuild = platform, version, build
					return nil, stop
				}
				d := newPublishTestDeps()
				cmd := command.new(d)
				cmd.FlagSet.SetOutput(io.Discard)
				var ipaPath string
				if test.missingInfoPlist {
					ipaPath = writePublishIPAFile(t, "Payload/Demo.app/Demo", []byte("app executable fixture"))
				} else {
					ipaPath = writePublishIPA(t, test.marker)
				}
				args := append([]string{"--app", "123", "--ipa", ipaPath, "--output", "json"}, command.args...)
				args = append(args, test.args...)
				if test.platform != "" {
					args = append(args, "--platform", test.platform)
				}
				if err := cmd.FlagSet.Parse(args); err != nil {
					t.Fatal(err)
				}
				var runErr error
				stdout, stderr := capturePublishDepsOutput(t, d, func() error {
					runErr = cmd.Exec(context.Background(), nil)
					return nil
				})
				if test.wantError != "" {
					if !errors.Is(runErr, flag.ErrHelp) || stderr != test.wantError || stdout != "" || clientCalls != 0 {
						t.Fatalf("mismatch: err=%v stderr=%q stdout=%q client calls=%d", runErr, stderr, stdout, clientCalls)
					}
				} else if test.wantExtractionError != "" {
					if runErr == nil || !strings.HasSuffix(runErr.Error(), test.wantExtractionError) || stdout != "" || clientCalls != 0 {
						t.Fatalf("extraction: err=%v stdout=%q client calls=%d", runErr, stdout, clientCalls)
					}
				} else if command.name == "appstore dry run" {
					var result asc.AppStorePublishResult
					if err := json.Unmarshal([]byte(stdout), &result); err != nil {
						t.Fatalf("decode dry-run: %v; command error: %v", err, runErr)
					}
					if runErr != nil || !result.DryRun || result.BuildVersion != test.wantVersion || result.BuildNumber != test.wantBuild || clientCalls != 0 || stderr != "" {
						t.Fatalf("dry-run: result=%+v err=%v stderr=%q client calls=%d", result, runErr, stderr, clientCalls)
					}
				} else if !errors.Is(runErr, stop) || string(gotPlatform) != test.wantPlatform || gotVersion != test.wantVersion || gotBuild != test.wantBuild || stderr != "" {
					t.Fatalf("upload: platform=%q version=%q build=%q err=%v stderr=%q", gotPlatform, gotVersion, gotBuild, runErr, stderr)
				}
				if *requests != 0 {
					t.Fatalf("unexpected HTTP requests: %d", *requests)
				}
			})
		}
	}
}

func writePublishIPA(t *testing.T, platform string) string {
	t.Helper()
	info := map[string]any{"CFBundleIdentifier": "com.example.app", "CFBundleShortVersionString": "1.2.3", "CFBundleVersion": "42"}
	if platform != "" {
		info["CFBundleSupportedPlatforms"] = []string{platform}
	}
	data, err := plist.Marshal(info, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	return writePublishIPAFile(t, "Payload/Demo.app/Info.plist", data)
}

func writePublishIPAFile(t *testing.T, name string, data []byte, extraNames ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Demo.ipa")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, name := range append([]string{name}, extraNames...) {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := errors.Join(writer.Close(), file.Close()); err != nil {
		t.Fatal(err)
	}
	return path
}
