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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
	"howett.net/plist"
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

func TestXcodeExportAcceptsPKGPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	var gotOptions localxcode.ExportOptions
	runExport = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		gotOptions = opts
		return &localxcode.ExportResult{
			ArchivePath: opts.ArchivePath,
			PKGPath:     opts.PKGPath,
			BundleID:    "com.example.mac",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--pkg-path", "Demo.pkg",
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
	if gotOptions.PKGPath != "Demo.pkg" || gotOptions.IPAPath != "" {
		t.Fatalf("export options = %+v, want only PKG path", gotOptions)
	}
	var payload struct {
		PKGPath string `json:"pkg_path"`
		IPAPath string `json:"ipa_path"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload.PKGPath != "Demo.pkg" || payload.IPAPath != "" {
		t.Fatalf("export payload = %+v, want only pkg_path", payload)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestXcodeExportRejectsMultipleArtifactPaths(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--pkg-path", "Demo.pkg",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want usage error", runErr)
	}
	if !strings.Contains(stderr, "Error: --ipa-path and --pkg-path are mutually exclusive") {
		t.Fatalf("stderr = %q, want artifact conflict", stderr)
	}
}

func TestXcodeExportGeneratesManualPKGOptions(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	var generatedOptions localxcode.ExportOptionsGenerateOptions
	runGenerateExportOptions = func(_ context.Context, opts localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		generatedOptions = opts
		return &localxcode.ExportOptionsGenerateResult{Path: filepath.Join(t.TempDir(), "ExportOptions.plist")}, nil
	}
	var exportedOptions localxcode.ExportOptions
	runExport = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		exportedOptions = opts
		return &localxcode.ExportResult{ArchivePath: opts.ArchivePath, PKGPath: opts.PKGPath}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	pkgPath := filepath.Join(t.TempDir(), "Demo.pkg")
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--pkg-path", pkgPath,
		"--signing-style", "manual",
		"--output", "json",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	stdout, stderr := captureCommandOutput(t, func() error {
		return cmd.Exec(context.Background(), nil)
	})
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if generatedOptions.Method != "app-store-connect" || generatedOptions.SigningStyle != "manual" || generatedOptions.Destination != "export" {
		t.Fatalf("generated options = %+v, want App Store manual export", generatedOptions)
	}
	if exportedOptions.PKGPath != pkgPath || exportedOptions.IPAPath != "" {
		t.Fatalf("export options = %+v, want only PKG path", exportedOptions)
	}
	if !strings.Contains(stdout, `"pkg_path"`) {
		t.Fatalf("stdout = %q, want JSON export result", stdout)
	}
}

func TestXcodeExportDirectUploadDoesNotRequireArtifactPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	isDirectUploadExportOptionsFn = func(string) bool { return true }
	runExport = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		if opts.IPAPath != "" || opts.PKGPath != "" {
			t.Fatalf("direct upload options = %+v, want no local artifact path", opts)
		}
		return &localxcode.ExportResult{
			ArchivePath: opts.ArchivePath,
			BundleID:    "com.example.mac",
			Version:     "1.2.3",
			BuildNumber: "42",
		}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "UploadExportOptions.plist",
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
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestXcodeInjectGeneratesPlistTextAndCopiesAsset(t *testing.T) {
	dir := t.TempDir()
	sourceAssetPath := filepath.Join(dir, "Assets", "AppIcon.appiconset", "Contents.json")
	if err := os.MkdirAll(filepath.Dir(sourceAssetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(sourceAssetPath, []byte(`{"images":[]}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() source asset error: %v", err)
	}

	manifestPath := filepath.Join(dir, ".asc", "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"values": {
			"bundle_id": "com.example.demo",
			"app_name": "Demo",
			"version": "1.2.3",
			"build_number": "42"
		},
		"outputs": [
			{
				"type": "plist",
				"path": "../Generated/Info.generated.plist",
				"values": {
					"CFBundleIdentifier": "${bundle_id}",
					"CFBundleDisplayName": "${app_name}",
					"CFBundleShortVersionString": "${version}",
					"CFBundleVersion": "${build_number}"
				}
			},
			{
				"type": "text",
				"path": "../Generated/Deployment.xcconfig",
				"contents": "PRODUCT_BUNDLE_IDENTIFIER = ${bundle_id}\nMARKETING_VERSION = ${version}\nCURRENT_PROJECT_VERSION = ${build_number}\n"
			},
			{
				"type": "copy",
				"source": "../Assets/AppIcon.appiconset/Contents.json",
				"path": "../Generated/Assets.xcassets/AppIcon.appiconset/Contents.json"
			}
		]
	}`)

	result, err := runXcodeInject(xcodeInjectOptions{
		ManifestPath: manifestPath,
		SetValues:    []string{"version=1.2.4"},
	})
	if err != nil {
		t.Fatalf("runXcodeInject() error: %v", err)
	}
	if result.DryRun {
		t.Fatal("expected non-dry-run result")
	}
	if len(result.Outputs) != 3 {
		t.Fatalf("expected 3 outputs, got %d", len(result.Outputs))
	}

	plistPath := filepath.Join(dir, "Generated", "Info.generated.plist")
	plistData, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("ReadFile() plist error: %v", err)
	}
	var info map[string]any
	if _, err := plist.Unmarshal(plistData, &info); err != nil {
		t.Fatalf("plist.Unmarshal() error: %v", err)
	}
	if info["CFBundleIdentifier"] != "com.example.demo" {
		t.Fatalf("expected bundle identifier, got %+v", info)
	}
	if info["CFBundleShortVersionString"] != "1.2.4" {
		t.Fatalf("expected overridden version, got %+v", info)
	}

	xcconfigPath := filepath.Join(dir, "Generated", "Deployment.xcconfig")
	xcconfigData, err := os.ReadFile(xcconfigPath)
	if err != nil {
		t.Fatalf("ReadFile() xcconfig error: %v", err)
	}
	if !strings.Contains(string(xcconfigData), "CURRENT_PROJECT_VERSION = 42") {
		t.Fatalf("expected build number in xcconfig, got %q", string(xcconfigData))
	}

	copiedAssetPath := filepath.Join(dir, "Generated", "Assets.xcassets", "AppIcon.appiconset", "Contents.json")
	copiedAssetData, err := os.ReadFile(copiedAssetPath)
	if err != nil {
		t.Fatalf("ReadFile() copied asset error: %v", err)
	}
	if string(copiedAssetData) != "{\"images\":[]}\n" {
		t.Fatalf("expected copied asset contents, got %q", string(copiedAssetData))
	}
}

func TestXcodeInjectDryRunDoesNotWriteFiles(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"values": {"bundle_id": "com.example.demo"},
		"outputs": [
			{
				"type": "text",
				"path": "Generated.xcconfig",
				"contents": "PRODUCT_BUNDLE_IDENTIFIER = ${bundle_id}\n"
			}
		]
	}`)

	result, err := runXcodeInject(xcodeInjectOptions{
		ManifestPath: manifestPath,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("runXcodeInject() error: %v", err)
	}
	if !result.DryRun {
		t.Fatal("expected dry_run result")
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(result.Outputs))
	}
	if got := result.Outputs[0].Action; got != "would_write" {
		t.Fatalf("expected would_write action, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "Generated.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected dry-run output not to exist, stat error: %v", err)
	}
}

func TestXcodeInjectDryRunRejectsExistingOutputWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "NEW = yes\n"}
		]
	}`)
	existingPath := filepath.Join(dir, "Generated.xcconfig")
	if err := os.WriteFile(existingPath, []byte("OLD = yes\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() existing output error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true})
	if err == nil {
		t.Fatal("expected dry-run existing output error")
	}
	if !strings.Contains(err.Error(), "already exists; use --overwrite") {
		t.Fatalf("expected overwrite guidance, got %v", err)
	}
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("ReadFile() existing output error: %v", err)
	}
	if string(data) != "OLD = yes\n" {
		t.Fatalf("expected existing output preserved, got %q", string(data))
	}
}

func TestXcodeInjectDryRunOverwriteRejectsSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "NEW = yes\n"}
		]
	}`)
	targetPath := filepath.Join(dir, "Generated.xcconfig")
	if err := os.Symlink(filepath.Join(dir, "real.xcconfig"), targetPath); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true, Overwrite: true})
	if err == nil {
		t.Fatal("expected dry-run symlink destination error")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite symlink") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}

func TestXcodeInjectDryRunOverwriteRejectsDirectoryDestination(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "NEW = yes\n"}
		]
	}`)
	targetPath := filepath.Join(dir, "Generated.xcconfig")
	if err := os.Mkdir(targetPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true, Overwrite: true})
	if err == nil {
		t.Fatal("expected dry-run directory destination error")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory refusal, got %v", err)
	}
}

func TestXcodeInjectRejectsDuplicateDestinationsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "Generated.xcconfig", "contents": "SECOND = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected duplicate destination error")
	}
	if !strings.Contains(err.Error(), "duplicate output path") {
		t.Fatalf("expected duplicate destination guidance, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Generated.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected duplicate validation before writing, stat error: %v", err)
	}
}

func TestXcodeInjectRejectsLaterRenderErrorBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "First.xcconfig", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "Second.xcconfig", "contents": "SECOND = ${missing}\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected missing placeholder error")
	}
	if !strings.Contains(err.Error(), `missing value for placeholder "missing"`) {
		t.Fatalf("expected missing placeholder error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "First.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected first output not to be written, stat error: %v", err)
	}
}

func TestXcodeInjectRejectsLaterCopySourceErrorBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "First.xcconfig", "contents": "FIRST = yes\n"},
			{"type": "copy", "source": "Missing/Contents.json", "path": "Copied/Contents.json"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected missing copy source error")
	}
	if _, err := os.Stat(filepath.Join(dir, "First.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected first output not to be written, stat error: %v", err)
	}
}

func TestXcodeInjectRejectsCopySourceFromManifestOutput(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "FIRST = yes\n"},
			{"type": "copy", "source": "Generated.xcconfig", "path": "Copied.xcconfig"}
		]
	}`)
	if err := os.WriteFile(filepath.Join(dir, "Generated.xcconfig"), []byte("STALE = yes\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() stale source error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, Overwrite: true})
	if err == nil {
		t.Fatal("expected copy source/output conflict error")
	}
	if !strings.Contains(err.Error(), "copy source") {
		t.Fatalf("expected copy source guidance, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Copied.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected copy output not to be written, stat error: %v", err)
	}
}

func TestXcodeInjectDryRunOverwriteRejectsDuplicateDestinations(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "Generated.xcconfig", "contents": "SECOND = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true, Overwrite: true})
	if err == nil {
		t.Fatal("expected duplicate dry-run overwrite destination error")
	}
	if !strings.Contains(err.Error(), "duplicate output path") {
		t.Fatalf("expected duplicate destination guidance, got %v", err)
	}
}

func TestXcodeInjectRejectsCaseOnlyDuplicateDestinations(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated/Info.plist", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "generated/info.plist", "contents": "SECOND = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true})
	if err == nil {
		t.Fatal("expected case-only duplicate destination error")
	}
	if !strings.Contains(err.Error(), "duplicate output path") {
		t.Fatalf("expected duplicate destination guidance, got %v", err)
	}
}

func TestXcodeInjectRejectsCaseOnlyNestedDestinationConflicts(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "generated/Info.plist", "contents": "SECOND = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true})
	if err == nil {
		t.Fatal("expected case-only nested destination conflict error")
	}
	if !strings.Contains(err.Error(), "conflicts with nested output path") {
		t.Fatalf("expected nested destination guidance, got %v", err)
	}
}

func TestXcodeInjectRejectsNestedDestinationConflictsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "Generated/Info.plist", "contents": "SECOND = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected nested destination conflict error")
	}
	if !strings.Contains(err.Error(), "conflicts with nested output path") {
		t.Fatalf("expected nested destination guidance, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Generated")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected nested validation before writing, stat error: %v", err)
	}
}

func TestXcodeInjectRejectsFileParentBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "First.xcconfig", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "Parent/Child.xcconfig", "contents": "SECOND = yes\n"}
		]
	}`)
	parentPath := filepath.Join(dir, "Parent")
	if err := os.WriteFile(parentPath, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() parent error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected file parent validation error")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("expected parent directory validation error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "First.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected first output not to be written, stat error: %v", err)
	}
}

func TestXcodeInjectDryRunRejectsFileParent(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Parent/Child.xcconfig", "contents": "SECOND = yes\n"}
		]
	}`)
	parentPath := filepath.Join(dir, "Parent")
	if err := os.WriteFile(parentPath, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() parent error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true})
	if err == nil {
		t.Fatal("expected dry-run file parent validation error")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("expected parent directory validation error, got %v", err)
	}
}

func TestXcodeInjectAllowsDirectorySymlinkParent(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "SharedGenerated")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir() real parent error: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() parent error: %v", err)
	}
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated/Info.plist", "contents": "FIRST = yes\n"}
		]
	}`)

	result, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err != nil {
		t.Fatalf("runXcodeInject() error: %v", err)
	}
	if len(result.Outputs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(result.Outputs))
	}
	data, err := os.ReadFile(filepath.Join(realDir, "Info.plist"))
	if err != nil {
		t.Fatalf("ReadFile() generated output error: %v", err)
	}
	if string(data) != "FIRST = yes\n" {
		t.Fatalf("unexpected generated output: %q", string(data))
	}
}

func TestXcodeInjectRejectsSymlinkedParentAliasDestinations(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "SharedGenerated")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir() real parent error: %v", err)
	}
	if err := os.Symlink(realDir, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() parent error: %v", err)
	}
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated/Info.plist", "contents": "FIRST = yes\n"},
			{"type": "text", "path": "SharedGenerated/Info.plist", "contents": "SECOND = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true})
	if err == nil {
		t.Fatal("expected symlinked parent alias conflict error")
	}
	if !strings.Contains(err.Error(), "duplicate output path") {
		t.Fatalf("expected duplicate destination guidance, got %v", err)
	}
}

func TestXcodeInjectRejectsFileSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "SharedGenerated")
	if err := os.WriteFile(realFile, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() real parent error: %v", err)
	}
	if err := os.Symlink(realFile, filepath.Join(dir, "Generated")); err != nil {
		t.Fatalf("Symlink() parent error: %v", err)
	}
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated/Info.plist", "contents": "FIRST = yes\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, DryRun: true})
	if err == nil {
		t.Fatal("expected file symlink parent validation error")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("expected parent directory validation error, got %v", err)
	}
}

func TestXcodeInjectOverwriteRejectsSymlinkDestination(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "NEW = yes\n"}
		]
	}`)
	targetPath := filepath.Join(dir, "Generated.xcconfig")
	if err := os.Symlink(filepath.Join(dir, "real.xcconfig"), targetPath); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, Overwrite: true})
	if err == nil {
		t.Fatal("expected symlink destination error")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite symlink") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}

func TestXcodeInjectOverwriteRejectsDirectoryDestination(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "NEW = yes\n"}
		]
	}`)
	targetPath := filepath.Join(dir, "Generated.xcconfig")
	if err := os.Mkdir(targetPath, 0o755); err != nil {
		t.Fatalf("Mkdir() error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath, Overwrite: true})
	if err == nil {
		t.Fatal("expected directory destination error")
	}
	if !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected directory refusal, got %v", err)
	}
}

func TestXcodeInjectExpandsNestedPlaceholders(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"values": {
			"version": "1.2.3",
			"env": "${version}-beta"
		},
		"outputs": [
			{
				"type": "text",
				"path": "Generated.xcconfig",
				"contents": "APP_CHANNEL = ${env}\n"
			}
		]
	}`)

	if _, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath}); err != nil {
		t.Fatalf("runXcodeInject() error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Generated.xcconfig"))
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "APP_CHANNEL = 1.2.3-beta\n" {
		t.Fatalf("expected nested placeholder expansion, got %q", string(data))
	}
}

func TestXcodeInjectRejectsUnclosedPlaceholder(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "APP_CHANNEL = ${env\n"}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected unclosed placeholder error")
	}
	if !strings.Contains(err.Error(), "unclosed placeholder") {
		t.Fatalf("expected unclosed placeholder error, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Generated.xcconfig")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected output not to be written, stat error: %v", err)
	}
}

func TestXcodeInjectRejectsInvalidOutputType(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "yaml", "path": "Generated.yml", "values": {"name": "Demo"}}
		]
	}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected invalid output type error")
	}
	if !strings.Contains(err.Error(), "type must be one of plist, json, text, copy") {
		t.Fatalf("expected output type validation error, got %v", err)
	}
}

func TestXcodeInjectRejectsMultipleJSONValues(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{"outputs": []} {"outputs": []}`)

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected multiple JSON values error")
	}
	if !strings.Contains(err.Error(), "multiple JSON values") {
		t.Fatalf("expected multiple JSON values error, got %v", err)
	}
}

func TestXcodeInjectRejectsExistingOutputWithoutOverwrite(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	writeXcodeInjectTestManifest(t, manifestPath, `{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "NEW = yes\n"}
		]
	}`)
	existingPath := filepath.Join(dir, "Generated.xcconfig")
	if err := os.WriteFile(existingPath, []byte("OLD = yes\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() existing output error: %v", err)
	}

	_, err := runXcodeInject(xcodeInjectOptions{ManifestPath: manifestPath})
	if err == nil {
		t.Fatal("expected existing output error")
	}
	if !strings.Contains(err.Error(), "already exists; use --overwrite") {
		t.Fatalf("expected overwrite guidance, got %v", err)
	}
	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("ReadFile() existing output error: %v", err)
	}
	if string(data) != "OLD = yes\n" {
		t.Fatalf("expected existing output preserved, got %q", string(data))
	}
}

func TestXcodeInjectCommandRequiresManifest(t *testing.T) {
	cmd := XcodeInjectCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --manifest is required") {
		t.Fatalf("expected manifest requirement in stderr, got %q", stderr)
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

func TestXcodeValidatePassesPKGAndRendersPKGPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	var gotOpts localxcode.ValidateOptions
	runValidate = func(_ context.Context, opts localxcode.ValidateOptions) (*localxcode.ValidateResult, error) {
		gotOpts = opts
		return &localxcode.ValidateResult{
			PKGPath:   opts.PKGPath,
			Validated: true,
		}, nil
	}

	cmd := XcodeValidateCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--pkg", "Demo.pkg", "--output", "json"}); err != nil {
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
	if stderr != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
	if gotOpts.PKGPath != "Demo.pkg" || gotOpts.IPAPath != "" {
		t.Fatalf("unexpected validate options: %+v", gotOpts)
	}

	var payload struct {
		IPAPath   string `json:"ipa_path"`
		PKGPath   string `json:"pkg_path"`
		Validated bool   `json:"validated"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if payload.PKGPath != "Demo.pkg" || payload.IPAPath != "" || !payload.Validated {
		t.Fatalf("unexpected validate payload: %+v", payload)
	}
	rows := validateResultRows(&localxcode.ValidateResult{PKGPath: "Demo.pkg", Validated: true})
	if len(rows) != 2 || rows[0][0] != "pkg_path" || rows[0][1] != "Demo.pkg" || rows[1][0] != "validated" {
		t.Fatalf("unexpected rendered PKG rows: %v", rows)
	}
}

func TestXcodeValidateRejectsMultipleArtifactPaths(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	cmd := XcodeValidateCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--ipa", "Demo.ipa", "--pkg", "Demo.pkg"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --ipa and --pkg are mutually exclusive") {
		t.Fatalf("expected mutually exclusive artifact error, got %q", stderr)
	}
}

func TestXcodeValidateRejectsExplicitlyEmptyArtifactPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	for _, tc := range []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "empty ipa with pkg", args: []string{"--ipa=", "--pkg", "Demo.pkg"}, wantErr: "--ipa must not be empty"},
		{name: "empty pkg with ipa", args: []string{"--ipa", "Demo.ipa", "--pkg="}, wantErr: "--pkg must not be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := XcodeValidateCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			var runErr error
			_, stderr := captureCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if !strings.Contains(stderr, "Error: "+tc.wantErr) {
				t.Fatalf("expected %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestXcodeArchiveRejectsExplicitlyEmptyConfiguration(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	runArchive = func(context.Context, localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		t.Fatal("runArchive must not be called for an explicitly empty configuration")
		return nil, nil
	}

	cmd := XcodeArchiveCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--archive-path", "Demo.xcarchive",
		"--configuration", "  ",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want usage error", runErr)
	}
	if runErr.Error() != "--configuration must not be empty" {
		t.Fatalf("Exec() error = %q, want %q", runErr, "--configuration must not be empty")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "--configuration must not be empty") {
		t.Fatalf("stderr = %q, want empty configuration usage error", stderr)
	}
}

func TestXcodeValidateRejectsNonIPAPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	cmd := XcodeValidateCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--ipa", "Demo.txt"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatal("expected flag.ErrHelp for non-.ipa path")
	}
	if !strings.Contains(stderr, "Error: --ipa must end with .ipa") {
		t.Fatalf("expected ipa extension usage error, got %q", stderr)
	}
}

func TestXcodeValidateRejectsNonPKGPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	cmd := XcodeValidateCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--pkg", "Demo.txt"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatal("expected flag.ErrHelp for non-.pkg path")
	}
	if !strings.Contains(stderr, "Error: --pkg must end with .pkg") {
		t.Fatalf("expected pkg extension usage error, got %q", stderr)
	}
}

func TestXcodeValidateRequiresAPIKeyAndIssuerTogether(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "api key only",
			args: []string{"--ipa", "Demo.ipa", "--api-key", "KEY123ABC"},
		},
		{
			name: "api issuer only",
			args: []string{"--ipa", "Demo.ipa", "--api-issuer", "issuer-123"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := XcodeValidateCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(tc.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			var runErr error
			_, stderr := captureCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatal("expected flag.ErrHelp for partial JWT auth flags")
			}
			if !strings.Contains(stderr, "Error: --api-key and --api-issuer must be provided together") {
				t.Fatalf("expected auth pairing usage error, got %q", stderr)
			}
		})
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

func TestXcodeExportRejectsNegativeTimeout(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--timeout", "-1s",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatal("expected flag.ErrHelp for negative timeout")
	}
	if !strings.Contains(stderr, "Error: --timeout must be zero or greater") {
		t.Fatalf("expected timeout usage error, got %q", stderr)
	}
}

func TestXcodeExportPassesTimeoutContextToLocalExport(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	runExport = func(ctx context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected export context deadline")
		}
		if time.Until(deadline) <= 0 {
			t.Fatalf("expected future deadline, got %s", deadline)
		}
		return &localxcode.ExportResult{
			ArchivePath: opts.ArchivePath,
			IPAPath:     opts.IPAPath,
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
		"--timeout", "10s",
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
		t.Fatalf("expected no stderr output, got %q", stderr)
	}
}

func TestXcodeExportReportsTimeout(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	runExport = func(ctx context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--export-options", "ExportOptions.plist",
		"--ipa-path", "Demo.ipa",
		"--timeout", "1ms",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, _ = captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(runErr.Error(), "timed out after 1ms while running xcodebuild -exportArchive") {
		t.Fatalf("expected timeout guidance, got %v", runErr)
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

func writeXcodeInjectTestManifest(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() manifest dir error: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile() manifest error: %v", err)
	}
}

func overrideXcodeCommandTestHooks(t *testing.T) func() {
	t.Helper()

	originalRunArchive := runArchive
	originalRunExport := runExport
	originalRunValidate := runValidate
	originalRunGenerateExportOptions := runGenerateExportOptions
	originalRunXcodeExportPreflight := runXcodeExportPreflight
	originalIsDirectUpload := isDirectUploadExportOptionsFn
	originalInferArchivePlatform := inferArchivePlatformFn
	originalGetASCClient := getASCClientFn
	originalResolveAppID := resolveAppIDWithExactLookupFn
	originalResolveBuildUploadID := resolveBuildUploadIDFn
	originalWaitForDiscovery := waitForBuildByNumberOrUploadFailureFn
	originalWaitForProcessing := waitForBuildProcessingFn
	originalWaitTimeout := resolveXcodeExportWaitTimeoutFn
	runXcodeExportPreflight = func(context.Context) error { return nil }

	return func() {
		runArchive = originalRunArchive
		runExport = originalRunExport
		runValidate = originalRunValidate
		runGenerateExportOptions = originalRunGenerateExportOptions
		runXcodeExportPreflight = originalRunXcodeExportPreflight
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
