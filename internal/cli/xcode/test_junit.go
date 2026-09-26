package xcode

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var (
	xcodeCommandGOOS    = runtime.GOOS
	loadXCResultSummary = func(ctx context.Context, path string) (*localxcode.TestSummary, error) {
		if xcodeCommandGOOS != "darwin" {
			return nil, fmt.Errorf("xcode test junit is supported on macOS only; current platform is %s", xcodeCommandGOOS)
		}
		return localxcode.ReadTestResultSummary(ctx, path)
	}
)

// SetXCResultSummaryLoaderForTesting replaces the xcresult reader. The restored
// loader still refuses non-macOS hosts.
func SetXCResultSummaryLoaderForTesting(fn func(context.Context, string) (*localxcode.TestSummary, error)) func() {
	previous := loadXCResultSummary
	if fn == nil {
		loadXCResultSummary = previous
	} else {
		loadXCResultSummary = fn
	}
	return func() { loadXCResultSummary = previous }
}

// XcodeTestJUnitCommand converts an existing .xcresult bundle to JUnit XML.
func XcodeTestJUnitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode test junit", flag.ExitOnError)
	xcresult := fs.String("xcresult", "", "Path to an existing .xcresult bundle")
	reportFile := fs.String("report-file", "", "Path to write the JUnit XML report")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "junit",
		ShortUsage: "asc xcode test junit --xcresult PATH --report-file PATH [flags]",
		ShortHelp:  "Convert an existing .xcresult bundle to JUnit XML.",
		LongHelp: `Convert an existing Xcode result bundle to JUnit XML without running tests.

The command reads the bundle with xcresulttool and writes the same report shape
as asc xcode test --report junit. It does not call App Store Connect.
The xcresulttool reads use the configured ASC timeout, defaulting to 30 seconds.
If aggregate summary loading succeeds but per-case enrichment fails, the command
fails closed and does not write a partial report.

Examples:
  asc xcode test junit --xcresult ./Test.xcresult --report-file ./junit.xml --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("xcode test junit does not accept positional arguments")
			}
			bundlePath := strings.TrimSpace(*xcresult)
			reportPath := strings.TrimSpace(*reportFile)
			if bundlePath == "" {
				return shared.UsageError("xcode test junit: --xcresult is required")
			}
			if reportPath == "" {
				return shared.UsageError("xcode test junit: --report-file is required")
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}
			readCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			summary, err := loadXCResultSummary(readCtx, bundlePath)
			if err != nil {
				return fmt.Errorf("xcode test junit: %w", err)
			}
			report := testResultJUnitReport(&localxcode.TestResult{
				Tests:            summary,
				ResultBundlePath: bundlePath,
				Success:          true,
				Action:           "junit",
			}, nil)
			report.Name = "asc xcode test junit"
			if err := report.Write(reportPath); err != nil {
				return fmt.Errorf("xcode test junit: %w", err)
			}
			receipt := junitReceipt(bundlePath, reportPath, report)
			return shared.PrintOutput(receipt, *output.Output, *output.Pretty)
		},
	}
}

func junitReceipt(bundlePath, reportPath string, report *shared.JUnitReport) *asc.XcodeJUnitResult {
	result := &asc.XcodeJUnitResult{
		XCResult:   bundlePath,
		ReportFile: reportPath,
	}
	if report == nil {
		return result
	}
	for _, testCase := range report.Tests {
		result.Tests++
		if testCase.Skipped {
			result.Skipped++
			continue
		}
		if testCase.Failure != "" {
			result.Failures++
			continue
		}
		result.Passed++
	}
	return result
}
