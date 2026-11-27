package cmdtest

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	xcodecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/xcode"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeBuildRejectsBlankPassthroughValue(t *testing.T) {
	assertUsageExitWithDeps(
		t,
		nil,
		[]string{"xcode", "build", "--project", "Demo.xcodeproj", "--scheme", "Demo", "--xcodebuild-flag="},
		`invalid value "" for flag --xcodebuild-flag: value cannot be empty`,
	)
}

func TestXcodeVersionHelpShowsCanonicalSubcommands(t *testing.T) {
	stdout, stderr, err := runRootWithDeps(t, nil, "xcode", "version")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

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
	stdout, stderr, runErr := runRootWithDeps(t, nil,
		"xcode", "version", "view", "--xcodebuild-settings-lookup", "sometimes",
	)

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
			wantStderr: `invalid boolean value "maybe" for --dry-run`,
		},
		{
			name:       "invalid overwrite boolean",
			args:       []string{"xcode", "inject", "--manifest", manifestPath, "--overwrite=inject"},
			wantStderr: `invalid boolean value "inject" for --overwrite`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExitWithDeps(t, nil, test.args, test.wantStderr)
		})
	}
}

func TestXcodeUsageErrors(t *testing.T) {
	runUsageExitCases(t, []usageExitCase{
		{
			name:    "ArchiveRequiresWorkspaceOrProject",
			args:    []string{"xcode", "archive", "--scheme", "Demo", "--archive-path", "Demo.xcarchive"},
			wantErr: "Error: exactly one of --workspace or --project is required",
			deps:    true,
		},
		{
			name:    "ArchiveRequiresScheme",
			args:    []string{"xcode", "archive", "--project", "Demo.xcodeproj", "--archive-path", "Demo.xcarchive"},
			wantErr: "Error: --scheme is required",
			deps:    true,
		},
		{
			name:    "ArchiveRequiresArchivePath",
			args:    []string{"xcode", "archive", "--project", "Demo.xcodeproj", "--scheme", "Demo"},
			wantErr: "Error: --archive-path is required",
			deps:    true,
		},
		{
			name:    "ExportRequiresArchivePath",
			args:    []string{"xcode", "export", "--export-options", "ExportOptions.plist", "--ipa-path", "Demo.ipa"},
			wantErr: "Error: --archive-path is required",
			deps:    true,
		},
		{
			name:    "ValidateRequiresArtifact",
			args:    []string{"xcode", "validate"},
			wantErr: "Error: --ipa or --pkg is required",
			deps:    true,
		},
		{
			name:    "ExportRequiresArtifactPath",
			args:    []string{"xcode", "export", "--archive-path", "Demo.xcarchive", "--export-options", "ExportOptions.plist"},
			wantErr: "Error: --ipa-path or --pkg-path is required for a local export",
			deps:    true,
		},
		{
			name:    "ArchiveRejectsWorkspaceAndProjectTogether",
			args:    []string{"xcode", "archive", "--workspace", "Demo.xcworkspace", "--project", "Demo.xcodeproj", "--scheme", "Demo", "--archive-path", "Demo.xcarchive"},
			wantErr: "Error: exactly one of --workspace or --project is required",
			deps:    true,
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

	stdout, stderr, runErr := runRootWithDeps(t, nil, "xcode", "export", "--archive-path", "Demo.xcarchive", "--ipa-path", "Demo.ipa")
	if runErr == nil || errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected archive generation error, got %v", runErr)
	}

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
			stdout, stderr, runErr := runRootWithDeps(t, nil, "xcode", "export", "--archive-path", "Demo.xcarchive", "--ipa-path", tc.ipaPath(t))
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
			args := append([]string{"xcode", "test", "junit", "--xcresult", "Test.xcresult", "--report-file", reportFile, "--output", "json"}, test.extraArgs...)
			stdout, stderr, runErr := runRootWithDeps(t, nil, args...)

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

func TestXcodeTestTypedRunFlagsReachXcodebuild(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("xcode test runs xcodebuild on macOS only")
	}
	tests := []struct {
		name     string
		action   string
		args     []string
		wantArgs []string
		wantErr  string
	}{
		{name: "retry", args: []string{"--retry-tests-on-failure"}, wantArgs: []string{"-retry-tests-on-failure"}},
		{name: "iterations", args: []string{"--test-iterations", "5"}, wantArgs: []string{"-test-iterations", "5"}},
		{name: "parallel on", args: []string{"--parallel-testing=true"}, wantArgs: []string{"-parallel-testing-enabled", "YES"}},
		{name: "parallel off", args: []string{"--parallel-testing=false"}, wantArgs: []string{"-parallel-testing-enabled", "NO"}},
		{name: "workers", args: []string{"--parallel-testing-workers", "4"}, wantArgs: []string{"-parallel-testing-worker-count", "4"}},
		{name: "address sanitizer", args: []string{"--address-sanitizer"}, wantArgs: []string{"-enableAddressSanitizer", "YES"}},
		{name: "thread sanitizer", args: []string{"--thread-sanitizer"}, wantArgs: []string{"-enableThreadSanitizer", "YES"}},
		{name: "undefined behavior sanitizer", args: []string{"--undefined-behavior-sanitizer"}, wantArgs: []string{"-enableUndefinedBehaviorSanitizer", "YES"}},
		{name: "sanitizer while building for testing", action: "build-for-testing", args: []string{"--address-sanitizer"}, wantArgs: []string{"-enableAddressSanitizer", "YES"}},
		{name: "raw passthrough alone", args: []string{"--xcodebuild-flag=-parallel-testing-enabled", "--xcodebuild-flag=NO"}, wantArgs: []string{"-parallel-testing-enabled", "NO"}},
		{name: "address and thread sanitizers", args: []string{"--address-sanitizer", "--thread-sanitizer"}, wantErr: "--address-sanitizer and --thread-sanitizer cannot be used together"},
		{name: "zero iterations", args: []string{"--test-iterations", "0"}, wantErr: "--test-iterations must be at least 1"},
		{name: "typed and raw retry", args: []string{"--retry-tests-on-failure", "--xcodebuild-flag=-retry-tests-on-failure"}, wantErr: `--retry-tests-on-failure conflicts with --xcodebuild-flag "-retry-tests-on-failure"; use one`},
		{name: "retry while building for testing", action: "build-for-testing", args: []string{"--retry-tests-on-failure"}, wantErr: "--retry-tests-on-failure cannot be used with --action build-for-testing"},
		{name: "iterations while building for testing", action: "build-for-testing", args: []string{"--test-iterations", "2"}, wantErr: "--test-iterations cannot be used with --action build-for-testing"},
		{name: "workers while building for testing", action: "build-for-testing", args: []string{"--parallel-testing-workers", "2"}, wantErr: "--parallel-testing-workers cannot be used with --action build-for-testing"},
		{name: "workers with parallel testing off", args: []string{"--parallel-testing=false", "--parallel-testing-workers", "2"}, wantErr: "--parallel-testing-workers cannot be combined with --parallel-testing=false"},
		{name: "sanitizer without building", action: "test-without-building", args: []string{"--thread-sanitizer"}, wantErr: "--thread-sanitizer cannot be used with --action test-without-building"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := test.action
			if action == "" {
				action = "test"
			}
			developerDir := filepath.Join(t.TempDir(), "Xcode.app", "Contents", "Developer")
			binDir := filepath.Join(developerDir, "usr", "bin")
			if err := os.MkdirAll(binDir, 0o755); err != nil {
				t.Fatalf("create fake developer directory: %v", err)
			}
			recordPath := filepath.Join(t.TempDir(), "args.txt")
			script := "#!/bin/sh\n" +
				"if [ \"$1\" = \"-version\" ]; then\n" +
				"  printf 'Xcode 16.0\\nBuild version 16A1\\n'\n" +
				"  exit 0\n" +
				"fi\n" +
				"printf '%s\\n' \"$@\" > '" + recordPath + "'\n" +
				"while [ $# -gt 0 ]; do\n" +
				"  if [ \"$1\" = \"-resultBundlePath\" ]; then mkdir \"$2\" || exit 1; fi\n" +
				"  shift\n" +
				"done\n"
			if err := os.WriteFile(filepath.Join(binDir, "xcodebuild"), []byte(script), 0o755); err != nil {
				t.Fatalf("write fake xcodebuild: %v", err)
			}
			fakeXcrun := filepath.Join(t.TempDir(), "xcrun")
			xcrunScript := "#!/bin/sh\n" +
				"if [ \"$1\" = \"--find\" ] && [ \"$2\" = \"xcodebuild\" ]; then\n  printf '%s\\n' \"$DEVELOPER_DIR/usr/bin/xcodebuild\"\n  exit 0\nfi\n" +
				"if [ \"$1\" = \"xcresulttool\" ] && [ \"$4\" = \"summary\" ]; then\n  printf '%s' '{\"totalTestCount\":1,\"passedTests\":1,\"failedTests\":0,\"skippedTests\":0,\"durationMs\":1}'\n  exit 0\nfi\n" +
				"if [ \"$1\" = \"xcresulttool\" ] && [ \"$4\" = \"tests\" ]; then\n  printf '%s' '{\"testNodes\":[{\"nodeType\":\"Test Case\",\"nodeIdentifier\":\"Demo/test\",\"name\":\"test\",\"result\":\"Passed\"}]}'\n  exit 0\nfi\n" +
				"exit 2\n"
			if err := os.WriteFile(fakeXcrun, []byte(xcrunScript), 0o700); err != nil {
				t.Fatalf("write fake trusted xcrun: %v", err)
			}
			t.Cleanup(localxcode.OverrideTrustedXcrunPathForTesting(fakeXcrun))
			t.Setenv("DEVELOPER_DIR", developerDir)

			projectPath := filepath.Join(t.TempDir(), "Demo.xcodeproj")
			if err := os.Mkdir(projectPath, 0o755); err != nil {
				t.Fatalf("create project: %v", err)
			}
			derivedDataPath := filepath.Join(t.TempDir(), "DerivedData")
			resultBundlePath := filepath.Join(t.TempDir(), "Demo.xcresult")
			selector := []string{"--project", projectPath, "--scheme", "Demo", "--derived-data-path", derivedDataPath}
			if action == "test-without-building" {
				xctestrunPath := filepath.Join(t.TempDir(), "Demo.xctestrun")
				if err := os.WriteFile(xctestrunPath, []byte("<plist/>"), 0o644); err != nil {
					t.Fatalf("write xctestrun: %v", err)
				}
				selector = []string{"--xctestrun", xctestrunPath}
			}
			args := append([]string{"xcode", "test", "--action", action, "--destination", "platform=macOS", "--result-bundle-path", resultBundlePath, "--output", "json"}, selector...)
			args = append(args, test.args...)

			stdout, stderr, runErr := runRootWithDeps(t, nil, args...)
			recorded, readErr := os.ReadFile(recordPath)
			if test.wantErr != "" {
				if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d (err %v)", got, rootcmd.ExitUsage, runErr)
				}
				assertUsageExitOutput(t, stdout, stderr, test.wantErr)
				if readErr == nil {
					t.Fatalf("xcodebuild ran with %q, want no run", recorded)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("run error = %v, stderr = %q", runErr, stderr)
			}
			if readErr != nil {
				t.Fatalf("read recorded xcodebuild args: %v", readErr)
			}
			want := append([]string{
				"-hideShellScriptEnvironment",
				"-project", projectPath, "-scheme", "Demo", "-destination", "platform=macOS",
				"-derivedDataPath", derivedDataPath, "-resultBundlePath", resultBundlePath,
			}, test.wantArgs...)
			want = append(want, action)
			if got := strings.Split(strings.TrimSuffix(string(recorded), "\n"), "\n"); !slices.Equal(got, want) {
				t.Fatalf("xcodebuild args = %q, want %q", got, want)
			}
		})
	}
}
