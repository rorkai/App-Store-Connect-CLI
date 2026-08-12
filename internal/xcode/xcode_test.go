package xcode

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
)

func TestArchiveUnsupportedPlatform(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "linux"
	t.Cleanup(restore)

	_, err := Archive(context.Background(), ArchiveOptions{
		ProjectPath: projectPath,
		Scheme:      "Demo",
		ArchivePath: filepath.Join(t.TempDir(), "Demo.xcarchive"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("expected macOS-only error, got %v", err)
	}
}

func TestArchiveMissingXcodebuild(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	t.Cleanup(restore)

	_, err := Archive(context.Background(), ArchiveOptions{
		ProjectPath: projectPath,
		Scheme:      "Demo",
		ArchivePath: filepath.Join(t.TempDir(), "Demo.xcarchive"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "xcodebuild not available") {
		t.Fatalf("expected xcodebuild error, got %v", err)
	}
}

func TestValidateExistingPathAllowsTrailingSeparator(t *testing.T) {
	workspacePath := filepath.Join(t.TempDir(), "Demo.xcworkspace")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	pathWithSeparator := workspacePath + string(os.PathSeparator)
	if err := validateExistingPath(pathWithSeparator, ".xcworkspace", "--workspace"); err != nil {
		t.Fatalf("expected trailing separator path to validate, got %v", err)
	}
}

func TestArchiveNormalizesTrailingSeparatorArchivePath(t *testing.T) {
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	archivePath := filepath.Join(tempDir, "artifacts", "Demo.xcarchive")
	result, err := Archive(context.Background(), ArchiveOptions{
		ProjectPath: projectPath,
		Scheme:      "Demo",
		ArchivePath: archivePath + string(os.PathSeparator),
	})
	if err != nil {
		t.Fatalf("Archive() error: %v", err)
	}

	if result.ArchivePath != archivePath {
		t.Fatalf("expected normalized archive path %q, got %q", archivePath, result.ArchivePath)
	}
	if _, err := os.Stat(filepath.Join(archivePath, "Info.plist")); err != nil {
		t.Fatalf("expected archive to be created at normalized path: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if !strings.Contains(string(logData), "|-archivePath|"+archivePath) {
		t.Fatalf("expected normalized archive path in invocation, got %q", string(logData))
	}
}

func TestArchiveCreatesArchiveAtExactPathAndReturnsMetadata(t *testing.T) {
	tempDir := t.TempDir()
	projectPath := filepath.Join(tempDir, "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	archivePath := filepath.Join(tempDir, "artifacts", "Demo.xcarchive")
	result, err := Archive(context.Background(), ArchiveOptions{
		ProjectPath:    projectPath,
		Scheme:         "Demo",
		Configuration:  "Release",
		ArchivePath:    archivePath,
		Clean:          true,
		Overwrite:      false,
		XcodebuildArgs: []string{"-destination", "generic/platform=iOS"},
	})
	if err != nil {
		t.Fatalf("Archive() error: %v", err)
	}

	if result.ArchivePath != archivePath {
		t.Fatalf("expected archive path %q, got %q", archivePath, result.ArchivePath)
	}
	if result.BundleID != "com.example.demo" {
		t.Fatalf("expected bundle id %q, got %q", "com.example.demo", result.BundleID)
	}
	if result.Version != "1.2.3" {
		t.Fatalf("expected version %q, got %q", "1.2.3", result.Version)
	}
	if result.BuildNumber != "42" {
		t.Fatalf("expected build number %q, got %q", "42", result.BuildNumber)
	}
	if result.Scheme != "Demo" {
		t.Fatalf("expected scheme %q, got %q", "Demo", result.Scheme)
	}
	if result.Configuration != "Release" {
		t.Fatalf("expected configuration %q, got %q", "Release", result.Configuration)
	}

	info, err := os.Stat(filepath.Join(archivePath, "Info.plist"))
	if err != nil {
		t.Fatalf("expected archive Info.plist: %v", err)
	}
	if info.IsDir() {
		t.Fatal("expected Info.plist file, got directory")
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 logged commands, got %d: %q", len(lines), string(logData))
	}
	if lines[0] != "xcodebuild|-version" {
		t.Fatalf("expected version probe, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "|clean|archive|") {
		t.Fatalf("expected clean archive invocation, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "|-archivePath|"+archivePath) {
		t.Fatalf("expected archive path in invocation, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "|-destination|generic/platform=iOS|") {
		t.Fatalf("expected custom xcodebuild args, got %q", lines[1])
	}
}

func TestExportUnsupportedPlatform(t *testing.T) {
	restore := overrideTestEnvironment(t)
	runtimeGOOS = "windows"
	t.Cleanup(restore)

	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:    filepath.Join(t.TempDir(), "Demo.xcarchive"),
		ExportOptions:  filepath.Join(t.TempDir(), "ExportOptions.plist"),
		IPAPath:        filepath.Join(t.TempDir(), "Demo.ipa"),
		XcodebuildArgs: nil,
		Overwrite:      false,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("expected macOS-only error, got %v", err)
	}
}

func TestExportMissingXcodebuild(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	if err := os.WriteFile(exportOptionsPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "", exec.ErrNotFound
	}
	t.Cleanup(restore)

	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       filepath.Join(tempDir, "Demo.ipa"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "xcodebuild not available") {
		t.Fatalf("expected xcodebuild error, got %v", err)
	}
}

func TestValidateExportXcodebuildArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "none"},
		{name: "supported provisioning flag", args: []string{"-allowProvisioningUpdates"}},
		{name: "supported authentication flags", args: []string{"-authenticationKeyPath", "/tmp/AuthKey.p8", "-authenticationKeyID", "KEY123", "-authenticationKeyIssuerID", "ISSUER456"}},
		{name: "blank passthrough", args: []string{""}, wantErr: "--xcodebuild-flag cannot be empty"},
		{name: "whitespace passthrough", args: []string{" \t"}, wantErr: "--xcodebuild-flag cannot be empty"},
		{name: "export operation", args: []string{"-exportArchive"}, wantErr: `cannot override asc-managed argument "-exportArchive"`},
		{name: "archive path", args: []string{"-archivePath=/tmp/Other.xcarchive"}, wantErr: `cannot override asc-managed argument "-archivePath"`},
		{name: "export path", args: []string{"-EXPORTPATH", "/tmp/elsewhere"}, wantErr: `cannot override asc-managed argument "-EXPORTPATH"`},
		{name: "export options", args: []string{"-exportOptionsPlist=/tmp/Other.plist"}, wantErr: `cannot override asc-managed argument "-exportOptionsPlist"`},
		{name: "different operation", args: []string{"build"}, wantErr: `cannot override asc-managed argument "build"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateExportXcodebuildArgs(test.args)
			if test.wantErr == "" && err != nil {
				t.Fatalf("ValidateExportXcodebuildArgs() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("ValidateExportXcodebuildArgs() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

// TestValidateExportXcodebuildArgsRejectsEveryArgumentExportEmits derives its
// expectations from the shipped command. A newly managed argument must not be
// overridable through --xcodebuild-flag.
func TestValidateExportXcodebuildArgsRejectsEveryArgumentExportEmits(t *testing.T) {
	opts := ExportOptions{
		ArchivePath:   "/tmp/AscManaged.xcarchive",
		ExportOptions: "/tmp/AscManagedExportOptions.plist",
	}
	exportDir := "/tmp/asc-managed-export"
	suppliedValues := map[string]struct{}{
		opts.ArchivePath:   {},
		opts.ExportOptions: {},
		exportDir:          {},
	}

	for _, arg := range buildExportCommand(opts, exportDir) {
		if _, isValue := suppliedValues[arg]; isValue {
			continue
		}
		t.Run(arg, func(t *testing.T) {
			if err := ValidateExportXcodebuildArgs([]string{arg}); err == nil {
				t.Fatalf("asc emits %q but --xcodebuild-flag can still override it", arg)
			}
		})
	}
}

func TestValidateUnsupportedPlatform(t *testing.T) {
	restore := overrideTestEnvironment(t)
	runtimeGOOS = "linux"
	t.Cleanup(restore)

	_, err := Validate(context.Background(), ValidateOptions{
		IPAPath: filepath.Join(t.TempDir(), "Demo.ipa"),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("expected macOS-only error, got %v", err)
	}
}

func TestValidateMissingXcrun(t *testing.T) {
	tempDir := t.TempDir()
	ipaPath := filepath.Join(tempDir, "Demo.ipa")
	if err := writeTestIPA(ipaPath); err != nil {
		t.Fatalf("writeTestIPA() error: %v", err)
	}

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		switch file {
		case "xcodebuild":
			return "/usr/bin/xcodebuild", nil
		case "xcrun":
			return "", exec.ErrNotFound
		default:
			return "", exec.ErrNotFound
		}
	}
	commandContextFn = helperCommandContext(t, filepath.Join(tempDir, "commands.log"))
	t.Cleanup(restore)

	_, err := Validate(context.Background(), ValidateOptions{IPAPath: ipaPath})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "xcrun not available") {
		t.Fatalf("expected xcrun error, got %v", err)
	}
}

func TestValidateRejectsPartialAPIKeyAuth(t *testing.T) {
	tempDir := t.TempDir()
	ipaPath := filepath.Join(tempDir, "Demo.ipa")
	if err := writeTestIPA(ipaPath); err != nil {
		t.Fatalf("writeTestIPA() error: %v", err)
	}

	_, err := Validate(context.Background(), ValidateOptions{
		IPAPath: ipaPath,
		APIKey:  "KEY123ABC",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--api-key and --api-issuer must be provided together") {
		t.Fatalf("expected auth pairing error, got %v", err)
	}
}

func TestValidateRunsAltoolWithAuthFlags(t *testing.T) {
	tempDir := t.TempDir()
	ipaPath := filepath.Join(tempDir, "Demo.ipa")
	if err := writeTestIPA(ipaPath); err != nil {
		t.Fatalf("writeTestIPA() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		switch file {
		case "xcodebuild":
			return "/usr/bin/xcodebuild", nil
		case "xcrun":
			return "/usr/bin/xcrun", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	result, err := Validate(context.Background(), ValidateOptions{
		IPAPath:   ipaPath,
		APIKey:    "KEY123ABC",
		APIIssuer: "issuer-123",
	})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if result.IPAPath != ipaPath {
		t.Fatalf("expected ipa path %q, got %q", ipaPath, result.IPAPath)
	}
	if !result.Validated {
		t.Fatal("expected validated result")
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 logged commands, got %d: %q", len(lines), string(logData))
	}
	if lines[0] != "xcodebuild|-version" {
		t.Fatalf("expected version probe, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "xcrun|altool|--validate-app|--file|"+ipaPath+"|--type|ios|--apiKey|KEY123ABC|--apiIssuer|issuer-123") {
		t.Fatalf("expected validate invocation with auth flags, got %q", lines[1])
	}
}

func TestValidateRunsAltoolWithTVOSPlatform(t *testing.T) {
	tempDir := t.TempDir()
	ipaPath := filepath.Join(tempDir, "Demo-tvOS.ipa")
	if err := writeTestIPAWithPlatform(ipaPath, "appletvos"); err != nil {
		t.Fatalf("writeTestIPAWithPlatform() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		switch file {
		case "xcodebuild":
			return "/usr/bin/xcodebuild", nil
		case "xcrun":
			return "/usr/bin/xcrun", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	result, err := Validate(context.Background(), ValidateOptions{
		IPAPath: ipaPath,
	})
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
	if result.IPAPath != ipaPath {
		t.Fatalf("expected ipa path %q, got %q", ipaPath, result.IPAPath)
	}
	if !result.Validated {
		t.Fatal("expected validated result")
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 logged commands, got %d: %q", len(lines), string(logData))
	}
	if !strings.Contains(lines[1], "xcrun|altool|--validate-app|--file|"+ipaPath+"|--type|appletvos") {
		t.Fatalf("expected validate invocation with tvOS platform, got %q", lines[1])
	}
}

func TestValidateClassifiesAltoolOutputWithZeroExit(t *testing.T) {
	tests := []struct {
		name           string
		stdout         string
		output         string
		wantErr        bool
		wantDetail     string
		maxDetailBytes int
	}{
		{
			name:       "timestamped server error",
			output:     "2026-08-10 06:47:56.580 ERROR: [ContentDelivery.Uploader] The bundle version must be higher than the previously uploaded version.\n",
			wantErr:    true,
			wantDetail: "The bundle version must be higher than the previously uploaded version.",
		},
		{
			name:       "legacy error",
			output:     "*** Error: Unable to validate archive './artifacts/Demo.ipa'.\n",
			wantErr:    true,
			wantDetail: "Unable to validate archive './artifacts/Demo.ipa'.",
		},
		{
			name:       "timestamped error before long diagnostics",
			output:     "2026-08-10 06:47:56.580 ERROR: Early server rejection.\n" + strings.Repeat("x", xcodebuildErrorTailLimit+1) + "\n",
			wantErr:    true,
			wantDetail: "Early server rejection.",
		},
		{
			name:       "unterminated stdout before timestamped stderr",
			stdout:     "Upload progress",
			output:     "2026-08-10 06:47:56.580 ERROR: Server rejected the archive.\n",
			wantErr:    true,
			wantDetail: "Server rejected the archive.",
		},
		{
			name:           "aggregate details from both streams stay bounded",
			stdout:         "2026-08-10 06:47:56.580 ERROR: " + strings.Repeat("a", xcodebuildErrorTailLimit/2) + "\n",
			output:         "*** Error: " + strings.Repeat("b", xcodebuildErrorTailLimit/2) + "\n",
			wantErr:        true,
			wantDetail:     strings.Repeat("a", 32),
			maxDetailBytes: xcodebuildErrorTailLimit,
		},
		{
			name:   "benign output containing error text",
			output: "2026-08-10 06:47:56.580 INFO: Validation completed.\nDiagnostic: no ERROR: records were returned.\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			ipaPath := filepath.Join(tempDir, "Demo.ipa")
			if err := writeTestIPA(ipaPath); err != nil {
				t.Fatalf("writeTestIPA() error: %v", err)
			}
			logPath := filepath.Join(tempDir, "commands.log")
			t.Setenv("ASC_XCODE_HELPER_VALIDATE_STDOUT", tt.stdout)
			t.Setenv("ASC_XCODE_HELPER_VALIDATE_OUTPUT", tt.output)

			restore := overrideTestEnvironment(t)
			runtimeGOOS = "darwin"
			lookPathFn = func(file string) (string, error) {
				switch file {
				case "xcodebuild":
					return "/usr/bin/xcodebuild", nil
				case "xcrun":
					return "/usr/bin/xcrun", nil
				default:
					return "", exec.ErrNotFound
				}
			}
			commandContextFn = helperCommandContext(t, logPath)
			t.Cleanup(restore)

			result, err := Validate(context.Background(), ValidateOptions{IPAPath: ipaPath})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate() error = nil, want server validation failure; result = %+v", result)
				}
				if !strings.Contains(err.Error(), tt.wantDetail) {
					t.Fatalf("Validate() error = %q, want detail %q", err, tt.wantDetail)
				}
				if tt.maxDetailBytes > 0 {
					detail := strings.TrimPrefix(err.Error(), "xcrun altool validate failed: ")
					if len(detail) > tt.maxDetailBytes {
						t.Fatalf("Validate() detail bytes = %d, want at most %d", len(detail), tt.maxDetailBytes)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("Validate() error: %v", err)
			}
			if result == nil || !result.Validated {
				t.Fatalf("Validate() result = %+v, want validated success", result)
			}
		})
	}
}

func TestValidatePreservesClassifiedAltoolOutputWithNonZeroExit(t *testing.T) {
	tests := []struct {
		name        string
		stdout      string
		output      string
		exitCode    string
		wantDetails []string
	}{
		{
			name:        "legacy error before long upload noise",
			output:      "*** Error: Unable to validate archive './artifacts/Demo.ipa'.\n" + strings.Repeat("x", xcodebuildErrorTailLimit+1) + "\n",
			exitCode:    "1",
			wantDetails: []string{"Unable to validate archive './artifacts/Demo.ipa'."},
		},
		{
			name:        "timestamped error on stdout before long upload noise",
			stdout:      "2026-08-10 06:47:56.580 ERROR: [ContentDelivery.Uploader] Early server rejection.\n",
			output:      strings.Repeat("x", xcodebuildErrorTailLimit+1) + "\n",
			exitCode:    "1",
			wantDetails: []string{"[ContentDelivery.Uploader] Early server rejection."},
		},
		{
			name:        "unclassified failure still reports the output tail",
			output:      "altool exited unexpectedly\n",
			exitCode:    "70",
			wantDetails: []string{"altool exited unexpectedly"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tempDir := t.TempDir()
			ipaPath := filepath.Join(tempDir, "Demo.ipa")
			if err := writeTestIPA(ipaPath); err != nil {
				t.Fatalf("writeTestIPA() error: %v", err)
			}
			logPath := filepath.Join(tempDir, "commands.log")
			t.Setenv("ASC_XCODE_HELPER_VALIDATE_STDOUT", test.stdout)
			t.Setenv("ASC_XCODE_HELPER_VALIDATE_OUTPUT", test.output)
			t.Setenv("ASC_XCODE_HELPER_VALIDATE_EXIT_CODE", test.exitCode)

			restore := overrideTestEnvironment(t)
			runtimeGOOS = "darwin"
			lookPathFn = func(file string) (string, error) {
				return "/usr/bin/" + file, nil
			}
			commandContextFn = helperCommandContext(t, logPath)
			t.Cleanup(restore)

			result, err := Validate(context.Background(), ValidateOptions{IPAPath: ipaPath})
			if err == nil {
				t.Fatalf("Validate() error = nil, want altool failure; result = %+v", result)
			}
			for _, want := range test.wantDetails {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("Validate() error = %q, want recognized detail %q", truncateErrorForLog(err), want)
				}
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("Validate() error = %T %v, want the altool exit status preserved", err, truncateErrorForLog(err))
			}
			if got := fmt.Sprintf("%d", exitErr.ExitCode()); got != test.exitCode {
				t.Fatalf("altool exit code = %s, want %s", got, test.exitCode)
			}
			if maxBytes := xcodebuildErrorTailLimit + 512; len(err.Error()) > maxBytes {
				t.Fatalf("Validate() error bytes = %d, want at most %d", len(err.Error()), maxBytes)
			}
		})
	}
}

func truncateErrorForLog(err error) string {
	const limit = 512
	message := err.Error()
	if len(message) <= limit {
		return message
	}
	return message[:limit] + "…"
}

func TestValidateRejectsOversizedInfoPlistBeforeAltool(t *testing.T) {
	tempDir := t.TempDir()
	ipaPath := writeIPAWithRawInfoPlist(t, buildSizedAppInfoPlist(t, infoplist.MaxBytes+1))
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	_, err := Validate(context.Background(), ValidateOptions{IPAPath: ipaPath})
	if err == nil {
		t.Fatal("expected oversized Info.plist rejection, got nil")
	}
	if !strings.Contains(err.Error(), "Info.plist limit") {
		t.Fatalf("expected Info.plist limit error, got %v", err)
	}
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if strings.Contains(string(logData), "--validate-app") {
		t.Fatalf("altool must not run after metadata rejection, got %q", string(logData))
	}
}

func TestBuildStatusRunsAltoolWithLookupFlags(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey_TEST12345.p8")
	if err := os.WriteFile(keyPath, []byte("test-key"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		switch file {
		case "xcodebuild":
			return "/usr/bin/xcodebuild", nil
		case "xcrun":
			return "/usr/bin/xcrun", nil
		default:
			return "", exec.ErrNotFound
		}
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	result, err := BuildStatus(context.Background(), BuildStatusOptions{
		AppleID:            "6747745091",
		BundleID:           "com.example.demo",
		BundleVersion:      "2026031905",
		BundleShortVersion: "1.2.3",
		Platform:           "IOS",
		APIKey:             "KEY123ABC",
		APIIssuer:          "issuer-123",
		P8FilePath:         keyPath,
	})
	if err != nil {
		t.Fatalf("BuildStatus() error: %v", err)
	}
	if result.BuildStatus != "FAILED" {
		t.Fatalf("expected build status FAILED, got %q", result.BuildStatus)
	}
	if len(result.ProcessingErrors) != 1 || result.ProcessingErrors[0] != "Invalid Siri Support. App Intent description cannot contain apple. (90626)" {
		t.Fatalf("expected parsed processing errors, got %+v", result.ProcessingErrors)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 logged commands, got %d: %q", len(lines), string(logData))
	}
	if lines[0] != "xcodebuild|-version" {
		t.Fatalf("expected version probe, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "xcrun|altool|--build-status|--apple-id|6747745091|--bundle-version|2026031905|--platform|ios|--output-format|json|--bundle-id|com.example.demo|--bundle-short-version-string|1.2.3|--apiKey|KEY123ABC|--apiIssuer|issuer-123|--p8-file-path|"+keyPath) {
		t.Fatalf("expected build-status invocation with lookup flags, got %q", lines[1])
	}
}

func TestParseBuildStatusOutputPrefersJSONPayload(t *testing.T) {
	result := parseBuildStatusOutput(`
		Running altool at path '/Applications/Xcode.app/.../altool'...
		{"buildStatus":"FAILED","deliveryUUID":"delivery-1","processingErrors":[{"code":"90626","description":"Invalid Siri Support. App Intent description cannot contain apple. (90626)"},{"description":"Extra JSON processing detail"}],"importStatus":"COMPLETE"}
	`)

	if result.BuildStatus != "FAILED" {
		t.Fatalf("expected build status FAILED, got %q", result.BuildStatus)
	}
	if result.DeliveryUUID != "delivery-1" {
		t.Fatalf("expected delivery UUID delivery-1, got %q", result.DeliveryUUID)
	}
	if result.ImportStatus != "COMPLETE" {
		t.Fatalf("expected import status COMPLETE, got %q", result.ImportStatus)
	}
	if len(result.ProcessingErrors) != 2 {
		t.Fatalf("expected 2 processing errors, got %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[0] != "Invalid Siri Support. App Intent description cannot contain apple. (90626)" {
		t.Fatalf("unexpected first processing error: %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[1] != "Extra JSON processing detail" {
		t.Fatalf("unexpected second processing error: %+v", result.ProcessingErrors)
	}
}

func TestParseBuildStatusOutputCollectsProcessingErrors(t *testing.T) {
	result := parseBuildStatusOutput(`
		2026-03-19 11:11:11.111 altool[12345:67890] =======================================
		BUILD-STATUS: FAILED
		DELIVERY-UUID: delivery-1
		PROCESSING-ERRORS:
		server_warning : Keep-alive warning
		code : 90626
		description : Invalid Siri Support. App Intent description cannot contain apple. (90626)
		Extra plain-text processing detail
		IMPORT-STATUS: COMPLETE
	`)

	if result.BuildStatus != "FAILED" {
		t.Fatalf("expected build status FAILED, got %q", result.BuildStatus)
	}
	if result.DeliveryUUID != "delivery-1" {
		t.Fatalf("expected delivery UUID delivery-1, got %q", result.DeliveryUUID)
	}
	if result.ImportStatus != "COMPLETE" {
		t.Fatalf("expected import status COMPLETE, got %q", result.ImportStatus)
	}
	if len(result.ProcessingErrors) != 2 {
		t.Fatalf("expected 2 processing errors, got %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[0] != "Invalid Siri Support. App Intent description cannot contain apple. (90626)" {
		t.Fatalf("unexpected first processing error: %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[1] != "Extra plain-text processing detail" {
		t.Fatalf("unexpected second processing error: %+v", result.ProcessingErrors)
	}
}

func TestParseBuildStatusOutputKeepsProcessingErrorsAfterUppercaseMetadata(t *testing.T) {
	result := parseBuildStatusOutput(`
		BUILD-STATUS: FAILED
		PROCESSING-ERRORS:
		SERVER-WARNING: Keep-alive warning
		ERROR: Validation details follow
		description : Invalid Siri Support. App Intent description cannot contain apple. (90626)
		IMPORT-STATUS: COMPLETE
	`)

	if result.BuildStatus != "FAILED" {
		t.Fatalf("expected build status FAILED, got %q", result.BuildStatus)
	}
	if result.ImportStatus != "COMPLETE" {
		t.Fatalf("expected import status COMPLETE, got %q", result.ImportStatus)
	}
	if len(result.ProcessingErrors) != 2 {
		t.Fatalf("expected 2 processing errors, got %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[0] != "ERROR: Validation details follow" {
		t.Fatalf("unexpected first processing error: %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[1] != "Invalid Siri Support. App Intent description cannot contain apple. (90626)" {
		t.Fatalf("unexpected second processing error: %+v", result.ProcessingErrors)
	}
}

func TestParseBuildStatusOutputHandlesLongProcessingErrorLines(t *testing.T) {
	longDetail := strings.Repeat("x", 70*1024)
	result := parseBuildStatusOutput(
		"BUILD-STATUS: FAILED\n" +
			"PROCESSING-ERRORS:\n" +
			"description : " + longDetail + "\n" +
			"IMPORT-STATUS: COMPLETE\n",
	)

	if result.BuildStatus != "FAILED" {
		t.Fatalf("expected build status FAILED, got %q", result.BuildStatus)
	}
	if result.ImportStatus != "COMPLETE" {
		t.Fatalf("expected import status COMPLETE, got %q", result.ImportStatus)
	}
	if len(result.ProcessingErrors) != 1 {
		t.Fatalf("expected 1 processing error, got %+v", result.ProcessingErrors)
	}
	if result.ProcessingErrors[0] != longDetail {
		t.Fatalf("expected long processing detail to survive parsing, got %d bytes", len(result.ProcessingErrors[0]))
	}
}

func TestSupportsBuildStatusBundleIDUsesHelpOutput(t *testing.T) {
	previous := altoolHelpOutputFn
	altoolHelpOutputFn = func(context.Context) (string, error) {
		return "altool --build-status --bundle-id <id>\n", nil
	}
	t.Cleanup(func() {
		altoolHelpOutputFn = previous
	})

	if !SupportsBuildStatusBundleID(context.Background()) {
		t.Fatal("expected build-status bundle-id support to be detected")
	}
}

func TestExportWritesIPAAtExactPathAndReturnsMetadata(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	if err := os.WriteFile(exportOptionsPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	ipaPath := filepath.Join(tempDir, "artifacts", "Demo.ipa")
	result, err := Export(context.Background(), ExportOptions{
		ArchivePath:    archivePath,
		ExportOptions:  exportOptionsPath,
		IPAPath:        ipaPath,
		Overwrite:      false,
		XcodebuildArgs: []string{"-allowProvisioningUpdates"},
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	if result.ArchivePath != archivePath {
		t.Fatalf("expected archive path %q, got %q", archivePath, result.ArchivePath)
	}
	if result.IPAPath != ipaPath {
		t.Fatalf("expected ipa path %q, got %q", ipaPath, result.IPAPath)
	}
	if result.BundleID != "com.example.demo" {
		t.Fatalf("expected bundle id %q, got %q", "com.example.demo", result.BundleID)
	}
	if result.Version != "1.2.3" {
		t.Fatalf("expected version %q, got %q", "1.2.3", result.Version)
	}
	if result.BuildNumber != "42" {
		t.Fatalf("expected build number %q, got %q", "42", result.BuildNumber)
	}

	if _, err := os.Stat(ipaPath); err != nil {
		t.Fatalf("expected IPA at exact path: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 logged commands, got %d: %q", len(lines), string(logData))
	}
	if lines[0] != "xcodebuild|-version" {
		t.Fatalf("expected version probe, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "|-exportArchive|") {
		t.Fatalf("expected export invocation, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "|-allowProvisioningUpdates") {
		t.Fatalf("expected custom xcodebuild arg, got %q", lines[1])
	}
}

func TestExportValidatesGeneratedIPABeforeReplacingDestination(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	if err := os.WriteFile(exportOptionsPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)
	t.Setenv("ASC_XCODE_HELPER_INVALID_IPA", "1")

	ipaPath := filepath.Join(tempDir, "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("existing ipa"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       ipaPath,
		Overwrite:     true,
	})
	if err == nil {
		t.Fatal("expected generated IPA metadata rejection, got nil")
	}
	if !strings.Contains(err.Error(), "inspect exported IPA before installation") {
		t.Fatalf("expected pre-install metadata error, got %v", err)
	}
	data, readErr := os.ReadFile(ipaPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if string(data) != "existing ipa" {
		t.Fatalf("expected existing IPA to survive failed export, got %q", data)
	}
}

func TestMoveExportedIPAAtomicallyReplacesExistingDestination(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "generated.ipa")
	destination := filepath.Join(directory, "release.ipa")
	if err := os.WriteFile(source, []byte("new ipa"), 0o644); err != nil {
		t.Fatalf("WriteFile(source) error: %v", err)
	}
	if err := os.WriteFile(destination, []byte("old ipa"), 0o644); err != nil {
		t.Fatalf("WriteFile(destination) error: %v", err)
	}

	if err := moveExportedIPA(source, destination, true); err != nil {
		t.Fatalf("moveExportedIPA() error: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "new ipa" {
		t.Fatalf("expected replacement IPA, got %q", data)
	}
}

func TestExportDirectUploadPreservesExistingIPAAndReturnsArchiveMetadata(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	writeExportOptionsPlist(t, exportOptionsPath, map[string]any{"destination": "upload"})
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	ipaPath := filepath.Join(tempDir, "artifacts", "Demo.ipa")
	if err := os.MkdirAll(filepath.Dir(ipaPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(ipaPath, []byte("existing ipa"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	result, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       ipaPath,
		Overwrite:     true,
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if result.ArchivePath != archivePath {
		t.Fatalf("expected archive path %q, got %q", archivePath, result.ArchivePath)
	}
	if result.IPAPath != "" {
		t.Fatalf("expected no local ipa path for direct upload, got %q", result.IPAPath)
	}
	if result.BundleID != "com.example.demo" {
		t.Fatalf("expected bundle id %q, got %q", "com.example.demo", result.BundleID)
	}
	if result.Version != "1.2.3" {
		t.Fatalf("expected version %q, got %q", "1.2.3", result.Version)
	}
	if result.BuildNumber != "42" {
		t.Fatalf("expected build number %q, got %q", "42", result.BuildNumber)
	}

	data, err := os.ReadFile(ipaPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "existing ipa" {
		t.Fatalf("expected existing IPA to be preserved, got %q", string(data))
	}
}

func TestExportDirectUploadCreatesIPAParentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	writeExportOptionsPlist(t, exportOptionsPath, map[string]any{"destination": "upload"})
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	ipaPath := filepath.Join(tempDir, "nested", "output", "Demo.ipa")
	result, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       ipaPath,
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if result.IPAPath != "" {
		t.Fatalf("expected no local ipa path for direct upload, got %q", result.IPAPath)
	}
	if _, err := os.Stat(filepath.Dir(ipaPath)); err != nil {
		t.Fatalf("expected IPA parent directory to exist, got %v", err)
	}
	if _, err := os.Stat(ipaPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no IPA artifact to be written, got %v", err)
	}
}

func TestExportDirectUploadReturnsArchiveInfoErrors(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	writeExportOptionsPlist(t, exportOptionsPath, map[string]any{"destination": "upload"})
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       filepath.Join(tempDir, "Demo.ipa"),
	})
	if err == nil {
		t.Fatal("expected archive metadata error, got nil")
	}
	if !strings.Contains(err.Error(), "read archive bundle info after direct upload") {
		t.Fatalf("expected archive bundle info error, got %v", err)
	}
}

func TestExportWarnsForBetaXcodeAppStoreExport(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	writeExportOptionsPlist(t, exportOptionsPath, map[string]any{
		"destination":  "upload",
		"method":       "app-store-connect",
		"signingStyle": "automatic",
	})
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Setenv("DEVELOPER_DIR", "/Applications/Xcode-beta.app/Contents/Developer")
	t.Cleanup(restore)

	var stderr bytes.Buffer
	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       filepath.Join(tempDir, "Demo.ipa"),
		LogWriter:     &stderr,
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if !strings.Contains(stderr.String(), `Warning: active Xcode developer directory "/Applications/Xcode-beta.app/Contents/Developer" appears to be a beta build`) {
		t.Fatalf("expected beta Xcode warning, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "App Store review can later reject builds for unsupported SDK/Xcode") {
		t.Fatalf("expected warning to explain App Store review risk, got %q", stderr.String())
	}
}

func TestExportDoesNotWarnForStableXcodeAppStoreExport(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	writeExportOptionsPlist(t, exportOptionsPath, map[string]any{
		"destination":  "upload",
		"method":       "app-store-connect",
		"signingStyle": "automatic",
	})
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Setenv("DEVELOPER_DIR", "/Applications/Xcode-26.3.0.app/Contents/Developer")
	t.Cleanup(restore)

	var stderr bytes.Buffer
	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       filepath.Join(tempDir, "Demo.ipa"),
		LogWriter:     &stderr,
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if strings.Contains(stderr.String(), "beta build") {
		t.Fatalf("did not expect beta Xcode warning, got %q", stderr.String())
	}
}

func TestExportDoesNotWarnForBetaXcodeDevelopmentExport(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	writeExportOptionsPlist(t, exportOptionsPath, map[string]any{
		"method":       "development",
		"signingStyle": "automatic",
	})
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, logPath)
	t.Setenv("DEVELOPER_DIR", "/Applications/Xcode-beta.app/Contents/Developer")
	t.Cleanup(restore)

	var stderr bytes.Buffer
	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       filepath.Join(tempDir, "Demo.ipa"),
		LogWriter:     &stderr,
	})
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if strings.Contains(stderr.String(), "beta build") {
		t.Fatalf("did not expect beta Xcode warning, got %q", stderr.String())
	}
}

func TestInferArchivePlatformReadsEmbeddedAppInfo(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
	if err := os.MkdirAll(filepath.Dir(appInfoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	data, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo",
		"DTPlatformName":     "appletvos",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}
	if err := os.WriteFile(appInfoPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	platform, err := InferArchivePlatform(archivePath)
	if err != nil {
		t.Fatalf("InferArchivePlatform() error: %v", err)
	}
	if platform != "TV_OS" {
		t.Fatalf("expected TV_OS, got %q", platform)
	}
}

func TestInferArchivePlatformReadsEmbeddedMacAppInfo(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Contents", "Info.plist")
	if err := os.MkdirAll(filepath.Dir(appInfoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	data, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo",
		"DTPlatformName":     "macosx",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}
	if err := os.WriteFile(appInfoPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	platform, err := InferArchivePlatform(archivePath)
	if err != nil {
		t.Fatalf("InferArchivePlatform() error: %v", err)
	}
	if platform != "MAC_OS" {
		t.Fatalf("expected MAC_OS, got %q", platform)
	}
}

func TestInferArchivePlatformReadsEmbeddedWatchAppInfo(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := writeArchiveInfoPlist(archivePath); err != nil {
		t.Fatalf("writeArchiveInfoPlist() error: %v", err)
	}
	appInfoPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app", "Info.plist")
	if err := os.MkdirAll(filepath.Dir(appInfoPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	data, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier": "com.example.demo",
		"DTPlatformName":     "watchos",
	}, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}
	if err := os.WriteFile(appInfoPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	platform, err := InferArchivePlatform(archivePath)
	if err != nil {
		t.Fatalf("InferArchivePlatform() error: %v", err)
	}
	if platform != "IOS" {
		t.Fatalf("expected IOS for standalone watchOS app, got %q", platform)
	}
}

func TestExportRejectsExistingIPAWithoutOverwrite(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "Demo.xcarchive")
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	exportOptionsPath := filepath.Join(tempDir, "ExportOptions.plist")
	if err := os.WriteFile(exportOptionsPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	ipaPath := filepath.Join(tempDir, "Demo.ipa")
	if err := os.WriteFile(ipaPath, []byte("existing"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	commandContextFn = helperCommandContext(t, filepath.Join(tempDir, "commands.log"))
	t.Cleanup(restore)

	_, err := Export(context.Background(), ExportOptions{
		ArchivePath:   archivePath,
		ExportOptions: exportOptionsPath,
		IPAPath:       ipaPath,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--ipa-path already exists") {
		t.Fatalf("expected existing ipa error, got %v", err)
	}
}

func TestRunXcodebuildFailurePreservesRecognizedErrorsAndFinalOutputWithinBound(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	tests := []struct {
		name                 string
		action               string
		preserveProcessError bool
	}{
		{name: "build", action: "build", preserveProcessError: true},
		{name: "archive", action: "archive"},
		{name: "export", action: "export"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := runCommandWithBoundedOutputMode(
				context.Background(),
				"xcodebuild",
				[]string{"fail-large-output"},
				nil,
				test.action,
				"xcodebuild",
				test.preserveProcessError,
			)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errorText := err.Error()
			for _, want := range []string{
				"file.m:4:3: error: root cause",
				"FINAL-DIAGNOSTIC",
				"output truncated to 65536 bytes; preserving recognized errors and final output",
			} {
				if !strings.Contains(errorText, want) {
					t.Fatalf("error = %q, want %q", errorText, want)
				}
			}

			var exitErr *exec.ExitError
			if got := errors.As(err, &exitErr); got != test.preserveProcessError {
				t.Fatalf("errors.As(*exec.ExitError) = %t, want %t; error = %v", got, test.preserveProcessError, err)
			}
		})
	}
}

func TestRunXcodebuildFailureStillStreamsCompleteOutputOnce(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "commands.log")

	restore := overrideTestEnvironment(t)
	commandContextFn = helperCommandContext(t, logPath)
	t.Cleanup(restore)

	var streamed bytes.Buffer
	err := runXcodebuild(context.Background(), []string{"fail-large-output"}, &streamed)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	streamedOutput := streamed.String()
	for _, want := range []string{"file.m:4:3: error: root cause", "FINAL-DIAGNOSTIC"} {
		if got := strings.Count(streamedOutput, want); got != 1 {
			t.Fatalf("streamed diagnostic %q count = %d, want 1", want, got)
		}
	}

	errorText := err.Error()
	for _, want := range []string{"file.m:4:3: error: root cause", "FINAL-DIAGNOSTIC"} {
		if !strings.Contains(errorText, want) {
			t.Fatalf("error = %q, want %q", errorText, want)
		}
	}
	if strings.Contains(errorText, "exit status 1") {
		t.Fatalf("legacy archive/export runner error text changed: %v", err)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		t.Fatalf("legacy archive/export runner unexpectedly exposes process error: %v", err)
	}
}

func TestRunXcodebuildInterruptionPreservesRecognizedErrorsAndFinalOutputWithinBound(t *testing.T) {
	tempDir := t.TempDir()
	restore := overrideTestEnvironment(t)
	commandContextFn = helperCommandContext(t, filepath.Join(tempDir, "commands.log"))
	t.Cleanup(restore)

	tests := []struct {
		name            string
		timeout         time.Duration
		cancelWhenReady bool
		wantCause       error
	}{
		{
			name:      "deadline",
			timeout:   2 * time.Second,
			wantCause: context.DeadlineExceeded,
		},
		{
			name:            "cancellation",
			cancelWhenReady: true,
			wantCause:       context.Canceled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			if test.timeout > 0 {
				ctx, cancel = context.WithTimeout(context.Background(), test.timeout)
			}
			defer cancel()

			readyPath := filepath.Join(tempDir, test.name+".ready")
			var readyResult <-chan error
			if test.cancelWhenReady {
				result := make(chan error, 1)
				readyResult = result
				go func() {
					err := waitForHelperSignal(readyPath, 2*time.Second)
					if err == nil {
						cancel()
					}
					result <- err
				}()
			}

			err := runXcodebuild(ctx, []string{"large-output-then-wait", readyPath}, nil)
			if readyResult != nil {
				if readyErr := <-readyResult; readyErr != nil {
					t.Fatalf("wait for helper readiness: %v", readyErr)
				}
			}
			if !errors.Is(err, test.wantCause) {
				t.Fatalf("runXcodebuild() error = %v, want %v", err, test.wantCause)
			}
			if _, statErr := os.Stat(readyPath); statErr != nil {
				t.Fatalf("helper readiness signal: %v", statErr)
			}
			for _, want := range []string{
				"file.m:4:3: error: root cause",
				"FINAL-BEFORE-INTERRUPTION",
				"output truncated to 65536 bytes; preserving recognized errors and final output",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error = %q, want %q", err, want)
				}
			}
		})
	}
}

func waitForHelperSignal(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for %s", path)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestTailBufferStringRemainsValidUTF8AfterByteTruncation(t *testing.T) {
	buffer := newTailBuffer(4)
	if _, err := buffer.Write([]byte("界界")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got := buffer.String()
	if !utf8.ValidString(got) {
		t.Fatalf("String() returned invalid UTF-8 after truncation: %q", got)
	}
	if !strings.Contains(got, "界") {
		t.Fatalf("String() = %q, want intact trailing diagnostic", got)
	}
}

func renderXcodeDiagnosticOutput(t *testing.T, input string) (*xcodeDiagnosticBuffer, string) {
	t.Helper()
	buffer := newXcodeDiagnosticBuffer(xcodebuildErrorTailLimit, nil)
	if _, err := io.WriteString(buffer, input); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	return buffer, buffer.String()
}

func requireBoundedXcodeDiagnosticOutput(t *testing.T, output string, values ...string) {
	t.Helper()
	if len(output) > xcodebuildErrorTailLimit || !utf8.ValidString(output) {
		t.Fatalf("rendered output bytes = %d, valid UTF-8 = %t", len(output), utf8.ValidString(output))
	}
	for _, value := range values {
		if !strings.Contains(output, value) {
			t.Fatalf("String() dropped %q", value)
		}
	}
}

func countExactOutputLines(output string, value string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == value {
			count++
		}
	}
	return count
}

func TestXcodeDiagnosticBufferPreservesUntruncatedOutput(t *testing.T) {
	for _, chunks := range [][]string{{"ab", "cd", "ef"}, {"abcd", "efgh"}} {
		buffer := newXcodeDiagnosticBuffer(8, nil)
		for _, chunk := range chunks {
			if _, err := io.WriteString(buffer, chunk); err != nil {
				t.Fatalf("Write(%q) error = %v", chunk, err)
			}
		}

		want := strings.Join(chunks, "")
		if buffer.Truncated() {
			t.Fatalf("Truncated() = true for %q, want false", want)
		}
		if got := buffer.String(); got != want {
			t.Fatalf("String() = %q, want %q", got, want)
		}
	}
}

func TestXcodeDiagnosticBufferPreservesMiddleCompilerErrorFromLegacyTail(t *testing.T) {
	const totalBytes = 90 * 1024
	diagnostic := "file.m:4:3: error: middle root cause"
	input := diagnostic + "\n"
	input += strings.Repeat("h", 40*1024-len(input)) + "\n" + diagnostic + "\n"
	input += strings.Repeat("t", totalBytes-len(input))

	legacyTail := newTailBuffer(xcodebuildErrorTailLimit)
	if _, err := legacyTail.Write([]byte(input)); err != nil {
		t.Fatalf("legacy tail Write() error = %v", err)
	}
	if !strings.Contains(legacyTail.String(), diagnostic) {
		t.Fatal("test precondition failed: legacy 64 KiB tail does not contain middle diagnostic")
	}

	_, got := renderXcodeDiagnosticOutput(t, input)
	if !strings.Contains(got, diagnostic) {
		t.Fatalf("String() dropped middle compiler diagnostic retained by legacy tail")
	}
	if got != legacyTail.String() {
		t.Fatalf("String() did not preserve legacy tail when diagnostic was already present")
	}
	if count := countExactOutputLines(got, diagnostic); count != 1 {
		t.Fatalf("diagnostic line count = %d, want 1", count)
	}
	requireBoundedXcodeDiagnosticOutput(t, got)
}

func TestXcodeDiagnosticBufferRenderedOutputStaysWithinLimit(t *testing.T) {
	input := "file.m:4:3: error: root cause\n" +
		strings.Repeat("界", xcodebuildErrorTailLimit) +
		"\nFINAL-DIAGNOSTIC\n"
	_, got := renderXcodeDiagnosticOutput(t, input)
	requireBoundedXcodeDiagnosticOutput(t, got, "file.m:4:3: error: root cause", "FINAL-DIAGNOSTIC")
}

func TestXcodeDiagnosticBufferRetainsMissingErrorsOnce(t *testing.T) {
	diagnostic := "/tmp/App/Assets.xcassets: error: App icon set missing"
	benign := "Build summary mentions error: only as prose"
	input := diagnostic + "\n" + diagnostic + "\n" + benign + "\n"
	input += strings.Repeat("x", xcodebuildErrorTailLimit+128) + "\nFINAL-DIAGNOSTIC\n"

	_, got := renderXcodeDiagnosticOutput(t, input)
	if count := countExactOutputLines(got, diagnostic); count != 1 {
		t.Fatalf("diagnostic count = %d, want 1", count)
	}
	if strings.Contains(got, benign) {
		t.Fatalf("String() retained benign prose as a diagnostic: %q", got)
	}
	requireBoundedXcodeDiagnosticOutput(t, got, "FINAL-DIAGNOSTIC")
}

func TestXcodeDiagnosticBufferRetainsDiagnosticDisplacedByMissingPrefix(t *testing.T) {
	missingDiagnostic := "error: " + strings.Repeat("a", 1024)
	displacedDiagnostic := "error: displaced from the start of the legacy tail"
	tail := "\n" + displacedDiagnostic + "\n"
	tail += strings.Repeat("x", xcodebuildErrorTailLimit-len(tail))
	input := missingDiagnostic + "\n" + strings.Repeat("o", 128) + tail

	legacyTail := newTailBuffer(xcodebuildErrorTailLimit)
	if _, err := legacyTail.Write([]byte(input)); err != nil {
		t.Fatalf("legacy tail Write() error = %v", err)
	}
	if !strings.Contains(legacyTail.String(), displacedDiagnostic) {
		t.Fatal("test precondition failed: displaced diagnostic is absent from the legacy tail")
	}
	if strings.Contains(legacyTail.String(), missingDiagnostic) {
		t.Fatal("test precondition failed: missing diagnostic is present in the legacy tail")
	}

	_, got := renderXcodeDiagnosticOutput(t, input)
	requireBoundedXcodeDiagnosticOutput(t, got, missingDiagnostic, displacedDiagnostic)
}

func TestXcodeDiagnosticBufferRetainsUncollectedTailBoundaryError(t *testing.T) {
	var input strings.Builder
	expectedDiagnostics := make(map[string]struct{})
	for i := range 1000 {
		diagnostic := fmt.Sprintf("error: early failure %04d", i)
		expectedDiagnostics[diagnostic] = struct{}{}
		input.WriteString(diagnostic + "\n")
	}
	boundaryDiagnostic := "error: actionable tail-boundary failure"
	expectedDiagnostics[boundaryDiagnostic] = struct{}{}
	input.WriteString(boundaryDiagnostic + "\n")
	input.WriteString(strings.Repeat("x", xcodebuildErrorTailLimit-len(boundaryDiagnostic)-1))

	legacyTail := newTailBuffer(xcodebuildErrorTailLimit)
	if _, err := io.WriteString(legacyTail, input.String()); err != nil {
		t.Fatalf("legacy tail Write() error = %v", err)
	}
	if !strings.Contains(legacyTail.String(), boundaryDiagnostic) {
		t.Fatal("test precondition failed: boundary diagnostic is absent from the legacy tail")
	}

	_, got := renderXcodeDiagnosticOutput(t, input.String())
	requireBoundedXcodeDiagnosticOutput(t, got, "error: early failure 0000", boundaryDiagnostic)
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "error:") {
			if _, exists := expectedDiagnostics[line]; !exists {
				t.Fatalf("String() emitted partial diagnostic line prefix %q", truncateUTF8Prefix(line, 120))
			}
		}
	}
}

func TestXcodeDiagnosticBufferDoesNotDeduplicateAgainstBenignTailSubstring(t *testing.T) {
	diagnostic := "error: root cause"
	benign := "note: previous error: root cause was discussed"
	input := diagnostic + "\n" + strings.Repeat("x", xcodebuildErrorTailLimit+128)
	input += "\n" + benign + "\nFINAL-DIAGNOSTIC\n"
	_, got := renderXcodeDiagnosticOutput(t, input)
	if exactCount := countExactOutputLines(got, diagnostic); exactCount != 1 {
		t.Fatalf("exact diagnostic line count = %d, want 1", exactCount)
	}
	if !strings.Contains(got, benign) {
		t.Fatalf("String() dropped benign final output")
	}
}

func TestXcodeDiagnosticBufferDoesNotLetVersionedProseExhaustDiagnosticBudget(t *testing.T) {
	var input strings.Builder
	for i := range 1000 {
		fmt.Fprintf(&input, "Build 1.%d: error: only prose\n", i)
	}
	diagnostic := "error: actual root cause"
	input.WriteString(diagnostic + "\n")
	input.WriteString(strings.Repeat("x", xcodebuildErrorTailLimit+128))
	input.WriteString("\nFINAL-DIAGNOSTIC\n")
	_, got := renderXcodeDiagnosticOutput(t, input.String())
	if !strings.Contains(got, diagnostic) {
		t.Fatalf("String() dropped real diagnostic after versioned prose")
	}
	if strings.Contains(got, "error: only prose") {
		t.Fatalf("String() retained versioned prose as diagnostics")
	}
}

func TestXcodeDiagnosticBufferIndexesDenseTailOnce(t *testing.T) {
	var input strings.Builder
	for i := range 32 {
		fmt.Fprintf(&input, "error: early failure %02d\n", i)
	}
	input.WriteString(strings.Repeat("\n", xcodebuildErrorTailLimit+128))
	buffer, _ := renderXcodeDiagnosticOutput(t, input.String())

	if allocations := testing.AllocsPerRun(1, func() {
		_ = buffer.String()
	}); allocations > 16 {
		t.Fatalf("String() allocations = %.0f, want at most 16", allocations)
	}
}

func TestXcodeDiagnosticBufferBoundsVeryLongErrorLine(t *testing.T) {
	longDiagnostic := "error: " + strings.Repeat("界", xcodeDiagnosticLineLimit)
	input := longDiagnostic + strings.Repeat("x", xcodebuildErrorTailLimit+128)
	buffer, got := renderXcodeDiagnosticOutput(t, input)
	if !strings.Contains(got, "error: ") {
		t.Fatalf("String() dropped bounded long diagnostic")
	}
	requireBoundedXcodeDiagnosticOutput(t, got)
	if buffer.diagnosticBytes > buffer.diagnosticBudget() {
		t.Fatalf("stored diagnostic bytes = %d, budget = %d", buffer.diagnosticBytes, buffer.diagnosticBudget())
	}
	for _, diagnostic := range buffer.diagnostics {
		if len(diagnostic) > xcodeDiagnosticLineLimit {
			t.Fatalf("stored diagnostic bytes = %d, line limit = %d", len(diagnostic), xcodeDiagnosticLineLimit)
		}
	}
}

func TestXcodeDiagnosticBufferBoundsManyUniqueErrors(t *testing.T) {
	var input strings.Builder
	for i := range 5000 {
		fmt.Fprintf(&input, "error: distinct failure %04d\n", i)
	}
	input.WriteString(strings.Repeat("x", xcodebuildErrorTailLimit+128))
	input.WriteString("\nFINAL-DIAGNOSTIC\n")
	buffer, got := renderXcodeDiagnosticOutput(t, input.String())
	requireBoundedXcodeDiagnosticOutput(t, got, "error: distinct failure 0000", "FINAL-DIAGNOSTIC")
	if buffer.diagnosticBytes > buffer.diagnosticBudget() {
		t.Fatalf("stored diagnostic bytes = %d, budget = %d", buffer.diagnosticBytes, buffer.diagnosticBudget())
	}
}

func TestXcodeDiagnosticBufferRecognizesDiagnosticSplitAcrossWrites(t *testing.T) {
	buffer := newXcodeDiagnosticBuffer(xcodebuildErrorTailLimit, nil)
	for _, chunk := range []string{
		"Sources/App.swift:12:7: er",
		"ror: split root cause",
		"\n",
		strings.Repeat("x", xcodebuildErrorTailLimit+128),
	} {
		if _, err := io.WriteString(buffer, chunk); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	got := buffer.String()
	if !strings.Contains(got, "Sources/App.swift:12:7: error: split root cause") {
		t.Fatalf("String() dropped diagnostic split across writes")
	}
}

func TestXcodeDiagnosticBufferSerializesConcurrentWrites(t *testing.T) {
	var streamed bytes.Buffer
	buffer := newXcodeDiagnosticBuffer(xcodebuildErrorTailLimit, &streamed)

	var writers sync.WaitGroup
	for range 2 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 100 {
				if _, err := buffer.Write([]byte("ordinary build output\n")); err != nil {
					t.Errorf("Write() error = %v", err)
					return
				}
			}
		}()
	}
	writers.Wait()

	if got := buffer.String(); got != streamed.String() {
		t.Fatalf("captured output differs from streamed output")
	}
}

func TestRunXcodebuildPreservesCombinedChildStreamOrder(t *testing.T) {
	restore := overrideTestEnvironment(t)
	commandContextFn = helperCommandContext(t, filepath.Join(t.TempDir(), "commands.log"))
	t.Cleanup(restore)

	const want = "FIRST-STDOUT\nSECOND-STDERR\nTHIRD-STDOUT\nFINAL-STDERR\n"
	var streamed bytes.Buffer
	if err := runXcodebuild(context.Background(), []string{"alternating-stream-output"}, &streamed); err != nil {
		t.Fatalf("runXcodebuild() error = %v", err)
	}
	if got := streamed.String(); got != want {
		t.Fatalf("streamed output = %q, want child write order %q", got, want)
	}
}

func TestIsXcodeErrorDiagnostic(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{name: "source location", line: "Sources/App.swift:12:7: error: cannot find value", want: true},
		{name: "line leading", line: "error: emit-module command failed", want: true},
		{name: "fatal line leading", line: "fatal error: module map missing", want: true},
		{name: "known tool", line: "clang: error: linker command failed", want: true},
		{name: "xcrun tool", line: "xcrun: error: invalid active developer path", want: true},
		{name: "path only", line: "/tmp/App/Assets.xcassets: error: App icon set missing", want: true},
		{name: "project path only", line: "App.xcodeproj: error: Missing package product", want: true},
		{name: "benign prose", line: "Build summary mentions error: only as prose"},
		{name: "versioned prose", line: "Build 1.2: error: only prose"},
		{name: "quoted note", line: "note: error: appeared in generated documentation"},
		{name: "empty error", line: "error:"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isXcodeErrorDiagnostic(test.line); got != test.want {
				t.Fatalf("isXcodeErrorDiagnostic(%q) = %t, want %t", test.line, got, test.want)
			}
		})
	}
}

func TestRunXcodebuildDoesNotWaitForDescendantHoldingOutputPipes(t *testing.T) {
	tempDir := t.TempDir()
	pidPath := filepath.Join(tempDir, "descendant.pid")

	restore := overrideTestEnvironment(t)
	commandContextFn = helperCommandContext(t, filepath.Join(tempDir, "commands.log"))
	t.Setenv("ASC_XCODE_HELPER_DESCENDANT_PID", pidPath)
	t.Cleanup(restore)
	t.Cleanup(func() {
		data, err := os.ReadFile(pidPath)
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return
		}
		if process, err := os.FindProcess(pid); err == nil {
			_ = process.Kill()
		}
	})

	started := time.Now()
	err := runXcodebuild(context.Background(), []string{"retain-output-after-exit"}, io.Discard)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("runXcodebuild() error = %v", err)
	}
	if elapsed >= time.Second {
		t.Fatalf("runXcodebuild() waited %s for a descendant after the direct process exited", elapsed)
	}
}

func TestRunXcodebuildPreservesContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	restore := overrideTestEnvironment(t)
	commandContextFn = helperCommandContext(t, filepath.Join(tempDir, "commands.log"))
	t.Cleanup(restore)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := runXcodebuild(ctx, []string{"wait-for-context-cancel"}, io.Discard)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runXcodebuild() error = %v, want context deadline exceeded", err)
	}
}

func overrideTestEnvironment(t *testing.T) func() {
	t.Helper()

	originalGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalActiveDeveloperDir := activeDeveloperDirFn
	return func() {
		runtimeGOOS = originalGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		activeDeveloperDirFn = originalActiveDeveloperDir
	}
}

func helperCommandContext(t *testing.T, logPath string) func(context.Context, string, ...string) *exec.Cmd {
	t.Helper()

	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		commandArgs := []string{"-test.run=TestXcodeHelperProcess", "--", name}
		commandArgs = append(commandArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
		cmd.Env = append(
			os.Environ(),
			"GO_WANT_XCODE_HELPER_PROCESS=1",
			"ASC_XCODE_HELPER_LOG="+logPath,
		)
		return cmd
	}
}

func TestXcodeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_XCODE_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	sep := -1
	for i, arg := range args {
		if arg == "--" {
			sep = i
			break
		}
	}
	if sep == -1 || sep+1 >= len(args) {
		fmt.Fprintln(os.Stderr, "missing helper args")
		os.Exit(2)
	}
	commandArgs := args[sep+1:]
	if err := appendHelperLog(os.Getenv("ASC_XCODE_HELPER_LOG"), commandArgs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "-version" {
		fmt.Fprintln(os.Stdout, "Xcode 16.2")
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcrun" && commandArgs[1] == "altool" {
		if helperContainsArg(commandArgs[2:], "--build-status") {
			if _, err := valueAfter(commandArgs[2:], "--apple-id"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if _, err := valueAfter(commandArgs[2:], "--bundle-version"); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			if got, err := valueAfter(commandArgs[2:], "--output-format"); err != nil || got != "json" {
				fmt.Fprintln(os.Stderr, "missing --output-format json")
				os.Exit(2)
			}
			// Modern altool often writes structured and informational output to stderr.
			fmt.Fprint(os.Stderr, `{"buildStatus":"FAILED","deliveryUUID":"delivery-1","processingErrors":[{"code":"90626","description":"Invalid Siri Support. App Intent description cannot contain apple. (90626)"}],"importStatus":"COMPLETE"}`)
			os.Exit(0)
		}
		if !helperContainsArg(commandArgs[2:], "--validate-app") {
			fmt.Fprintln(os.Stderr, "missing --validate-app")
			os.Exit(2)
		}
		if _, err := valueAfter(commandArgs[2:], "--file"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if output := os.Getenv("ASC_XCODE_HELPER_VALIDATE_STDOUT"); output != "" {
			fmt.Fprint(os.Stdout, output)
			time.Sleep(50 * time.Millisecond)
		}
		if output := os.Getenv("ASC_XCODE_HELPER_VALIDATE_OUTPUT"); output != "" {
			fmt.Fprint(os.Stderr, output)
		}
		if code := os.Getenv("ASC_XCODE_HELPER_VALIDATE_EXIT_CODE"); code != "" {
			parsed, err := strconv.Atoi(code)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			os.Exit(parsed)
		}
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "agvtool" {
		switch commandArgs[1] {
		case "what-marketing-version":
			if os.Getenv("ASC_XCODE_HELPER_VARIABLE_VERSION") == "1" {
				fmt.Fprint(os.Stdout, "App=$(MARKETING_VERSION)\n")
				os.Exit(0)
			}
			if os.Getenv("ASC_XCODE_HELPER_DIVERGENT_CONFIGURATIONS") == "1" {
				fmt.Fprint(os.Stdout, "App=1.2.3\nApp=2.0.0\n")
				os.Exit(0)
			}
			if os.Getenv("ASC_XCODE_HELPER_SINGLE_TARGET") == "1" {
				fmt.Fprint(os.Stdout, "App=1.2.3\n")
				os.Exit(0)
			}
			fmt.Fprint(os.Stdout, "App=1.2.3\nExtension=2.0.0\n")
			os.Exit(0)
		case "what-version":
			if os.Getenv("ASC_XCODE_HELPER_SINGLE_TARGET") == "1" {
				fmt.Fprint(os.Stdout, "App=41\n")
				os.Exit(0)
			}
			fmt.Fprint(os.Stdout, "App=41\nExtension=7\n")
			os.Exit(0)
		case "new-marketing-version", "new-version", "next-version":
			os.Exit(0)
		}
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "-showBuildSettings" {
		fmt.Fprint(os.Stdout, "Build settings for action build and target App:\n    MARKETING_VERSION = 4.5.6\n    CURRENT_PROJECT_VERSION = 99\n")
		os.Exit(0)
	}

	if len(commandArgs) >= 1 && commandArgs[0] == "xcodebuild" && helperContainsArg(commandArgs[1:], "archive") {
		archivePath, err := valueAfter(commandArgs[1:], "-archivePath")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := writeArchiveInfoPlist(archivePath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	if len(commandArgs) >= 1 && commandArgs[0] == "xcodebuild" && helperContainsArg(commandArgs[1:], "-exportArchive") {
		exportPath, err := valueAfter(commandArgs[1:], "-exportPath")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		exportOptionsPath, err := valueAfter(commandArgs[1:], "-exportOptionsPlist")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.MkdirAll(exportPath, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if isDirectUploadMode(exportOptionsPath) {
			os.Exit(0)
		}
		if os.Getenv("ASC_XCODE_HELPER_INVALID_IPA") == "1" {
			if err := os.WriteFile(filepath.Join(exportPath, "Exported.ipa"), []byte("not an ipa"), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			os.Exit(0)
		}
		if err := writeTestIPA(filepath.Join(exportPath, "Exported.ipa")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "retain-output-after-exit" {
		descendantArgs := []string{"-test.run=TestXcodeHelperProcess", "--", "xcodebuild", "hold-output-descriptors"}
		descendant := exec.Command(os.Args[0], descendantArgs...)
		descendant.Env = os.Environ()
		descendant.Stdout = os.Stdout
		descendant.Stderr = os.Stderr
		if err := descendant.Start(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := os.WriteFile(os.Getenv("ASC_XCODE_HELPER_DESCENDANT_PID"), []byte(strconv.Itoa(descendant.Process.Pid)), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "hold-output-descriptors" {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "wait-for-context-cancel" {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "fail-large-output" {
		fmt.Fprint(os.Stderr, "file.m:4:3: error: root cause\n")
		fmt.Fprint(os.Stderr, strings.Repeat("x", xcodebuildErrorTailLimit+128))
		fmt.Fprint(os.Stderr, "\nFINAL-DIAGNOSTIC\n")
		os.Exit(1)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "alternating-stream-output" {
		stdoutInfo, stdoutErr := os.Stdout.Stat()
		stderrInfo, stderrErr := os.Stderr.Stat()
		fmt.Fprint(os.Stdout, "FIRST-STDOUT\n")
		fmt.Fprint(os.Stderr, "SECOND-STDERR\n")
		fmt.Fprint(os.Stdout, "THIRD-STDOUT\n")
		fmt.Fprint(os.Stderr, "FINAL-STDERR\n")
		if stdoutErr != nil || stderrErr != nil || !os.SameFile(stdoutInfo, stderrInfo) {
			fmt.Fprintln(os.Stderr, "stdout and stderr do not share one ordered descriptor")
			os.Exit(3)
		}
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "large-output-then-wait" {
		if len(commandArgs) < 3 {
			fmt.Fprintln(os.Stderr, "missing helper readiness path")
			os.Exit(2)
		}
		fmt.Fprint(os.Stderr, "file.m:4:3: error: root cause\n")
		fmt.Fprint(os.Stderr, strings.Repeat("x", xcodebuildErrorTailLimit+128))
		fmt.Fprint(os.Stderr, "\nFINAL-BEFORE-INTERRUPTION\n")
		if err := os.WriteFile(commandArgs[2], []byte("ready"), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "unexpected helper invocation: %v\n", commandArgs)
	os.Exit(2)
}

func appendHelperLog(path string, args []string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, strings.Join(args, "|"))
	return err
}

func helperContainsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func valueAfter(args []string, flagName string) (string, error) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flagName {
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("missing %s", flagName)
}

func writeArchiveInfoPlist(archivePath string) error {
	if err := os.MkdirAll(archivePath, 0o755); err != nil {
		return err
	}
	payload := map[string]any{
		"ApplicationProperties": map[string]any{
			"ApplicationPath":            "Applications/Demo.app",
			"CFBundleIdentifier":         "com.example.demo",
			"CFBundleShortVersionString": "1.2.3",
			"CFBundleVersion":            "42",
		},
	}
	data, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(archivePath, "Info.plist"), data, 0o644)
}

func writeExportOptionsPlist(t *testing.T, path string, payload map[string]any) {
	t.Helper()

	data, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		t.Fatalf("plist.Marshal() error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
}

func writeTestIPA(path string) error {
	return writeTestIPAWithPlatform(path, "iphoneos")
}

func writeTestIPAWithPlatform(path, platform string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	entry, err := writer.Create("Payload/Demo.app/Info.plist")
	if err != nil {
		return err
	}
	payload := map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleShortVersionString": "1.2.3",
		"CFBundleVersion":            "42",
		"CFBundleSupportedPlatforms": []string{platform},
		"DTPlatformName":             platform,
	}
	data, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		return err
	}
	if _, err := entry.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return fmt.Errorf("expected non-empty IPA")
	}
	if !bytes.HasPrefix(content, []byte("PK")) {
		return fmt.Errorf("expected zip archive")
	}
	return nil
}
