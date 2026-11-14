package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	xcodecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/xcode"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

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

func TestXcodeUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "ArchiveRequiresWorkspaceOrProject",
			args:    []string{"xcode", "archive", "--scheme", "Demo", "--archive-path", "Demo.xcarchive"},
			wantErr: "Error: exactly one of --workspace or --project is required",
		},
		{
			name:    "ArchiveRequiresScheme",
			args:    []string{"xcode", "archive", "--project", "Demo.xcodeproj", "--archive-path", "Demo.xcarchive"},
			wantErr: "Error: --scheme is required",
		},
		{
			name:    "ArchiveRequiresArchivePath",
			args:    []string{"xcode", "archive", "--project", "Demo.xcodeproj", "--scheme", "Demo"},
			wantErr: "Error: --archive-path is required",
		},
		{
			name:    "ExportRequiresArchivePath",
			args:    []string{"xcode", "export", "--export-options", "ExportOptions.plist", "--ipa-path", "Demo.ipa"},
			wantErr: "Error: --archive-path is required",
		},
		{
			name:    "ValidateRequiresArtifact",
			args:    []string{"xcode", "validate"},
			wantErr: "Error: --ipa or --pkg is required",
		},
		{
			name:    "ExportRequiresArtifactPath",
			args:    []string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--export-options", "ExportOptions.plist"},
			wantErr: "Error: --ipa-path or --pkg-path is required for a local export",
		},
		{
			name:    "ArchiveRejectsWorkspaceAndProjectTogether",
			args:    []string{"xcode", "archive", "--workspace", "Demo.xcworkspace", "--project", "Demo.xcodeproj", "--scheme", "Demo", "--archive-path", "Demo.xcarchive"},
			wantErr: "Error: exactly one of --workspace or --project is required",
		},
	})
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

func TestXcodeTestJUnitExitsNonZeroWhenBundleHasFailedTests(t *testing.T) {
	failing := &localxcode.TestSummary{Total: 2, Passed: 1, Failed: 1, Cases: []localxcode.TestCase{
		{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Status: "passed"},
		{Identifier: "DemoTests/Smoke/testFail", Name: "testFail", Status: "failed", Message: "expected true"},
	}}
	passing := &localxcode.TestSummary{Total: 1, Passed: 1, Cases: []localxcode.TestCase{
		{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Status: "passed"},
	}}
	// A failure the report cannot show: the passing cases alone fill the
	// report's case limit, so only the summary still says a test failed.
	manyPassing := make([]localxcode.TestCase, 10000)
	for index := range manyPassing {
		manyPassing[index] = localxcode.TestCase{Identifier: "DemoTests/Smoke/testPass" + strconv.Itoa(index), Name: "testPass" + strconv.Itoa(index), Status: "passed"}
	}
	unlisted := &localxcode.TestSummary{Total: len(manyPassing) + 1, Passed: len(manyPassing), Failed: 1, Cases: manyPassing}
	tests := []struct {
		name       string
		summary    *localxcode.TestSummary
		extraArgs  []string
		wantExit   int
		wantStderr string
	}{
		{name: "failed tests", summary: failing, wantExit: rootcmd.ExitError, wantStderr: "Error: xcode test junit: 1 failed test(s); report written to "},
		{name: "failed test missing from the report", summary: unlisted, wantExit: rootcmd.ExitError, wantStderr: "Error: xcode test junit: 1 failed test(s); report written to "},
		{name: "failed tests allowed", summary: failing, extraArgs: []string{"--allow-failures"}, wantExit: rootcmd.ExitSuccess},
		{name: "passing tests", summary: passing, wantExit: rootcmd.ExitSuccess},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Cleanup(xcodecli.SetXCResultSummaryLoaderForTesting(func(context.Context, string) (*localxcode.TestSummary, error) {
				return test.summary, nil
			}))
			reportFile := filepath.Join(t.TempDir(), "junit.xml")
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			args := append([]string{"xcode", "test", "junit", "--xcresult", "Test.xcresult", "--report-file", reportFile, "--output", "json"}, test.extraArgs...)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if got := rootcmd.ExitCodeFromError(runErr); got != test.wantExit {
				t.Fatalf("exit code = %d, want %d (err %v)", got, test.wantExit, runErr)
			}
			if !strings.Contains(stdout, `"reportFile":"`+reportFile+`"`) {
				t.Fatalf("stdout = %q, want JSON receipt", stdout)
			}
			if _, err := os.Stat(reportFile); err != nil {
				t.Fatalf("report not written: %v", err)
			}
			wantStderr := ""
			if test.wantStderr != "" {
				wantStderr = test.wantStderr + reportFile + "\n"
			}
			if stderr != wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, wantStderr)
			}
		})
	}
}
