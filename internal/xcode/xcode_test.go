package xcode

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
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

func TestRunXcodebuildWithLogWriterKeepsOnlyTailInErrorMessage(t *testing.T) {
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
	if !strings.Contains(streamedOutput, "EARLY-MARKER") {
		t.Fatalf("expected streamed output to include early marker, got %q", streamedOutput)
	}
	if !strings.Contains(streamedOutput, "LATE-MARKER") {
		t.Fatalf("expected streamed output to include late marker, got %q", streamedOutput)
	}

	errorText := err.Error()
	if !strings.Contains(errorText, "showing last") {
		t.Fatalf("expected truncated tail message, got %v", err)
	}
	if strings.Contains(errorText, "EARLY-MARKER") {
		t.Fatalf("expected early marker to be dropped from tail, got %v", err)
	}
	if !strings.Contains(errorText, "LATE-MARKER") {
		t.Fatalf("expected late marker in error tail, got %v", err)
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
		cmd.Env = append(os.Environ(),
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
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "agvtool" {
		switch commandArgs[1] {
		case "what-marketing-version":
			fmt.Fprint(os.Stdout, "App=1.2.3\nExtension=2.0.0\n")
			os.Exit(0)
		case "what-version":
			fmt.Fprint(os.Stdout, "App=41\nExtension=7\n")
			os.Exit(0)
		case "new-marketing-version", "new-version", "next-version":
			os.Exit(0)
		}
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
		if err := writeTestIPA(filepath.Join(exportPath, "Exported.ipa")); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}

	if len(commandArgs) >= 2 && commandArgs[0] == "xcodebuild" && commandArgs[1] == "fail-large-output" {
		fmt.Fprint(os.Stderr, "EARLY-MARKER\n")
		fmt.Fprint(os.Stderr, strings.Repeat("x", xcodebuildErrorTailLimit+128))
		fmt.Fprint(os.Stderr, "\nLATE-MARKER\n")
		os.Exit(1)
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
