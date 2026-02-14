package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/update"
)

const (
	updateCheckInterval = 12 * time.Hour
	asyncUpdateTimeout  = 3 * time.Second
)

var (
	cachedUpdateAvailableFn = update.CachedUpdateAvailable
	checkAndUpdateFn        = update.CheckAndUpdate
	restartFn               = update.Restart
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

	// Match gh-style root invocation: plain `asc` (or only root flags)
	// prints root help and exits successfully.
	if !hasPositionalArgs(root.FlagSet, args) {
		fmt.Fprint(os.Stdout, root.UsageFunc(root))
		return ExitSuccess
	}

	updateOpts := update.Options{
		CurrentVersion: versionInfo,
		NoUpdate:       shared.NoUpdate(),
		Output:         os.Stderr,
		ShowProgress:   shared.ProgressEnabled(),
		CheckInterval:  updateCheckInterval,
	}
	waitForAsyncUpdateCheck := func() {}
	defer func() {
		waitForAsyncUpdateCheck()
	}()

	cachedUpdateAvailable, cacheErr := cachedUpdateAvailableFn(updateOpts)
	if cacheErr != nil {
		fmt.Fprintf(os.Stderr, "Update check failed: %v\n", cacheErr)
	}
	if cachedUpdateAvailable {
		updateOpts.AutoUpdate = true
		updateResult, err := checkAndUpdateFn(context.Background(), updateOpts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Update check failed: %v\n", err)
		}
		if updateResult.Updated {
			exitCode, restartErr := restartFn(updateResult.ExecutablePath, os.Args, os.Environ())
			if restartErr != nil {
				fmt.Fprintf(os.Stderr, "Restart failed after update: %v\n", restartErr)
			} else {
				return exitCode
			}
		}
	} else {
		waitForAsyncUpdateCheck = startAsyncUpdateCheck(updateOpts)
	}

	start := time.Now()
	runErr := root.Run(context.Background())
	elapsed := time.Since(start)

	// Get command name (full subcommand path)
	commandName := getCommandName(root, args)

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
		if _, ok := errors.AsType[ReportedError](runErr); ok {
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

func startAsyncUpdateCheck(opts update.Options) func() {
	asyncOpts := opts
	asyncOpts.AutoUpdate = false
	asyncOpts.ShowProgress = false
	asyncOpts.Output = io.Discard
	done := make(chan struct{})

	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), asyncUpdateTimeout)
		defer cancel()
		_, _ = checkAndUpdateFn(ctx, asyncOpts)
	}()

	return func() {
		select {
		case <-done:
		case <-time.After(asyncUpdateTimeout):
		}
	}
}

// getCommandName extracts the full subcommand path from the parsed args.
// args is os.Args[1:] (without program name).
// It finds the first token matching a known subcommand name, then walks the tree.
func getCommandName(root *ffcli.Command, args []string) string {
	current := root
	path := []string{current.Name}

	// Backward compatibility: tolerate args that include argv[0].
	if len(args) > 0 && strings.EqualFold(args[0], root.Name) {
		args = args[1:]
	}

	for i := 0; i < len(args); {
		token := args[i]
		if token == "" {
			i++
			continue
		}

		if sub := findDirectSubcommand(current, token); sub != nil {
			path = append(path, sub.Name)
			current = sub
			i++
			continue
		}

		nextIdx, consumed := consumeFlagToken(current.FlagSet, token, args, i)
		if consumed {
			i = nextIdx
			continue
		}

		// First positional arg that isn't a subcommand ends traversal.
		break
	}

	return strings.Join(path, " ")
}

func findDirectSubcommand(current *ffcli.Command, token string) *ffcli.Command {
	for _, sub := range current.Subcommands {
		if strings.EqualFold(sub.Name, token) {
			return sub
		}
	}
	return nil
}

func consumeFlagToken(fs *flag.FlagSet, token string, args []string, idx int) (int, bool) {
	if fs == nil || token == "" || token == "-" || !strings.HasPrefix(token, "-") {
		return idx, false
	}

	if token == "--" {
		return idx + 1, true
	}

	trimmed := strings.TrimLeft(token, "-")
	if trimmed == "" {
		return idx, false
	}

	name, hasInlineValue := splitFlagToken(trimmed)
	f := fs.Lookup(name)
	if f == nil {
		return idx, false
	}

	if hasInlineValue || isBoolFlag(f) {
		return idx + 1, true
	}
	if idx+1 < len(args) {
		return idx + 2, true
	}
	return idx + 1, true
}

func hasPositionalArgs(fs *flag.FlagSet, args []string) bool {
	for i := 0; i < len(args); {
		token := args[i]
		if token == "" {
			i++
			continue
		}
		if token == "--" {
			return i+1 < len(args)
		}

		nextIdx, consumed := consumeFlagToken(fs, token, args, i)
		if consumed {
			i = nextIdx
			continue
		}

		return true
	}
	return false
}

func splitFlagToken(token string) (name string, hasInlineValue bool) {
	if before, _, ok := strings.Cut(token, "="); ok {
		return before, true
	}
	return token, false
}

func isBoolFlag(f *flag.Flag) bool {
	type boolFlag interface {
		IsBoolFlag() bool
	}
	v, ok := f.Value.(boolFlag)
	return ok && v.IsBoolFlag()
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
