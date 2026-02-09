package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

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
	commandName := root.Name
	if commandName == "" {
		commandName = "asc"
	}

	// Write JUnit report if requested
	if reportErr := writeJUnitReport(commandName, runErr, elapsed); reportErr != nil {
		// If command succeeded but report failed, warn but don't change exit
		if runErr == nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write JUnit report: %v\n", reportErr)
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

// writeJUnitReport writes a JUnit XML report if --report junit --report-file is configured.
func writeJUnitReport(commandName string, runErr error, elapsed time.Duration) error {
	if shared.ReportFormat() != shared.ReportFormatJUnit {
		return nil
	}

	reportFile := shared.ReportFile()
	if reportFile == "" {
		return nil
	}

	testCase := shared.JUnitTestCase{
		Name:      commandName,
		Classname: "asc",
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
