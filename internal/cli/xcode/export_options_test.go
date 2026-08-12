package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
	"howett.net/plist"
)

func TestXcodeExportOptionsGenerateWritesRequestedAutomaticOptionsAndJSON(t *testing.T) {
	archivePath := writeXcodeExportOptionsTestArchive(t)
	outputPath := filepath.Join(t.TempDir(), "generated", "ExportOptions.plist")

	cmd := XcodeExportOptionsCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", archivePath,
		"--output-path", outputPath,
		"--destination", "upload",
		"--signing-style", "automatic",
		"--team-id", "TEAM123456",
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
		t.Fatalf("Exec() error: %v\nstderr=%s", runErr, stderr)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected no stderr output, got %q", stderr)
	}

	var result struct {
		Path         string `json:"path"`
		ArchivePath  string `json:"archive_path"`
		Method       string `json:"method"`
		Destination  string `json:"destination"`
		SigningStyle string `json:"signing_style"`
		TeamID       string `json:"team_id"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v\nstdout=%s", err, stdout)
	}
	if result.Path != outputPath || result.ArchivePath != archivePath {
		t.Fatalf("unexpected generated paths: %+v", result)
	}
	if result.Method != "app-store-connect" || result.Destination != "upload" || result.SigningStyle != "automatic" || result.TeamID != "TEAM123456" {
		t.Fatalf("unexpected generated result: %+v", result)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() generated options error: %v", err)
	}
	var options map[string]any
	if _, err := plist.Unmarshal(data, &options); err != nil {
		t.Fatalf("plist.Unmarshal() error: %v", err)
	}
	if options["method"] != "app-store-connect" || options["destination"] != "upload" || options["signingStyle"] != "automatic" || options["teamID"] != "TEAM123456" {
		t.Fatalf("unexpected generated plist: %+v", options)
	}
	if _, found := options["provisioningProfiles"]; found {
		t.Fatalf("automatic signing must not write provisioning profile mappings: %+v", options)
	}
}

func TestXcodeExportOptionsGenerateRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		message string
	}{
		{
			name:    "destination",
			args:    []string{"--archive-path", "Demo.xcarchive", "--destination", "archive"},
			message: "Error: --destination must be one of: export, upload",
		},
		{
			name:    "signing style",
			args:    []string{"--archive-path", "Demo.xcarchive", "--signing-style", "heuristic"},
			message: "Error: --signing-style must be one of: automatic, manual",
		},
		{
			name:    "empty signing style",
			args:    []string{"--archive-path", "Demo.xcarchive", "--signing-style", ""},
			message: "Error: --signing-style must be one of: automatic, manual",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := XcodeExportOptionsCommand()
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
			if !strings.Contains(stderr, tc.message) {
				t.Fatalf("expected %q, got %q", tc.message, stderr)
			}
		})
	}
}

func TestXcodeExportOptionsGenerateRejectsPositionalArguments(t *testing.T) {
	cmd := XcodeExportOptionsCommand()
	cmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), []string{"unexpected"})
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: xcode export-options generate does not accept positional arguments") {
		t.Fatalf("expected positional argument usage error, got %q", stderr)
	}
}

func TestXcodeExportGeneratesOptionsWhenPathIsOmitted(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	archivePath := writeXcodeExportOptionsTestArchive(t)
	deterministicPath := localxcode.DefaultExportOptionsPathForArchive(archivePath)
	deterministicContents := []byte("pre-existing archive-adjacent options\n")
	if err := os.WriteFile(deterministicPath, deterministicContents, 0o600); err != nil {
		t.Fatalf("WriteFile() deterministic export options error: %v", err)
	}
	var got localxcode.ExportOptions
	runExport = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		got = opts
		return &localxcode.ExportResult{ArchivePath: opts.ArchivePath, IPAPath: opts.IPAPath}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", archivePath,
		"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
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
		t.Fatalf("Exec() error: %v\nstderr=%s", runErr, stderr)
	}
	if got.ArchivePath != archivePath || strings.TrimSpace(got.ExportOptions) == "" {
		t.Fatalf("expected generated export options for archive, got %+v", got)
	}
	if got.ExportOptions == deterministicPath {
		t.Fatalf("implicit generation must use a unique path, got deterministic path %q", got.ExportOptions)
	}
	if !strings.HasPrefix(got.ExportOptions, strings.TrimSuffix(deterministicPath, ".plist")+"-") || !strings.HasSuffix(got.ExportOptions, ".plist") {
		t.Fatalf("expected unique archive-adjacent export options path, got %q", got.ExportOptions)
	}
	data, err := os.ReadFile(got.ExportOptions)
	if err != nil {
		t.Fatalf("ReadFile() generated export options error: %v", err)
	}
	var options map[string]any
	if _, err := plist.Unmarshal(data, &options); err != nil {
		t.Fatalf("plist.Unmarshal() generated options error: %v", err)
	}
	if options["method"] != "app-store-connect" || options["destination"] != "export" || options["signingStyle"] != "automatic" {
		t.Fatalf("unexpected automatically generated export options: %+v", options)
	}
	if !strings.Contains(stdout, `"archive_path"`) {
		t.Fatalf("expected export JSON output, got %q", stdout)
	}
	preserved, err := os.ReadFile(deterministicPath)
	if err != nil {
		t.Fatalf("ReadFile() deterministic export options error: %v", err)
	}
	if string(preserved) != string(deterministicContents) {
		t.Fatalf("implicit generation overwrote deterministic export options: %q", preserved)
	}
}

func TestXcodeExportThreadsManualSigningOptionsToImplicitGeneration(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	archivePath := writeXcodeExportOptionsTestArchive(t)
	wantErr := errors.New("stop after generation")
	var generatedOptions localxcode.ExportOptionsGenerateOptions
	runGenerateExportOptions = func(_ context.Context, opts localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		generatedOptions = opts
		return &localxcode.ExportOptionsGenerateResult{Path: opts.OutputPath}, nil
	}
	runExport = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return nil, wantErr
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", archivePath,
		"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
		"--signing-style", "manual",
		"--team-id", "TEAM123456",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	runErr := cmd.Exec(context.Background(), nil)
	if !errors.Is(runErr, wantErr) {
		t.Fatalf("expected export sentinel, got %v", runErr)
	}
	if generatedOptions.SigningStyle != "manual" {
		t.Fatalf("expected manual signing style, got %q", generatedOptions.SigningStyle)
	}
	if generatedOptions.TeamID != "TEAM123456" {
		t.Fatalf("expected team ID passthrough, got %q", generatedOptions.TeamID)
	}
}

func TestXcodeExportRejectsGenerationFlagsWithExplicitOptions(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "signing style", args: []string{"--signing-style", "automatic"}},
		{name: "team ID", args: []string{"--team-id", "TEAM123456"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := overrideXcodeCommandTestHooks(t)
			defer restore()

			runXcodeExportPreflight = func(context.Context) error {
				t.Fatal("preflight must not run for conflicting generation flags")
				return nil
			}
			runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
				t.Fatal("generator must not run for conflicting generation flags")
				return nil, nil
			}
			runExport = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				t.Fatal("export must not run for conflicting generation flags")
				return nil, nil
			}

			args := []string{
				"--archive-path", "Demo.xcarchive",
				"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
				"--export-options", "ExportOptions.plist",
			}
			args = append(args, tc.args...)
			cmd := XcodeExportCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			runErr := cmd.Exec(context.Background(), nil)
			if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "--export-options cannot be combined with --signing-style or --team-id") {
				t.Fatalf("expected explicit-options conflict usage error, got %v", runErr)
			}
		})
	}
}

func TestXcodeExportRejectsInvalidSigningStyleBeforeSideEffects(t *testing.T) {
	for _, signingStyle := range []string{"heuristic", ""} {
		t.Run(fmt.Sprintf("value=%q", signingStyle), func(t *testing.T) {
			restore := overrideXcodeCommandTestHooks(t)
			defer restore()

			runXcodeExportPreflight = func(context.Context) error {
				t.Fatal("preflight must not run for an invalid signing style")
				return nil
			}
			runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
				t.Fatal("generator must not run for an invalid signing style")
				return nil, nil
			}

			cmd := XcodeExportCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--archive-path", "Demo.xcarchive",
				"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
				"--signing-style", signingStyle,
			}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			runErr := cmd.Exec(context.Background(), nil)
			if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "--signing-style must be one of: automatic, manual") {
				t.Fatalf("expected invalid signing-style usage error, got %v", runErr)
			}
		})
	}
}

func TestXcodeExportRejectsManagedXcodebuildFlagsBeforeSideEffects(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	t.Cleanup(restore)

	runXcodeExportPreflight = func(context.Context) error {
		t.Fatal("preflight must not run for a managed xcodebuild argument")
		return nil
	}
	runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		t.Fatal("export options generation must not run for a managed xcodebuild argument")
		return nil, nil
	}
	runExport = func(context.Context, localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		t.Fatal("export must not run for a managed xcodebuild argument")
		return nil, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--ipa-path", "Demo.ipa",
		"--xcodebuild-flag=-exportPath=/tmp/elsewhere",
	}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want usage error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `--xcodebuild-flag cannot override asc-managed argument "-exportPath"`) {
		t.Fatalf("stderr = %q, want managed-argument error", stderr)
	}
}

func TestXcodeExportRejectsExplicitlyEmptyTeamIDBeforeSideEffects(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	runXcodeExportPreflight = func(context.Context) error {
		t.Fatal("preflight must not run for an empty team ID")
		return nil
	}
	runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		t.Fatal("generator must not run for an empty team ID")
		return nil, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", "Demo.xcarchive",
		"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
		"--team-id", "",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	runErr := cmd.Exec(context.Background(), nil)
	if !errors.Is(runErr, flag.ErrHelp) || !strings.Contains(runErr.Error(), "--team-id must not be empty") {
		t.Fatalf("expected empty team-id usage error, got %v", runErr)
	}
}

func TestXcodeExportSigningFlagsAreDiscoverable(t *testing.T) {
	cmd := XcodeExportCommand()
	for _, name := range []string{"signing-style", "team-id"} {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected --%s in xcode export help", name)
		}
	}
}

func TestXcodeExportPreflightsBeforeImplicitOptionGeneration(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	wantErr := errors.New("preflight sentinel")
	outputParent := filepath.Join(t.TempDir(), "nested", "output")
	runXcodeExportPreflight = func(context.Context) error { return wantErr }
	runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		t.Fatal("export options must not be generated before Xcode preflight succeeds")
		return nil, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", filepath.Join(t.TempDir(), "Demo.xcarchive"),
		"--ipa-path", filepath.Join(outputParent, "Demo.ipa"),
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, wantErr) {
		t.Fatalf("expected preflight error, got %v", err)
	}
	if _, err := os.Stat(outputParent); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Xcode preflight failure created IPA output parent: %v", err)
	}
}

func TestXcodeExportValidatesIPADestinationBeforeImplicitOptionGeneration(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ipaPath   func(*testing.T) string
		errorHint string
	}{
		{
			name:      "invalid extension",
			ipaPath:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "Demo.zip") },
			errorHint: "must end with .ipa",
		},
		{
			name: "existing output without overwrite",
			ipaPath: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "Demo.ipa")
				if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
					t.Fatal(err)
				}
				return path
			},
			errorHint: "already exists",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := overrideXcodeCommandTestHooks(t)
			defer restore()

			runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
				t.Fatal("export options must not be generated for an unusable IPA destination")
				return nil, nil
			}
			cmd := XcodeExportCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--archive-path", filepath.Join(t.TempDir(), "Demo.xcarchive") + string(filepath.Separator),
				"--ipa-path", tc.ipaPath(t),
			}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			if err := cmd.Exec(context.Background(), nil); err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), tc.errorHint) {
				t.Fatalf("expected %q error, got %v", tc.errorHint, err)
			}
		})
	}
}

func TestXcodeExportWaitPreflightsIPAParentBeforeImplicitOptionGeneration(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	parent := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(parent, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGenerateExportOptions = func(context.Context, localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		t.Fatal("upload export options must not be generated for an unusable IPA parent")
		return nil, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", filepath.Join(t.TempDir(), "Demo.xcarchive"),
		"--ipa-path", filepath.Join(parent, "Demo.ipa"),
		"--wait",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), nil)
	if err == nil || errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "ipa output parent") {
		t.Fatalf("expected runtime IPA-parent error, got %v", err)
	}
}

func TestXcodeExportWaitGeneratesUploadOptionsWhenPathIsOmitted(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	archivePath := writeXcodeExportOptionsTestArchive(t)
	wantErr := errors.New("stop after generation")
	var generatedOptions localxcode.ExportOptionsGenerateOptions
	runGenerateExportOptions = func(_ context.Context, opts localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
		generatedOptions = opts
		return &localxcode.ExportOptionsGenerateResult{Path: opts.OutputPath}, nil
	}
	isDirectUploadExportOptionsFn = func(path string) bool {
		return path == generatedOptions.OutputPath
	}
	runExport = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		return nil, wantErr
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", archivePath,
		"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
		"--wait",
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	runErr := cmd.Exec(context.Background(), nil)
	if !errors.Is(runErr, wantErr) {
		t.Fatalf("expected export sentinel, got %v", runErr)
	}
	if generatedOptions.Destination != "upload" {
		t.Fatalf("expected --wait generation destination upload, got %q", generatedOptions.Destination)
	}
	if generatedOptions.Overwrite {
		t.Fatal("implicit generation must not enable overwrite")
	}
}

func TestXcodeExportRejectsEmptyGeneratedOptionsResult(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result *localxcode.ExportOptionsGenerateResult
	}{
		{name: "nil result", result: nil},
		{name: "empty path", result: &localxcode.ExportOptionsGenerateResult{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := overrideXcodeCommandTestHooks(t)
			defer restore()

			archivePath := writeXcodeExportOptionsTestArchive(t)
			runGenerateExportOptions = func(_ context.Context, _ localxcode.ExportOptionsGenerateOptions) (*localxcode.ExportOptionsGenerateResult, error) {
				return tc.result, nil
			}
			runExport = func(_ context.Context, _ localxcode.ExportOptions) (*localxcode.ExportResult, error) {
				t.Fatal("export must not run with an empty generated options result")
				return nil, nil
			}

			cmd := XcodeExportCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--archive-path", archivePath,
				"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
			}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			runErr := cmd.Exec(context.Background(), nil)
			if runErr == nil || !strings.Contains(runErr.Error(), "generator returned an empty path") {
				t.Fatalf("expected empty generator path error, got %v", runErr)
			}
		})
	}
}

func TestXcodeExportPreservesExplicitExportOptionsPath(t *testing.T) {
	restore := overrideXcodeCommandTestHooks(t)
	defer restore()

	archivePath := writeXcodeExportOptionsTestArchive(t)
	explicitPath := filepath.Join(t.TempDir(), "CustomExportOptions.plist")
	explicitData := []byte("custom export options must remain authoritative\n")
	if err := os.WriteFile(explicitPath, explicitData, 0o600); err != nil {
		t.Fatalf("WriteFile() explicit export options error: %v", err)
	}
	var got localxcode.ExportOptions
	runExport = func(_ context.Context, opts localxcode.ExportOptions) (*localxcode.ExportResult, error) {
		got = opts
		return &localxcode.ExportResult{ArchivePath: opts.ArchivePath, IPAPath: opts.IPAPath}, nil
	}

	cmd := XcodeExportCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--archive-path", archivePath,
		"--export-options", explicitPath,
		"--ipa-path", filepath.Join(t.TempDir(), "Demo.ipa"),
	}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error: %v\nstderr=%s", runErr, stderr)
	}
	if got.ExportOptions != explicitPath {
		t.Fatalf("expected explicit export options path %q, got %q", explicitPath, got.ExportOptions)
	}
	data, err := os.ReadFile(explicitPath)
	if err != nil {
		t.Fatalf("ReadFile() explicit export options error: %v", err)
	}
	if string(data) != string(explicitData) {
		t.Fatalf("explicit export options were modified: %q", data)
	}
}

func writeXcodeExportOptionsTestArchive(t *testing.T) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "Demo.xcarchive")
	appPath := filepath.Join(archivePath, "Products", "Applications", "Demo.app")
	if err := os.MkdirAll(appPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() archive app error: %v", err)
	}
	archiveInfo := map[string]any{
		"ApplicationProperties": map[string]any{
			"ApplicationPath":    "Applications/Demo.app",
			"CFBundleIdentifier": "com.example.demo",
			"Team":               "ARCHIVETEAM",
		},
	}
	appInfo := map[string]any{
		"CFBundleIdentifier":         "com.example.demo",
		"CFBundleShortVersionString": "1.2.3",
		"CFBundleVersion":            "42",
		"DTPlatformName":             "iphoneos",
	}
	writePlist := func(path string, value any) {
		t.Helper()
		data, err := plist.Marshal(value, plist.XMLFormat)
		if err != nil {
			t.Fatalf("plist.Marshal() error: %v", err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("WriteFile() plist error: %v", err)
		}
	}
	writePlist(filepath.Join(archivePath, "Info.plist"), archiveInfo)
	writePlist(filepath.Join(appPath, "Info.plist"), appInfo)
	return archivePath
}
