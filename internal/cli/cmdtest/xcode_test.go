package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeCommandExists(t *testing.T) {
	root := RootCommand("1.2.3")

	xcodeCmd := findSubcommand(root, "xcode")
	if xcodeCmd == nil {
		t.Fatal("expected xcode command")
		return
	}

	buildCmd := findSubcommand(root, "xcode", "build")
	if buildCmd == nil {
		t.Fatal("expected xcode build command")
		return
	}

	for _, name := range []string{"project", "workspace", "scheme", "configuration", "destination", "derived-data-path", "result-bundle-path", "clean", "no-code-signing", "xcodebuild-flag", "output"} {
		if buildCmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected xcode build to expose --%s", name)
		}
	}
	if findSubcommand(root, "xcode", "archive") == nil {
		t.Fatal("expected xcode archive command")
	}
	if findSubcommand(root, "xcode", "test") == nil {
		t.Fatal("expected xcode test command")
	}
	testCmd := findSubcommand(root, "xcode", "test")

	for _, name := range []string{"project", "workspace", "scheme", "action", "configuration", "destination", "test-plan", "xctestrun", "only-testing", "skip-testing", "derived-data-path", "result-bundle-path", "clean", "no-code-signing", "xcodebuild-flag", "output"} {
		if testCmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected xcode test to expose --%s", name)
		}
	}
	if findSubcommand(root, "xcode", "export") == nil {
		t.Fatal("expected xcode export command")
	}
	validateCmd := findSubcommand(root, "xcode", "validate")
	if validateCmd == nil {
		t.Fatal("expected xcode validate command")
	}
	for _, name := range []string{"ipa", "pkg", "api-key", "api-issuer", "output"} {
		if validateCmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected xcode validate to expose --%s", name)
		}
	}
	if findSubcommand(root, "xcode", "version") == nil {
		t.Fatal("expected xcode version command")
	}
	if findSubcommand(root, "xcode", "version", "view") == nil {
		t.Fatal("expected xcode version view command")
	}
	viewCmd := findSubcommand(root, "xcode", "version", "view")
	if viewCmd == nil {
		t.Fatal("expected xcode version view command")
		return
	}
	if viewCmd.FlagSet.Lookup("project") == nil {
		t.Fatal("expected xcode version view to expose --project")
	}
	if viewCmd.FlagSet.Lookup("xcodebuild-settings-lookup") == nil {
		t.Fatal("expected xcode version view to expose --xcodebuild-settings-lookup")
	}
	editCmd := findSubcommand(root, "xcode", "version", "edit")
	if editCmd == nil {
		t.Fatal("expected xcode version edit command")
		return
	}
	if editCmd.FlagSet.Lookup("project") == nil {
		t.Fatal("expected xcode version edit to expose --project")
	}
	if editCmd.FlagSet.Lookup("target") == nil {
		t.Fatal("expected xcode version edit to expose --target")
	}
	if editCmd.FlagSet.Lookup("configuration") == nil {
		t.Fatal("expected xcode version edit to expose --configuration")
	}
	if editCmd.FlagSet.Lookup("next-build-number") == nil {
		t.Fatal("expected xcode version edit to expose --next-build-number")
	}
	if editCmd.FlagSet.Lookup("xcodebuild-settings-lookup") == nil {
		t.Fatal("expected xcode version edit to expose --xcodebuild-settings-lookup")
	}
	if flag := editCmd.FlagSet.Lookup("allow-external-xcconfig"); flag == nil {
		t.Fatal("expected xcode version edit to expose --allow-external-xcconfig")
	}
	bumpCmd := findSubcommand(root, "xcode", "version", "bump")
	if bumpCmd == nil {
		t.Fatal("expected xcode version bump command")
		return
	}
	if bumpCmd.FlagSet.Lookup("project") == nil {
		t.Fatal("expected xcode version bump to expose --project")
	}
	if bumpCmd.FlagSet.Lookup("target") == nil {
		t.Fatal("expected xcode version bump to expose --target")
	}
	if bumpCmd.FlagSet.Lookup("configuration") == nil {
		t.Fatal("expected xcode version bump to expose --configuration")
	}
	if bumpCmd.FlagSet.Lookup("next-build-number") == nil {
		t.Fatal("expected xcode version bump to expose --next-build-number")
	}
	if bumpCmd.FlagSet.Lookup("xcodebuild-settings-lookup") == nil {
		t.Fatal("expected xcode version bump to expose --xcodebuild-settings-lookup")
	}
	if flag := bumpCmd.FlagSet.Lookup("allow-external-xcconfig"); flag == nil {
		t.Fatal("expected xcode version bump to expose --allow-external-xcconfig")
	}
	if findSubcommand(root, "xcode", "version", "get") != nil {
		t.Fatal("expected xcode version get command to be absent")
	}
	if findSubcommand(root, "xcode", "version", "set") != nil {
		t.Fatal("expected xcode version set command to be absent")
	}
}

func TestXcodeBuildRejectsBlankPassthroughValue(t *testing.T) {
	assertUsageExit(
		t,
		[]string{"xcode", "build", "--project", "Demo.xcodeproj", "--scheme", "Demo", "--xcodebuild-flag="},
		`invalid value "" for flag -xcodebuild-flag: value cannot be empty`,
	)
}

func TestXcodeVersionHelpShowsCanonicalSubcommands(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "version"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, want := range []string{"view", "edit", "bump", "asc xcode version view", "asc xcode version edit"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected help to contain %q, got %q", want, stderr)
		}
	}
	for _, hidden := range []string{"\n  get", "\n  set", "asc xcode version get", "asc xcode version set"} {
		if strings.Contains(stderr, hidden) {
			t.Fatalf("expected help to hide %q, got %q", hidden, stderr)
		}
	}
}

func TestXcodeVersionSettingsLookupInvalidValueReturnsUsageExitCode(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"xcode", "version", "view", "--xcodebuild-settings-lookup", "sometimes",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--xcodebuild-settings-lookup must be one of: auto, never") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitUsage)
	}
}

func TestXcodeExportHelpMentionsDirectUploadMode(t *testing.T) {
	root := RootCommand("1.2.3")

	exportCmd := findSubcommand(root, "xcode", "export")
	if exportCmd == nil {
		t.Fatal("expected xcode export command")
		return
	}
	if !strings.Contains(exportCmd.ShortHelp, "direct upload") {
		t.Fatalf("expected short help to mention direct upload, got %q", exportCmd.ShortHelp)
	}
	if !strings.Contains(exportCmd.LongHelp, "destination=upload") {
		t.Fatalf("expected long help to mention destination=upload, got %q", exportCmd.LongHelp)
	}
	if !strings.Contains(exportCmd.LongHelp, "without writing a local") {
		t.Fatalf("expected long help to explain no local IPA is written, got %q", exportCmd.LongHelp)
	}
	if !strings.Contains(exportCmd.LongHelp, "--timeout 10m") {
		t.Fatalf("expected long help to show local export timeout usage, got %q", exportCmd.LongHelp)
	}
	if got := exportCmd.FlagSet.Lookup("ipa-path").Usage; !strings.Contains(got, "when one is produced") {
		t.Fatalf("expected ipa-path usage to mention produced IPA behavior, got %q", got)
	}
	if got := exportCmd.FlagSet.Lookup("pkg-path").Usage; !strings.Contains(got, "macOS .pkg") {
		t.Fatalf("expected pkg-path usage to mention macOS PKG behavior, got %q", got)
	}
	if exportCmd.FlagSet.Lookup("timeout") == nil {
		t.Fatal("expected xcode export to expose --timeout")
	}
}

func TestXcodeInjectInvalidFlagValuesExitUsage(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "deployment.json")
	if err := os.WriteFile(manifestPath, []byte(`{
		"outputs": [
			{"type": "text", "path": "Generated.xcconfig", "contents": "VERSION = ${version}\n"}
		]
	}`), 0o644); err != nil {
		t.Fatalf("WriteFile() manifest error: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "malformed set",
			args:       []string{"xcode", "inject", "--manifest", manifestPath, "--set", "version"},
			wantStderr: "--set values must use key=value",
		},
		{
			name:       "invalid dry-run boolean",
			args:       []string{"xcode", "inject", "--manifest", manifestPath, "--dry-run=maybe"},
			wantStderr: `invalid boolean value "maybe" for -dry-run`,
		},
		{
			name:       "invalid overwrite boolean",
			args:       []string{"xcode", "inject", "--manifest", manifestPath, "--overwrite=inject"},
			wantStderr: `invalid boolean value "inject" for -overwrite`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantStderr)
		})
	}
}

func TestXcodeValidateHelpMentionsAltool(t *testing.T) {
	root := RootCommand("1.2.3")

	validateCmd := findSubcommand(root, "xcode", "validate")
	if validateCmd == nil {
		t.Fatal("expected xcode validate command")
		return
	}
	if !strings.Contains(validateCmd.LongHelp, "xcrun altool --validate-app") {
		t.Fatalf("expected long help to mention altool validation, got %q", validateCmd.LongHelp)
	}
	if validateCmd.FlagSet.Lookup("ipa") == nil {
		t.Fatal("expected xcode validate to expose --ipa")
	}
	if validateCmd.FlagSet.Lookup("api-key") == nil {
		t.Fatal("expected xcode validate to expose --api-key")
	}
	if validateCmd.FlagSet.Lookup("api-issuer") == nil {
		t.Fatal("expected xcode validate to expose --api-issuer")
	}
}

func TestXcodeArchiveRequiresWorkspaceOrProject(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "archive", "--scheme", "Demo", "--archive-path", "Demo.xcarchive"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: exactly one of --workspace or --project is required") {
		t.Fatalf("expected workspace/project error, got %q", stderr)
	}
}

func TestXcodeArchiveRejectsWorkspaceAndProjectTogether(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"xcode", "archive",
			"--workspace", "Demo.xcworkspace",
			"--project", "Demo.xcodeproj",
			"--scheme", "Demo",
			"--archive-path", "Demo.xcarchive",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: exactly one of --workspace or --project is required") {
		t.Fatalf("expected workspace/project error, got %q", stderr)
	}
}

func TestXcodeArchiveRequiresScheme(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "archive", "--project", "Demo.xcodeproj", "--archive-path", "Demo.xcarchive"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --scheme is required") {
		t.Fatalf("expected scheme error, got %q", stderr)
	}
}

func TestXcodeArchiveRequiresArchivePath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "archive", "--project", "Demo.xcodeproj", "--scheme", "Demo"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --archive-path is required") {
		t.Fatalf("expected archive-path error, got %q", stderr)
	}
}

func TestXcodeExportRequiresArchivePath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "export", "--export-options", "ExportOptions.plist", "--ipa-path", "Demo.ipa"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --archive-path is required") {
		t.Fatalf("expected archive-path error, got %q", stderr)
	}
}

func TestXcodeExportWithoutExportOptionsPreflightsBeforeGeneration(t *testing.T) {
	if runtime.GOOS == "darwin" {
		developerDir := filepath.Join(t.TempDir(), "Xcode.app", "Contents", "Developer")
		binDir := filepath.Join(developerDir, "usr", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("create fake developer directory: %v", err)
		}
		xcodebuildPath := filepath.Join(binDir, "xcodebuild")
		script := "#!/bin/sh\n" +
			"if [ \"$1\" = \"-version\" ]; then\n" +
			"  printf 'Xcode 16.0\\nBuild version 16A1\\n'\n" +
			"  exit 0\n" +
			"fi\n" +
			"printf 'unexpected xcodebuild invocation: %s\\n' \"$*\" >&2\n" +
			"exit 1\n"
		if err := os.WriteFile(xcodebuildPath, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake xcodebuild: %v", err)
		}
		fakeXcrun := filepath.Join(t.TempDir(), "xcrun")
		xcrunScript := "#!/bin/sh\nif [ \"$1\" = \"--find\" ] && [ \"$2\" = \"xcodebuild\" ]; then\n  printf '%s\\n' \"$DEVELOPER_DIR/usr/bin/xcodebuild\"\n  exit 0\nfi\nexit 2\n"
		if err := os.WriteFile(fakeXcrun, []byte(xcrunScript), 0o700); err != nil {
			t.Fatalf("write fake trusted xcrun: %v", err)
		}
		t.Cleanup(localxcode.OverrideTrustedXcrunPathForTesting(fakeXcrun))
		t.Setenv("DEVELOPER_DIR", developerDir)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--ipa-path", "Demo.ipa"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
		if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
			t.Fatalf("expected archive generation error, got %v", runErr)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.TrimSpace(stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runtime.GOOS != "darwin" {
		if !strings.Contains(runErr.Error(), "supported on macOS only") || !strings.Contains(runErr.Error(), runtime.GOOS) {
			t.Fatalf("expected platform preflight error, got %v", runErr)
		}
		return
	}
	if !strings.Contains(runErr.Error(), "generate export options") || !strings.Contains(runErr.Error(), "Demo.xcarchive") {
		t.Fatalf("expected automatic export-options generation error after preflight, got %v", runErr)
	}
}

func TestXcodeExportRejectsInvalidImplicitDestinationAsUsage(t *testing.T) {
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
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--ipa-path", tc.ipaPath(t)}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", runErr)
			}
			if stdout != "" || !strings.Contains(stderr, tc.errorHint) {
				t.Fatalf("unexpected output: stdout=%q stderr=%q", stdout, stderr)
			}
		})
	}
}

func TestXcodeValidateRequiresArtifact(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "validate"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --ipa or --pkg is required") {
		t.Fatalf("expected artifact error, got %q", stderr)
	}
}

func TestXcodeExportRequiresArtifactPath(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--export-options", "ExportOptions.plist"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --ipa-path or --pkg-path is required for a local export") {
		t.Fatalf("expected artifact path error, got %q", stderr)
	}
}
