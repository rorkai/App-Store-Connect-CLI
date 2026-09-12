package publish

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestPublishTestFlightPrebuiltPKGUploadOnly(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		if appID != "friendly-app" {
			t.Fatalf("expected friendly-app lookup, got %q", appID)
		}
		return "app-123", nil
	}
	validatePublishPKGPathFn = func(path string) (os.FileInfo, error) {
		if path != "Demo.pkg" {
			t.Fatalf("expected Demo.pkg validation, got %q", path)
		}
		return newPublishTestFileInfo(t)
	}
	uploadCalls := 0
	uploadPKGBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, path string, _ os.FileInfo, version, buildNumber string, platform asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
		if appID != "app-123" || path != "Demo.pkg" {
			t.Fatalf("unexpected PKG upload target: app=%q path=%q", appID, path)
		}
		if version != "1.2.3" || buildNumber != "42" || platform != asc.PlatformMacOS {
			t.Fatalf("unexpected PKG upload metadata: version=%q build=%q platform=%q", version, buildNumber, platform)
		}
		return publishUploadOnlyResult(version, buildNumber, asc.BuildProcessingStateProcessing), nil
	}
	requestCalls := rejectPublishUploadOnlyHTTP(t)

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--pkg", "Demo.pkg",
		"--version", "1.2.3",
		"--build-number", "42",
		"--upload-only",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	stdout, stderr := capturePublishCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if uploadCalls != 1 || *requestCalls != 0 {
		t.Fatalf("expected one PKG upload and no distribution calls; uploads=%d requests=%d", uploadCalls, *requestCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModePKGUpload) || payload["uploaded"] != true {
		t.Fatalf("unexpected PKG upload result: %#v", payload)
	}
}

func TestPublishAppStorePrebuiltPKGDryRun(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	validatePublishPKGPathFn = func(path string) (os.FileInfo, error) {
		if path != "Demo.pkg" {
			t.Fatalf("expected Demo.pkg validation, got %q", path)
		}
		return newPublishTestFileInfo(t)
	}

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--pkg", "Demo.pkg",
		"--version", "1.2.3",
		"--build-number", "42",
		"--dry-run",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	stdout, stderr := capturePublishCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModePKGUpload) {
		t.Fatalf("expected pkg_upload mode, got %#v", payload["mode"])
	}
	plan, ok := payload["plan"].([]any)
	if !ok || len(plan) == 0 {
		t.Fatalf("expected publish plan, got %#v", payload["plan"])
	}
	uploadStep, ok := plan[0].(map[string]any)
	if !ok || !strings.Contains(uploadStep["message"].(string), "PKG") {
		t.Fatalf("expected PKG upload step, got %#v", plan[0])
	}
}

func TestPublishAppStorePrebuiltPKGUploadsAndAttaches(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	validatePublishPKGPathFn = func(path string) (os.FileInfo, error) {
		if path != "Demo.pkg" {
			t.Fatalf("expected Demo.pkg validation, got %q", path)
		}
		return newPublishTestFileInfo(t)
	}
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		if appID != "friendly-app" {
			t.Fatalf("expected friendly-app lookup, got %q", appID)
		}
		return "app-123", nil
	}
	uploadCalls := 0
	uploadPKGBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, path string, _ os.FileInfo, version, buildNumber string, platform asc.Platform, pollInterval, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
		if appID != "app-123" || path != "Demo.pkg" || version != "1.2.3" || buildNumber != "42" {
			t.Fatalf("unexpected PKG upload: app=%q path=%q version=%q build=%q", appID, path, version, buildNumber)
		}
		if platform != asc.PlatformMacOS || pollInterval != 5*time.Second {
			t.Fatalf("unexpected PKG upload options: platform=%q poll=%s", platform, pollInterval)
		}
		return publishUploadOnlyResult(version, buildNumber, asc.BuildProcessingStateValid), nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCount := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"MAC_OS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case 2:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/build" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case 3:
			if req.Method != http.MethodPatch || req.URL.Path != "/v1/appStoreVersions/version-1/relationships/build" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--pkg", "Demo.pkg",
		"--version", "1.2.3",
		"--build-number", "42",
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	stdout, stderr := capturePublishCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if uploadCalls != 1 || requestCount != 3 {
		t.Fatalf("expected one PKG upload and three App Store requests; uploads=%d requests=%d", uploadCalls, requestCount)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModePKGUpload) || payload["uploaded"] != true || payload["attached"] != true {
		t.Fatalf("unexpected PKG App Store result: %#v", payload)
	}
}

func TestPublishCommandsRejectPKGPlatformMismatches(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
		wantErr string
	}{
		{
			name:    "testflight PKG",
			command: PublishTestFlightCommand,
			args:    []string{"--app", "123", "--pkg", "Demo.pkg", "--version", "1.2.3", "--build-number", "42", "--platform", "IOS", "--upload-only"},
			wantErr: "--platform IOS does not match PKG platform MAC_OS",
		},
		{
			name:    "appstore PKG",
			command: PublishAppStoreCommand,
			args:    []string{"--app", "123", "--pkg", "Demo.pkg", "--version", "1.2.3", "--build-number", "42", "--platform", "IOS", "--dry-run"},
			wantErr: "--platform IOS does not match PKG platform MAC_OS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()
			validatePublishIPAPathFn = func(path string) (os.FileInfo, error) { return os.Stat(path) }
			validatePublishPKGPathFn = func(string) (os.FileInfo, error) { return newPublishTestFileInfo(t) }

			cmd := test.command()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
		})
	}
}

func TestPublishMacOSIPARemainsAvailableWithDeprecationWarning(t *testing.T) {
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		platform, err := validatePublishPrebuiltArtifactPlatform("Demo.ipa", "", string(asc.PlatformMacOS), true)
		if err != nil {
			return err
		}
		if platform != string(asc.PlatformMacOS) {
			t.Fatalf("expected MAC_OS platform, got %q", platform)
		}
		return nil
	})
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	for _, want := range []string{"Warning:", "--ipa with --platform MAC_OS is deprecated", "Use --pkg with --version and --build-number"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected warning to contain %q, got %q", want, stderr)
		}
	}
}

func TestPublishPrebuiltPKGRequiresVersionAndBuildNumber(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
		wantErr string
	}{
		{name: "testflight both", command: PublishTestFlightCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--upload-only"}, wantErr: "--version and --build-number required for PKG uploads"},
		{name: "testflight version", command: PublishTestFlightCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--version", "1.2.3", "--upload-only"}, wantErr: "--build-number required for PKG uploads"},
		{name: "testflight build", command: PublishTestFlightCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--build-number", "42", "--upload-only"}, wantErr: "--version required for PKG uploads"},
		{name: "appstore both", command: PublishAppStoreCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--dry-run"}, wantErr: "--version and --build-number required for PKG uploads"},
		{name: "appstore version", command: PublishAppStoreCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--version", "1.2.3", "--dry-run"}, wantErr: "--build-number required for PKG uploads"},
		{name: "appstore build", command: PublishAppStoreCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--build-number", "42", "--dry-run"}, wantErr: "--version required for PKG uploads"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected %q, got %v", test.wantErr, err)
			}
		})
	}
}

func TestPublishPrebuiltArtifactsAreMutuallyExclusive(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "testflight", command: PublishTestFlightCommand, args: []string{"--app", "123", "--ipa", "Demo.ipa", "--pkg", "Demo.pkg", "--version", "1.2.3", "--build-number", "42", "--upload-only"}},
		{name: "appstore", command: PublishAppStoreCommand, args: []string{"--app", "123", "--ipa", "Demo.ipa", "--pkg", "Demo.pkg", "--version", "1.2.3", "--build-number", "42", "--dry-run"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), "--ipa and --pkg are mutually exclusive") {
				t.Fatalf("expected mutually exclusive artifact error, got %v", err)
			}
		})
	}
}

func TestPublishPrebuiltPKGConflictsWithLocalBuild(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "testflight", command: PublishTestFlightCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--workspace", "Demo.xcworkspace", "--scheme", "Demo", "--version", "1.2.3", "--build-number", "42", "--upload-only"}},
		{name: "appstore", command: PublishAppStoreCommand, args: []string{"--app", "123", "--pkg", "Demo.pkg", "--project", "Demo.xcodeproj", "--scheme", "Demo", "--version", "1.2.3", "--build-number", "42", "--dry-run"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.command()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := cmd.Exec(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), "--pkg cannot be combined with --workspace or --project") {
				t.Fatalf("expected local-build conflict, got %v", err)
			}
		})
	}
}

func TestPublishPKGFlagsAreDiscoverable(t *testing.T) {
	for _, command := range []*ffcli.Command{PublishTestFlightCommand(), PublishAppStoreCommand()} {
		pkgFlag := command.FlagSet.Lookup("pkg")
		if pkgFlag == nil {
			t.Fatalf("%s: expected --pkg flag", command.Name)
		}
		if !strings.Contains(pkgFlag.Usage, "macOS") || !strings.Contains(pkgFlag.Usage, "--version") || !strings.Contains(pkgFlag.Usage, "--build-number") {
			t.Fatalf("%s: expected PKG metadata help, got %q", command.Name, pkgFlag.Usage)
		}
	}
}
