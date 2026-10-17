package xcode

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var runBuild = localxcode.Build

// XcodeBuildCommand returns the ordinary local build command.
func XcodeBuildCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode build", flag.ExitOnError)

	workspacePath := fs.String("workspace", "", "Path to .xcworkspace directory")
	projectPath := fs.String("project", "", "Path to .xcodeproj directory")
	scheme := fs.String("scheme", "", "Xcode scheme name (required)")
	configuration := fs.String("configuration", "", "Build configuration (for example Debug or Release)")
	destination := fs.String("destination", "", "Xcode destination specifier")
	derivedDataPath := fs.String("derived-data-path", "", "DerivedData directory (defaults to a stable asc cache path)")
	resultBundlePath := fs.String("result-bundle-path", "", "Destination for a new Xcode result bundle (must not already exist)")
	clean := fs.Bool("clean", false, "Run clean before build")
	noCodeSigning := fs.Bool("no-code-signing", false, "Set CODE_SIGNING_ALLOWED=NO explicitly")
	var xcodebuildFlags shared.MultiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass a raw argument through to xcodebuild (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: "asc xcode build [flags]",
		ShortHelp:  "Compile an Xcode scheme for a simulator or device.",
		LongHelp: `Compile an Xcode scheme with an ordinary xcodebuild build action.

Provide exactly one of --workspace or --project, plus --scheme. Use
--destination to select a simulator or device. Signing follows Xcode defaults;
--no-code-signing explicitly sets CODE_SIGNING_ALLOWED=NO for validation builds.

When --derived-data-path is omitted, asc uses a stable cache path outside the
source checkout. Use --result-bundle-path to write a new .xcresult bundle; the
destination must not already exist. Xcode logs are written to stderr and the
selected structured result format is written to stdout.

Examples:
  asc xcode build --project App.xcodeproj --scheme App
  asc xcode build --project App.xcodeproj --scheme App --destination 'platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0' --no-code-signing --output json
  asc xcode build --workspace App.xcworkspace --scheme App --configuration Release --destination 'generic/platform=iOS' --derived-data-path /tmp/AppDerivedData --result-bundle-path /tmp/App.xcresult
  asc xcode build --project App.xcodeproj --scheme App --xcodebuild-flag=-quiet --xcodebuild-flag=SWIFT_ACTIVE_COMPILATION_CONDITIONS=CI`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode build does not accept positional arguments")
				return flag.ErrHelp
			}
			opts := localxcode.BuildOptions{
				WorkspacePath:    strings.TrimSpace(*workspacePath),
				ProjectPath:      strings.TrimSpace(*projectPath),
				Scheme:           strings.TrimSpace(*scheme),
				Configuration:    strings.TrimSpace(*configuration),
				Destination:      strings.TrimSpace(*destination),
				DerivedDataPath:  strings.TrimSpace(*derivedDataPath),
				ResultBundlePath: strings.TrimSpace(*resultBundlePath),
				Clean:            *clean,
				NoCodeSigning:    *noCodeSigning,
				XcodebuildArgs:   []string(xcodebuildFlags),
				LogWriter:        os.Stderr,
			}
			if err := localxcode.ValidateBuildOptions(opts); err != nil {
				return shared.UsageError(err.Error())
			}
			if emptyFlag := firstExplicitlyEmptyFlag(fs, "configuration", "destination", "derived-data-path", "result-bundle-path"); emptyFlag != "" {
				return shared.UsageErrorf("--%s must not be empty", emptyFlag)
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return shared.UsageError(err.Error())
			}

			result, buildErr := runBuild(ctx, opts)
			if result != nil {
				if outputErr := printBuildResult(result, *output.Output, *output.Pretty); outputErr != nil {
					if buildErr != nil {
						return shared.NewErrorWithCause(outputErr, buildErr)
					}
					return outputErr
				}
			}
			if buildErr != nil {
				if result == nil {
					return xcodeCommandError("xcode build", buildErr)
				}
				reportBuildFailure(result, buildErr)
				return shared.NewStderrReportedError(classifyXcodeLocalFailure(fmt.Errorf("xcode build: %w", buildErr)))
			}
			if result == nil {
				return fmt.Errorf("xcode build: builder returned no result")
			}
			return nil
		},
	}
}

func reportBuildFailure(result *localxcode.BuildResult, buildErr error) {
	message := "xcode build failed"
	if result.ExitStatus != nil {
		message = fmt.Sprintf("%s with exit status %d", message, *result.ExitStatus)
	} else {
		message = fmt.Sprintf("%s: %v", message, buildInterruptionReason(buildErr))
	}
	fmt.Fprintf(os.Stderr, "Error: %s\n", message)
}

func buildInterruptionReason(err error) error {
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

func printBuildResult(result *localxcode.BuildResult, output string, pretty bool) error {
	return shared.PrintOutputWithRenderers(
		result,
		output,
		pretty,
		func() error {
			asc.RenderTable([]string{"field", "value"}, buildResultRows(result))
			return nil
		},
		func() error {
			asc.RenderMarkdown([]string{"field", "value"}, buildResultRows(result))
			return nil
		},
	)
}

func buildResultRows(result *localxcode.BuildResult) [][]string {
	rows := make([][]string, 0, 15)
	if result.WorkspacePath != "" {
		rows = append(rows, []string{"workspace", result.WorkspacePath})
	}
	if result.ProjectPath != "" {
		rows = append(rows, []string{"project", result.ProjectPath})
	}
	rows = append(rows, []string{"scheme", result.Scheme})
	if result.Configuration != "" {
		rows = append(rows, []string{"configuration", result.Configuration})
	}
	if result.Destination != "" {
		rows = append(rows, []string{"destination", result.Destination})
	}
	rows = append(rows, []string{"derived_data_path", result.DerivedDataPath})
	if result.ResultBundlePath != "" {
		rows = append(rows, []string{"result_bundle_path", result.ResultBundlePath})
	}
	if result.BuildProductsPath != "" {
		rows = append(rows, []string{"build_products_path", result.BuildProductsPath})
	}
	rows = append(
		rows,
		[]string{"clean", fmt.Sprintf("%t", result.Clean)},
		[]string{"no_code_signing", fmt.Sprintf("%t", result.NoCodeSigning)},
		[]string{"success", fmt.Sprintf("%t", result.Success)},
		[]string{"duration_ms", fmt.Sprintf("%d", result.DurationMS)},
	)
	if result.ExitStatus != nil {
		rows = append(rows, []string{"exit_status", fmt.Sprintf("%d", *result.ExitStatus)})
	}
	return rows
}
