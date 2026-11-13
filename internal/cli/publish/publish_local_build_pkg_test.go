package publish

import (
	"context"
	"flag"
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
)

func TestUploadPKGBuildAndWaitForIDUsesPKGUTI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Demo.pkg")
	if err := os.WriteFile(path, []byte("pkg"), 0o600); err != nil {
		t.Fatalf("write PKG: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat PKG: %v", err)
	}

	requestCount := 0
	swapTransport(t, publishCommandRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildUploads","id":"upload-1"}}`)
		case 2:
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("read request body: %v", readErr)
			}
			if !strings.Contains(string(body), `"fileName":"Demo.pkg"`) || !strings.Contains(string(body), `"uti":"com.apple.pkg"`) {
				t.Fatalf("expected PKG file reservation, got %s", body)
			}
			return publishCommandJSONResponse(http.StatusCreated, `{"data":{"type":"buildUploadFiles","id":"file-1","attributes":{"fileName":"Demo.pkg","fileSize":3}}}`)
		default:
			t.Fatalf("unexpected request %d: %s %s", requestCount, req.Method, req.URL.String())
			return nil, nil
		}
	}))

	_, err = uploadPKGBuildAndWaitForID(context.Background(), newPublishCommandTestClient(t), "app-1", path, fileInfo, "1.2.3", "42", asc.PlatformMacOS, time.Second, time.Second, true)
	if err == nil || !strings.Contains(err.Error(), "no upload operations returned") {
		t.Fatalf("expected upload-operation sentinel after reservation, got %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected two reservation requests, got %d", requestCount)
	}
}

func TestResolveLocalBuildConfigUsesPKGForMacOS(t *testing.T) {
	fs := flag.NewFlagSet("publish local macos", flag.ContinueOnError)
	values := bindPublishLocalBuildFlags(fs)
	if err := fs.Parse([]string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--export-options", "ExportOptions.plist",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := validatePublishExportOptionsFlags(values, collectSetFlags(fs)); err != nil {
		t.Fatalf("validate export options flags: %v", err)
	}

	config, err := resolveLocalBuildConfig(values, "MAC_OS", "1.2.3", "42")
	if err != nil {
		t.Fatalf("resolveLocalBuildConfig() error: %v", err)
	}
	if config.PKGPath != filepath.Join(".asc", "artifacts", "Demo-MAC_OS-1.2.3-42.pkg") {
		t.Fatalf("unexpected PKG path: %q", config.PKGPath)
	}
	if config.IPAPath != "" {
		t.Fatalf("expected no IPA path for MAC_OS, got %q", config.IPAPath)
	}
}

func TestResolveLocalBuildConfigRejectsInvalidPKGDestination(t *testing.T) {
	fs := flag.NewFlagSet("publish local macos invalid pkg", flag.ContinueOnError)
	values := bindPublishLocalBuildFlags(fs)
	if err := fs.Parse([]string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--export-options", "ExportOptions.plist",
		"--pkg-path", "Demo.zip",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := validatePublishExportOptionsFlags(values, collectSetFlags(fs)); err != nil {
		t.Fatalf("validate export options flags: %v", err)
	}

	_, err := resolveLocalBuildConfig(values, "MAC_OS", "1.2.3", "42")
	if err == nil || !strings.Contains(err.Error(), "--pkg-path must end with .pkg") {
		t.Fatalf("expected invalid PKG destination error, got %v", err)
	}
}

func TestValidateLocalBuildArtifactFlagsMatchPlatform(t *testing.T) {
	for _, tc := range []struct {
		name     string
		platform string
		args     []string
		wantErr  string
	}{
		{
			name:     "pkg only for macos",
			platform: "IOS",
			args:     []string{"--pkg-path", "Demo.pkg"},
			wantErr:  "--pkg-path is only supported with --platform MAC_OS",
		},
		{
			name:     "pkg path cannot be empty",
			platform: "MAC_OS",
			args:     []string{"--pkg-path", ""},
			wantErr:  "--pkg-path must not be empty",
		},
		{
			name:     "ipa rejected for macos",
			platform: "MAC_OS",
			args:     []string{"--ipa-path", "Demo.ipa"},
			wantErr:  "--ipa-path is not supported with --platform MAC_OS; use --pkg-path",
		},
		{
			name:     "artifact paths mutually exclusive",
			platform: "MAC_OS",
			args:     []string{"--ipa-path", "Demo.ipa", "--pkg-path", "Demo.pkg"},
			wantErr:  "--ipa-path and --pkg-path are mutually exclusive",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet(tc.name, flag.ContinueOnError)
			values := bindPublishLocalBuildFlags(fs)
			args := append([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo"}, tc.args...)
			if err := fs.Parse(args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			err := validateLocalBuildArtifactFlags(values, collectSetFlags(fs), tc.platform)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestResolveLocalBuildConfigRejectsGeneratedManualSigningForMacOS(t *testing.T) {
	t.Chdir(t.TempDir())
	fs := flag.NewFlagSet("publish local macos manual", flag.ContinueOnError)
	values := bindPublishLocalBuildFlags(fs)
	if err := fs.Parse([]string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--signing-style", "manual",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := validatePublishExportOptionsFlags(values, collectSetFlags(fs)); err != nil {
		t.Fatalf("validate export options flags: %v", err)
	}

	_, err := resolveLocalBuildConfig(values, "MAC_OS", "1.2.3", "42")
	if err == nil || !strings.Contains(err.Error(), "manual signing for a macOS local build requires an explicit --export-options plist") {
		t.Fatalf("expected macOS manual-signing usage error, got %v", err)
	}
}

func TestRunPublishLocalBuildExportsAndUploadsPKGForMacOS(t *testing.T) {
	restore := overridePublishCommandTestHooks(t)
	defer restore()

	tempDir := t.TempDir()
	pkgPath := filepath.Join(tempDir, "Demo.pkg")
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")

	preflightPublishXcodeFn = func(context.Context) error { return nil }
	runPublishArchiveFn = func(_ context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		if !containsString(opts.XcodebuildArgs, "generic/platform=macOS") {
			t.Fatalf("expected macOS archive destination, got %v", opts.XcodebuildArgs)
		}
		return &localxcode.ArchiveResult{
			ArchivePath: archivePath,
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
			Scheme:      "Demo",
		}, nil
	}
	runPublishExportFn = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		if opts.PKGPath != pkgPath || opts.IPAPath != "" {
			t.Fatalf("expected PKG-only export destination, got IPA=%q PKG=%q", opts.IPAPath, opts.PKGPath)
		}
		return &localxcode.ExportResult{
			ArchivePath: archivePath,
			PKGPath:     pkgPath,
			BundleID:    "com.example.demo",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}
	validatePublishPKGPathFn = func(path string) (os.FileInfo, error) {
		if path != pkgPath {
			t.Fatalf("expected PKG validation for %q, got %q", pkgPath, path)
		}
		return newPublishTestFileInfo(t)
	}
	uploadPKGBuildAndWaitForIDFn = func(_ context.Context, _ *asc.Client, appID, path string, _ os.FileInfo, version, buildNumber string, platform asc.Platform, _ time.Duration, _ time.Duration, _ bool) (*publishUploadResult, error) {
		if appID != "app-123" || path != pkgPath || platform != asc.PlatformMacOS {
			t.Fatalf("unexpected PKG upload: app=%q path=%q platform=%q", appID, path, platform)
		}
		return &publishUploadResult{
			Build:   &asc.BuildResponse{Data: asc.Resource[asc.BuildAttributes]{ID: "build-123"}},
			Version: version, BuildNumber: buildNumber,
		}, nil
	}

	result, err := runPublishLocalBuild(
		context.Background(), nil, "app-123", "MAC_OS", "1.2.3", "42",
		shared.PublishDefaultPollInterval, time.Minute, false,
		publishLocalBuildConfig{
			ProjectPath: "Demo.xcodeproj", Scheme: "Demo", Configuration: "Release",
			ArchivePath: archivePath, PKGPath: pkgPath, ExportOptionsPath: "ExportOptions.plist",
		},
	)
	if err != nil {
		t.Fatalf("runPublishLocalBuild() error: %v", err)
	}
	if result.Export.PKGPath != pkgPath || result.Export.IPAPath != "" {
		t.Fatalf("unexpected export result: %#v", result.Export)
	}
	if !result.Uploaded || result.Build.Data.ID != "build-123" {
		t.Fatalf("unexpected upload result: %#v", result)
	}
}

func TestPlannedAppStoreMacOSLocalBuildUsesPKG(t *testing.T) {
	config := publishLocalBuildConfig{
		Scheme: "Demo", Configuration: "Release",
		ArchivePath: "Demo.xcarchive", PKGPath: "Demo.pkg",
	}
	result := plannedAppStorePublishResult(asc.PublishModeLocalBuild, "1.2.3", "42", false, false, false, true, config)
	if result.Export == nil || result.Export.PKGPath != "Demo.pkg" || result.Export.IPAPath != "" {
		t.Fatalf("unexpected planned export: %#v", result.Export)
	}
	if len(result.Plan) < 3 || !strings.Contains(result.Plan[1].Message, "PKG") || !strings.Contains(result.Plan[2].Message, "PKG") {
		t.Fatalf("expected PKG-specific export and upload plan, got %#v", result.Plan)
	}
}
