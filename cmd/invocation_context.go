package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared/suggest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

type invocationAnalysis struct {
	command      *ffcli.Command
	shape        telemetry.InvocationShape
	unknownToken string
	unknownFlag  bool
}

func analyzeInvocation(root *ffcli.Command, args []string) invocationAnalysis {
	current := root
	sawFlag := false

	for i := 0; i < len(args); {
		token := args[i]
		if token == "" {
			i++
			continue
		}
		if sub := findDirectSubcommand(current, token); sub != nil {
			current = sub
			i++
			continue
		}
		if isHelpToken(token) {
			sawFlag = true
			i++
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			next, consumed := consumeFlagToken(current.FlagSet, token, args, i)
			if consumed {
				sawFlag = true
				i = next
				continue
			}
			return invocationAnalysis{
				command:      current,
				shape:        shapeForCommand(current, true),
				unknownToken: token,
				unknownFlag:  true,
			}
		}
		if len(current.Subcommands) > 0 {
			return invocationAnalysis{
				command:      current,
				shape:        telemetry.InvocationShapeUnknownChild,
				unknownToken: token,
			}
		}
		return invocationAnalysis{command: current, shape: telemetry.InvocationShapeLeaf}
	}

	return invocationAnalysis{command: current, shape: shapeForCommand(current, sawFlag)}
}

func rejectUnexpectedHybridArgs(command *ffcli.Command) {
	for _, subcommand := range command.Subcommands {
		rejectUnexpectedHybridArgs(subcommand)
		if subcommand.Exec == nil || len(subcommand.Subcommands) == 0 {
			continue
		}

		exec := subcommand.Exec
		subcommand.Exec = func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " "))
			}
			return exec(ctx, args)
		}
	}
}

func shapeForCommand(command *ffcli.Command, sawFlag bool) telemetry.InvocationShape {
	if command == nil || len(command.Subcommands) == 0 {
		return telemetry.InvocationShapeLeaf
	}
	if sawFlag {
		return telemetry.InvocationShapeGroupWithFlags
	}
	return telemetry.InvocationShapeBareGroup
}

func isHelpToken(token string) bool {
	return token == "-h" || token == "--help" || strings.HasPrefix(token, "--help=")
}

func parseFailureContext(analysis invocationAnalysis) telemetry.EventContext {
	kind := telemetry.ErrorKindInvalidValue
	if analysis.unknownFlag {
		kind = telemetry.ErrorKindUnknownFlag
	} else if analysis.shape == telemetry.InvocationShapeUnknownChild {
		kind = telemetry.ErrorKindOther
	}
	return telemetry.EventContext{
		InvocationShape: analysis.shape,
		ErrorKind:       kind,
		FailureStage:    telemetry.FailureStageParse,
	}
}

func validationFailureContext(analysis invocationAnalysis, err error) telemetry.EventContext {
	kind := telemetry.ErrorKindOther
	switch shared.ClassifyUsageError(err) {
	case shared.UsageErrorMissingRequired:
		kind = telemetry.ErrorKindMissingRequired
	case shared.UsageErrorInvalidValue:
		kind = telemetry.ErrorKindInvalidValue
	}
	if analysis.shape == telemetry.InvocationShapeUnknownChild {
		kind = telemetry.ErrorKindOther
	}
	return telemetry.EventContext{
		InvocationShape: analysis.shape,
		ErrorKind:       kind,
		FailureStage:    telemetry.FailureStageValidation,
	}
}

func runtimeFailureContext(analysis invocationAnalysis, err error, exitCode int) telemetry.EventContext {
	if errors.Is(err, flag.ErrHelp) || analysis.shape == telemetry.InvocationShapeUnknownChild {
		return validationFailureContext(analysis, err)
	}

	eventContext := telemetry.EventContext{
		InvocationShape: analysis.shape,
		ErrorKind:       telemetry.ErrorKindOther,
		FailureStage:    telemetry.FailureStageExecution,
	}
	switch {
	case errors.Is(err, shared.ErrMissingAuth):
		eventContext.FailureStage = telemetry.FailureStageValidation
	case errors.Is(err, context.DeadlineExceeded):
		eventContext.FailureStage = telemetry.FailureStageRequest
	case exitCode == ExitConflict:
		eventContext.ErrorKind = telemetry.ErrorKindAPIConflict
		eventContext.FailureStage = telemetry.FailureStageRequest
	case exitCode >= 60 && exitCode <= 99:
		eventContext.ErrorKind = telemetry.ErrorKindAPI5xx
		eventContext.FailureStage = telemetry.FailureStageRequest
	case exitCode == ExitAuth || exitCode == ExitNotFound || (exitCode >= 10 && exitCode <= 59):
		eventContext.FailureStage = telemetry.FailureStageRequest
	}
	return eventContext
}

func shouldRenderGroupHelp(analysis invocationAnalysis, err error) bool {
	if !errors.Is(err, flag.ErrHelp) || shared.ClassifyUsageError(err) != "" || analysis.command == nil {
		return false
	}
	if analysis.unknownToken != "" || len(analysis.command.Subcommands) == 0 || hasDefinedFlags(analysis.command.FlagSet) {
		return false
	}
	return analysis.shape == telemetry.InvocationShapeBareGroup ||
		analysis.shape == telemetry.InvocationShapeGroupWithFlags
}

func hasDefinedFlags(flagSet *flag.FlagSet) bool {
	if flagSet == nil {
		return false
	}
	found := false
	flagSet.VisitAll(func(*flag.Flag) { found = true })
	return found
}

func printUnknownFlagSuggestion(analysis invocationAnalysis) {
	if !analysis.unknownFlag || analysis.command == nil || analysis.command.FlagSet == nil {
		return
	}
	input := strings.TrimLeft(strings.SplitN(analysis.unknownToken, "=", 2)[0], "-")
	candidates := make([]string, 0)
	analysis.command.FlagSet.VisitAll(func(f *flag.Flag) {
		candidates = append(candidates, f.Name)
	})
	printSuggestions(input, candidates, "--")
}

func printUnknownSubcommandSuggestion(analysis invocationAnalysis) {
	if analysis.shape != telemetry.InvocationShapeUnknownChild || analysis.command == nil || analysis.command.Name == "asc" {
		return
	}
	candidates := make([]string, 0, len(analysis.command.Subcommands))
	for _, sub := range analysis.command.Subcommands {
		candidates = append(candidates, sub.Name)
	}
	printSuggestions(analysis.unknownToken, candidates, "")
}

func printSuggestions(input string, candidates []string, prefix string) {
	suggestions := suggest.Commands(input, candidates)
	if len(suggestions) == 0 {
		return
	}
	for i, item := range suggestions {
		suggestions[i] = prefix + shared.SanitizeTerminal(item)
	}
	fmt.Fprintf(os.Stderr, "Did you mean: %s?\n", strings.Join(suggestions, ", "))
}
