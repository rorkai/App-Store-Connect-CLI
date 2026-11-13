package xcode

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var runToolchainDoctor = localxcode.InspectToolchain

// XcodeDoctorCommand returns the read-only local Xcode toolchain diagnostic.
func XcodeDoctorCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode doctor", flag.ExitOnError)

	developerDir := fs.String("developer-dir", "", "Xcode developer directory or .app to inspect (overrides DEVELOPER_DIR and xcode-select)")
	sdk := fs.String("sdk", "", "SDK name to resolve with xcrun (for example iphoneos or iphonesimulator)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "doctor",
		ShortUsage: "asc xcode doctor [flags]",
		ShortHelp:  "Verify the effective local Xcode toolchain without changing system state.",
		LongHelp: `Verify the effective local Xcode toolchain without changing system state.

The candidate is selected in this order: --developer-dir, a non-empty
DEVELOPER_DIR environment variable, then xcode-select --print-path. The
command checks the selected developer directory, xcodebuild -version, and
xcrun's xcodebuild resolution. Pass --sdk to add an SDK availability check.

This command is read-only. It does not switch the active developer directory,
install Xcode, invoke sudo, make network requests, or use App Store Connect
authentication. Xcode command output is written to stderr; the report is
written to stdout. Exit status is 0 for a passing or warning report, 1 when a
toolchain check fails, and 2 for invalid flags or positional arguments.

Examples:
  asc xcode doctor
  asc xcode doctor --developer-dir /Applications/Xcode.app --sdk iphonesimulator --output json --pretty
  DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer asc xcode doctor --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if emptyFlag := firstExplicitlyEmptyFlag(fs, "developer-dir", "sdk"); emptyFlag != "" {
				return toolchainUsageError(fmt.Sprintf("--%s must not be empty", emptyFlag), "--"+emptyFlag)
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return toolchainUsageError(err.Error(), "--output")
			}

			report, doctorErr := runToolchainDoctor(ctx, localxcode.ToolchainOptions{
				DeveloperDir: strings.TrimSpace(*developerDir),
				SDK:          strings.TrimSpace(*sdk),
				LogWriter:    os.Stderr,
			})
			if report != nil {
				if outputErr := shared.PrintOutput(toolchainReportOutput(report), *output.Output, *output.Pretty); outputErr != nil {
					if doctorErr != nil {
						return shared.NewErrorWithCause(outputErr, doctorErr)
					}
					return outputErr
				}
			}
			if doctorErr != nil {
				if report == nil {
					return fmt.Errorf("xcode doctor: %w", doctorErr)
				}
				fmt.Fprintln(os.Stderr, "Error: xcode doctor: toolchain checks failed")
				return shared.NewStderrReportedError(fmt.Errorf("xcode doctor: %w", doctorErr))
			}
			if report == nil {
				return fmt.Errorf("xcode doctor: inspector returned no report")
			}
			if report.Status == localxcode.ToolchainStatusFail {
				fmt.Fprintln(os.Stderr, "Error: xcode doctor: toolchain checks failed")
				return shared.NewStderrReportedError(fmt.Errorf("xcode doctor: toolchain checks failed"))
			}
			return nil
		},
	}
}

func toolchainUsageError(message, parameter string) error {
	trimmed := strings.TrimSpace(message)
	if trimmed != "" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", trimmed)
	}
	return shared.WithDiagnostic(
		shared.NewReportedUsageError(shared.UsageErrorInvalidValue, trimmed),
		shared.DiagnosticInvalidInput,
		parameter,
	)
}

func toolchainReportOutput(report *localxcode.ToolchainReport) *asc.XcodeToolchainDoctorResult {
	checks := make([]asc.XcodeToolchainDoctorCheck, 0, len(report.Checks))
	for _, check := range report.Checks {
		checks = append(checks, asc.XcodeToolchainDoctorCheck{
			Name:    check.Name,
			Status:  string(check.Status),
			Path:    check.Path,
			Message: check.Message,
		})
	}
	return &asc.XcodeToolchainDoctorResult{
		Status:       string(report.Status),
		Source:       string(report.Source),
		DeveloperDir: report.DeveloperDir,
		XcodePath:    report.XcodePath,
		XcodeVersion: report.XcodeVersion,
		XcodeBuild:   report.XcodeBuild,
		Beta:         report.Beta,
		Checks:       checks,
	}
}
