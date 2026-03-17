package xcode

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
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeExportWaitRequiresDirectUpload(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	isDirectUploadExportOptionsFn = func(string) bool { return false }

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--wait",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatal("expected flag.ErrHelp when --wait is used without direct upload mode")
	}
	if !strings.Contains(stderr, "Error: --wait requires ExportOptions.plist with destination=upload") {
		t.Fatalf("expected direct upload usage error, got %q", stderr)
	}
}

func TestXcodeValidatePassesIPAAndAuthFlags(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	var gotOpts localxcode.ValidateOptions
	runValidate = func(_ context.Context, opts localxcode.ValidateOptions) (*localxcode.ValidateResult, error) {
		gotOpts = opts
		return &localxcode.ValidateResult{
			IPAPath:   opts.IPAPath,
			Validated: true,
		}, nil
	}

	cmd := XcodeValidateCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--ipa", "Demo.ipa",
		"--api-key", "KEY123ABC",
		"--api-issuer", "issuer-123",
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if gotOpts.IPAPath != "Demo.ipa" {
		t.Fatalf("expected ipa path Demo.ipa, got %q", gotOpts.IPAPath)
	}
	if gotOpts.APIKey != "KEY123ABC" {
		t.Fatalf("expected api key KEY123ABC, got %q", gotOpts.APIKey)
	}
	if gotOpts.APIIssuer != "issuer-123" {
		t.Fatalf("expected api issuer issuer-123, got %q", gotOpts.APIIssuer)
	}

	var payload struct {
		IPAPath   string `json:"ipa_path"`
		Validated bool   `json:"validated"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload.IPAPath != "Demo.ipa" || !payload.Validated {
		t.Fatalf("unexpected validate payload: %+v", payload)
	}
}

func TestXcodeExportWaitRequiresPositivePollInterval(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	isDirectUploadExportOptionsFn = func(string) bool { return true }

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--wait",
		"--poll-interval", "0s",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatal("expected flag.ErrHelp for non-positive poll interval")
	}
	if !strings.Contains(stderr, "Error: --poll-interval must be greater than 0") {
		t.Fatalf("expected poll interval usage error, got %q", stderr)
	}
}

func TestXcodeExportAllowsPollIntervalWithoutWait(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	runExport = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: "/tmp/Demo.xcarchive",
			IPAPath:     "/tmp/Demo.ipa",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--poll-interval", "0s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Fatal("expected JSON output")
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output without --wait, got %q", stderr)
	}
}

func TestXcodeExportWaitPollsForUploadedBuild(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	isDirectUploadExportOptionsFn = func(string) bool { return true }
	runExport = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: "/tmp/Demo.xcarchive",
			IPAPath:     "",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	inferArchivePlatformFn = func(string) (string, error) { return "IOS", nil }
	getASCClientFn = func() (*asc.Client, error) { return &asc.Client{}, nil }
	resolveAppIDWithExactLookupFn = func(_ context.Context, _ *asc.Client, app string) (string, error) {
		if app != "com.example.demo" {
			t.Fatalf("expected bundle ID app lookup, got %q", app)
		}
		return "123456789", nil
	}
	resolveBuildUploadIDFn = func(_ context.Context, _ *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time, pollInterval time.Duration) (string, error) {
		if appID != "123456789" {
			t.Fatalf("expected resolved app ID for upload lookup, got %q", appID)
		}
		if version != "1.2.3" || buildNumber != "42" || platform != "IOS" {
			t.Fatalf("unexpected upload lookup params: version=%q build=%q platform=%q", version, buildNumber, platform)
		}
		if pollInterval != 5*time.Second {
			t.Fatalf("expected 5s poll interval, got %s", pollInterval)
		}
		if exportStartedAt.IsZero() {
			t.Fatal("expected export start time for upload lookup")
		}
		if exportCompletedAt.IsZero() {
			t.Fatal("expected export completion time for upload lookup")
		}
		if exportCompletedAt.Before(exportStartedAt) {
			t.Fatalf("expected export completion time after start, got start=%s end=%s", exportStartedAt, exportCompletedAt)
		}
		return "upload-123", nil
	}
	waitForBuildByNumberOrUploadFailureFn = func(_ context.Context, _ *asc.Client, appID, uploadID, version, buildNumber, platform string, pollInterval time.Duration) (*asc.BuildResponse, error) {
		if appID != "123456789" {
			t.Fatalf("expected resolved app ID, got %q", appID)
		}
		if uploadID != "upload-123" {
			t.Fatalf("expected upload-123 upload ID for xcode export wait, got %q", uploadID)
		}
		if version != "1.2.3" || buildNumber != "42" || platform != "IOS" {
			t.Fatalf("unexpected wait lookup params: version=%q build=%q platform=%q", version, buildNumber, platform)
		}
		if pollInterval != 5*time.Second {
			t.Fatalf("expected 5s poll interval, got %s", pollInterval)
		}
		return &asc.BuildResponse{
			Data: asc.Resource[asc.BuildAttributes]{
				ID: "build-123",
				Attributes: asc.BuildAttributes{
					Version:         "42",
					ProcessingState: asc.BuildProcessingStateValid,
				},
			},
		}, nil
	}
	waitForBuildProcessingFn = func(_ context.Context, _ *asc.Client, buildID string, pollInterval time.Duration) (*asc.BuildResponse, error) {
		if buildID != "build-123" {
			t.Fatalf("expected build-123, got %q", buildID)
		}
		if pollInterval != 5*time.Second {
			t.Fatalf("expected 5s poll interval, got %s", pollInterval)
		}
		return &asc.BuildResponse{
			Data: asc.Resource[asc.BuildAttributes]{
				ID: "build-123",
				Attributes: asc.BuildAttributes{
					Version:         "42",
					ProcessingState: asc.BuildProcessingStateValid,
				},
			},
		}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--wait",
		"--poll-interval", "5s",
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v", runErr)
	}

	if strings.TrimSpace(stdout) == "" {
		t.Fatal("expected JSON output")
	}
	var payload struct {
		ArchivePath     string `json:"archive_path"`
		IPAPath         string `json:"ipa_path"`
		BuildID         string `json:"build_id"`
		ProcessingState string `json:"processing_state"`
		BundleID        string `json:"bundle_id"`
		Version         string `json:"version"`
		BuildNumber     string `json:"build_number"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload.BuildID != "build-123" {
		t.Fatalf("expected build_id build-123, got %q", payload.BuildID)
	}
	if payload.ProcessingState != asc.BuildProcessingStateValid {
		t.Fatalf("expected processing state VALID, got %q", payload.ProcessingState)
	}
	if !strings.Contains(stderr, "Waiting for build 42 (1.2.3) to appear in App Store Connect...") {
		t.Fatalf("expected discovery wait message, got %q", stderr)
	}
	if !strings.Contains(stderr, "Build build-123 discovered; waiting for processing...") {
		t.Fatalf("expected processing wait message, got %q", stderr)
	}
}

func TestXcodeExportWaitRejectsNilProcessedBuildResponse(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	isDirectUploadExportOptionsFn = func(string) bool { return true }
	runExport = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: "/tmp/Demo.xcarchive",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	inferArchivePlatformFn = func(string) (string, error) { return "IOS", nil }
	getASCClientFn = func() (*asc.Client, error) { return &asc.Client{}, nil }
	resolveAppIDWithExactLookupFn = func(context.Context, *asc.Client, string) (string, error) {
		return "123456789", nil
	}
	resolveBuildUploadIDFn = func(context.Context, *asc.Client, string, string, string, string, time.Time, time.Time, time.Duration) (string, error) {
		return "upload-123", nil
	}
	waitForBuildByNumberOrUploadFailureFn = func(context.Context, *asc.Client, string, string, string, string, string, time.Duration) (*asc.BuildResponse, error) {
		return &asc.BuildResponse{
			Data: asc.Resource[asc.BuildAttributes]{
				ID: "build-123",
			},
		}, nil
	}
	waitForBuildProcessingFn = func(context.Context, *asc.Client, string, time.Duration) (*asc.BuildResponse, error) {
		return nil, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--wait",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, _ = captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil {
		t.Fatal("expected error for nil processed build response")
	}
	if !strings.Contains(runErr.Error(), "failed to resolve processed build state for build \"build-123\"") {
		t.Fatalf("expected nil processed build error, got %v", runErr)
	}
}

func TestXcodeExportWaitRejectsMissingBuildUploadID(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	isDirectUploadExportOptionsFn = func(string) bool { return true }
	runExport = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return &localxcode.ExportResult{
			ArchivePath: "/tmp/Demo.xcarchive",
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	inferArchivePlatformFn = func(string) (string, error) { return "IOS", nil }
	getASCClientFn = func() (*asc.Client, error) { return &asc.Client{}, nil }
	resolveAppIDWithExactLookupFn = func(context.Context, *asc.Client, string) (string, error) {
		return "123456789", nil
	}
	resolveBuildUploadIDFn = func(context.Context, *asc.Client, string, string, string, string, time.Time, time.Time, time.Duration) (string, error) {
		return "", nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--wait",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, _ = captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil {
		t.Fatal("expected error for missing build upload ID")
	}
	if !strings.Contains(runErr.Error(), "failed to resolve build upload for version \"1.2.3\" build \"42\"") {
		t.Fatalf("expected missing build upload error, got %v", runErr)
	}
}

func TestFindRecentBuildUploadIDIgnoresUndatedUploadsAfterExportStarts(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = xcodeCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/buildUploads" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("filter[cfBundleShortVersionString]") != "1.2.3" {
			return nil, fmt.Errorf("unexpected short version filter: %q", values.Get("filter[cfBundleShortVersionString]"))
		}
		if values.Get("filter[cfBundleVersion]") != "42" {
			return nil, fmt.Errorf("unexpected build version filter: %q", values.Get("filter[cfBundleVersion]"))
		}
		if values.Get("filter[platform]") != "IOS" {
			return nil, fmt.Errorf("unexpected platform filter: %q", values.Get("filter[platform]"))
		}
		if values.Get("sort") != "-uploadedDate" {
			return nil, fmt.Errorf("unexpected sort: %q", values.Get("sort"))
		}
		if values.Get("limit") != "200" {
			return nil, fmt.Errorf("unexpected limit: %q", values.Get("limit"))
		}
		return xcodeCommandJSONResponse(`{
			"data": [
				{
					"type": "buildUploads",
					"id": "stale-undated",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS"
					}
				}
			],
			"links": {}
		}`)
	})

	client := newXcodeCommandTestClient(t)
	exportStartedAt := time.Date(2026, time.March, 16, 12, 0, 0, 0, time.UTC)
	exportCompletedAt := exportStartedAt.Add(30 * time.Second)
	uploadID, found, err := findRecentBuildUploadID(context.Background(), client, "app-123", "1.2.3", "42", "IOS", exportStartedAt, exportCompletedAt)
	if err != nil {
		t.Fatalf("findRecentBuildUploadID() error: %v", err)
	}
	if found {
		t.Fatalf("expected undated uploads to be ignored after export start, got upload ID %q", uploadID)
	}
	if uploadID != "" {
		t.Fatalf("expected empty upload ID when only undated uploads exist, got %q", uploadID)
	}
}

func TestFindRecentBuildUploadIDPrefersLatestUploadWithinCompletedExportWindow(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = xcodeCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/buildUploads" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "200" {
			return nil, fmt.Errorf("unexpected limit: %q", req.URL.Query().Get("limit"))
		}
		return xcodeCommandJSONResponse(`{
			"data": [
				{
					"type": "buildUploads",
					"id": "later-retry",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"uploadedDate": "2026-03-16T12:00:35Z"
					}
				},
				{
					"type": "buildUploads",
					"id": "current-export",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"uploadedDate": "2026-03-16T12:00:25Z"
					}
				},
				{
					"type": "buildUploads",
					"id": "older-upload",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"uploadedDate": "2026-03-16T12:00:05Z"
					}
				}
			],
			"links": {}
		}`)
	})

	client := newXcodeCommandTestClient(t)
	exportStartedAt := time.Date(2026, time.March, 16, 12, 0, 10, 0, time.UTC)
	exportCompletedAt := time.Date(2026, time.March, 16, 12, 0, 30, 0, time.UTC)
	uploadID, found, err := findRecentBuildUploadID(context.Background(), client, "app-123", "1.2.3", "42", "IOS", exportStartedAt, exportCompletedAt)
	if err != nil {
		t.Fatalf("findRecentBuildUploadID() error: %v", err)
	}
	if !found {
		t.Fatal("expected to find a matching upload in the completed export window")
	}
	if uploadID != "current-export" {
		t.Fatalf("expected latest upload within completed export window, got %q", uploadID)
	}
}

func TestFindRecentBuildUploadIDUsesCreatedDateForCompletedExportCutoff(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = xcodeCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/buildUploads" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		if req.URL.Query().Get("limit") != "200" {
			return nil, fmt.Errorf("unexpected limit: %q", req.URL.Query().Get("limit"))
		}
		return xcodeCommandJSONResponse(`{
			"data": [
				{
					"type": "buildUploads",
					"id": "later-retry",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"createdDate": "2026-03-16T12:00:40Z",
						"uploadedDate": "2026-03-16T12:00:45Z"
					}
				},
				{
					"type": "buildUploads",
					"id": "current-export",
					"attributes": {
						"cfBundleShortVersionString": "1.2.3",
						"cfBundleVersion": "42",
						"platform": "IOS",
						"createdDate": "2026-03-16T12:00:28Z",
						"uploadedDate": "2026-03-16T12:00:35Z"
					}
				}
			],
			"links": {}
		}`)
	})

	client := newXcodeCommandTestClient(t)
	exportStartedAt := time.Date(2026, time.March, 16, 12, 0, 10, 0, time.UTC)
	exportCompletedAt := time.Date(2026, time.March, 16, 12, 0, 30, 0, time.UTC)
	uploadID, found, err := findRecentBuildUploadID(context.Background(), client, "app-123", "1.2.3", "42", "IOS", exportStartedAt, exportCompletedAt)
	if err != nil {
		t.Fatalf("findRecentBuildUploadID() error: %v", err)
	}
	if !found {
		t.Fatal("expected createdDate within the export window to keep the current upload eligible")
	}
	if uploadID != "current-export" {
		t.Fatalf("expected current export upload selected via createdDate, got %q", uploadID)
	}
}

func TestFindRecentBuildUploadIDPaginatesUntilUploadWithinCompletedExportWindow(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = xcodeCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/buildUploads" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}

		requests++
		switch req.URL.Query().Get("cursor") {
		case "":
			if req.URL.Query().Get("limit") != "200" {
				return nil, fmt.Errorf("unexpected limit: %q", req.URL.Query().Get("limit"))
			}
			return xcodeCommandJSONResponse(`{
				"data": [
					{
						"type": "buildUploads",
						"id": "latest-retry",
						"attributes": {
							"cfBundleShortVersionString": "1.2.3",
							"cfBundleVersion": "42",
							"platform": "IOS",
							"uploadedDate": "2026-03-16T12:00:45Z"
						}
					},
					{
						"type": "buildUploads",
						"id": "earlier-retry",
						"attributes": {
							"cfBundleShortVersionString": "1.2.3",
							"cfBundleVersion": "42",
							"platform": "IOS",
							"uploadedDate": "2026-03-16T12:00:40Z"
						}
					}
				],
				"links": {
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-123/buildUploads?cursor=page-2"
				}
			}`)
		case "page-2":
			return xcodeCommandJSONResponse(`{
				"data": [
					{
						"type": "buildUploads",
						"id": "current-export",
						"attributes": {
							"cfBundleShortVersionString": "1.2.3",
							"cfBundleVersion": "42",
							"platform": "IOS",
							"uploadedDate": "2026-03-16T12:00:25Z"
						}
					}
				],
				"links": {
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-123/buildUploads?cursor=page-3"
				}
			}`)
		case "page-3":
			t.Fatal("did not expect third page fetch after finding current export upload")
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected cursor: %q", req.URL.Query().Get("cursor"))
		}
	})

	client := newXcodeCommandTestClient(t)
	exportStartedAt := time.Date(2026, time.March, 16, 12, 0, 10, 0, time.UTC)
	exportCompletedAt := time.Date(2026, time.March, 16, 12, 0, 30, 0, time.UTC)
	uploadID, found, err := findRecentBuildUploadID(context.Background(), client, "app-123", "1.2.3", "42", "IOS", exportStartedAt, exportCompletedAt)
	if err != nil {
		t.Fatalf("findRecentBuildUploadID() error: %v", err)
	}
	if !found {
		t.Fatal("expected to find a matching upload across paginated results")
	}
	if uploadID != "current-export" {
		t.Fatalf("expected current export upload across pages, got %q", uploadID)
	}
	if requests != 2 {
		t.Fatalf("expected 2 paginated build upload requests, got %d", requests)
	}
}

func TestFindRecentBuildUploadIDContinuesPagingForCreatedDateOnlyUploads(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := 0
	http.DefaultTransport = xcodeCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return nil, fmt.Errorf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-123/buildUploads" {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}

		requests++
		switch req.URL.Query().Get("cursor") {
		case "":
			if req.URL.Query().Get("limit") != "200" {
				return nil, fmt.Errorf("unexpected limit: %q", req.URL.Query().Get("limit"))
			}
			return xcodeCommandJSONResponse(`{
				"data": [
					{
						"type": "buildUploads",
						"id": "later-retry",
						"attributes": {
							"cfBundleShortVersionString": "1.2.3",
							"cfBundleVersion": "42",
							"platform": "IOS",
							"uploadedDate": "2026-03-16T12:00:45Z"
						}
					}
				],
				"links": {
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-123/buildUploads?cursor=page-2"
				}
			}`)
		case "page-2":
			return xcodeCommandJSONResponse(`{
				"data": [
					{
						"type": "buildUploads",
						"id": "older-uploaded",
						"attributes": {
							"cfBundleShortVersionString": "1.2.3",
							"cfBundleVersion": "42",
							"platform": "IOS",
							"uploadedDate": "2026-03-16T12:00:05Z"
						}
					}
				],
				"links": {
					"next": "https://api.appstoreconnect.apple.com/v1/apps/app-123/buildUploads?cursor=page-3"
				}
			}`)
		case "page-3":
			return xcodeCommandJSONResponse(`{
				"data": [
					{
						"type": "buildUploads",
						"id": "current-export-created-only",
						"attributes": {
							"cfBundleShortVersionString": "1.2.3",
							"cfBundleVersion": "42",
							"platform": "IOS",
							"createdDate": "2026-03-16T12:00:20Z"
						}
					}
				],
				"links": {}
			}`)
		default:
			return nil, fmt.Errorf("unexpected cursor: %q", req.URL.Query().Get("cursor"))
		}
	})

	client := newXcodeCommandTestClient(t)
	exportStartedAt := time.Date(2026, time.March, 16, 12, 0, 10, 0, time.UTC)
	exportCompletedAt := time.Date(2026, time.March, 16, 12, 0, 30, 0, time.UTC)
	uploadID, found, err := findRecentBuildUploadID(context.Background(), client, "app-123", "1.2.3", "42", "IOS", exportStartedAt, exportCompletedAt)
	if err != nil {
		t.Fatalf("findRecentBuildUploadID() error: %v", err)
	}
	if !found {
		t.Fatal("expected to keep paging until a createdDate-only upload in the export window was found")
	}
	if uploadID != "current-export-created-only" {
		t.Fatalf("expected createdDate-only upload from later page, got %q", uploadID)
	}
	if requests != 3 {
		t.Fatalf("expected 3 paginated build upload requests before finding createdDate-only upload, got %d", requests)
	}
}

func overrideXcodeCommandTestHooks(t *testing.T) func() {
	t.Helper()

	originalRunArchive := runArchive
	originalRunExport := runExport
	originalRunValidate := runValidate
	originalIsDirectUpload := isDirectUploadExportOptionsFn
	originalInferArchivePlatform := inferArchivePlatformFn
	originalGetASCClient := getASCClientFn
	originalResolveAppID := resolveAppIDWithExactLookupFn
	originalResolveBuildUploadID := resolveBuildUploadIDFn
	originalWaitForDiscovery := waitForBuildByNumberOrUploadFailureFn
	originalWaitForProcessing := waitForBuildProcessingFn
	originalWaitTimeout := resolveXcodeExportWaitTimeoutFn

	return func() {
		runArchive = originalRunArchive
		runExport = originalRunExport
		runValidate = originalRunValidate
		isDirectUploadExportOptionsFn = originalIsDirectUpload
		inferArchivePlatformFn = originalInferArchivePlatform
		getASCClientFn = originalGetASCClient
		resolveAppIDWithExactLookupFn = originalResolveAppID
		resolveBuildUploadIDFn = originalResolveBuildUploadID
		waitForBuildByNumberOrUploadFailureFn = originalWaitForDiscovery
		waitForBuildProcessingFn = originalWaitForProcessing
		resolveXcodeExportWaitTimeoutFn = originalWaitTimeout
	}
}

func captureCommandOutput(t *testing.T, fn func() error) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
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

type xcodeCommandRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn xcodeCommandRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func xcodeCommandJSONResponse(body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func newXcodeCommandTestClient(t *testing.T) *asc.Client {
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
