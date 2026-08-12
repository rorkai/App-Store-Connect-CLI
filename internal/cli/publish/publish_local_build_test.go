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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
	"howett.net/plist"
)

func TestPublishTestFlightLocalBuildJSONIncludesNestedStages(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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

func TestPublishLocalBuildRejectsManagedExportFlagsBeforeSideEffects(t *testing.T) {
	for _, test := range []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{
			name:    "testflight",
			command: PublishTestFlightCommand,
			args: []string{
				"--app", "app-1",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--group", "External",
			},
		},
		{
			name:    "appstore",
			command: PublishAppStoreCommand,
			args: []string{
				"--app", "app-1",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()

			getPublishASCClientFn = func(time.Duration) (*asc.Client, error) {
				t.Fatal("client creation ran before export flag validation")
				return nil, nil
			}
			preflightPublishXcodeFn = func(context.Context) error {
				t.Fatal("Xcode preflight ran before export flag validation")
				return nil
			}
			runPublishArchiveFn = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
				t.Fatal("archive ran before export flag validation")
				return nil, nil
			}
			generatePublishExportOptionsFn = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
				t.Fatal("export-options generation ran before export flag validation")
				return nil, nil
			}
			runPublishExportFn = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				t.Fatal("export ran before export flag validation")
				return nil, nil
			}

			cmd := test.command()
			cmd.FlagSet.SetOutput(io.Discard)
			args := append(append([]string(nil), test.args...), "--export-xcodebuild-flag=-exportPath=/tmp/elsewhere")
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stdout, stderr := capturePublishCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			wantError := `--export-xcodebuild-flag cannot override asc-managed argument "-exportPath"`
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("Exec() error = %T %v, want usage error", runErr, runErr)
			}
			if runErr.Error() != wantError {
				t.Fatalf("Exec() error = %q, want %q", runErr.Error(), wantError)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "Error: "+wantError+"\n" {
				t.Fatalf("stderr = %q, want %q", stderr, "Error: "+wantError+"\n")
			}
		})
	}
}

func TestPublishTestFlightLocalBuildRetriesPostUploadBuildPropagationWithoutRepeatingBuildStages(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}

	archiveCalls := 0
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		archiveCalls++
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}

	exportCalls := 0
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		exportCalls++
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}

	uploadCalls := 0
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
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
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-123/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'build-123'"}]}`)
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-123" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-123","attributes":{"version":"42","processingState":"VALID"}}}`)
		case 4:
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
		"--app", "app-123",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--export-options", "ExportOptions.plist",
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
	if archiveCalls != 1 || exportCalls != 1 || uploadCalls != 1 {
		t.Fatalf("build stage calls = archive:%d export:%d upload:%d, want each exactly once", archiveCalls, exportCalls, uploadCalls)
	}
	if requestCount != 4 {
		t.Fatalf("request count = %d, want 4", requestCount)
	}
	if !strings.Contains(stdout, `"buildId":"build-123"`) {
		t.Fatalf("expected uploaded build ID in output, got %q", stdout)
	}
	if strings.Contains(stdout, "still propagating") {
		t.Fatalf("retry diagnostics must not be written to stdout, got %q", stdout)
	}
	wantDiagnostic := "Uploaded TestFlight build build-123 is still propagating; retrying beta-group assignment (attempt 2/7) immediately.\n"
	if stderr != wantDiagnostic {
		t.Fatalf("stderr = %q, want retry diagnostic %q", stderr, wantDiagnostic)
	}
}

func TestPublishTestFlightLocalBuildReportsStructuredRecoveryResultAfterNotificationFailure(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}

	archiveCalls := 0
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		archiveCalls++
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}

	exportCalls := 0
	runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		exportCalls++
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}

	uploadCalls := 0
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
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
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-123/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-123/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'build-123'"}]}`)
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-123",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--notify",
		"--export-options", "ExportOptions.plist",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || !strings.Contains(runErr.Error(), `beta groups were added to build "build-123", but checking notification state failed`) {
		t.Fatalf("expected notification partial error, got %v", runErr)
	}
	if archiveCalls != 1 || exportCalls != 1 || uploadCalls != 1 {
		t.Fatalf("build stage calls = archive:%d export:%d upload:%d, want each exactly once", archiveCalls, exportCalls, uploadCalls)
	}

	var result asc.TestFlightPublishResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial publish result: %v\nstdout=%s", err, stdout)
	}
	if result.Status != publishPartialStatus || result.FailureStage != publishFailureStageNotification {
		t.Fatalf("unexpected partial status: status=%q stage=%q", result.Status, result.FailureStage)
	}
	if result.BuildID != "build-123" || !result.Uploaded {
		t.Fatalf("expected recoverable uploaded build, got buildId=%q uploaded=%t", result.BuildID, result.Uploaded)
	}
	wantCompleted := []string{publishCompletedStageArchive, publishCompletedStageExport, publishCompletedStageUpload, publishCompletedStageBetaDistribution}
	if !slices.Equal(result.CompletedStages, wantCompleted) {
		t.Fatalf("completed stages = %v, want %v", result.CompletedStages, wantCompleted)
	}
	if result.Archive == nil || result.Export == nil || result.Publish == nil {
		t.Fatalf("expected completed local stage details, got archive=%#v export=%#v publish=%#v", result.Archive, result.Export, result.Publish)
	}
	if result.Publish.BuildID != "build-123" || !result.Publish.Uploaded {
		t.Fatalf("unexpected nested publish recovery result: %#v", result.Publish)
	}
	if strings.Contains(stderr, "still propagating") {
		t.Fatalf("notification follow-up failure must not enter relationship retry path, stderr=%q", stderr)
	}
}

func TestPublishTestFlightUploadReportsStructuredRecoveryResultAfterBetaReviewFailure(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadCalls := 0
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
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
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
		case 2:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/builds/build-123/relationships/betaGroups" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		case 3:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-123/buildBetaDetail" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`)
		case 4:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/buildBetaNotifications" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildBetaNotifications","id":"notification-1"}}`)
		case 5:
			if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-123/betaAppReviewSubmission" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case 6:
			if req.Method != http.MethodPost || req.URL.Path != "/v1/betaAppReviewSubmissions" {
				t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			}
			return publishCommandJSONResponse(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"STATE_ERROR","title":"Submission rejected","detail":"review state is not ready"}]}`)
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			return nil, nil
		}
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-123",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--notify",
		"--submit",
		"--confirm",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "Submission rejected: review state is not ready") {
		t.Fatalf("expected beta review error, got %v", runErr)
	}
	if uploadCalls != 1 {
		t.Fatalf("upload calls = %d, want 1", uploadCalls)
	}

	var result asc.TestFlightPublishResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial publish result: %v\nstdout=%s", err, stdout)
	}
	if result.Status != publishPartialStatus || result.FailureStage != publishFailureStageBetaReview {
		t.Fatalf("unexpected partial status: status=%q stage=%q", result.Status, result.FailureStage)
	}
	if result.BuildID != "build-123" || !result.Uploaded {
		t.Fatalf("expected recoverable uploaded build, got buildId=%q uploaded=%t", result.BuildID, result.Uploaded)
	}
	wantCompleted := []string{publishCompletedStageUpload, publishCompletedStageBetaDistribution}
	if !slices.Equal(result.CompletedStages, wantCompleted) {
		t.Fatalf("completed stages = %v, want %v", result.CompletedStages, wantCompleted)
	}
	if result.Notified == nil || !*result.Notified || result.NotificationAction != asc.BuildBetaGroupsNotificationActionManual {
		t.Fatalf("notification result = notified:%v action:%q, want true/%q", result.Notified, result.NotificationAction, asc.BuildBetaGroupsNotificationActionManual)
	}
}

func TestPublishTestFlightUploadReportsTerminalProcessingStateAfterWaitFailure(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		return &publishUploadResult{
			Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
				ID: "build-123",
				Attributes: asc.BuildAttributes{
					Version:         buildNumber,
					ProcessingState: asc.BuildProcessingStateProcessing,
				},
			}},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}
	waitForPublishBuildProcessingFn = func(context.Context, *asc.Client, string, time.Duration) (*asc.BuildResponse, error) {
		return &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
			ID: "build-123",
			Attributes: asc.BuildAttributes{
				Version:         "42",
				ProcessingState: asc.BuildProcessingStateFailed,
			},
		}}, errors.New("build processing failed: FAILED")
	}
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/betaGroups" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
	})

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-123",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--wait",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "build processing failed: FAILED") {
		t.Fatalf("expected build processing failure, got %v", runErr)
	}

	var result asc.TestFlightPublishResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial publish result: %v\nstdout=%s", err, stdout)
	}
	if result.ProcessingState != asc.BuildProcessingStateFailed {
		t.Fatalf("processingState = %q, want %q", result.ProcessingState, asc.BuildProcessingStateFailed)
	}
	if result.FailureStage != publishFailureStageBuildProcessing {
		t.Fatalf("failureStage = %q, want %q", result.FailureStage, publishFailureStageBuildProcessing)
	}
	if !slices.Equal(result.CompletedStages, []string{publishCompletedStageUpload}) {
		t.Fatalf("completedStages = %v, want [%s]", result.CompletedStages, publishCompletedStageUpload)
	}
}

func TestPublishTestFlightLocalBuildStopsPropagationRetryForTerminalProcessingState(t *testing.T) {
	for _, state := range []string{asc.BuildProcessingStateFailed, asc.BuildProcessingStateInvalid} {
		t.Run(state, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()

			getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
			resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
				return "app-123", nil
			}
			validatePublishIPAPathFn = func(string) (os.FileInfo, error) { return newPublishTestFileInfo(t) }

			archiveCalls := 0
			runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
				archiveCalls++
				return &localxcode.ArchiveResult{
					ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
					Version:     "1.2.3",
					BuildNumber: "42",
				}, nil
			}
			exportCalls := 0
			runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				exportCalls++
				return &localxcode.ExportResult{
					ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
					IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
					Version:     "1.2.3",
					BuildNumber: "42",
				}, nil
			}
			uploadCalls := 0
			uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
				uploadCalls++
				return &publishUploadResult{
					Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
						ID: "build-123",
						Attributes: asc.BuildAttributes{
							Version:         buildNumber,
							ProcessingState: asc.BuildProcessingStateProcessing,
						},
					}},
					Version:     version,
					BuildNumber: buildNumber,
				}, nil
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
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
					return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'build-123'"}]}`)
				case 3:
					if req.Method != http.MethodGet || req.URL.Path != "/v1/builds/build-123" {
						t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
					}
					return publishCommandJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"builds","id":"build-123","attributes":{"version":"42","processingState":%q}}}`, state))
				default:
					t.Fatalf("unexpected retry request %d: %s %s", requestCount, req.Method, req.URL.String())
					return nil, nil
				}
			})

			cmd := PublishTestFlightCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--app", "app-123",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--build-number", "42",
				"--group", "External",
				"--export-options", "ExportOptions.plist",
				"--output", "json",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stdout, stderr := capturePublishCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if runErr == nil || !strings.Contains(runErr.Error(), "build processing failed: "+state) {
				t.Fatalf("expected terminal processing failure, got %v", runErr)
			}
			if archiveCalls != 1 || exportCalls != 1 || uploadCalls != 1 {
				t.Fatalf("build stage calls = archive:%d export:%d upload:%d, want each exactly once", archiveCalls, exportCalls, uploadCalls)
			}
			if requestCount != 3 {
				t.Fatalf("request count = %d, want 3", requestCount)
			}
			if strings.Contains(stderr, "still propagating") {
				t.Fatalf("terminal processing failure must not wait or retry, stderr=%q", stderr)
			}

			var result asc.TestFlightPublishResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("decode partial publish result: %v\nstdout=%s", err, stdout)
			}
			if result.ProcessingState != state {
				t.Fatalf("processingState = %q, want %q", result.ProcessingState, state)
			}
			if result.FailureStage != publishFailureStageBuildProcessing {
				t.Fatalf("failureStage = %q, want %q", result.FailureStage, publishFailureStageBuildProcessing)
			}
			wantCompleted := []string{publishCompletedStageArchive, publishCompletedStageExport, publishCompletedStageUpload}
			if !slices.Equal(result.CompletedStages, wantCompleted) {
				t.Fatalf("completedStages = %v, want %v", result.CompletedStages, wantCompleted)
			}
		})
	}
}

func TestPublishTestFlightLocalBuildStagesRunOnceWhenDistributionRetriesFail(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "retry exhaustion",
			err:  fmt.Errorf("beta-group relationship still reported uploaded build missing: %w", postUploadBuildMissingError("build-123")),
			want: asc.ErrNotFound,
		},
		{
			name: "retry cancellation",
			err:  context.Canceled,
			want: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()

			getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
			resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
				return "app-123", nil
			}
			validatePublishIPAPathFn = func(string) (os.FileInfo, error) { return newPublishTestFileInfo(t) }

			archiveCalls := 0
			runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
				archiveCalls++
				return &localxcode.ArchiveResult{
					ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
					Version:     "1.2.3",
					BuildNumber: "42",
				}, nil
			}
			exportCalls := 0
			runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				exportCalls++
				return &localxcode.ExportResult{
					ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
					IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
					Version:     "1.2.3",
					BuildNumber: "42",
				}, nil
			}
			uploadCalls := 0
			uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
				uploadCalls++
				return &publishUploadResult{
					Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
						ID: "build-123",
						Attributes: asc.BuildAttributes{
							Version:         buildNumber,
							ProcessingState: asc.BuildProcessingStateValid,
						},
					}},
					Version:     version,
					BuildNumber: buildNumber,
				}, nil
			}
			addCalls := 0
			addUploadedBuildBetaGroupsFn = func(context.Context, postUploadBuildDistributionClient, string, []shared.ResolvedBetaGroup, shared.AddBuildBetaGroupsOptions) (*shared.AddBuildBetaGroupsResult, error) {
				addCalls++
				return nil, tt.err
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-123/betaGroups" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`)
			})

			cmd := PublishTestFlightCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--app", "app-123",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--build-number", "42",
				"--group", "External",
				"--export-options", "ExportOptions.plist",
				"--output", "json",
			}); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stdout, _ := capturePublishCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if !errors.Is(runErr, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, runErr)
			}
			if archiveCalls != 1 || exportCalls != 1 || uploadCalls != 1 || addCalls != 1 {
				t.Fatalf("calls = archive:%d export:%d upload:%d add:%d, want each exactly once", archiveCalls, exportCalls, uploadCalls, addCalls)
			}
			var result asc.TestFlightPublishResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("decode partial result: %v\nstdout=%s", err, stdout)
			}
			wantCompleted := []string{publishCompletedStageArchive, publishCompletedStageExport, publishCompletedStageUpload}
			if !slices.Equal(result.CompletedStages, wantCompleted) {
				t.Fatalf("completed stages = %v, want %v", result.CompletedStages, wantCompleted)
			}
		})
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

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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
		if err := req.Context().Err(); err != nil {
			t.Fatalf("expected healthy post-upload request context, got %v", err)
		}
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

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["buildVersion"] != "1.2.3" {
		t.Fatalf("expected buildVersion 1.2.3, got %#v", payload["buildVersion"])
	}
	if payload["buildNumber"] != "42" {
		t.Fatalf("expected buildNumber 42, got %#v", payload["buildNumber"])
	}
	if payload["buildId"] != "build-123" {
		t.Fatalf("expected buildId build-123, got %#v", payload["buildId"])
	}
}

func TestRunPublishLocalBuildRejectsMissingExportedIPAWithoutUploading(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

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
			IPAPath:     "",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		t.Fatal("did not expect IPA validation when export produced no IPA")
		return nil, nil
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, _ string, _ string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		t.Fatal("did not expect upload when export produced no IPA")
		return nil, nil
	}

	result, err := runPublishLocalBuild(
		context.Background(),
		nil,
		"app-123",
		"IOS",
		"1.2.3",
		"42",
		5*time.Second,
		30*time.Second,
		false,
		publishLocalBuildConfig{
			WorkspacePath:     "Demo.xcworkspace",
			Scheme:            "Demo",
			Configuration:     "Release",
			ExportOptionsPath: "ExportOptions.plist",
			ArchivePath:       ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:           ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
		},
	)
	if err == nil {
		t.Fatal("expected missing IPA error, got nil")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	if !strings.Contains(err.Error(), "expected a local IPA artifact for publish upload") {
		t.Fatalf("expected missing IPA error, got %v", err)
	}
}

func TestPublishTestFlightIPAUploadResolvesAppIDBeforeGroupLookupAndUpload(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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

func TestPublishAppStoreLocalBuildDefersMissingExportOptionsUntilAfterArchive(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()
	t.Chdir(t.TempDir())

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	runPublishArchiveFn = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		return nil, errors.New("archive sentinel")
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
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected archive error after deferred generation, got %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "archive sentinel") {
		t.Fatalf("expected archive sentinel error, got %v", runErr)
	}
	if strings.Contains(stderr, "--export-options is required") {
		t.Fatalf("missing export options should be generated after archive, got %q", stderr)
	}
}

func TestPublishAppStoreMetadataDirAppliesAfterEnsureVersionBeforeAttach(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()
	t.Setenv("ASC_TIMEOUT", "500ms")

	metadataDir := t.TempDir()
	writePublishVersionMetadataFixture(t, metadataDir, "1.2.3")
	sequence := make([]string, 0, 4)

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		return appID, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(stageCtx context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		_, ok := stageCtx.Deadline()
		if !ok {
			t.Fatal("expected upload stage deadline")
		}
		time.Sleep(550 * time.Millisecond)
		return &publishUploadResult{
			Build: &asc.BuildResponse{
				Data: asc.Resource[asc.BuildAttributes]{
					ID:         "build-42",
					Attributes: asc.BuildAttributes{Version: "42"},
				},
			},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}
	applyPublishVersionMetadataFn = func(metadataCtx context.Context, _ *asc.Client, opts publishVersionMetadataOptions) ([]asc.LocalizationUploadLocaleResult, error) {
		sequence = append(sequence, "apply_metadata")
		if _, ok := metadataCtx.Deadline(); ok {
			t.Fatal("metadata batch should receive the healthy command parent without a batch-wide request deadline")
		}
		if err := metadataCtx.Err(); err != nil {
			t.Fatalf("expected healthy metadata parent context: %v", err)
		}
		if opts.VersionID != "version-1" {
			t.Fatalf("expected metadata version ID version-1, got %q", opts.VersionID)
		}
		if opts.Version != "1.2.3" {
			t.Fatalf("expected metadata version 1.2.3, got %q", opts.Version)
		}
		if opts.Dir != metadataDir {
			t.Fatalf("expected metadata dir %q, got %q", metadataDir, opts.Dir)
		}
		if got := opts.ValuesByLocale["en-US"]["description"]; got != "Updated description" {
			t.Fatalf("expected preflighted metadata values, got %+v", opts.ValuesByLocale)
		}
		time.Sleep(550 * time.Millisecond)
		return nil, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			sequence = append(sequence, "ensure_version")
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 350*time.Millisecond {
				t.Fatalf("expected fresh post-upload stage deadline, remaining=%s", time.Until(deadline))
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			sequence = append(sequence, "lookup_build")
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 350*time.Millisecond {
				t.Fatalf("expected fresh post-metadata stage deadline, remaining=%s", time.Until(deadline))
			}
			time.Sleep(450 * time.Millisecond)
			return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/version-1/relationships/build":
			sequence = append(sequence, "attach_build")
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 350*time.Millisecond {
				t.Fatalf("expected fresh attach deadline, remaining=%s", time.Until(deadline))
			}
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
			return nil, nil
		}
	})

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--ipa", "app.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--metadata-dir", metadataDir,
		"--timeout", "500ms",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("cmd.Exec() error: %v", runErr)
	}
	if !strings.Contains(stdout, `"attached":true`) {
		t.Fatalf("expected attached result, got %s", stdout)
	}

	wantSequence := strings.Join([]string{"ensure_version", "apply_metadata", "lookup_build", "attach_build"}, ",")
	if gotSequence := strings.Join(sequence, ","); gotSequence != wantSequence {
		t.Fatalf("expected sequence %s, got %s", wantSequence, gotSequence)
	}
}

func TestPublishAppStoreMetadataInputFailureUsesUsageExit(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	metadataDir := t.TempDir()
	writePublishVersionMetadataFixture(t, metadataDir, "1.2.3")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected input validation request: %s %s", req.Method, req.URL.Path)
		}
		return publishCommandJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`)
	})
	inputClient := newPublishCommandTestClient(t)
	_, _, inputErr := shared.UploadVersionLocalizationsWithWarnings(
		context.Background(),
		inputClient,
		"version-1",
		map[string]map[string]string{"en-US": {"promotionalText": ""}},
		false,
		shared.SubmitReadinessOptions{},
	)
	if inputErr == nil || !shared.IsLocalizationInputError(inputErr) {
		t.Fatalf("failed to construct localization input error: %v", inputErr)
	}

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		return appID, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		return &publishUploadResult{
			Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
				ID:         "build-42",
				Attributes: asc.BuildAttributes{Version: buildNumber},
			}},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}
	applyPublishVersionMetadataFn = func(context.Context, *asc.Client, publishVersionMetadataOptions) ([]asc.LocalizationUploadLocaleResult, error) {
		return nil, inputErr
	}

	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" {
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--ipa", "app.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--metadata-dir", metadataDir,
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %T: %v", runErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `Error: --metadata-dir "`+metadataDir+`"`) || strings.Contains(stderr, "0 locale(s) failed") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestPublishAppStoreRefreshesVersionContextAfterProcessingWait(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()
	t.Setenv("ASC_TIMEOUT", "500ms")

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		return appID, nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) {
		return newPublishTestFileInfo(t)
	}
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		return &publishUploadResult{
			Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
				ID:         "build-42",
				Attributes: asc.BuildAttributes{Version: buildNumber},
			}},
			Version:     version,
			BuildNumber: buildNumber,
		}, nil
	}
	waitForPublishBuildProcessingFn = func(waitCtx context.Context, _ *asc.Client, _ string, _ time.Duration) (*asc.BuildResponse, error) {
		if _, ok := waitCtx.Deadline(); !ok {
			t.Fatal("expected processing wait stage deadline")
		}
		time.Sleep(550 * time.Millisecond)
		return &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
			ID:         "build-42",
			Attributes: asc.BuildAttributes{Version: "42"},
		}}, nil
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions":
			deadline, ok := req.Context().Deadline()
			if !ok || time.Until(deadline) < 350*time.Millisecond {
				t.Fatalf("expected fresh post-wait version deadline, remaining=%s", time.Until(deadline))
			}
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/version-1/build":
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-42","attributes":{"version":"42"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
			return nil, nil
		}
	})

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "app-1",
		"--ipa", "app.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--wait",
		"--timeout", "500ms",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	stdout, stderr := capturePublishCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if stderr != "" || !strings.Contains(stdout, `"attached":true`) {
		t.Fatalf("unexpected publish result: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestPublishAppStoreDryRunPlanIncludesMetadataStepWhenMetadataDirProvided(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	ipaPath := filepath.Join(t.TempDir(), "app.ipa")
	if err := os.WriteFile(ipaPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write IPA fixture: %v", err)
	}
	metadataDir := t.TempDir()
	writePublishVersionMetadataFixture(t, metadataDir, "1.2.3")

	cmd := PublishAppStoreCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--ipa", ipaPath,
		"--version", "1.2.3",
		"--build-number", "42",
		"--metadata-dir", metadataDir,
		"--submit",
		"--dry-run",
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, _ := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("cmd.Exec() error: %v", runErr)
	}

	var payload struct {
		Plan []struct {
			Name string `json:"name"`
		} `json:"plan"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	planNames := make([]string, 0, len(payload.Plan))
	for _, step := range payload.Plan {
		planNames = append(planNames, step.Name)
	}
	expectedPlanNames := []string{
		"upload_build",
		"ensure_version",
		"apply_metadata",
		"attach_build",
		"submit_review",
	}
	if strings.Join(planNames, ",") != strings.Join(expectedPlanNames, ",") {
		t.Fatalf("expected plan %v, got %v", expectedPlanNames, planNames)
	}
}

func writePublishVersionMetadataFixture(t *testing.T, dir, version string) {
	t.Helper()

	versionDir := filepath.Join(dir, "version", version)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatalf("create version metadata dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "en-US.json"), []byte(`{"description":"Updated description"}`), 0o600); err != nil {
		t.Fatalf("write version metadata fixture: %v", err)
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

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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
		case "/v1/appStoreVersions/version-1/build":
			return publishCommandJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
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

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["buildVersion"] != "1.2.3" {
		t.Fatalf("expected buildVersion 1.2.3, got %#v", payload["buildVersion"])
	}
	if payload["buildNumber"] != "42" {
		t.Fatalf("expected buildNumber 42, got %#v", payload["buildNumber"])
	}
	if payload["buildId"] != "build-123" {
		t.Fatalf("expected buildId build-123, got %#v", payload["buildId"])
	}
	if payload["versionId"] != "version-1" {
		t.Fatalf("expected versionId version-1, got %#v", payload["versionId"])
	}
}

func TestPublishAppStoreIPAUploadResolvesAppIDBeforeUploadAndAttach(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
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
	if payload["buildVersion"] != "1.2.3" {
		t.Fatalf("expected buildVersion 1.2.3, got %#v", payload["buildVersion"])
	}
	if payload["buildNumber"] != "42" {
		t.Fatalf("expected buildNumber 42, got %#v", payload["buildNumber"])
	}
	if payload["versionId"] != "version-1" {
		t.Fatalf("expected versionId version-1, got %#v", payload["versionId"])
	}
}

func overridePublishCommandTestHooks(t *testing.T) func() {
	t.Helper()

	originalArchive := runPublishArchiveFn
	originalExport := runPublishExportFn
	originalPreflightXcode := preflightPublishXcodeFn
	originalGenerateExportOptions := generatePublishExportOptionsFn
	originalGetClient := getPublishASCClientFn
	originalResolveNextBuildNumber := resolvePublishNextBuildNumberFn
	originalValidateIPAPath := validatePublishIPAPathFn
	originalUploadBuildAndWait := uploadBuildAndWaitForIDFn
	originalResolveAppID := resolvePublishAppIDWithLookupFn
	originalWaitForProcessing := waitForPublishBuildProcessingFn
	originalMetadataApply := applyPublishVersionMetadataFn
	originalAddUploadedBuildBetaGroups := addUploadedBuildBetaGroupsFn
	preflightPublishXcodeFn = func(context.Context) error { return nil }

	return func() {
		runPublishArchiveFn = originalArchive
		runPublishExportFn = originalExport
		preflightPublishXcodeFn = originalPreflightXcode
		generatePublishExportOptionsFn = originalGenerateExportOptions
		getPublishASCClientFn = originalGetClient
		resolvePublishNextBuildNumberFn = originalResolveNextBuildNumber
		validatePublishIPAPathFn = originalValidateIPAPath
		uploadBuildAndWaitForIDFn = originalUploadBuildAndWait
		resolvePublishAppIDWithLookupFn = originalResolveAppID
		waitForPublishBuildProcessingFn = originalWaitForProcessing
		applyPublishVersionMetadataFn = originalMetadataApply
		addUploadedBuildBetaGroupsFn = originalAddUploadedBuildBetaGroups
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
