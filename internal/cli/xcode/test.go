package xcode

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var runTest = localxcode.Test

const maxJUnitAggregateCases = 10000

// exactTestStringFlag keeps Xcode's structured selector syntax byte-for-byte
// intact. Unlike generic repeatable flags, destination and test identifiers
// may intentionally contain spaces that must reach xcodebuild unchanged.
type exactTestStringFlag []string

func (f *exactTestStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *exactTestStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// XcodeTestCommand returns the local Xcode test command.
func XcodeTestCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode test", flag.ExitOnError)

	workspacePath := fs.String("workspace", "", "Path to .xcworkspace directory")
	projectPath := fs.String("project", "", "Path to .xcodeproj directory")
	scheme := fs.String("scheme", "", "Xcode scheme name (required except for test-without-building)")
	action := fs.String("action", string(localxcode.TestActionTest), "Xcode test action: test, build-for-testing, or test-without-building")
	configuration := fs.String("configuration", "", "Build configuration (for example Debug or Release)")
	var destinations exactTestStringFlag
	fs.Var(&destinations, "destination", "Xcode destination specifier (repeatable; required)")
	testPlan := fs.String("test-plan", "", "Xcode test plan name")
	xctestrunPath := fs.String("xctestrun", "", "Path to an existing .xctestrun file for test-without-building")
	var onlyTesting exactTestStringFlag
	fs.Var(&onlyTesting, "only-testing", "Run only the selected test target or identifier (repeatable)")
	var skipTesting exactTestStringFlag
	fs.Var(&skipTesting, "skip-testing", "Skip the selected test target or identifier (repeatable)")
	derivedDataPath := fs.String("derived-data-path", "", "DerivedData directory (defaults to a stable asc cache path)")
	resultBundlePath := fs.String("result-bundle-path", "", "Destination for a new Xcode result bundle")
	clean := fs.Bool("clean", false, "Run clean before the selected Xcode action")
	noCodeSigning := fs.Bool("no-code-signing", false, "Set CODE_SIGNING_ALLOWED=NO explicitly")
	var xcodebuildFlags exactTestStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass a raw argument through to xcodebuild (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "test",
		ShortUsage: "asc xcode test [flags]",
		ShortHelp:  "Run local Xcode tests and report structured results.",
		LongHelp: `Run a local Xcode test action and report structured results.

For test and build-for-testing, provide exactly one of --workspace or --project,
plus --scheme and at least one --destination. The default action is test. Use
--action build-for-testing to produce test products, or use
--action test-without-building with an existing --xctestrun file. Test actions
write a new .xcresult bundle automatically when --result-bundle-path is omitted.
The test-without-building action rejects project/build controls, including
--configuration and --derived-data-path. Convert an existing result bundle
with asc xcode test junit.

Xcode diagnostics are written to stderr and the selected structured result
format is written to stdout. This command never calls App Store Connect or
changes project files.

Examples:
  asc xcode test --project App.xcodeproj --scheme App --destination 'platform=iOS Simulator,name=iPhone 17 Pro' --output json
  asc xcode test --workspace App.xcworkspace --scheme App --destination 'platform=iOS Simulator,name=iPhone 17 Pro' --destination 'platform=iOS Simulator,name=iPad Pro (13-inch)'
  asc xcode test --project App.xcodeproj --scheme App --action build-for-testing --destination 'platform=iOS Simulator,name=iPhone 17 Pro'
  asc xcode test --action test-without-building --xctestrun App_iphonesimulator.xctestrun --destination 'platform=iOS Simulator,name=iPhone 17 Pro'`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeTestJUnitCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode test does not accept positional arguments")
				return flag.ErrHelp
			}
			if emptyFlag := firstExplicitlyEmptyFlag(fs, "workspace", "project", "scheme", "action", "configuration", "test-plan", "xctestrun", "derived-data-path", "result-bundle-path"); emptyFlag != "" {
				return shared.UsageErrorf("--%s must not be empty", emptyFlag)
			}
			opts := localxcode.TestOptions{
				WorkspacePath:    strings.TrimSpace(*workspacePath),
				ProjectPath:      strings.TrimSpace(*projectPath),
				Scheme:           strings.TrimSpace(*scheme),
				Action:           strings.TrimSpace(*action),
				Configuration:    strings.TrimSpace(*configuration),
				Destinations:     []string(destinations),
				TestPlan:         strings.TrimSpace(*testPlan),
				XctestrunPath:    strings.TrimSpace(*xctestrunPath),
				OnlyTesting:      []string(onlyTesting),
				SkipTesting:      []string(skipTesting),
				DerivedDataPath:  strings.TrimSpace(*derivedDataPath),
				ResultBundlePath: strings.TrimSpace(*resultBundlePath),
				Clean:            *clean,
				NoCodeSigning:    *noCodeSigning,
				XcodebuildArgs:   []string(xcodebuildFlags),
				LogWriter:        os.Stderr,
			}
			if err := localxcode.ValidateTestOptions(opts); err != nil {
				return shared.UsageError(err.Error())
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			result, testErr := runTest(ctx, opts)
			if result != nil {
				if outputErr := printTestResult(result, *output.Output, *output.Pretty); outputErr != nil {
					if testErr != nil {
						return shared.NewErrorWithCause(outputErr, testErr)
					}
					return outputErr
				}
				if shared.ReportFormat() == shared.ReportFormatJUnit && shared.ReportFile() != "" {
					shared.SetJUnitReport(testResultJUnitReport(result, testErr))
				}
			}
			if testErr != nil {
				if result == nil {
					return fmt.Errorf("xcode test: %w", testErr)
				}
				reportTestFailure(result, testErr)
				return shared.NewReportedError(fmt.Errorf("xcode test: %w", testErr))
			}
			if result == nil {
				return fmt.Errorf("xcode test: tester returned no result")
			}
			return nil
		},
	}
}

func reportTestFailure(result *localxcode.TestResult, testErr error) {
	message := "xcode test failed"
	if result.ExitStatus != nil {
		message = fmt.Sprintf("%s with exit status %d", message, *result.ExitStatus)
	} else {
		message = fmt.Sprintf("%s: %v", message, testInterruptionReason(testErr))
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
}

func testInterruptionReason(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() < 0 {
		return exitErr
	}
	return err
}

func printTestResult(result *localxcode.TestResult, output string, pretty bool) error {
	return shared.PrintOutput(toXcodeTestResult(result), output, pretty)
}

func toXcodeTestResult(result *localxcode.TestResult) *asc.XcodeTestResult {
	if result == nil {
		return nil
	}
	output := &asc.XcodeTestResult{
		Workspace:        result.WorkspacePath,
		Project:          result.ProjectPath,
		Scheme:           result.Scheme,
		Action:           result.Action,
		Configuration:    result.Configuration,
		Destinations:     append([]string(nil), result.Destinations...),
		TestPlan:         result.TestPlan,
		XctestrunPath:    result.XctestrunPath,
		DerivedDataPath:  result.DerivedDataPath,
		ResultBundlePath: result.ResultBundlePath,
		Clean:            result.Clean,
		NoCodeSigning:    result.NoCodeSigning,
		Success:          result.Success,
		DurationMs:       result.DurationMS,
		ExitStatus:       result.ExitStatus,
	}
	if result.Tests == nil {
		return output
	}
	output.Tests = &asc.XcodeTestSummary{
		Total:            result.Tests.Total,
		Passed:           result.Tests.Passed,
		Failed:           result.Tests.Failed,
		Skipped:          result.Tests.Skipped,
		ExpectedFailures: result.Tests.ExpectedFailures,
		DurationMs:       result.Tests.DurationMS,
	}
	if len(result.Tests.Cases) > 0 {
		output.Tests.Cases = make([]asc.XcodeTestCase, 0, len(result.Tests.Cases))
		for _, testCase := range result.Tests.Cases {
			output.Tests.Cases = append(output.Tests.Cases, asc.XcodeTestCase{
				Identifier: testCase.Identifier,
				Name:       testCase.Name,
				Classname:  testCase.Classname,
				Status:     testCase.Status,
				DurationMs: testCase.DurationMS,
				Message:    testCase.Message,
			})
		}
	}
	if len(result.Tests.Failures) > 0 {
		output.Tests.Failures = make([]asc.XcodeTestFailure, 0, len(result.Tests.Failures))
		for _, failure := range result.Tests.Failures {
			output.Tests.Failures = append(output.Tests.Failures, asc.XcodeTestFailure{
				Identifier: failure.Identifier,
				Message:    failure.Message,
			})
		}
	}
	return output
}

const maxJUnitFailureMessage = 4096

func testResultJUnitReport(result *localxcode.TestResult, commandErr error) *shared.JUnitReport {
	report := &shared.JUnitReport{Timestamp: time.Now(), Name: "asc xcode test"}
	if result == nil {
		return report
	}

	var summary *localxcode.TestSummary
	if result.Tests != nil {
		summary = result.Tests
	}
	tests := make([]shared.JUnitTestCase, 0)
	if summary != nil {
		capacity := len(summary.Cases)
		if remaining := maxJUnitAggregateCases - capacity; remaining > 0 {
			capacity += min(remaining, max(0, summary.Total))
		}
		tests = make([]shared.JUnitTestCase, 0, capacity)
	}
	for _, testCase := range summaryCases(summary) {
		name := testCase.Name
		if name == "" {
			name = testCase.Identifier
		}
		if name == "" {
			name = "unnamed-test"
		}
		failure := ""
		message := ""
		skipped := strings.EqualFold(testCase.Status, "skipped")
		if strings.EqualFold(testCase.Status, "failed") {
			failure = "FAILURE"
			message = testCase.Message
			if strings.TrimSpace(message) == "" {
				message = testFailureMessage(result.Tests, testCase.Identifier)
			}
		}
		tests = append(tests, shared.JUnitTestCase{
			Name:      name,
			Classname: testCase.Classname,
			Time:      durationFromMilliseconds(testCase.DurationMS),
			Skipped:   skipped,
			Failure:   failure,
			Message:   message,
		})
	}

	if summary != nil {
		actualPassed, actualFailed, actualSkipped := junitStatusCounts(tests)
		missingPassed := max(0, summary.Passed+summary.ExpectedFailures-actualPassed)
		missingFailed := max(0, summary.Failed-actualFailed)
		missingSkipped := max(0, summary.Skipped-actualSkipped)
		syntheticCount := 0
		// Synthesize failures first. Aggregate counts can exceed the flattened
		// cases for multi-destination and repeated runs, and unrepresented
		// passes must never consume the cap ahead of unrepresented failures, or
		// a failing invocation would marshal as a passing JUnit suite.
		for index := 0; index < missingFailed && len(tests) < maxJUnitAggregateCases; index++ {
			tests = append(tests, syntheticJUnitTestCase("failed", syntheticCount, "test action reported failure"))
			syntheticCount++
		}
		for index := 0; index < missingSkipped && len(tests) < maxJUnitAggregateCases; index++ {
			tests = append(tests, syntheticJUnitTestCase("skipped", syntheticCount, ""))
			syntheticCount++
		}
		for index := 0; index < missingPassed && len(tests) < maxJUnitAggregateCases; index++ {
			tests = append(tests, syntheticJUnitTestCase("passed", syntheticCount, ""))
			syntheticCount++
		}
		// JUnit derives suite time by summing testcase durations, which drops
		// setup, teardown, and repeated or multi-destination work. Report the
		// aggregate at the suite level so it survives whether or not
		// reconciliation synthesized any row, and leave every parsed case's own
		// duration untouched.
		report.Duration = durationFromMilliseconds(summary.DurationMS)
	}

	if shouldAddJUnitInfrastructureFailure(result, summary, tests, commandErr) {
		message := "xcode test did not complete successfully"
		if commandErr != nil {
			message = boundJUnitFailureMessage(commandErr.Error())
		} else if result.ExitStatus != nil && *result.ExitStatus != 0 {
			message = fmt.Sprintf("xcodebuild exited with status %d", *result.ExitStatus)
		}
		tests = append(tests, syntheticJUnitTestCase("failed", len(tests), message))
	}
	report.Tests = tests
	return report
}

func summaryCases(summary *localxcode.TestSummary) []localxcode.TestCase {
	if summary == nil {
		return nil
	}
	return summary.Cases
}

func junitStatusCounts(tests []shared.JUnitTestCase) (passed, failed, skipped int) {
	for _, testCase := range tests {
		switch {
		case testCase.Skipped:
			skipped++
		case testCase.Failure != "":
			failed++
		default:
			passed++
		}
	}
	return passed, failed, skipped
}

func shouldAddJUnitInfrastructureFailure(result *localxcode.TestResult, summary *localxcode.TestSummary, tests []shared.JUnitTestCase, commandErr error) bool {
	if result == nil || result.Success {
		return false
	}
	if summary == nil {
		return true
	}
	if errors.Is(commandErr, context.Canceled) || errors.Is(commandErr, context.DeadlineExceeded) {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(commandErr, &exitErr) {
		if exitErr.ExitCode() < 0 {
			return true
		}
		// A normal nonzero xcodebuild exit is commonly the result of test
		// failures. When the report already carries those failures, adding an
		// infrastructure testcase would double-count the same outcome. Keep a
		// synthetic testcase only when the exit has no represented failure.
		return !junitReportsFailure(tests)
	}
	// xcodebuild can also exit zero while the parsed xcresult reports failures.
	// That post-processing cause is typed, so it is distinguishable from real
	// infrastructure errors and must not double-count represented failures.
	var reportedFailures *localxcode.ReportedTestFailuresError
	if errors.As(commandErr, &reportedFailures) {
		return !junitReportsFailure(tests)
	}
	// Preserve a synthetic row for generic errors and typed infrastructure
	// failures even when the summary contains ordinary failed test cases.
	return true
}

// junitReportsFailure asks whether the emitted report already carries a failing
// testcase. Suppression is deliberately based on the report rather than the
// summary: the aggregate case cap can leave a summary-reported failure with no
// row of its own, and in that case the infrastructure row is the only remaining
// failure signal.
func junitReportsFailure(tests []shared.JUnitTestCase) bool {
	_, failed, _ := junitStatusCounts(tests)
	return failed > 0
}

func syntheticJUnitTestCase(status string, index int, message string) shared.JUnitTestCase {
	testCase := shared.JUnitTestCase{
		Name:      fmt.Sprintf("aggregate-%s-%d", status, index+1),
		Classname: "asc xcode test",
		Message:   message,
	}
	switch status {
	case "failed":
		testCase.Failure = "FAILURE"
	case "skipped":
		testCase.Skipped = true
	}
	return testCase
}

func testFailureMessage(summary *localxcode.TestSummary, identifier string) string {
	if summary == nil {
		return ""
	}
	for _, failure := range summary.Failures {
		if failure.Identifier == identifier {
			return failure.Message
		}
	}
	return ""
}

func durationFromMilliseconds(value int64) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func boundJUnitFailureMessage(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxJUnitFailureMessage {
		return value
	}
	value = value[:maxJUnitFailureMessage]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
