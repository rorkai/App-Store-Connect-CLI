package publish

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestPublishTestFlightUploadOnlyIPAWaitsThenStopsBeforeDistribution(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	uploadCalls := configurePublishUploadOnlyIPA(t, asc.BuildProcessingStateProcessing)
	requestCalls := rejectPublishUploadOnlyHTTP(t)
	waitCalls := 0
	waitForPublishBuildProcessingFn = func(_ context.Context, _ *asc.Client, buildID string, _ time.Duration) (*asc.BuildResponse, error) {
		waitCalls++
		if buildID != "build-123" {
			t.Fatalf("expected build-123, got %q", buildID)
		}
		return publishUploadOnlyResult("1.2.3", "42", asc.BuildProcessingStateValid).Build, nil
	}

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--upload-only",
		"--wait",
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
	if *uploadCalls != 1 {
		t.Fatalf("expected exactly one upload, got %d", *uploadCalls)
	}
	if waitCalls != 1 {
		t.Fatalf("expected exactly one processing wait, got %d", waitCalls)
	}
	if *requestCalls != 0 {
		t.Fatalf("expected no group or review client calls, got %d", *requestCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModeIPAUpload) {
		t.Fatalf("expected ipa_upload mode, got %#v", payload["mode"])
	}
	if payload["buildId"] != "build-123" || payload["buildVersion"] != "1.2.3" || payload["buildNumber"] != "42" {
		t.Fatalf("expected uploaded build metadata, got %#v", payload)
	}
	if payload["uploaded"] != true || payload["processingState"] != asc.BuildProcessingStateValid {
		t.Fatalf("expected uploaded valid build, got %#v", payload)
	}
	if payload["uploadOnly"] != true {
		t.Fatalf("expected upload-only marker, got %#v", payload)
	}
	if _, ok := payload["groupIds"]; ok {
		t.Fatalf("did not expect groupIds in upload-only output: %#v", payload)
	}
	if _, ok := payload["betaReviewSubmitted"]; ok {
		t.Fatalf("did not expect betaReviewSubmitted in upload-only output: %#v", payload)
	}
}

func TestPublishTestFlightUploadOnlyLocalBuildStopsBeforeDistribution(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) { return newPublishTestFileInfo(t) }
	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	ipaPath := filepath.Join(t.TempDir(), "Demo.ipa")
	archiveCalls := 0
	runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		archiveCalls++
		return &localxcode.ArchiveResult{
			ArchivePath: archivePath,
			BundleID:    "com.example.demo", Version: "1.2.3", BuildNumber: "42", Scheme: "Demo", Configuration: "Release",
		}, nil
	}
	generateCalls := 0
	generatePublishExportOptionsFn = func(_ context.Context, opts localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		generateCalls++
		if opts.SigningStyle != "manual" || opts.TeamID != "TEAM123456" {
			t.Fatalf("expected manual signing passthrough, got style=%q team=%q", opts.SigningStyle, opts.TeamID)
		}
		return &localxcode.ExportOptionsGenerateResult{Path: opts.OutputPath}, nil
	}
	exportCalls := 0
	runPublishExportFn = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		exportCalls++
		if strings.TrimSpace(opts.ExportOptions) == "" {
			t.Fatal("expected generated export options path")
		}
		return &localxcode.ExportResult{
			ArchivePath: archivePath, IPAPath: ipaPath,
			BundleID: "com.example.demo", Version: "1.2.3", BuildNumber: "42",
		}, nil
	}
	uploadCalls := 0
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
		return publishUploadOnlyResult(version, buildNumber, asc.BuildProcessingStateProcessing), nil
	}
	requestCalls := rejectPublishUploadOnlyHTTP(t)

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--signing-style", "manual",
		"--team-id", "TEAM123456",
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
	if archiveCalls != 1 || generateCalls != 1 || exportCalls != 1 || uploadCalls != 1 || *requestCalls != 0 {
		t.Fatalf("expected one archive, export-options generation, export, and upload with no distribution calls; archives=%d generations=%d exports=%d uploads=%d requests=%d", archiveCalls, generateCalls, exportCalls, uploadCalls, *requestCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload["mode"] != string(asc.PublishModeLocalBuild) || payload["buildId"] != "build-123" {
		t.Fatalf("unexpected local-build upload-only output: %#v", payload)
	}
	if payload["uploadOnly"] != true {
		t.Fatalf("expected upload-only marker, got %#v", payload)
	}
	if _, ok := payload["archive"].(map[string]any); !ok {
		t.Fatalf("expected archive stage output, got %#v", payload["archive"])
	}
	if _, ok := payload["export"].(map[string]any); !ok {
		t.Fatalf("expected export stage output, got %#v", payload["export"])
	}
	publishStage, ok := payload["publish"].(map[string]any)
	if !ok || publishStage["uploadOnly"] != true {
		t.Fatalf("expected nested upload-only publish stage, got %#v", payload["publish"])
	}
}

func TestPublishTestFlightUploadOnlyRejectsContradictoryFlagsDeterministically(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "group before notify", args: []string{"--group", "group-1", "--notify"}, wantErr: "--group cannot be used with --upload-only"},
		{name: "notify", args: []string{"--notify"}, wantErr: "--notify cannot be used with --upload-only"},
		{name: "submit", args: []string{"--submit", "--confirm"}, wantErr: "--submit cannot be used with --upload-only"},
		{name: "confirm", args: []string{"--confirm"}, wantErr: "--confirm cannot be used with --upload-only"},
		{name: "test notes", args: []string{"--test-notes", "Try it", "--locale", "en-US"}, wantErr: "--test-notes cannot be used with --upload-only"},
		{name: "locale", args: []string{"--locale", "en-US"}, wantErr: "--locale cannot be used with --upload-only"},
		{name: "existing build", args: []string{"--build-id", "build-1"}, wantErr: "--build-id cannot be used with --upload-only"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restore := overridePublishCommandTestHooks(t)
			defer restore()
			getPublishASCClientFn = func(time.Duration) (*asc.Client, error) {
				t.Fatal("validation must complete before client construction")
				return nil, nil
			}

			args := []string{
				"--app", "123456789",
				"--ipa", "Demo.ipa",
				"--version", "1.2.3",
				"--build-number", "42",
				"--upload-only",
			}
			args = append(args, test.args...)

			cmd := PublishTestFlightCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			var runErr error
			stdout, stderr := capturePublishCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if got := runErr.Error(); got != test.wantErr {
				t.Fatalf("expected returned error %q, got %q", test.wantErr, got)
			}
			if wantStderr := "Error: " + test.wantErr + "\n"; stderr != wantStderr {
				t.Fatalf("expected stderr %q, got %q", wantStderr, stderr)
			}
		})
	}
}

func TestPublishTestFlightUploadOnlyRequiresUploadSource(t *testing.T) {
	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "123456789",
		"--build-number", "42",
		"--upload-only",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	const wantErr = "--upload-only requires --ipa, --pkg, --workspace, or --project"
	if got := runErr.Error(); got != wantErr {
		t.Fatalf("expected returned error %q, got %q", wantErr, got)
	}
	if stdout != "" || stderr != "Error: "+wantErr+"\n" {
		t.Fatalf("unexpected validation output: stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestPublishTestFlightUploadOnlyProcessingFailurePrintsPartialBuildAndDoesNotReupload(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	uploadCalls := configurePublishUploadOnlyIPA(t, asc.BuildProcessingStateProcessing)
	requestCalls := rejectPublishUploadOnlyHTTP(t)
	waitCalls := 0
	waitForPublishBuildProcessingFn = func(_ context.Context, _ *asc.Client, buildID string, _ time.Duration) (*asc.BuildResponse, error) {
		waitCalls++
		if buildID != "build-123" {
			t.Fatalf("expected build-123, got %q", buildID)
		}
		return nil, errors.New("processing sentinel")
	}

	cmd := PublishTestFlightCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--app", "friendly-app",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--upload-only",
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
	if runErr == nil || !strings.Contains(runErr.Error(), "processing sentinel") {
		t.Fatalf("expected processing failure, got %v", runErr)
	}
	if *uploadCalls != 1 || waitCalls != 1 || *requestCalls != 0 {
		t.Fatalf("unexpected calls: uploads=%d waits=%d requests=%d", *uploadCalls, waitCalls, *requestCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("expected partial JSON output, got %q: %v", stdout, err)
	}
	if payload["buildId"] != "build-123" || payload["buildVersion"] != "1.2.3" || payload["buildNumber"] != "42" {
		t.Fatalf("expected partial uploaded build metadata, got %#v", payload)
	}
	if payload["uploadOnly"] != true || payload["status"] != publishPartialStatus || payload["failureStage"] != publishFailureStageBuildProcessing {
		t.Fatalf("expected canonical upload-only partial status, got %#v", payload)
	}
	if failure, ok := payload["failure"].(string); !ok || !strings.Contains(failure, "processing sentinel") {
		t.Fatalf("expected canonical structured failure, got %#v", payload["failure"])
	}
	completedStages, ok := payload["completedStages"].([]any)
	if !ok || len(completedStages) != 1 || completedStages[0] != publishCompletedStageUpload {
		t.Fatalf("expected exactly the completed upload stage, got %#v", payload["completedStages"])
	}
}

func configurePublishUploadOnlyIPA(t *testing.T, processingState string) *int {
	t.Helper()

	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		if appID != "friendly-app" {
			t.Fatalf("expected friendly-app lookup, got %q", appID)
		}
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) { return newPublishTestFileInfo(t) }
	uploadCalls := 0
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		uploadCalls++
		if appID != "app-123" {
			t.Fatalf("expected resolved app ID, got %q", appID)
		}
		return publishUploadOnlyResult(version, buildNumber, processingState), nil
	}
	return &uploadCalls
}

func publishUploadOnlyResult(version, buildNumber, processingState string) *publishUploadResult {
	return &publishUploadResult{
		Build: &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{
			ID:         "build-123",
			Attributes: asc.BuildAttributes{Version: buildNumber, ProcessingState: processingState},
		}},
		Version: version, BuildNumber: buildNumber,
	}
}

func rejectPublishUploadOnlyHTTP(t *testing.T) *int {
	t.Helper()

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requestCalls := 0
	http.DefaultTransport = publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCalls++
		t.Fatalf("upload-only must not make post-upload client call: %s %s", req.Method, req.URL.String())
		return nil, nil
	})
	return &requestCalls
}
