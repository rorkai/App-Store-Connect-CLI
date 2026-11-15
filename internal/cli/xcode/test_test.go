package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeTestPassesTypedOptionsAndPrintsJSON(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })

	var gotOpts localxcode.TestOptions
	runTest = func(_ context.Context, opts localxcode.TestOptions) (*localxcode.TestResult, error) {
		gotOpts = opts
		exitStatus := 0
		return &localxcode.TestResult{
			ProjectPath:      opts.ProjectPath,
			Scheme:           opts.Scheme,
			Action:           opts.Action,
			Configuration:    opts.Configuration,
			Destinations:     opts.Destinations,
			TestPlan:         opts.TestPlan,
			DerivedDataPath:  opts.DerivedDataPath,
			ResultBundlePath: opts.ResultBundlePath,
			Coverage: &localxcode.Coverage{LineCoverage: 0.75, CoveredLines: 30, ExecutableLines: 40, Targets: []localxcode.Coverage{
				{Name: "Demo.app", LineCoverage: 0.75, CoveredLines: 30, ExecutableLines: 40},
			}},
			Tests: &localxcode.TestSummary{
				Total:      2,
				Passed:     1,
				Failed:     1,
				DurationMS: 1250,
				Failures: []localxcode.TestFailure{{
					Identifier: "DemoTests/LoginTests/testInvalidPassword",
					Message:    "assertion failed",
				}},
			},
			Success:    true,
			DurationMS: 1400,
			ExitStatus: &exitStatus,
		}, nil
	}

	cmd := XcodeTestCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--project", "Demo App.xcodeproj",
		"--scheme", "Demo App",
		"--action", "test",
		"--configuration", "Debug",
		"--destination", "platform=iOS Simulator,name=iPhone 17 Pro",
		"--destination", "platform=iOS Simulator,name=iPad Pro",
		"--test-plan", "DemoTests",
		"--only-testing", "DemoTests/LoginTests",
		"--skip-testing", "DemoTests/FlakyTests",
		"--derived-data-path", "/tmp/Derived Data",
		"--result-bundle-path", "/tmp/Results/Demo.xcresult",
		"--clean",
		"--no-code-signing",
		"--code-coverage",
		"--xcodebuild-flag=-quiet",
		"--xcodebuild-flag=OTHER_SWIFT_FLAGS=-D ASC_TEST",
		"--output", "json",
	}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error = %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if gotOpts.ProjectPath != "Demo App.xcodeproj" || gotOpts.WorkspacePath != "" || gotOpts.Scheme != "Demo App" {
		t.Fatalf("unexpected selector options: %+v", gotOpts)
	}
	if gotOpts.Action != "test" || gotOpts.Configuration != "Debug" || gotOpts.TestPlan != "DemoTests" {
		t.Fatalf("unexpected action options: %+v", gotOpts)
	}
	if len(gotOpts.Destinations) != 2 || gotOpts.Destinations[1] != "platform=iOS Simulator,name=iPad Pro" {
		t.Fatalf("unexpected destinations: %#v", gotOpts.Destinations)
	}
	if len(gotOpts.OnlyTesting) != 1 || gotOpts.OnlyTesting[0] != "DemoTests/LoginTests" || len(gotOpts.SkipTesting) != 1 {
		t.Fatalf("unexpected test filters: %+v", gotOpts)
	}
	if !gotOpts.Clean || !gotOpts.NoCodeSigning || !gotOpts.CodeCoverage {
		t.Fatalf("expected clean, no-code-signing, and code-coverage: %+v", gotOpts)
	}
	wantRaw := []string{"-quiet", "OTHER_SWIFT_FLAGS=-D ASC_TEST"}
	if len(gotOpts.XcodebuildArgs) != len(wantRaw) || gotOpts.XcodebuildArgs[0] != wantRaw[0] || gotOpts.XcodebuildArgs[1] != wantRaw[1] {
		t.Fatalf("XcodebuildArgs = %#v, want %#v", gotOpts.XcodebuildArgs, wantRaw)
	}

	var payload asc.XcodeTestResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if payload.Action != "test" || payload.Tests == nil || payload.Tests.Total != 2 || payload.Tests.Failed != 1 || payload.DurationMs != 1400 {
		t.Fatalf("unexpected JSON payload: %+v", payload)
	}
	if !strings.Contains(stdout, `"coverage":{"lineCoverage":0.75,"coveredLines":30,"executableLines":40,"targets":[{"name":"Demo.app","lineCoverage":0.75,"coveredLines":30,"executableLines":40}]}`) {
		t.Fatalf("JSON output missing coverage: %s", stdout)
	}
	if strings.Contains(stdout, "duration_ms") || strings.Contains(stdout, "no_code_signing") {
		t.Fatalf("JSON output used legacy snake_case keys: %s", stdout)
	}
}

func TestXcodeTestPreservesSelectorWhitespace(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })
	var gotOpts localxcode.TestOptions
	runTest = func(_ context.Context, opts localxcode.TestOptions) (*localxcode.TestResult, error) {
		gotOpts = opts
		return &localxcode.TestResult{Action: opts.Action, Success: true}, nil
	}

	wantDestination := " platform=iOS Simulator,name=iPhone 17 Pro "
	wantOnlyTesting := " DemoTests/Smoke "
	wantSkipTesting := " DemoTests/Flaky "
	wantRawFlag := "  OTHER_SWIFT_FLAGS=-D ASC_TEST  "
	cmd := XcodeTestCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--destination", wantDestination,
		"--only-testing", wantOnlyTesting,
		"--skip-testing", wantSkipTesting,
		"--xcodebuild-flag", wantRawFlag,
		"--output", "json",
	}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	_, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error = %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if len(gotOpts.Destinations) != 1 || gotOpts.Destinations[0] != wantDestination {
		t.Fatalf("Destinations = %#v, want %#v", gotOpts.Destinations, []string{wantDestination})
	}
	if len(gotOpts.OnlyTesting) != 1 || gotOpts.OnlyTesting[0] != wantOnlyTesting {
		t.Fatalf("OnlyTesting = %#v, want %#v", gotOpts.OnlyTesting, []string{wantOnlyTesting})
	}
	if len(gotOpts.SkipTesting) != 1 || gotOpts.SkipTesting[0] != wantSkipTesting {
		t.Fatalf("SkipTesting = %#v, want %#v", gotOpts.SkipTesting, []string{wantSkipTesting})
	}
	if len(gotOpts.XcodebuildArgs) != 1 || gotOpts.XcodebuildArgs[0] != wantRawFlag {
		t.Fatalf("XcodebuildArgs = %#v, want %#v", gotOpts.XcodebuildArgs, []string{wantRawFlag})
	}
}

func TestXcodeTestValidationErrorsAreUsageErrors(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })
	runTest = func(context.Context, localxcode.TestOptions) (*localxcode.TestResult, error) {
		t.Fatal("runTest must not be called for invalid input")
		return nil, nil
	}

	packageWithoutManifest := t.TempDir()
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing selector", args: []string{"--scheme", "Demo", "--destination", "generic/platform=iOS"}, want: "exactly one of --workspace, --project, or --package-path"},
		{name: "both selectors", args: []string{"--project", "Demo.xcodeproj", "--workspace", "Demo.xcworkspace", "--scheme", "Demo", "--destination", "generic/platform=iOS"}, want: "exactly one of --workspace, --project, or --package-path"},
		{name: "package and project", args: []string{"--package-path", "DemoPackage", "--project", "Demo.xcodeproj", "--scheme", "Demo", "--destination", "generic/platform=iOS"}, want: "exactly one of --workspace, --project, or --package-path"},
		{name: "package without manifest", args: []string{"--package-path", packageWithoutManifest, "--scheme", "Demo", "--destination", "generic/platform=iOS"}, want: "--package-path must contain a Package.swift file"},
		{name: "missing scheme", args: []string{"--project", "Demo.xcodeproj", "--destination", "generic/platform=iOS"}, want: "--scheme is required"},
		{name: "missing destination", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo"}, want: "--destination is required"},
		{name: "invalid action", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--destination", "generic/platform=iOS", "--action", "archive"}, want: "--action must be one of"},
		{name: "without building missing xctestrun", args: []string{"--action", "test-without-building", "--destination", "generic/platform=iOS"}, want: "--xctestrun is required"},
		{name: "without building rejects configuration", args: []string{"--action", "test-without-building", "--xctestrun", "Demo.xctestrun", "--destination", "generic/platform=iOS", "--configuration", "Debug"}, want: "--configuration cannot be used"},
		{name: "without building rejects derived data path", args: []string{"--action", "test-without-building", "--xctestrun", "Demo.xctestrun", "--destination", "generic/platform=iOS", "--derived-data-path", "/tmp/DerivedData"}, want: "--derived-data-path cannot be used"},
		{name: "code coverage conflicts with raw flag", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--destination", "generic/platform=iOS", "--code-coverage", "--xcodebuild-flag=-enableCodeCoverage", "--xcodebuild-flag=YES"}, want: `--code-coverage conflicts with --xcodebuild-flag "-enableCodeCoverage"; use one`},
		{name: "reserved raw action", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--destination", "generic/platform=iOS", "--xcodebuild-flag=test"}, want: "cannot override asc-managed argument"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := XcodeTestCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestXcodeTestRejectsAuthenticationPassthroughUsageErrorsBeforeChild(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })
	runTest = func(context.Context, localxcode.TestOptions) (*localxcode.TestResult, error) {
		t.Fatal("runTest must not be called for invalid authentication passthrough input")
		return nil, nil
	}

	baseArgs := []string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--destination", "generic/platform=iOS",
	}
	tests := make([]struct {
		name string
		raw  []string
		want string
	}, 0, 20)
	for _, authFlag := range []string{"-authenticationKeyPath", "-authenticationKeyID", "-authenticationKeyIssuerID"} {
		tests = append(tests, struct {
			name string
			raw  []string
			want string
		}{
			name: authFlag + " missing value",
			raw:  []string{authFlag},
			want: fmt.Sprintf("--xcodebuild-flag %q requires a following value", authFlag),
		})
	}
	for _, next := range []string{"-authenticationKeyID", "-destination", "CODE_SIGNING_ALLOWED=NO", "clean", "test"} {
		tests = append(tests, struct {
			name string
			raw  []string
			want string
		}{
			name: "authentication value is " + next,
			raw:  []string{"-authenticationKeyPath", next},
			want: fmt.Sprintf("--xcodebuild-flag %q requires a value; %q is a recognized xcodebuild option or asc-managed argument", "-authenticationKeyPath", next),
		})
	}
	tests = append(
		tests,
		struct {
			name string
			raw  []string
			want string
		}{name: "empty value", raw: []string{"-authenticationKeyPath", ""}, want: "--xcodebuild-flag cannot be empty"},
		struct {
			name string
			raw  []string
			want string
		}{name: "control value", raw: []string{"-authenticationKeyPath", "AuthKey\x00.p8"}, want: "--xcodebuild-flag cannot contain control characters"},
	)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := XcodeTestCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			args := append([]string(nil), baseArgs...)
			for _, raw := range test.raw {
				args = append(args, "--xcodebuild-flag="+raw)
			}
			if err := cmd.FlagSet.Parse(args); err != nil {
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
			if runErr.Error() != test.want {
				t.Fatalf("Exec() error = %q, want %q", runErr, test.want)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "Error: "+test.want+"\n" {
				t.Fatalf("stderr = %q, want exact usage diagnostic", stderr)
			}
		})
	}
}

func TestXcodeTestPreservesAuthenticationPassthroughPairs(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })

	var gotOpts localxcode.TestOptions
	runTest = func(_ context.Context, opts localxcode.TestOptions) (*localxcode.TestResult, error) {
		gotOpts = opts
		return &localxcode.TestResult{Action: opts.Action, Success: true}, nil
	}

	raw := []string{
		"-authenticationKeyPath", "  /tmp/Auth Key.p8  ",
		"-authenticationKeyID", "Key ID 123",
		"-authenticationKeyIssuerID", "Issuer ID 456",
		"-quiet",
	}
	cmd := XcodeTestCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	args := []string{
		"--project", "Demo.xcodeproj",
		"--scheme", "Demo",
		"--destination", "generic/platform=iOS",
		"--no-code-signing",
		"--clean",
		"--output", "json",
	}
	for _, value := range raw {
		args = append(args, "--xcodebuild-flag="+value)
	}
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error = %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if !reflect.DeepEqual(gotOpts.XcodebuildArgs, raw) {
		t.Fatalf("XcodebuildArgs = %#v, want %#v", gotOpts.XcodebuildArgs, raw)
	}
	if !strings.Contains(stdout, `"success":true`) {
		t.Fatalf("stdout = %q, want successful structured output", stdout)
	}
}

func TestXcodeTestRendersTableAndMarkdown(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })
	runTest = func(_ context.Context, opts localxcode.TestOptions) (*localxcode.TestResult, error) {
		exitStatus := 0
		return &localxcode.TestResult{
			WorkspacePath: opts.WorkspacePath,
			Scheme:        opts.Scheme,
			Action:        opts.Action,
			Destinations:  opts.Destinations,
			Tests:         &localxcode.TestSummary{Total: 1, Passed: 1, DurationMS: 10},
			Success:       true,
			ExitStatus:    &exitStatus,
		}, nil
	}

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			cmd := XcodeTestCommand()
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--destination", "generic/platform=iOS", "--output", format,
			}); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			var runErr error
			stdout, stderr := captureCommandOutput(t, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if runErr != nil {
				t.Fatalf("Exec() error = %v", runErr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			for _, want := range []string{"workspace", "Demo.xcworkspace", "destination", "generic/platform=iOS", "tests", "success", "exit_status", "0"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output = %q, want %q", format, stdout, want)
				}
			}
		})
	}
}

func TestXcodeTestPrintsStructuredFailureBeforeReturningError(t *testing.T) {
	originalRunTest := runTest
	t.Cleanup(func() { runTest = originalRunTest })
	runTest = func(_ context.Context, opts localxcode.TestOptions) (*localxcode.TestResult, error) {
		_, _ = io.WriteString(opts.LogWriter, "test failed\n")
		exitStatus := 65
		return &localxcode.TestResult{
			ProjectPath:  opts.ProjectPath,
			Scheme:       opts.Scheme,
			Action:       opts.Action,
			Destinations: opts.Destinations,
			Tests:        &localxcode.TestSummary{Total: 1, Failed: 1, DurationMS: 400},
			Success:      false,
			DurationMS:   400,
			ExitStatus:   &exitStatus,
		}, errors.New("xcodebuild test failed: test failed")
	}

	cmd := XcodeTestCommand()
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--project", "Demo.xcodeproj", "--scheme", "Demo",
		"--destination", "generic/platform=iOS", "--output", "json",
	}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "xcodebuild test failed: test failed") {
		t.Fatalf("Exec() error = %v, want wrapped test failure", runErr)
	}
	var reportedErr shared.ReportedError
	if !errors.As(runErr, &reportedErr) {
		t.Fatalf("Exec() error = %T %v, want ReportedError", runErr, runErr)
	}
	if got := strings.Count(stderr, "test failed"); got != 2 {
		t.Fatalf("stderr = %q, test diagnostic count = %d, want stream and concise error", stderr, got)
	}
	if !strings.Contains(stderr, "Error: xcode test failed with exit status 65") {
		t.Fatalf("stderr = %q, want concise final test error", stderr)
	}
	var payload asc.XcodeTestResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if payload.Success || payload.ExitStatus == nil || *payload.ExitStatus != 65 || payload.Tests == nil || payload.Tests.Failed != 1 {
		t.Fatalf("unexpected failure payload: %+v", payload)
	}
}

func TestXcodeTestJUnitIncludesStructuredCases(t *testing.T) {
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests: &localxcode.TestSummary{
			Total:   3,
			Passed:  1,
			Failed:  1,
			Skipped: 1,
			Cases: []localxcode.TestCase{
				{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Classname: "DemoTests", Status: "passed", DurationMS: 250},
				{Identifier: "DemoTests/Smoke/testFail", Name: "testFail", Classname: "DemoTests", Status: "failed", Message: "assertion failed", DurationMS: 400},
				{Identifier: "DemoTests/Smoke/testSkip", Name: "testSkip", Classname: "DemoTests", Status: "skipped"},
			},
		},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if got := strings.Count(string(data), "<testcase "); got != 3 {
		t.Fatalf("JUnit testcase count = %d, want 3\n%s", got, data)
	}
	for _, want := range []string{"testPass", "testFail", "testSkip", "FAILURE", "assertion failed", "<skipped></skipped>"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("JUnit output = %s, want %q", data, want)
		}
	}
}

func TestXcodeTestJUnitReconcilesSummaryCountsWithoutDroppingCases(t *testing.T) {
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests: &localxcode.TestSummary{
			Total:   3,
			Passed:  1,
			Failed:  1,
			Skipped: 1,
			Cases: []localxcode.TestCase{
				{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Status: "passed"},
				{Identifier: "DemoTests/Smoke/testFail", Name: "testFail", Status: "failed", Message: "assertion failed"},
			},
		},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if got := strings.Count(string(data), "<testcase "); got != 3 {
		t.Fatalf("JUnit testcase count = %d, want 3\n%s", got, data)
	}
	for _, want := range []string{"testPass", "testFail", "aggregate-skipped", `failures="1"`, "<skipped></skipped>"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("JUnit output = %s, want %q", data, want)
		}
	}
}

func TestXcodeTestJUnitPreservesAggregateDurationWithFlattenedCases(t *testing.T) {
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests: &localxcode.TestSummary{
			Total:      3,
			Passed:     3,
			DurationMS: 1000,
			Cases: []localxcode.TestCase{
				{Identifier: "DemoTests/Smoke/testA", Name: "testA", Status: "passed", DurationMS: 100},
				{Identifier: "DemoTests/Smoke/testB", Name: "testB", Status: "passed", DurationMS: 100},
			},
		},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `time="1.000"`) {
		t.Fatalf("JUnit output = %s, want aggregate duration preserved", data)
	}
}

func TestXcodeTestJUnitSynthesizesAggregateFailuresBeforeFillingCap(t *testing.T) {
	// A multi-destination or repeated run can report more aggregate results
	// than flattened cases. Unrepresented passes must never consume the cap
	// ahead of unrepresented failures, or a failing run marshals as a passing
	// JUnit suite.
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: false,
		Tests: &localxcode.TestSummary{
			Total:  maxJUnitAggregateCases + 1,
			Passed: maxJUnitAggregateCases,
			Failed: 1,
		},
	}, &localxcode.ReportedTestFailuresError{Failed: 1})
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if strings.Contains(string(data), `failures="0"`) {
		t.Fatalf("JUnit output reported no failures for a failing run\n%s", string(data)[:min(len(string(data)), 400)])
	}
	if !strings.Contains(string(data), "aggregate-failed") {
		t.Fatalf("JUnit output = %s, want a synthesized failure within the cap", string(data)[:min(len(string(data)), 400)])
	}
}

func TestXcodeTestJUnitKeepsInfrastructureRowWhenReportHasNoFailure(t *testing.T) {
	// If parsed cases alone exhaust the cap and none of them failed, the
	// summary's failure count cannot be represented. The infrastructure row is
	// then the only remaining failure signal and must not be suppressed.
	cases := make([]localxcode.TestCase, maxJUnitAggregateCases)
	for index := range cases {
		cases[index] = localxcode.TestCase{
			Identifier: "DemoTests/Smoke/testPass" + strconv.Itoa(index),
			Name:       "testPass" + strconv.Itoa(index),
			Status:     "passed",
		}
	}
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: false,
		Tests: &localxcode.TestSummary{
			Total:  maxJUnitAggregateCases + 1,
			Passed: maxJUnitAggregateCases,
			Failed: 1,
			Cases:  cases,
		},
	}, &localxcode.ReportedTestFailuresError{Failed: 1})
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if strings.Contains(string(data), `failures="0"`) {
		t.Fatalf("JUnit output reported no failures for a failing run\n%s", string(data)[:min(len(string(data)), 400)])
	}
}

func TestXcodeTestJUnitPreservesAggregateDurationWithoutSyntheticCases(t *testing.T) {
	// Every aggregate test has a parsed case, so reconciliation adds no row.
	// The leaf durations still sum to less than the aggregate, and the suite
	// time must reflect the aggregate rather than the leaf sum.
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests: &localxcode.TestSummary{
			Total:      2,
			Passed:     2,
			DurationMS: 1000,
			Cases: []localxcode.TestCase{
				{Identifier: "DemoTests/Smoke/testA", Name: "testA", Status: "passed", DurationMS: 100},
				{Identifier: "DemoTests/Smoke/testB", Name: "testB", Status: "passed", DurationMS: 100},
			},
		},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `<testsuite name="asc xcode test" tests="2"`) {
		t.Fatalf("JUnit output = %s, want exactly the parsed cases", data)
	}
	if !strings.Contains(string(data), `time="1.000"`) {
		t.Fatalf("JUnit output = %s, want aggregate duration preserved without synthetic cases", data)
	}
}

func TestXcodeTestJUnitZeroSummaryProducesNoSyntheticCase(t *testing.T) {
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests:   &localxcode.TestSummary{},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if got := strings.Count(string(data), "<testcase "); got != 0 {
		t.Fatalf("JUnit testcase count = %d, want zero\n%s", got, data)
	}
	if !strings.Contains(string(data), `tests="0"`) || !strings.Contains(string(data), `failures="0"`) {
		t.Fatalf("JUnit output = %s, want zero summary", data)
	}
}

func TestXcodeTestJUnitPreservesInfrastructureFailureWithPassingCases(t *testing.T) {
	exitStatus := 65
	report := testResultJUnitReport(&localxcode.TestResult{
		Success:    false,
		ExitStatus: &exitStatus,
		Tests: &localxcode.TestSummary{
			Total:  1,
			Passed: 1,
			Cases:  []localxcode.TestCase{{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Status: "passed"}},
		},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), "testPass") || !strings.Contains(string(data), "xcodebuild exited with status 65") {
		t.Fatalf("JUnit output = %s, want passing case and infrastructure failure", data)
	}
	if !strings.Contains(string(data), `failures="1"`) {
		t.Fatalf("JUnit output = %s, want one infrastructure failure", data)
	}
}

func TestXcodeTestJUnitPreservesInfrastructureFailureWithFailedCases(t *testing.T) {
	tests := []struct {
		name       string
		exitStatus *int
		cause      error
	}{
		{name: "nonzero exit", exitStatus: func() *int { value := 65; return &value }(), cause: errors.New("xcodebuild stopped after a failing test")},
		{name: "cancellation", cause: context.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := testResultJUnitReport(&localxcode.TestResult{
				Success:    false,
				ExitStatus: test.exitStatus,
				Tests: &localxcode.TestSummary{
					Total:  1,
					Failed: 1,
					Cases:  []localxcode.TestCase{{Identifier: "DemoTests/Smoke/testFail", Name: "testFail", Status: "failed", Message: "assertion failed"}},
				},
			}, test.cause)
			data, err := report.Marshal()
			if err != nil {
				t.Fatalf("JUnit Marshal() error = %v", err)
			}
			if !strings.Contains(string(data), "testFail") || !strings.Contains(string(data), test.cause.Error()) {
				t.Fatalf("JUnit output = %s, want test and infrastructure causes", data)
			}
			if !strings.Contains(string(data), `failures="2"`) {
				t.Fatalf("JUnit output = %s, want test plus infrastructure failures", data)
			}
		})
	}
}

func TestXcodeTestJUnitDoesNotDuplicateOrdinaryExitErrorForRepresentedFailures(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 65").Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("command error = %T %v, want *exec.ExitError", err, err)
	}
	exitStatus := exitErr.ExitCode()
	report := testResultJUnitReport(&localxcode.TestResult{
		Success:    false,
		ExitStatus: &exitStatus,
		Tests: &localxcode.TestSummary{
			Total:  1,
			Failed: 1,
			Cases: []localxcode.TestCase{{
				Identifier: "DemoTests/Smoke/testFail",
				Name:       "testFail",
				Status:     "failed",
				Message:    "assertion failed",
			}},
		},
	}, err)
	data, marshalErr := report.Marshal()
	if marshalErr != nil {
		t.Fatalf("JUnit Marshal() error = %v", marshalErr)
	}
	if got := strings.Count(string(data), "<testcase "); got != 1 {
		t.Fatalf("JUnit testcase count = %d, want one represented test\n%s", got, data)
	}
	if !strings.Contains(string(data), `failures="1"`) || strings.Contains(string(data), "aggregate-failed") {
		t.Fatalf("JUnit output = %s, want only represented test failure", data)
	}
}

func TestXcodeTestJUnitDoesNotDuplicateReportedFailuresAfterZeroExit(t *testing.T) {
	// xcodebuild can exit zero while the parsed xcresult still reports failing
	// tests. Test() surfaces that as a typed post-processing error rather than
	// an *exec.ExitError, and those failures are already represented.
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: false,
		Tests: &localxcode.TestSummary{
			Total:  1,
			Failed: 1,
			Cases: []localxcode.TestCase{{
				Identifier: "DemoTests/Smoke/testFail",
				Name:       "testFail",
				Status:     "failed",
				Message:    "assertion failed",
			}},
		},
	}, &localxcode.ReportedTestFailuresError{Failed: 1})
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if got := strings.Count(string(data), "<testcase "); got != 1 {
		t.Fatalf("JUnit testcase count = %d, want one represented test\n%s", got, data)
	}
	if !strings.Contains(string(data), `failures="1"`) || strings.Contains(string(data), "aggregate-failed") {
		t.Fatalf("JUnit output = %s, want only represented test failure", data)
	}
}

func TestXcodeTestJUnitPreservesReportedFailuresErrorWithoutRepresentedFailures(t *testing.T) {
	// Defensive: if the typed error ever disagrees with the summary, keep the
	// synthetic row rather than silently dropping the failure signal.
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: false,
		Tests: &localxcode.TestSummary{
			Total:  1,
			Passed: 1,
			Cases:  []localxcode.TestCase{{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Status: "passed"}},
		},
	}, &localxcode.ReportedTestFailuresError{Failed: 1})
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), "aggregate-failed") || !strings.Contains(string(data), `failures="1"`) {
		t.Fatalf("JUnit output = %s, want preserved synthetic failure", data)
	}
}

func TestXcodeTestJUnitFallbackMarksSummaryFailure(t *testing.T) {
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests:   &localxcode.TestSummary{Total: 1, Failed: 1, DurationMS: 600},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `failures="1"`) || !strings.Contains(string(data), "FAILURE") {
		t.Fatalf("JUnit output = %s, want one failure", data)
	}
}

func TestXcodeTestJUnitPreservesPreflightCause(t *testing.T) {
	result := &localxcode.TestResult{}
	cause := errors.New("project path does not exist")
	report := testResultJUnitReport(result, cause)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), cause.Error()) {
		t.Fatalf("JUnit output = %s, want actionable preflight cause", data)
	}
}

func TestXcodeTestJUnitFailureTruncationPreservesUTF8(t *testing.T) {
	message := strings.Repeat("a", maxJUnitFailureMessage-1) + "é"
	got := boundJUnitFailureMessage(message)
	if len(got) != maxJUnitFailureMessage-1 || !utf8.ValidString(got) {
		t.Fatalf("bounded message length/encoding = %d/%v, want %d/valid UTF-8", len(got), utf8.ValidString(got), maxJUnitFailureMessage-1)
	}
}

func TestXcodeTestJUnitTreatsExpectedFailuresAsNonfailing(t *testing.T) {
	report := testResultJUnitReport(&localxcode.TestResult{
		Success: true,
		Tests: &localxcode.TestSummary{
			Total:            2,
			Passed:           1,
			ExpectedFailures: 1,
			Cases: []localxcode.TestCase{
				{Identifier: "DemoTests/Smoke/testPass", Name: "testPass", Status: "passed"},
				{Identifier: "DemoTests/Smoke/testKnownIssue", Name: "testKnownIssue", Status: "expected-failure"},
			},
		},
	}, nil)
	data, err := report.Marshal()
	if err != nil {
		t.Fatalf("JUnit Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `tests="2"`) || !strings.Contains(string(data), `failures="0"`) {
		t.Fatalf("JUnit output = %s, want expected failure treated as nonfailing", data)
	}
	if got := strings.Count(string(data), "<testcase "); got != 2 {
		t.Fatalf("JUnit testcase count = %d, want 2\n%s", got, data)
	}
}
