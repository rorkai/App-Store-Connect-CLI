package cmd

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/install"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/errfmt"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

const unknownFlagErrorPrefix = "flag provided but not defined:"

var (
	maybeScheduleSkillsUpdateCheck = install.MaybeScheduleSkillsUpdateCheck
	emitTelemetry                  = telemetry.EmitWithContext
)

// Run executes the CLI using the provided args (not including argv[0]) and version string.
// It returns the intended process exit code.
func Run(args []string, versionInfo string) int {
	defer shared.CleanupTempPrivateKeys()

	// Fast path for the most common version check invocation. This avoids
	// building/parsing the entire command tree just to print the version.
	if isVersionOnlyInvocation(args) {
		fmt.Fprintln(os.Stdout, versionInfo)
		return ExitSuccess
	}

	root := rootCommandForArgs(versionInfo, args)
	args = normalizeSpacedBooleanFlags(root, args)
	analysis := analyzeInvocation(root, args)
	runCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignals()

	parseOutput := &bytes.Buffer{}
	restoreFlagOutputs := prepareFlagParsing(root, args, parseOutput)
	parseErr := root.Parse(args)
	restoreFlagOutputs()
	if parseErr != nil {
		if errors.Is(parseErr, flag.ErrHelp) {
			fmt.Fprint(os.Stderr, parseOutput.String())
			return ExitSuccess
		}
		if err := shared.ValidateReportFlags(); err != nil {
			fmt.Fprint(os.Stderr, errfmt.FormatStderr(err))
			emitImmediateTelemetry(args, root, versionInfo, validationFailureContext(analysis, err))
			return ExitUsage
		}
		if analysis.unknownFlag && isUnknownFlagParseFailure(parseErr, parseOutput.String()) {
			printConciseUnknownFlag(analysis, getCommandName(root, args))
		} else {
			printParseFailure(parseErr, parseOutput.String(), analysis)
		}
		// Every non-help error returned by command-tree parsing is invalid usage,
		// including NoExecError cases that do not write flag output.
		if analysis.unknownFlag && isUnknownFlagParseFailure(parseErr, parseOutput.String()) {
			commandName := getCommandName(root, args)
			writeUsageJUnitReport(commandName, unknownFlagError(analysis, commandName))
		}
		emitImmediateTelemetry(args, root, versionInfo, parseFailureContext(analysis))
		return ExitUsage
	}

	// Validate CI report flags after parsing
	if err := shared.ValidateReportFlags(); err != nil {
		fmt.Fprint(os.Stderr, errfmt.FormatStderr(err))
		emitImmediateTelemetry(args, root, versionInfo, validationFailureContext(analysis, err))
		return ExitUsage
	}

	if versionRequested {
		// A root informational flag must never dispatch a trailing subcommand.
		// Printing directly also keeps `asc --version true ...` as harmless as
		// the conventional `asc --version` form.
		fmt.Fprintln(os.Stdout, versionInfo)
		return ExitSuccess
	}

	// Match gh-style root invocation: plain `asc` (or only root flags)
	// prints root help and exits successfully.
	if !hasPositionalArgs(root.FlagSet, args) {
		fmt.Fprint(os.Stdout, root.UsageFunc(root))
		return ExitSuccess
	}

	commandName := getCommandName(root, args)
	if shouldRenderConciseUnknownChild(root, analysis, commandName) {
		printConciseUnknownCommand(analysis, commandName)
		writeUsageJUnitReport(commandName, unknownCommandError(analysis, commandName))
		emitImmediateTelemetry(args, root, versionInfo, validationFailureContext(analysis, flag.ErrHelp))
		return ExitUsage
	}

	runUsageOutput := &bytes.Buffer{}
	restoreRunUsageOutput := redirectCommandFlagOutput(analysis.command, runUsageOutput)
	start := time.Now()
	runErr := root.Run(runCtx)
	elapsed := time.Since(start)
	restoreRunUsageOutput()

	if shouldCancelRunContextAfterError(runErr) {
		stopSignals()
	}
	renderGroupHelp := shouldRenderGroupHelp(analysis, runErr)
	if renderGroupHelp {
		fmt.Fprint(os.Stdout, runUsageOutput.String())
	} else if runUsageOutput.Len() > 0 {
		fmt.Fprint(os.Stderr, runUsageOutput.String())
	}

	if !renderGroupHelp && shouldRunSkillsUpdateCheck(commandName, runCtx, runErr) {
		maybeScheduleSkillsUpdateCheck()
	}

	// Write JUnit report if requested
	if shared.ReportFormat() == shared.ReportFormatJUnit && shared.ReportFile() != "" {
		reportRunErr := runErr
		if renderGroupHelp {
			reportRunErr = nil
		}
		reportErr := writeJUnitReport(commandName, reportRunErr, elapsed)
		if reportErr != nil {
			// Report write failure is a hard error - CI depends on it
			fmt.Fprintf(os.Stderr, "Error: failed to write JUnit report: %v\n", reportErr)
			if reportRunErr == nil {
				emitTelemetry(commandName, versionInfo, elapsed, ExitError, telemetry.EventContext{
					InvocationShape: analysis.shape,
					ErrorKind:       telemetry.ErrorKindOther,
					FailureStage:    telemetry.FailureStageExecution,
				})
				return ExitError
			}
		}
	}

	if renderGroupHelp {
		return ExitSuccess
	}

	if runErr != nil {
		if _, ok := errors.AsType[shared.ReportedError](runErr); ok {
			exitCode := ExitCodeFromError(runErr)
			emitTelemetry(commandName, versionInfo, elapsed, exitCode, runtimeFailureContext(analysis, runErr, exitCode))
			return exitCode
		}
		if errors.Is(runErr, flag.ErrHelp) {
			printUnknownSubcommandSuggestion(analysis, commandName)
			emitTelemetry(commandName, versionInfo, elapsed, ExitUsage, validationFailureContext(analysis, runErr))
			return ExitUsage
		}
		printUnknownSubcommandSuggestion(analysis, commandName)
		fmt.Fprint(os.Stderr, errfmt.FormatStderr(runErr))
		exitCode := ExitCodeFromError(runErr)
		emitTelemetry(commandName, versionInfo, elapsed, exitCode, runtimeFailureContext(analysis, runErr, exitCode))
		return exitCode
	}

	emitTelemetry(commandName, versionInfo, elapsed, ExitSuccess, telemetry.EventContext{
		InvocationShape: analysis.shape,
	})
	return ExitSuccess
}

// normalizeSpacedBooleanFlags preserves the CLI's liberal support for both
// `--flag=false` and `--flag false`. The standard flag package stops parsing at
// the value after a bare bool flag, which can otherwise leave later safety
// flags unparsed and make an explicit false behave as true.
//
// Commands with positional payloads retain the standard flag ambiguity for
// compatibility: in `asc snitch --dry-run false`, for example, v3.2.0 treated
// `--dry-run` as true and `false` as the report description. Callers of those
// commands can use `--dry-run=false` when they intend an explicit bool value.
func normalizeSpacedBooleanFlags(root *ffcli.Command, args []string) []string {
	command := root
	commandPath := make([]string, 0, 4)
	if root != nil && root.Name != "" {
		commandPath = append(commandPath, root.Name)
	}
	normalized := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		token := args[index]
		if token == "--" {
			normalized = append(normalized, args[index:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			if subcommand := findDirectSubcommand(command, token); subcommand != nil {
				command = subcommand
				commandPath = append(commandPath, subcommand.Name)
				normalized = append(normalized, token)
				continue
			}
			normalized = append(normalized, args[index:]...)
			break
		}

		name := strings.TrimLeft(strings.SplitN(token, "=", 2)[0], "-")
		if command == nil || command.FlagSet == nil || name == "" {
			normalized = append(normalized, token)
			continue
		}
		item := command.FlagSet.Lookup(name)
		if item == nil {
			// Parsing will report the unknown flag. Do not reinterpret anything
			// after the token that stops standard flag parsing.
			normalized = append(normalized, args[index:]...)
			break
		}
		if strings.ContainsRune(token, '=') {
			normalized = append(normalized, token)
			continue
		}

		boolFlag, isBool := item.Value.(interface{ IsBoolFlag() bool })
		if isBool && boolFlag.IsBoolFlag() {
			if commandAcceptsPositionalPayload(commandPath) {
				normalized = append(normalized, token)
				continue
			}
			if index+1 < len(args) {
				next := strings.TrimSpace(args[index+1])
				if value, err := strconv.ParseBool(next); err == nil {
					normalized = append(normalized, token+"="+strconv.FormatBool(value))
					index++
					continue
				}
				if len(command.Subcommands) == 0 && !strings.HasPrefix(next, "-") {
					normalized = append(normalized, token+"="+next)
					index++
					continue
				}
			}
			normalized = append(normalized, token)
			continue
		}

		// A non-bool flag consumes its following value even when that value
		// looks like another flag. Never normalize inside the value.
		normalized = append(normalized, token)
		if index+1 < len(args) {
			index++
			normalized = append(normalized, args[index])
		}
	}
	return normalized
}

// commandAcceptsPositionalPayload lists the commands whose positional strings
// are part of their public contract. A positional value can legitimately be the
// word "true" or "false", so consuming it as a spaced boolean would break the
// standard flag behavior these commands exposed before spaced bool recovery.
func commandAcceptsPositionalPayload(commandPath []string) bool {
	switch strings.Join(commandPath, " ") {
	case "asc docs show",
		"asc schema",
		"asc search",
		"asc snitch",
		"asc workflow run":
		return true
	default:
		return false
	}
}

func printParseFailure(parseErr error, parseOutput string, analysis invocationAnalysis) {
	if !analysis.unknownFlag || !isUnknownFlagParseFailure(parseErr, parseOutput) {
		if parseOutput != "" {
			fmt.Fprint(os.Stderr, parseOutput)
			return
		}
		fmt.Fprint(os.Stderr, errfmt.FormatStderr(parseErr))
		return
	}

	flagName := strings.SplitN(analysis.unknownToken, "=", 2)[0]
	fmt.Fprintf(os.Stderr, "Unknown flag: %s\n", shared.SanitizeTerminal(flagName))
	firstLine, usage, found := strings.Cut(parseOutput, "\n")
	if found && strings.HasPrefix(firstLine, unknownFlagErrorPrefix) {
		fmt.Fprint(os.Stderr, usage)
		return
	}
	if parseOutput != "" {
		fmt.Fprint(os.Stderr, parseOutput)
	}
}

func isUnknownFlagParseFailure(parseErr error, parseOutput string) bool {
	return strings.HasPrefix(parseErr.Error(), unknownFlagErrorPrefix) || strings.HasPrefix(parseOutput, unknownFlagErrorPrefix)
}

func redirectCommandFlagOutput(command *ffcli.Command, output io.Writer) func() {
	if command == nil || command.FlagSet == nil {
		return func() {}
	}
	original := command.FlagSet.Output()
	command.FlagSet.SetOutput(output)
	return func() { command.FlagSet.SetOutput(original) }
}

func emitImmediateTelemetry(
	args []string,
	root *ffcli.Command,
	versionInfo string,
	eventContext telemetry.EventContext,
) {
	emitTelemetry(getCommandName(root, args), versionInfo, 0, ExitUsage, eventContext)
}

func prepareFlagParsing(command *ffcli.Command, args []string, output *bytes.Buffer) func() {
	type preparedFlagSet struct {
		flagSet *flag.FlagSet
		output  io.Writer
	}
	prepared := []preparedFlagSet{}

	for command != nil {
		if command.FlagSet == nil {
			command.FlagSet = flag.NewFlagSet(command.Name, flag.ContinueOnError)
		}
		prepared = append(prepared, preparedFlagSet{
			flagSet: command.FlagSet,
			output:  command.FlagSet.Output(),
		})
		command.FlagSet.Init(command.FlagSet.Name(), flag.ContinueOnError)
		command.FlagSet.SetOutput(output)

		var next *ffcli.Command
		var remaining []string
		for i := 0; i < len(args); {
			token := args[i]
			if token == "" {
				i++
				continue
			}
			if sub := findDirectSubcommand(command, token); sub != nil {
				next = sub
				remaining = args[i+1:]
				break
			}
			nextIndex, consumed := consumeFlagToken(command.FlagSet, token, args, i)
			if consumed {
				i = nextIndex
				continue
			}
			break
		}
		command = next
		args = remaining
	}
	return func() {
		for _, item := range prepared {
			item.flagSet.SetOutput(item.output)
		}
	}
}

func shouldCancelRunContextAfterError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func shouldRunSkillsUpdateCheck(commandName string, runCtx context.Context, runErr error) bool {
	// The external `skills check` command is not read-only: current releases
	// route it through the updater and can rewrite globally installed skills.
	// Keep automatic execution disabled until a reviewed, read-only protocol
	// exists. Explicit `asc install-skills` remains available.
	return false
}

func isVersionOnlyInvocation(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch strings.TrimSpace(args[0]) {
	case "--version", "--version=true":
		return true
	default:
		return false
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

func writeUsageJUnitReport(commandName string, usageErr error) {
	if shared.ReportFormat() != shared.ReportFormatJUnit || shared.ReportFile() == "" {
		return
	}
	if err := writeJUnitReport(commandName, usageErr, 0); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write JUnit report: %v\n", err)
	}
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
