package publish

import (
	"archive/zip"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

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
		}{
			{name: "visionOS", marker: "XROS", wantPlatform: "VISION_OS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "tvOS", marker: "AppleTVOS", wantPlatform: "TV_OS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "legacy default", wantPlatform: "IOS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "legacy explicit", platform: "VISION_OS", wantPlatform: "VISION_OS", wantVersion: "1.2.3", wantBuild: "42"},
			{name: "version overrides", marker: "XROS", wantPlatform: "VISION_OS", args: []string{"--version", "2.0", "--build-number", "99"}, wantVersion: "2.0", wantBuild: "99"},
			{name: "mismatch with overrides", marker: "XROS", platform: "IOS", args: []string{"--version", "2.0", "--build-number", "99"}, wantError: "Error: --platform IOS does not match IPA platform VISION_OS\n"},
		} {
			if command.name == "appstore dry run" && test.wantError == "" {
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
				args := append([]string{"--app", "123", "--ipa", writePublishIPA(t, test.marker), "--output", "json"}, command.args...)
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
	path := filepath.Join(t.TempDir(), "Demo.ipa")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("Payload/Demo.app/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(writer.Close(), file.Close()); err != nil {
		t.Fatal(err)
	}
	return path
}
