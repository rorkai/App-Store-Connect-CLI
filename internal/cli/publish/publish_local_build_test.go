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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
	"howett.net/plist"
)

func TestPublishTestFlightLocalBuildJSONIncludesNestedStages(t *testing.T) {
	stubPublishCommandSeams(t)
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		if appID != "friendly-app" {
			t.Fatalf("expected unresolved app input to be passed through lookup, got %q", appID)
		}
		return "app-123", nil
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
	stages := stubPublishArchiveAndExport()
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
		return publishUploadOnlyResult(version, buildNumber, asc.BuildProcessingStateValid), nil
	}
	scriptPublishRequests(t,
		publishListExternalBetaGroups,
		publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
	)

	stdout, stderr, runErr := runPublishCommand(t, PublishTestFlightCommand, []string{
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
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}

	gotArchiveOpts := stages.archiveOpts
	gotExportOpts := stages.exportOpts
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

			args := append(append([]string(nil), test.args...), "--export-xcodebuild-flag=-exportPath=/tmp/elsewhere")
			stdout, stderr, runErr := runPublishCommand(t, test.command, args)
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

func TestPublishLocalBuildAcceptsActionNamedAuthenticationValues(t *testing.T) {
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

			wantErr := errors.New("reached client creation")
			getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return nil, wantErr }
			preflightPublishXcodeFn = func(context.Context) error {
				t.Fatal("Xcode preflight ran after failed client creation")
				return nil
			}

			cmd := test.command()
			cmd.FlagSet.SetOutput(io.Discard)
			args := append(
				append([]string(nil), test.args...),
				"--export-xcodebuild-flag=-authenticationKeyPath",
				"--export-xcodebuild-flag=archive",
				"--export-xcodebuild-flag=-authenticationKeyID",
				"--export-xcodebuild-flag=build",
				"--export-xcodebuild-flag=-authenticationKeyIssuerID",
				"--export-xcodebuild-flag=clean",
				"--export-xcodebuild-flag=-authenticationKeyPath",
				"--export-xcodebuild-flag=-exportPath=AuthKey.p8",
			)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}

			runErr := cmd.Exec(context.Background(), nil)
			if !errors.Is(runErr, wantErr) {
				t.Fatalf("Exec() error = %v, want client-creation sentinel", runErr)
			}
		})
	}
}

func TestPublishTestFlightLocalBuildRetriesPostUploadBuildPropagationWithoutRepeatingBuildStages(t *testing.T) {
	stubPublishCommandSeams(t)
	stages := stubPublishArchiveAndExport()
	uploadCalls := stubPublishIPAUpload(asc.BuildProcessingStateValid)
	requestCount := scriptPublishRequests(t,
		publishListExternalBetaGroups,
		publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNotFound, publishBuildNotFoundBody},
		publishRequest{http.MethodGet, "/v1/builds/build-123", http.StatusOK, `{"data":{"type":"builds","id":"build-123","attributes":{"version":"42","processingState":"VALID"}}}`},
		publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
	)

	stdout, stderr, runErr := runPublishCommand(t, PublishTestFlightCommand, publishTestFlightLocalBuildArgs)
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if stages.archiveCalls != 1 || stages.exportCalls != 1 || *uploadCalls != 1 {
		t.Fatalf("build stage calls = archive:%d export:%d upload:%d, want each exactly once", stages.archiveCalls, stages.exportCalls, *uploadCalls)
	}
	if *requestCount != 4 {
		t.Fatalf("request count = %d, want 4", *requestCount)
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
	stubPublishCommandSeams(t)
	stages := stubPublishArchiveAndExport()
	uploadCalls := stubPublishIPAUpload(asc.BuildProcessingStateValid)
	scriptPublishRequests(t,
		publishListExternalBetaGroups,
		publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
		publishRequest{http.MethodGet, "/v1/builds/build-123/buildBetaDetail", http.StatusNotFound, publishBuildNotFoundBody},
	)

	stdout, stderr, runErr := runPublishCommand(t, PublishTestFlightCommand, append(slices.Clone(publishTestFlightLocalBuildArgs), "--notify"))
	if runErr == nil || !strings.Contains(runErr.Error(), `beta groups were added to build "build-123", but checking notification state failed`) {
		t.Fatalf("expected notification partial error, got %v", runErr)
	}
	if stages.archiveCalls != 1 || stages.exportCalls != 1 || *uploadCalls != 1 {
		t.Fatalf("build stage calls = archive:%d export:%d upload:%d, want each exactly once", stages.archiveCalls, stages.exportCalls, *uploadCalls)
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
	stubPublishCommandSeams(t)
	uploadCalls := stubPublishIPAUpload(asc.BuildProcessingStateValid)
	scriptPublishRequests(t,
		publishListExternalBetaGroups,
		publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
		publishRequest{http.MethodGet, "/v1/builds/build-123/buildBetaDetail", http.StatusOK, `{"data":{"type":"buildBetaDetails","id":"detail-1","attributes":{"autoNotifyEnabled":false}}}`},
		publishRequest{http.MethodPost, "/v1/buildBetaNotifications", http.StatusCreated, `{"data":{"type":"buildBetaNotifications","id":"notification-1"}}`},
		publishRequest{http.MethodGet, "/v1/builds/build-123/betaAppReviewSubmission", http.StatusNotFound, publishNotFoundBody},
		publishRequest{http.MethodPost, "/v1/betaAppReviewSubmissions", http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"STATE_ERROR","title":"Submission rejected","detail":"review state is not ready"}]}`},
	)

	stdout, _, runErr := runPublishCommand(t, PublishTestFlightCommand, []string{
		"--app", "app-123",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--notify",
		"--submit",
		"--confirm",
		"--output", "json",
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "Submission rejected: review state is not ready") {
		t.Fatalf("expected beta review error, got %v", runErr)
	}
	if *uploadCalls != 1 {
		t.Fatalf("upload calls = %d, want 1", *uploadCalls)
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

func TestPublishTestFlightUploadTestNotesFailurePreservesStructuredRecoveryContext(t *testing.T) {
	stubPublishCommandSeams(t)
	testNotes := "Line one\nLine two\x1b[31m"
	uploadCalls := stubPublishIPAUpload(asc.BuildProcessingStateProcessing)
	waitForPublishBuildProcessingFn = func(context.Context, *asc.Client, string, time.Duration) (*asc.BuildResponse, error) {
		return publishUploadOnlyResult("1.2.3", "42", asc.BuildProcessingStateValid).Build, nil
	}
	scriptPublishRequests(t,
		publishListExternalBetaGroups,
		publishRequest{http.MethodGet, "/v1/betaAppLocalizations", http.StatusOK, `{"data":[{"type":"betaAppLocalizations","id":"bal-1","attributes":{"locale":"en-US"}}]}`},
		publishRequest{http.MethodGet, "/v1/builds/build-123/betaBuildLocalizations", http.StatusOK, `{"data":[]}`},
		publishRequest{http.MethodPost, "/v1/betaBuildLocalizations", http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"The provided entity has an invalid attribute","detail":"What to Test was rejected by the server"}]}`},
	)

	stdout, _, runErr := runPublishCommand(t, PublishTestFlightCommand, []string{
		"--app", "app-123",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--test-notes", testNotes,
		"--locale", "en-US",
		"--output", "json",
	})
	if runErr == nil {
		t.Fatal("expected post-upload test notes rejection")
	}
	if *uploadCalls != 1 {
		t.Fatalf("upload calls = %d, want 1", *uploadCalls)
	}
	wantParts := []string{
		`build "build-123" is available`,
		`locale "en-US"`,
		"The provided entity has an invalid attribute: What to Test was rejected by the server",
		"retry without uploading the build again",
		"reuse the original notes",
		"asc builds test-notes create --build-id BUILD_ID --locale LOCALE --whats-new NOTES",
	}
	for _, want := range wantParts {
		if !strings.Contains(runErr.Error(), want) {
			t.Fatalf("expected recovery error to contain %q, got %v", want, runErr)
		}
	}
	if asc.HasInterpretedTerminalSequence(runErr.Error()) || strings.Contains(runErr.Error(), "Line one") {
		t.Fatalf("expected sanitized human recovery without embedded notes, got %q", runErr)
	}

	var result asc.TestFlightPublishResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode partial publish result: %v\nstdout=%s", err, stdout)
	}
	if result.Status != publishPartialStatus || result.FailureStage != publishFailureStageTestNotes {
		t.Fatalf("unexpected partial status: status=%q stage=%q", result.Status, result.FailureStage)
	}
	if result.BuildID != "build-123" || !result.Uploaded {
		t.Fatalf("expected recoverable uploaded build, got buildId=%q uploaded=%t", result.BuildID, result.Uploaded)
	}
	wantCompleted := []string{publishCompletedStageUpload, publishCompletedStageBuildProcessing}
	if !slices.Equal(result.CompletedStages, wantCompleted) {
		t.Fatalf("completed stages = %v, want %v", result.CompletedStages, wantCompleted)
	}
	for _, want := range wantParts {
		if !strings.Contains(result.Failure, want) {
			t.Fatalf("expected structured failure to contain %q, got %q", want, result.Failure)
		}
	}
	if result.Recovery == nil {
		t.Fatalf("expected typed test-notes recovery, got %#v", result)
	}
	if result.Recovery.BuildID != "build-123" || result.Recovery.Locale != "en-US" || result.Recovery.SubmittedNotes != testNotes {
		t.Fatalf("structured recovery lost exact values: %#v", result.Recovery)
	}
	wantArguments := []string{
		"builds", "test-notes", "create",
		"--build-id", "build-123",
		"--locale", "en-US",
		"--whats-new", testNotes,
	}
	if result.Recovery.Command != "asc" || !slices.Equal(result.Recovery.Arguments, wantArguments) {
		t.Fatalf("structured recovery command = %q %#v, want asc %#v", result.Recovery.Command, result.Recovery.Arguments, wantArguments)
	}
}

func TestPublishTestFlightUploadReportsTerminalProcessingStateAfterWaitFailure(t *testing.T) {
	stubPublishCommandSeams(t)
	stubPublishIPAUpload(asc.BuildProcessingStateProcessing)
	waitForPublishBuildProcessingFn = func(context.Context, *asc.Client, string, time.Duration) (*asc.BuildResponse, error) {
		return publishUploadOnlyResult("1.2.3", "42", asc.BuildProcessingStateFailed).Build, errors.New("build processing failed: FAILED")
	}
	scriptPublishRequests(t, publishListExternalBetaGroups)

	stdout, _, runErr := runPublishCommand(t, PublishTestFlightCommand, []string{
		"--app", "app-123",
		"--ipa", "Demo.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "External",
		"--wait",
		"--output", "json",
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
			stubPublishCommandSeams(t)
			stages := stubPublishArchiveAndExportVersionOnly()
			uploadCalls := stubPublishIPAUpload(asc.BuildProcessingStateProcessing)
			requestCount := scriptPublishRequests(t,
				publishListExternalBetaGroups,
				publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNotFound, publishBuildNotFoundBody},
				publishRequest{http.MethodGet, "/v1/builds/build-123", http.StatusOK, fmt.Sprintf(`{"data":{"type":"builds","id":"build-123","attributes":{"version":"42","processingState":%q}}}`, state)},
			)

			stdout, stderr, runErr := runPublishCommand(t, PublishTestFlightCommand, publishTestFlightLocalBuildArgs)
			if runErr == nil || !strings.Contains(runErr.Error(), "build processing failed: "+state) {
				t.Fatalf("expected terminal processing failure, got %v", runErr)
			}
			if stages.archiveCalls != 1 || stages.exportCalls != 1 || *uploadCalls != 1 {
				t.Fatalf("build stage calls = archive:%d export:%d upload:%d, want each exactly once", stages.archiveCalls, stages.exportCalls, *uploadCalls)
			}
			if *requestCount != 3 {
				t.Fatalf("request count = %d, want 3", *requestCount)
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
			stubPublishCommandSeams(t)
			stages := stubPublishArchiveAndExportVersionOnly()
			uploadCalls := stubPublishIPAUpload(asc.BuildProcessingStateValid)
			addCalls := 0
			addUploadedBuildBetaGroupsFn = func(context.Context, postUploadBuildDistributionClient, string, []shared.ResolvedBetaGroup, shared.AddBuildBetaGroupsOptions) (*shared.AddBuildBetaGroupsResult, error) {
				addCalls++
				return nil, tt.err
			}
			scriptPublishRequests(t, publishListExternalBetaGroups)

			stdout, _, runErr := runPublishCommand(t, PublishTestFlightCommand, publishTestFlightLocalBuildArgs)
			if !errors.Is(runErr, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, runErr)
			}
			if stages.archiveCalls != 1 || stages.exportCalls != 1 || *uploadCalls != 1 || addCalls != 1 {
				t.Fatalf("calls = archive:%d export:%d upload:%d add:%d, want each exactly once", stages.archiveCalls, stages.exportCalls, *uploadCalls, addCalls)
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

func TestPublishLocalBuildRejectsDirectUploadExportOptions(t *testing.T) {
	for _, test := range []struct {
		name     string
		command  func() *ffcli.Command
		args     []string
		requests []publishRequest
	}{
		{
			name:    "testflight",
			command: PublishTestFlightCommand,
			args:    []string{"--build-number", "44", "--group", "group-1"},
			requests: []publishRequest{
				publishListGroup1BetaGroups,
				{http.MethodPost, "/v1/builds/build-789/relationships/betaGroups", http.StatusNoContent, ""},
			},
		},
		{
			name:    "appstore",
			command: PublishAppStoreCommand,
			args:    []string{"--build-number", "42"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stubPublishCommandSeams(t)
			exportOptionsPath := filepath.Join(t.TempDir(), "UploadExportOptions.plist")
			payload, err := plist.Marshal(map[string]any{"destination": "upload"}, plist.XMLFormat)
			if err != nil {
				t.Fatalf("plist.Marshal() error: %v", err)
			}
			if err := os.WriteFile(exportOptionsPath, payload, 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}
			runPublishArchiveFn = func(_ context.Context, _ localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
				t.Fatal("did not expect archive to run for unsupported direct-upload export options")
				return nil, nil
			}
			runPublishExportFn = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				t.Fatal("did not expect export to run for unsupported direct-upload export options")
				return nil, nil
			}
			scriptPublishRequests(t, test.requests...)

			args := append([]string{
				"--app", "friendly-app",
				"--project", "Demo.xcodeproj",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--export-options", exportOptionsPath,
				"--poll-interval", "5s",
				"--output", "json",
			}, test.args...)
			stdout, stderr, runErr := runPublishCommand(t, test.command, args)
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected flag.ErrHelp, got %v", runErr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--export-options with destination=upload is not supported by publish") {
				t.Fatalf("expected direct-upload rejection, got %q", stderr)
			}
		})
	}
}

func TestPublishTestFlightLocalBuildTableIncludesArchiveAndExportSections(t *testing.T) {
	stubPublishCommandSeams(t)
	stubPublishArchiveAndExport()
	stubPublishIPAUpload(asc.BuildProcessingStateValid)
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if err := req.Context().Err(); err != nil {
			t.Fatalf("expected healthy post-upload request context, got %v", err)
		}
		switch req.URL.Path {
		case "/v1/apps/app-123/betaGroups":
			return publishCommandJSONResponse(http.StatusOK, publishGroup1BetaGroupsBody)
		case "/v1/builds/build-123/relationships/betaGroups":
			return publishCommandJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	stdout, _, runErr := runPublishCommand(t, PublishTestFlightCommand, []string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "group-1",
		"--export-options", "ExportOptions.plist",
		"--output", "table",
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
	stubPublishCommandSeams(t)

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

	stages := stubPublishArchiveAndExport()
	stubPublishIPAUpload(asc.BuildProcessingStateValid)
	scriptPublishRequests(t,
		publishListGroup1BetaGroups,
		publishRequest{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
	)

	_, _, runErr := runPublishCommand(t, PublishTestFlightCommand, []string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
		"--group", "group-1",
		"--output", "json",
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if stages.exportOpts.ExportOptions != defaultPublishExportOptionsPath {
		t.Fatalf("expected default export options path %q, got %q", defaultPublishExportOptionsPath, stages.exportOpts.ExportOptions)
	}
}

func TestPublishLocalBuildUsesFreshUploadTimeoutAfterArchive(t *testing.T) {
	for _, test := range []struct {
		name        string
		command     func() *ffcli.Command
		args        []string
		requests    []publishRequest
		wantPayload map[string]any
	}{
		{
			name:    "testflight",
			command: PublishTestFlightCommand,
			args:    []string{"--group", "group-1"},
			requests: []publishRequest{
				publishListGroup1BetaGroups,
				{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
			},
			wantPayload: map[string]any{"buildVersion": "1.2.3", "buildNumber": "42", "buildId": "build-123"},
		},
		{
			name:     "appstore",
			command:  PublishAppStoreCommand,
			requests: publishAttachToPreparedVersion,
			wantPayload: map[string]any{
				"buildVersion": "1.2.3", "buildNumber": "42", "buildId": "build-123", "versionId": "version-1",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stubPublishCommandSeams(t)
			resolvePublishNextBuildNumberFn = func(_ context.Context, _ *asc.Client, _ shared.NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
				return &asc.BuildsNextBuildNumberResult{NextBuildNumber: "42"}, nil
			}
			stubPublishArchiveAndExport()
			archive := runPublishArchiveFn
			runPublishArchiveFn = func(ctx context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
				time.Sleep(150 * time.Millisecond)
				return archive(ctx, opts)
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
				return publishUploadOnlyResult(version, buildNumber, asc.BuildProcessingStateValid), nil
			}
			scriptPublishRequests(t, test.requests...)

			stdout, stderr, runErr := runPublishCommand(t, test.command, append([]string{
				"--app", "friendly-app",
				"--workspace", "Demo.xcworkspace",
				"--scheme", "Demo",
				"--version", "1.2.3",
				"--export-options", "ExportOptions.plist",
				"--timeout", "100ms",
				"--output", "json",
			}, test.args...))
			if runErr != nil {
				t.Fatalf("Exec() error: %v", runErr)
			}
			if strings.TrimSpace(stderr) != "" {
				t.Fatalf("expected no stderr output, got %q", stderr)
			}
			assertPublishPayload(t, stdout, test.wantPayload)
		})
	}
}

func TestRunPublishLocalBuildRejectsMissingExportedIPAWithoutUploading(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	stubPublishArchiveAndExport()
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

func TestPublishIPAUploadResolvesAppIDBeforeUpload(t *testing.T) {
	for _, test := range []struct {
		name        string
		command     func() *ffcli.Command
		args        []string
		requests    []publishRequest
		wantPayload map[string]any
	}{
		{
			name:    "testflight",
			command: PublishTestFlightCommand,
			args:    []string{"--group", "group-1"},
			requests: []publishRequest{
				publishListGroup1BetaGroups,
				{http.MethodPost, "/v1/builds/build-123/relationships/betaGroups", http.StatusNoContent, ""},
			},
			wantPayload: map[string]any{"mode": string(asc.PublishModeIPAUpload), "uploaded": true, "buildId": "build-123"},
		},
		{
			name:     "appstore",
			command:  PublishAppStoreCommand,
			requests: publishAttachToPreparedVersion,
			wantPayload: map[string]any{
				"mode": string(asc.PublishModeIPAUpload), "uploaded": true, "buildId": "build-123",
				"buildVersion": "1.2.3", "buildNumber": "42", "versionId": "version-1",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			stubPublishCommandSeams(t)
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
				return publishUploadOnlyResult(version, buildNumber, asc.BuildProcessingStateValid), nil
			}
			scriptPublishRequests(t, test.requests...)

			stdout, stderr, runErr := runPublishCommand(t, test.command, append([]string{
				"--app", "friendly-app",
				"--ipa", "Demo.ipa",
				"--version", "1.2.3",
				"--build-number", "42",
				"--poll-interval", "5s",
				"--output", "json",
			}, test.args...))
			if runErr != nil {
				t.Fatalf("Exec() error: %v", runErr)
			}
			if strings.TrimSpace(stderr) != "" {
				t.Fatalf("expected no stderr output, got %q", stderr)
			}
			if lookupCalls != 1 {
				t.Fatalf("expected exactly one app lookup, got %d", lookupCalls)
			}
			assertPublishPayload(t, stdout, test.wantPayload)
		})
	}
}

func TestPublishAppStoreLocalBuildDefersMissingExportOptionsUntilAfterArchive(t *testing.T) {
	stubPublishCommandSeams(t)
	t.Chdir(t.TempDir())
	runPublishArchiveFn = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		return nil, errors.New("archive sentinel")
	}

	_, stderr, runErr := runPublishCommand(t, PublishAppStoreCommand, []string{
		"--app", "friendly-app",
		"--workspace", "Demo.xcworkspace",
		"--scheme", "Demo",
		"--version", "1.2.3",
		"--build-number", "42",
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
	stubPublishCommandSeams(t)
	t.Setenv("ASC_TIMEOUT", "500ms")

	metadataDir := t.TempDir()
	writePublishVersionMetadataFixture(t, metadataDir, "1.2.3")
	sequence := make([]string, 0, 4)

	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		return appID, nil
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

	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
			return publishCommandJSONResponse(http.StatusNotFound, publishNotFoundBody)
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
	}))

	stdout, _, runErr := runPublishCommand(t, PublishAppStoreCommand, []string{
		"--app", "app-1",
		"--ipa", "app.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--metadata-dir", metadataDir,
		"--timeout", "500ms",
		"--output", "json",
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
	stubPublishCommandSeams(t)

	metadataDir := t.TempDir()
	writePublishVersionMetadataFixture(t, metadataDir, "1.2.3")
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionLocalizations" {
			t.Fatalf("unexpected input validation request: %s %s", req.Method, req.URL.Path)
		}
		return publishCommandJSONResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`)
	}))
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

	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		return appID, nil
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

	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/appStoreVersions" {
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		}
		t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		return nil, nil
	}))

	stdout, stderr, runErr := runPublishCommand(t, PublishAppStoreCommand, []string{
		"--app", "app-1",
		"--ipa", "app.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--metadata-dir", metadataDir,
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
	stubPublishCommandSeams(t)
	t.Setenv("ASC_TIMEOUT", "500ms")

	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, appID string) (string, error) {
		return appID, nil
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

	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
	}))

	stdout, stderr, _ := runPublishCommand(t, PublishAppStoreCommand, []string{
		"--app", "app-1",
		"--ipa", "app.ipa",
		"--version", "1.2.3",
		"--build-number", "42",
		"--wait",
		"--timeout", "500ms",
		"--output", "json",
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

	stdout, _, runErr := runPublishCommand(t, PublishAppStoreCommand, []string{
		"--app", "123456789",
		"--ipa", ipaPath,
		"--version", "1.2.3",
		"--build-number", "42",
		"--metadata-dir", metadataDir,
		"--submit",
		"--dry-run",
		"--output", "json",
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

var publishTestFlightLocalBuildArgs = []string{
	"--app", "app-123",
	"--workspace", "Demo.xcworkspace",
	"--scheme", "Demo",
	"--version", "1.2.3",
	"--build-number", "42",
	"--group", "External",
	"--export-options", "ExportOptions.plist",
	"--output", "json",
}

const (
	publishGroup1BetaGroupsBody = `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"group-1","isInternalGroup":false}}]}`
	publishBuildNotFoundBody    = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"There is no resource of type 'builds' with id 'build-123'"}]}`
	publishNotFoundBody         = `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`
)

var (
	publishListExternalBetaGroups = publishRequest{http.MethodGet, "/v1/apps/app-123/betaGroups", http.StatusOK, `{"data":[{"type":"betaGroups","id":"group-1","attributes":{"name":"External","isInternalGroup":false}}]}`}
	publishListGroup1BetaGroups   = publishRequest{http.MethodGet, "/v1/apps/app-123/betaGroups", http.StatusOK, publishGroup1BetaGroupsBody}

	publishAttachToPreparedVersion = []publishRequest{
		{http.MethodGet, "/v1/apps/app-123/appStoreVersions", http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`},
		{http.MethodGet, "/v1/appStoreVersions/version-1/build", http.StatusNotFound, publishNotFoundBody},
		{http.MethodPatch, "/v1/appStoreVersions/version-1/relationships/build", http.StatusNoContent, ""},
	}
)

// stubPublishCommandSeams restores every publish seam when the test ends and
// stubs the client, an app lookup that resolves to app-123, and IPA validation.
func stubPublishCommandSeams(t *testing.T) {
	t.Helper()

	t.Cleanup(overridePublishCommandTestHooks(t))
	getPublishASCClientFn = func(time.Duration) (*asc.Client, error) { return newPublishCommandTestClient(t), nil }
	resolvePublishAppIDWithLookupFn = func(_ context.Context, _ *asc.Client, _ string) (string, error) {
		return "app-123", nil
	}
	validatePublishIPAPathFn = func(string) (os.FileInfo, error) { return newPublishTestFileInfo(t) }
}

type publishBuildStages struct {
	archiveCalls int
	exportCalls  int
	archiveOpts  localxcode.ArchiveOptions
	exportOpts   localxcode.ExportOptions
}

// stubPublishArchiveAndExport makes archive and export produce the Demo 1.2.3
// (42) artifacts and records the calls and options each stage received.
func stubPublishArchiveAndExport() *publishBuildStages {
	stages := &publishBuildStages{}
	runPublishArchiveFn = func(_ context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		stages.archiveCalls++
		stages.archiveOpts = opts
		return &localxcode.ArchiveResult{
			ArchivePath:   ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			BundleID:      "com.example.demo",
			Version:       "1.2.3",
			BuildNumber:   "42",
			Scheme:        "Demo",
			Configuration: "Release",
		}, nil
	}
	runPublishExportFn = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		stages.exportCalls++
		stages.exportOpts = opts
		return &localxcode.ExportResult{
			ArchivePath: ".asc/artifacts/Demo-IOS-1.2.3-42.xcarchive",
			IPAPath:     ".asc/artifacts/Demo-IOS-1.2.3-42.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	return stages
}

// stubPublishArchiveAndExportVersionOnly is stubPublishArchiveAndExport with
// results that carry no bundle ID, scheme or configuration.
func stubPublishArchiveAndExportVersionOnly() *publishBuildStages {
	stages := stubPublishArchiveAndExport()
	archive, export := runPublishArchiveFn, runPublishExportFn
	runPublishArchiveFn = func(ctx context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		result, err := archive(ctx, opts)
		result.BundleID, result.Scheme, result.Configuration = "", "", ""
		return result, err
	}
	runPublishExportFn = func(ctx context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		result, err := export(ctx, opts)
		result.BundleID = ""
		return result, err
	}
	return stages
}

func stubPublishIPAUpload(processingState string) *int {
	calls := 0
	uploadBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, _ string, _ string, _ os.FileInfo, version, buildNumber string, _ asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		calls++
		return publishUploadOnlyResult(version, buildNumber, processingState), nil
	}
	return &calls
}

type publishRequest struct {
	method string
	path   string
	status int
	body   string
}

// scriptPublishRequests answers the requests in order and fails the test on
// any request that is out of order or beyond the script. It returns the number
// of requests received.
func scriptPublishRequests(t *testing.T, requests ...publishRequest) *int {
	t.Helper()

	fixture := handlertest.New(t)
	count := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		count++
		if count > len(requests) {
			return nil, fixture.Errorf("unexpected request %d: %s %s", count, req.Method, req.URL.String())
		}
		want := requests[count-1]
		if req.Method != want.method || req.URL.Path != want.path {
			return nil, fixture.Errorf("unexpected request %d: %s %s, want %s %s", count, req.Method, req.URL.String(), want.method, want.path)
		}
		return publishCommandJSONResponse(want.status, want.body)
	}))
	return &count
}

func runPublishCommand(t *testing.T, newCommand func() *ffcli.Command, args []string) (string, string, error) {
	t.Helper()

	cmd := newCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	var runErr error
	stdout, stderr := capturePublishCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	return stdout, stderr, runErr
}

func assertPublishPayload(t *testing.T, stdout string, want map[string]any) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	for key, value := range want {
		if payload[key] != value {
			t.Fatalf("expected %s %v, got %#v", key, value, payload[key])
		}
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
	originalValidatePKGPath := validatePublishPKGPathFn
	originalUploadBuildAndWait := uploadBuildAndWaitForIDFn
	originalUploadPKGBuildAndWait := uploadPKGBuildAndWaitForIDFn
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
		validatePublishPKGPathFn = originalValidatePKGPath
		uploadBuildAndWaitForIDFn = originalUploadBuildAndWait
		uploadPKGBuildAndWaitForIDFn = originalUploadPKGBuildAndWait
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

func TestPublishBuildProcessingWaitIncludesProcessingDetails(t *testing.T) {
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Cleanup(shared.SetBuildUploadFailureDiagnosticsForTesting(func(context.Context, *asc.Client, string, *asc.BuildUploadResponse) (string, error) {
		return "ITMS-90000: processing details", nil
	}))

	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/builds/build-1":
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"build-1","attributes":{"version":"42","processingState":"FAILED"}}}`)
		case "/v1/builds/build-1/app":
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"apps","id":"app-1"}}`)
		case "/v1/builds/build-1/preReleaseVersion":
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"preReleaseVersions","id":"pre-1","attributes":{"version":"1.2.3","platform":"IOS"}}}`)
		case "/v1/builds":
			return publishCommandJSONResponse(http.StatusOK, `{"data":[{"type":"builds","id":"build-1","attributes":{"version":"42"},"relationships":{"buildUpload":{"data":{"type":"buildUploads","id":"upload-1"}}}}],"links":{}}`)
		case "/v1/buildUploads/upload-1":
			return publishCommandJSONResponse(http.StatusOK, `{"data":{"type":"buildUploads","id":"upload-1","attributes":{"cfBundleShortVersionString":"1.2.3","cfBundleVersion":"42","platform":"IOS"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	}))

	build, err := waitForPublishBuildProcessingFn(context.Background(), newPublishCommandTestClient(t), "build-1", time.Millisecond)
	if want := "build processing failed: FAILED; App Store Connect processing details: ITMS-90000: processing details"; err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if build == nil || build.Data.Attributes.ProcessingState != asc.BuildProcessingStateFailed {
		t.Fatalf("build = %#v, want the failed build for partial-failure reporting", build)
	}
}
