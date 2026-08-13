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
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
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

func shapeForCommand(command *ffcli.Command, sawFlag bool) telemetry.InvocationShape {
	if command == nil || len(command.Subcommands) == 0 {
		return telemetry.InvocationShapeLeaf
	}
	if sawFlag {
		return telemetry.InvocationShapeGroupWithFlags
	}
	return telemetry.InvocationShapeBareGroup
}

func shouldRenderConciseUnknownChild(root *ffcli.Command, analysis invocationAnalysis, commandName string) bool {
	if analysis.shape != telemetry.InvocationShapeUnknownChild || analysis.command == nil {
		return false
	}
	if analysis.command != root && commandName == "asc snitch" {
		return false
	}
	return analysis.command == root || !preservesLegacyChild(analysis, commandName)
}

func printConciseUnknownCommand(analysis invocationAnalysis, commandName string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", unknownCommandError(analysis, commandName))

	candidates := visibleSubcommandNames(analysis.command)
	suggestions := suggest.Commands(analysis.unknownToken, candidates)
	if len(suggestions) > 2 {
		suggestions = suggestions[:2]
	}
	if len(suggestions) > 0 {
		fmt.Fprintln(os.Stderr, "Try:")
		for _, suggestion := range suggestions {
			fmt.Fprintf(os.Stderr, "  %s %s\n", commandName, shared.SanitizeTerminal(suggestion))
		}
	}
	fmt.Fprintln(os.Stderr, "For help:")
	fmt.Fprintf(os.Stderr, "  %s --help\n", commandName)
}

func printConciseUnknownFlag(analysis invocationAnalysis, commandName string) {
	flagName := unknownFlagName(analysis)
	fmt.Fprintf(os.Stderr, "Error: %s\n", unknownFlagError(analysis, commandName))

	visibleFlags := shared.VisibleHelpFlags(analysis.command.FlagSet)
	candidates := make([]string, 0, len(visibleFlags))
	for _, item := range visibleFlags {
		if isDeprecatedFlagHelp(item.Usage) {
			continue
		}
		candidates = append(candidates, item.Name)
	}
	suggestions := suggest.Flags(strings.TrimLeft(flagName, "-"), candidates)
	if len(suggestions) > 2 {
		suggestions = suggestions[:2]
	}
	if len(suggestions) > 0 {
		fmt.Fprintln(os.Stderr, "Try:")
		for _, suggestion := range suggestions {
			fmt.Fprintf(os.Stderr, "  --%s\n", shared.SanitizeTerminal(suggestion))
		}
	}
	fmt.Fprintln(os.Stderr, "For help:")
	fmt.Fprintf(os.Stderr, "  %s --help\n", commandName)
}

func unknownCommandError(analysis invocationAnalysis, commandName string) error {
	return fmt.Errorf(
		"unknown command `%s %s`",
		commandName,
		shared.SanitizeTerminal(analysis.unknownToken),
	)
}

func unknownFlagName(analysis invocationAnalysis) string {
	return strings.SplitN(analysis.unknownToken, "=", 2)[0]
}

func unknownFlagError(analysis invocationAnalysis, commandName string) error {
	return fmt.Errorf(
		"unknown flag `%s` for `%s`",
		shared.SanitizeTerminal(unknownFlagName(analysis)),
		commandName,
	)
}

func visibleSubcommandNames(command *ffcli.Command) []string {
	if command == nil {
		return nil
	}
	names := make([]string, 0, len(command.Subcommands))
	for _, subcommand := range command.Subcommands {
		if subcommand == nil || isDeprecatedCommandHelp(subcommand.ShortHelp) {
			continue
		}
		names = append(names, subcommand.Name)
	}
	return names
}

func isDeprecatedFlagHelp(help string) bool {
	normalized := strings.ToLower(strings.TrimSpace(help))
	return strings.HasPrefix(normalized, "deprecated") ||
		strings.HasPrefix(normalized, "[deprecated") ||
		strings.HasSuffix(normalized, " (deprecated)")
}

func isDeprecatedCommandHelp(help string) bool {
	normalized := strings.ToLower(strings.TrimSpace(help))
	return strings.HasPrefix(normalized, "deprecated") ||
		strings.HasPrefix(normalized, "manage deprecated ") ||
		strings.HasSuffix(normalized, "(deprecated by apple).")
}

func preservesLegacyChild(analysis invocationAnalysis, commandName string) bool {
	token := strings.TrimSpace(analysis.unknownToken)
	if token == "get" && findDirectSubcommand(analysis.command, "view") != nil {
		return true
	}
	if token == "set" && findDirectSubcommand(analysis.command, "edit") != nil {
		return true
	}

	switch commandName {
	case "asc apps":
		return token == "create"
	case "asc review":
		return token == "items-get"
	case "asc review items":
		return token == "view"
	case "asc submit":
		return token == "create" || token == "preflight"
	default:
		return false
	}
}

func isHelpToken(token string) bool {
	return token == "-h" || token == "--help" || strings.HasPrefix(token, "--help=")
}

func parseFailureContext(analysis invocationAnalysis) telemetry.EventContext {
	kind := telemetry.ErrorKindInvalidValue
	parameter := ""
	if analysis.unknownFlag {
		kind = telemetry.ErrorKindUnknownFlag
		parameter = analysis.unknownToken
	} else if analysis.shape == telemetry.InvocationShapeUnknownChild {
		kind = telemetry.ErrorKindOther
	}
	return telemetry.EventContext{
		InvocationShape:  analysis.shape,
		ErrorKind:        kind,
		FailureStage:     telemetry.FailureStageParse,
		FailureParameter: parameter,
		OutcomeKind:      telemetry.OutcomeUsageError,
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
		InvocationShape:  analysis.shape,
		ErrorKind:        kind,
		FailureStage:     telemetry.FailureStageValidation,
		FailureParameter: failureParameterFromError(err),
		OutcomeKind:      telemetry.OutcomeUsageError,
	}
}

func runtimeFailureContext(analysis invocationAnalysis, err error, exitCode int) telemetry.EventContext {
	if errors.Is(err, flag.ErrHelp) || shared.IsReportedUsageError(err) || analysis.shape == telemetry.InvocationShapeUnknownChild {
		return validationFailureContext(analysis, err)
	}

	eventContext := telemetry.EventContext{
		InvocationShape:  analysis.shape,
		ErrorKind:        telemetry.ErrorKindOther,
		FailureStage:     telemetry.FailureStageExecution,
		HTTPStatus:       httpStatusFromError(err),
		PublicStorefront: isPublicStorefrontError(err),
	}
	switch {
	case errors.Is(err, shared.ErrMissingAuth):
		eventContext.FailureStage = telemetry.FailureStageValidation
	case shared.IsValidationError(err):
		eventContext.FailureStage = telemetry.FailureStageValidation
	case errors.Is(err, context.DeadlineExceeded):
		eventContext.FailureStage = telemetry.FailureStageRequest
	case eventContext.HTTPStatus == 409:
		eventContext.ErrorKind = telemetry.ErrorKindAPIConflict
		eventContext.FailureStage = telemetry.FailureStageRequest
	case eventContext.HTTPStatus >= 500:
		eventContext.ErrorKind = telemetry.ErrorKindAPI5xx
		eventContext.FailureStage = telemetry.FailureStageRequest
	case eventContext.HTTPStatus >= 400:
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
	eventContext.OutcomeKind = runtimeOutcomeKind(err, exitCode, eventContext)
	return eventContext
}

func runtimeOutcomeKind(err error, exitCode int, eventContext telemetry.EventContext) telemetry.OutcomeKind {
	switch {
	case errors.Is(err, context.Canceled):
		return telemetry.OutcomeCancelled
	case errors.Is(err, shared.ErrMissingAuth), errors.Is(err, webcore.ErrInvalidAppleAccountCredentials), exitCode == ExitAuth:
		return telemetry.OutcomeAuthError
	case shared.IsValidationError(err):
		return telemetry.OutcomeExpectedNegative
	case eventContext.PublicStorefront && (eventContext.HTTPStatus == 401 || eventContext.HTTPStatus == 403):
		return telemetry.OutcomeAPIClientError
	case eventContext.HTTPStatus == 401 || eventContext.HTTPStatus == 403:
		return telemetry.OutcomeAuthError
	case eventContext.HTTPStatus == 404:
		return telemetry.OutcomeNotFound
	case eventContext.HTTPStatus == 409:
		return telemetry.OutcomeConflict
	case eventContext.HTTPStatus >= 400 && eventContext.HTTPStatus < 500:
		return telemetry.OutcomeAPIClientError
	case eventContext.HTTPStatus >= 500:
		return telemetry.OutcomeAPIServerError
	case exitCode == ExitNotFound:
		return telemetry.OutcomeNotFound
	case exitCode == ExitConflict:
		return telemetry.OutcomeConflict
	case errors.Is(err, context.DeadlineExceeded), eventContext.FailureStage == telemetry.FailureStageRequest:
		return telemetry.OutcomeTransportError
	default:
		return telemetry.OutcomeInternalError
	}
}

func isPublicStorefrontError(err error) bool {
	var storefrontError interface{ PublicStorefrontError() bool }
	return errors.As(err, &storefrontError) && storefrontError.PublicStorefrontError()
}

func httpStatusFromError(err error) int {
	var statusError interface{ HTTPStatusCode() int }
	if !errors.As(err, &statusError) {
		return 0
	}
	status := statusError.HTTPStatusCode()
	if status < 400 || status > 599 {
		return 0
	}
	return status
}

func failureParameterFromError(err error) string {
	if err == nil {
		return ""
	}
	for _, field := range strings.Fields(err.Error()) {
		candidate := strings.Trim(field, "`'\"(),:;.")
		if strings.HasPrefix(candidate, "--") {
			return candidate
		}
	}
	return ""
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

func printUnknownSubcommandSuggestion(analysis invocationAnalysis, commandName string) {
	if analysis.shape != telemetry.InvocationShapeUnknownChild || analysis.command == nil || analysis.command.Name == "asc" {
		return
	}
	if isRemovedReviewItemDetailInvocation(analysis, commandName) {
		return
	}
	candidates := make([]string, 0, len(analysis.command.Subcommands))
	for _, sub := range analysis.command.Subcommands {
		candidates = append(candidates, sub.Name)
	}
	printSuggestions(analysis.unknownToken, candidates, "")
}

func isRemovedReviewItemDetailInvocation(analysis invocationAnalysis, commandName string) bool {
	token := strings.TrimSpace(analysis.unknownToken)
	return (commandName == "asc review" && token == "items-get") ||
		(commandName == "asc review items" && token == "view")
}

func printSuggestions(input string, candidates []string, prefix string) {
	suggestions := suggest.Commands(input, candidates)
	printSuggestionList(suggestions, prefix)
}

func printSuggestionList(suggestions []string, prefix string) {
	if len(suggestions) == 0 {
		return
	}
	for i, item := range suggestions {
		suggestions[i] = prefix + shared.SanitizeTerminal(item)
	}
	fmt.Fprintf(os.Stderr, "Did you mean: %s?\n", strings.Join(suggestions, ", "))
}
