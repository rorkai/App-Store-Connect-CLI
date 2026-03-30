package publish

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
	"howett.net/plist"
)

func TestPublishTestFlightLocalBuildJSONIncludesNestedStages(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		if appID != "friendly-app" {
			t.Fatalf("expected unresolved app input to be passed through lookup, got %q", appID)
		}
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	resolvePublishNextBuildNumberFn = func(_ context.Context, _ *asc.Client, opts shared.NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
		if opts.LatestBuildSelectionOptions.AppID != "app-123" {
			t.Fatalf("expected resolved app ID for next build number, got %q", opts.LatestBuildSelectionOptions.AppID)
		}
		if opts.LatestBuildSelectionOptions.Version != "1.2.3" {
			t.Fatalf("expected version 1.2.3 for next build number, got %q", opts.LatestBuildSelectionOptions.Version)
		}
		return &asc.BuildsNextBuildNumberResult{NextBuildNumber: "42"}, nil
	}

	var gotArchiveOpts localxcode.ArchiveOptions
	runPublishArchiveFn = func(_ context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		gotArchiveOpts = opts
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}

	var gotExportOpts localxcode.ExportOptions
	runPublishExportFn = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		gotExportOpts = opts
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}

	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, ipaPath string, _ os.FileInfo, version, buildNumber string, platform asc.Platform, pollInterval, _ time.Duration, _ bool) (*publishUploadResult, error) {
		if appID != "app-123" {
			t.Fatalf("expected resolved app ID for upload, got %q", appID)
		}
		if ipaPath != ".asc/artifacts/Demo-IOS-1.2.3-42.ipa" {
			t.Fatalf("expected exported IPA path, got %q", ipaPath)
		}
		if version != "1.2.3" || buildNumber != "42" {
			t.Fatalf("unexpected upload metadata: version=%q build=%q", version, buildNumber)
		}
		if platform != asc.Platform("IOS") {
			t.Fatalf("expected IOS platform, got %q", platform)
		}
		if pollInterval != 5*time.Second {
			t.Fatalf("expected poll interval 5s, got %s", pollInterval)
		}
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         "42",
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	requestCount := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-123/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--group", "External",
		"--export-options", "ExportOptions.plist",
		"--archive-xcodebuild-flag=-quiet",
		"--export-xcodebuild-flag=-skipUnavailableActions",
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}

	if gotArchiveOpts.Configuration != "Release" {
		t.Fatalf("expected Release configuration, got %q", gotArchiveOpts.Configuration)
	}
	if !containsString(gotArchiveOpts.XcodebuildArgs, "-destination") || !containsString(gotArchiveOpts.XcodebuildArgs, "generic/platform=iOS") {
		t.Fatalf("expected archive destination defaults, got %v", gotArchiveOpts.XcodebuildArgs)
	}
	if !containsString(gotArchiveOpts.XcodebuildArgs, "MARKETING_VERSION=1.2.3") {
		t.Fatalf("expected MARKETING_VERSION override, got %v", gotArchiveOpts.XcodebuildArgs)
	}
	if !containsString(gotArchiveOpts.XcodebuildArgs, "CURRENT_PROJECT_VERSION=42") {
		t.Fatalf("expected CURRENT_PROJECT_VERSION override, got %v", gotArchiveOpts.XcodebuildArgs)
	}
	if !containsString(gotArchiveOpts.XcodebuildArgs, "-allowProvisioningUpdates") {
		t.Fatalf("expected archive provisioning updates flag, got %v", gotArchiveOpts.XcodebuildArgs)
	}
	if gotArchiveOpts.XcodebuildArgs[len(gotArchiveOpts.XcodebuildArgs)-1] != "-quiet" {
		t.Fatalf("expected custom archive arg to be appended last, got %v", gotArchiveOpts.XcodebuildArgs)
	}
	if gotExportOpts.ExportOptions != "ExportOptions.plist" {
		t.Fatalf("expected explicit export options path, got %q", gotExportOpts.ExportOptions)
	}
	if !containsString(gotExportOpts.XcodebuildArgs, "-allowProvisioningUpdates") {
		t.Fatalf("expected export provisioning updates flag, got %v", gotExportOpts.XcodebuildArgs)
	}
	if gotExportOpts.XcodebuildArgs[len(gotExportOpts.XcodebuildArgs)-1] != "-skipUnavailableActions" {
		t.Fatalf("expected custom export arg to be appended last, got %v", gotExportOpts.XcodebuildArgs)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModeLocalBuild) {
		t.Fatalf("expected local_build mode, got %#v", payload["mode"])
	}
	if payload["buildId"] != "build-123" {
		t.Fatalf("expected buildId build-123, got %#v", payload["buildId"])
	}
	if payload["buildVersion"] != "1.2.3" {
		t.Fatalf("expected buildVersion 1.2.3, got %#v", payload["buildVersion"])
	}
	if payload["buildNumber"] != "42" {
		t.Fatalf("expected buildNumber 42, got %#v", payload["buildNumber"])
	}
	if payload["uploaded"] != true {
		t.Fatalf("expected uploaded=true, got %#v", payload["uploaded"])
	}
	archivePayload, ok := payload["archive"].(map[string]any)
	if !ok {
		t.Fatalf("expected archive payload, got %#v", payload["archive"])
	}
	if archivePayload["archivePath"] != ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive" {
		t.Fatalf("unexpected archivePath: %#v", archivePayload["archivePath"])
	}
	exportPayload, ok := payload["export"].(map[string]any)
	if !ok {
		t.Fatalf("expected export payload, got %#v", payload["export"])
	}
	if exportPayload["ipaPath"] != ".asc/artifacts/Demo-IOS-1.2.3-42.ipa" {
		t.Fatalf("unexpected ipaPath: %#v", exportPayload["ipaPath"])
	}
	if exportPayload["directUpload"] != false {
		t.Fatalf("expected directUpload=false, got %#v", exportPayload["directUpload"])
	}
	publishPayload, ok := payload["publish"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested publish payload, got %#v", payload["publish"])
	}
	if publishPayload["buildId"] != "build-123" {
		t.Fatalf("unexpected nested buildId: %#v", publishPayload["buildId"])
	}
	if !strings.Contains(stdout, `"archivePath"`) || !strings.Contains(stdout, `"exportOptionsPath"`) {
		t.Fatalf("expected camelCase nested keys, got %s", stdout)
	}
	if strings.Contains(stdout, `"archive_path"`) || strings.Contains(stdout, `"export_options_path"`) {
		t.Fatalf("expected no snake_case nested keys, got %s", stdout)
	}
}

func TestPublishTestFlightLocalBuildRejectsDirectUploadExportOptions(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	tempDir := t.TempDir()
	exportOptionsPath := filepath.Join(tempDir, "UploadExportOptions.plist")
	payload, err := plist.Marshal(map[string]any{"destination": "upload"}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}
	if err := os.WriteFile(exportOptionsPath, payload, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		t.Fatal("did not expect archive to run for unsupported direct-upload export options")
		return nil, nil
	}
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		t.Fatal("did not expect export to run for unsupported direct-upload export options")
		return nil, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	requestCount := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"group-1","isInternalGroup":false}}]}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-789/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "44",
		"--group", "group-1",
		"--export-options", exportOptionsPath,
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--export-options with destination=upload is not supported by publish") {
		t.Fatalf("expected direct-upload rejection, got %q", stderr)
	}
}

func TestPublishTestFlightLocalBuildTableIncludesArchiveAndExportSections(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         buildNumber,
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-123/betaGroups":
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"group-1","isInternalGroup":false}}]}`)
		case "/v1/builds/build-123/relationships/betaGroups":
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "group-1",
		"--export-options", "ExportOptions.plist",
		"--output", "table",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if !strings.Contains(stdout, "Build ID") || !strings.Contains(stdout, "build-123") {
		t.Fatalf("expected top-level publish summary in table output, got %s", stdout)
	}
	if !strings.Contains(stdout, "archive_path") || !strings.Contains(stdout, "ipa_path") {
		t.Fatalf("expected archive/export sections in table output, got %s", stdout)
	}
}

func TestPublishTestFlightLocalBuildUsesDefaultExportOptionsPath(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	tempDir := t.TempDir()
	defaultPath := filepath.Join(tempDir, ".asc", "export-options-app-store.plist")
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(defaultPath, []byte("plist"), 0o600); err != nil {
		t.Fatalf("write default export options: %v", err)
	}
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}
	var gotExportOptionsPath string
	runPublishExportFn = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		gotExportOptionsPath = opts.ExportOptions
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         buildNumber,
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-123/betaGroups":
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"group-1","isInternalGroup":false}}]}`)
		case "/v1/builds/build-123/relationships/betaGroups":
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "group-1",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	_, _ = capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if gotExportOptionsPath != defaultPublishExportOptionsPath {
		t.Fatalf("expected default export options path %q, got %q", defaultPublishExportOptionsPath, gotExportOptionsPath)
	}
}

func TestPublishTestFlightLocalBuildUsesFreshUploadTimeoutAfterArchive(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	resolvePublishNextBuildNumberFn = func(_ context.Context, _ *asc.Client, _ shared.NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
		return &asc.BuildsNextBuildNumberResult{NextBuildNumber: "42"}, nil
	}
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		time.Sleep(150 * time.Millisecond)
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(ctx context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, timeout time.Duration, timeoutOverride bool) (*publishUploadResult, error) {
		if !timeoutOverride {
			t.Fatal("expected timeout override for local-build upload")
		}
		if timeout != 100*time.Millisecond {
			t.Fatalf("expected upload timeout 100ms, got %s", timeout)
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("expected fresh upload context after archive/export, got %v", err)
		}
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         buildNumber,
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-123/betaGroups":
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"group-1","isInternalGroup":false}}]}`)
		case "/v1/builds/build-123/relationships/betaGroups":
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--group", "group-1",
		"--export-options", "ExportOptions.plist",
		"--timeout", "100ms",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
}

func TestPublishTestFlightIPAUploadResolvesAppIDBeforeGroupLookupAndUpload(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}

	lookupCalls := 0
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		lookupCalls++
		if appID != "friendly-app" {
			t.Fatalf("expected unresolved app input to be passed through lookup, got %q", appID)
		}
		return "app-123", nil
	}

	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, _ string, _ os.FileInfo, version, buildNumber string, platform asc.Platform, pollInterval, _ time.Duration, _ bool) (*publishUploadResult, error) {
		if appID != "app-123" {
			t.Fatalf("expected resolved app ID for upload, got %q", appID)
		}
		if version != "1.2.3" || buildNumber != "42" {
			t.Fatalf("unexpected upload metadata: version=%q build=%q", version, buildNumber)
		}
		if platform != asc.Platform("IOS") {
			t.Fatalf("expected IOS platform, got %q", platform)
		}
		if pollInterval != 5*time.Second {
			t.Fatalf("expected poll interval 5s, got %s", pollInterval)
		}
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         buildNumber,
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	requestCount := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"group-1","isInternalGroup":false}}]}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-123/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request count %d", requestCount)
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "group-1",
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if lookupCalls != 1 {
		t.Fatalf("expected exactly one app lookup, got %d", lookupCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModeIPAUpload) {
		t.Fatalf("expected ipa_upload mode, got %#v", payload["mode"])
	}
	if payload["uploaded"] != true {
		t.Fatalf("expected uploaded=true, got %#v", payload["uploaded"])
	}
	if payload["buildId"] != "build-123" {
		t.Fatalf("expected buildId build-123, got %#v", payload["buildId"])
	}
}

func TestPublishAppStoreLocalBuildRequiresExportOptionsWhenDefaultMissing(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	_, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "--export-options is required in local-build mode when .asc/export-options-app-store.plist is missing") {
		t.Fatalf("expected missing export-options error, got %q", stderr)
	}
}

func TestPublishAppStoreLocalBuildRejectsDirectUploadExportOptions(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	tempDir := t.TempDir()
	exportOptionsPath := filepath.Join(tempDir, "UploadExportOptions.plist")
	payload, err := plist.Marshal(map[string]any{"destination": "upload"}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}
	if err := os.WriteFile(exportOptionsPath, payload, 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		t.Fatal("did not expect archive to run for unsupported direct-upload export options")
		return nil, nil
	}
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		t.Fatal("did not expect export to run for unsupported direct-upload export options")
		return nil, nil
	}

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--export-options", exportOptionsPath,
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--export-options with destination=upload is not supported by publish") {
		t.Fatalf("expected direct-upload rejection, got %q", stderr)
	}
}

func TestPublishAppStoreLocalBuildUsesFreshUploadTimeoutAfterArchive(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	resolvePublishNextBuildNumberFn = func(_ context.Context, _ *asc.Client, _ shared.NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
		return &asc.BuildsNextBuildNumberResult{NextBuildNumber: "42"}, nil
	}
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		time.Sleep(150 * time.Millisecond)
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(ctx context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, timeout time.Duration, timeoutOverride bool) (*publishUploadResult, error) {
		if !timeoutOverride {
			t.Fatal("expected timeout override for local-build upload")
		}
		if timeout != 100*time.Millisecond {
			t.Fatalf("expected upload timeout 100ms, got %s", timeout)
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("expected fresh upload context after archive/export, got %v", err)
		}
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         buildNumber,
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/apps/app-123/appStoreVersions":
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case "/v1/appStoreVersions/version-1/relationships/build":
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--export-options", "ExportOptions.plist",
		"--timeout", "100ms",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); err != nil {
		t.Fatalf("Exec() error: %v", err)
	}
}

func TestPublishAppStoreIPAUploadResolvesAppIDBeforeUploadAndAttach(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func() (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}

	lookupCalls := 0
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		lookupCalls++
		if appID != "friendly-app" {
			t.Fatalf("expected unresolved app input to be passed through lookup, got %q", appID)
		}
		return "app-123", nil
	}

	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, _ string, _ os.FileInfo, version, buildNumber string, platform asc.Platform, pollInterval, _ time.Duration, _ bool) (*publishUploadResult, error) {
		if appID != "app-123" {
			t.Fatalf("expected resolved app ID for upload, got %q", appID)
		}
		if version != "1.2.3" || buildNumber != "42" {
			t.Fatalf("unexpected upload metadata: version=%q build=%q", version, buildNumber)
		}
		if platform != asc.Platform("IOS") {
			t.Fatalf("expected IOS platform, got %q", platform)
		}
		if pollInterval != 5*time.Second {
			t.Fatalf("expected poll interval 5s, got %s", pollInterval)
		}
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID: "build-123",
					Attributes: asc.BuildAttributes{
						Version:         buildNumber,
						ProcessingState: asc.BuildProcessingStateValid,
					},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	requestCount := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/appStoreVersions" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case 2:
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
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if lookupCalls != 1 {
		t.Fatalf("expected exactly one app lookup, got %d", lookupCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModeIPAUpload) {
		t.Fatalf("expected ipa_upload mode, got %#v", payload["mode"])
	}
	if payload["uploaded"] != true {
		t.Fatalf("expected uploaded=true, got %#v", payload["uploaded"])
	}
	if payload["buildId"] != "build-123" {
		t.Fatalf("expected buildId build-123, got %#v", payload["buildId"])
	}
	if payload["versionId"] != "version-1" {
		t.Fatalf("expected versionId version-1, got %#v", payload["versionId"])
	}
}

func overridePublishCommandTestHooks(t *testing.T) func() {
	t.Helper()

	originalArchive := runPublishArchiveFn
	originalExport := runPublishExportFn
	originalGetClient := getPublishASCClientFn
	originalResolveNextBuildNumber := resolvePublishNextBuildNumberFn
	originalValidateIPAPath := validatePublishIPAPathFn
	originalUploadBuildAndWait := uploadBuildAndWaitForIDFn
	originalResolveAppID := resolvePublishAppIDWithLookupFn
	originalWaitForProcessing := waitForPublishBuildProcessingFn

	return func() {
		runPublishArchiveFn = originalArchive
		runPublishExportFn = originalExport
		getPublishASCClientFn = originalGetClient
		resolvePublishNextBuildNumberFn = originalResolveNextBuildNumber
		validatePublishIPAPathFn = originalValidateIPAPath
		uploadBuildAndWaitForIDFn = originalUploadBuildAndWait
		resolvePublishAppIDWithLookupFn = originalResolveAppID
		waitForPublishBuildProcessingFn = originalWaitForProcessing
	}
}

func newPublishCommandTestClient(t *testing.T) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("encode pem: nil")
	}

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func capturePublishCommandOutput(t *testing.T, fn func() error) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		_ = rOut.Close()
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		_ = rErr.Close()
		errC <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = wOut.Close()
		_ = wErr.Close()
	}()

	_ = fn()

	_ = wOut.Close()
	_ = wErr.Close()

	stdout := <-outC
	stderr := <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return stdout, stderr
}

func newPublishTestFileInfo(t *testing.T) (os.FileInfo, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "Demo.ipa")
	if err := os.WriteFile(path, []byte("ipa"), 0o600); err != nil {
		return nil, err
	}
	return os.Stat(path)
}

type publishCommandRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn publishCommandRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func publishCommandJSONResponse(statusCode int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
