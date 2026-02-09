package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/update"
)

// Run executes the CLI using the provided args (not including argv[0]) and version string.
// It returns the intended process exit code.
func Run(args []string, versionInfo string) int {
	root := RootCommand(versionInfo)
	defer CleanupTempPrivateKeys()

	if err := root.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ExitSuccess
		}
		fmt.Fprint(os.Stderr, errfmt.FormatStderr(err))
		return ExitCodeFromError(err)
	}

	// Validate CI report flags after parsing
	if err := shared.ValidateReportFlags(); err != nil {
		fmt.Fprint(os.Stderr, errfmt.FormatStderr(err))
		return ExitUsage
	}

	if versionRequested {
		if err := root.Run(context.Background()); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return ExitUsage
			}
			fmt.Fprint(os.Stderr, errfmt.FormatStderr(err))
			return ExitCodeFromError(err)
		}
		return ExitSuccess
	}

	updateResult, err := update.CheckAndUpdate(context.Background(), update.Options{
		CurrentVersion: versionInfo,
		AutoUpdate:     true,
		NoUpdate:       shared.NoUpdate(),
		Output:         os.Stderr,
		ShowProgress:   shared.ProgressEnabled(),
		CheckInterval:  24 * time.Hour,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
	}
	if updateResult.Updated {
		exitCode, restartErr := update.Restart(updateResult.ExecutablePath, os.Args, os.Environ())
		if restartErr != nil {
			fmt.Fprintf(os.Stderr, "Restart failed after update: %v\n", restartErr)
		} else {
			return exitCode
		}
	}

	start := time.Now()
	runErr := root.Run(context.Background())
	elapsed := time.Since(start)

	// Get command name (full subcommand path)
	commandName := getCommandName(root)

	// Write JUnit report if requested
	if shared.ReportFormat() == shared.ReportFormatJUnit && shared.ReportFile() != "" {
		reportErr := writeJUnitReport(commandName, runErr, elapsed)
		if reportErr != nil {
			// Report write failure is a hard error - CI depends on it
			fmt.Fprintf(os.Stderr, "Error: failed to write JUnit report: %v\n", reportErr)
			if runErr == nil {
				return ExitError
			}
		}
	}

	if runErr != nil {
		var reported ReportedError
		if errors.As(runErr, &reported) {
			return ExitCodeFromError(runErr)
		}
		if errors.Is(runErr, flag.ErrHelp) {
			return ExitUsage
		}
		fmt.Fprint(os.Stderr, errfmt.FormatStderr(runErr))
		return ExitCodeFromError(runErr)
	}

	return ExitSuccess
}

// getCommandName extracts the full subcommand path from a parsed command tree.
// After Parse, each command's FlagSet.Args() starts with the next subcommand name,
// so we walk the tree to reconstruct the path (e.g. "builds list").
func getCommandName(cmd *ffcli.Command) string {
	var parts []string
	current := cmd
	for {
		remaining := current.FlagSet.Args()
		if len(remaining) == 0 {
			break
		}
		found := false
		for _, sub := range current.Subcommands {
			if strings.EqualFold(remaining[0], sub.Name) {
				parts = append(parts, sub.Name)
				current = sub
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	if len(parts) == 0 {
		return cmd.Name
	}
	return strings.Join(parts, " ")
}

// writeJUnitReport writes a JUnit XML report if --report junit --report-file is configured.
func writeJUnitReport(commandName string, runErr error, elapsed time.Duration) error {
	reportFile := shared.ReportFile()
	if reportFile == "" {
		return nil
	}

	testCase := shared.JUnitTestCase{
		Name:      commandName,
		Classname: commandName,
		Time:      elapsed,
	}

	if runErr != nil {
		testCase.Failure = "ERROR"
		testCase.Message = runErr.Error()
	}

	report := shared.JUnitReport{
		Tests:     []shared.JUnitTestCase{testCase},
		Timestamp: time.Now(),
		Name:      "asc",
	}

	return report.Write(reportFile)
}
