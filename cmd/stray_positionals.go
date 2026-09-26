package cmd

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// commandAcceptsOperandsPath lists the leaf commands whose positional operands
// are part of their published contract. It is deliberately a reviewed list
// rather than a guess: everything else in the tree takes flags only, so a bare
// operand there is a mistake that used to be ignored silently.
//
// A leaf command that starts taking operands must appear both here and in its
// own ShortUsage. TestLeafOperandContractMatchesDeclaredUsage walks the tree
// and fails when the two disagree.
func commandAcceptsOperandsPath(commandName string) bool {
	switch commandName {
	case "asc api",
		"asc docs show",
		"asc schema",
		"asc search",
		"asc signing run",
		"asc workflow run":
		return true
	default:
		return false
	}
}

// strayPositionalOperands returns the operands a flag-only leaf command was
// handed and cannot use. It returns nothing for command groups, which report
// their own unknown-child diagnostics, and nothing for the commands that accept
// operands. A `--` terminator is not an exemption: it stops flag parsing, but
// it does not turn an operand into something a flag-only command can use.
//
// The operands come from the leaf command's own FlagSet, so they are exactly
// what ffcli would have passed to Exec. A command the tree walk resolved but
// ffcli did not select has an unparsed FlagSet and therefore no operands, which
// keeps a disagreement between the two walks from inventing a diagnostic.
func strayPositionalOperands(analysis invocationAnalysis, commandName string) []string {
	command := analysis.command
	if command == nil || len(command.Subcommands) > 0 || command.FlagSet == nil {
		return nil
	}
	if commandAcceptsOperands(command, commandName) {
		return nil
	}
	operands := command.FlagSet.Args()
	if len(operands) == 0 {
		return nil
	}
	return operands
}

// commandAcceptsOperands reports whether a resolved leaf command takes
// positional operands, from the reviewed list or from its own declared usage.
// Consulting the usage string too means a command that documents an operand is
// never rejected for accepting one, even before the list catches up.
func commandAcceptsOperands(command *ffcli.Command, commandName string) bool {
	if command == nil {
		return true
	}
	if commandAcceptsOperandsPath(commandName) {
		return true
	}
	return shared.UsageDeclaresOperands(command.ShortUsage)
}

// reportedStrayOperands narrows the leftover tokens to the ones worth naming.
// Flag parsing stops at the first operand, so a flag spelling after it was
// never parsed and is a consequence of the stray token rather than a second
// mistake. Naming the whole tail would bury the one token the caller has to
// remove under the flags they got right.
func reportedStrayOperands(operands []string) []string {
	for index, operand := range operands {
		if !strings.HasPrefix(operand, "-") || operand == "-" {
			continue
		}
		if index == 0 {
			return operands[:1]
		}
		return operands[:index]
	}
	return operands
}

// strayPositionalError renders the diagnostic for operands a flag-only command
// cannot use. The rejected tokens are quoted so an empty or whitespace-only
// operand is still visible, and sanitized so a crafted value cannot rewrite the
// terminal around the message.
func strayPositionalError(operands []string) error {
	quoted := make([]string, 0, len(operands))
	for _, operand := range reportedStrayOperands(operands) {
		quoted = append(quoted, fmt.Sprintf("%q", shared.SanitizeTerminal(operand)))
	}
	if len(quoted) == 1 {
		return fmt.Errorf("unexpected argument %s", quoted[0])
	}
	return fmt.Errorf("unexpected arguments %s", strings.Join(quoted, ", "))
}

// strayPositionalHint renders the copyable correction for one operand, using
// the same rules as the command-path recovery suggestions: POSIX single-quoting
// everywhere, and no suggestion at all on Windows for an operand that cannot be
// rendered safely for both cmd.exe and PowerShell. Naming the token is the part
// that matters; a hint the caller cannot paste verbatim is worse than none.
func strayPositionalHint(commandName, flagName, operand, goos string) (string, bool) {
	if goos == "windows" && !isWindowsRecoverySafeArg(operand) {
		return "", false
	}
	rendered, ok := shared.ShellQuoteForOS(operand, goos)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s --%s %s", commandName, flagName, rendered), true
}

// printStrayPositionalOperands writes the diagnostic, then the flag the caller
// most likely meant. The hint is offered only when the operand is the single
// leftover token and the command defines exactly one primary identifier flag
// that the caller has not already passed, so it always names a flag that
// command defines and a value the caller actually typed.
//
// The hint is meant to be copied into a shell, so the operand is quoted by
// strayPositionalHint rather than interpolated raw.
func printStrayPositionalOperands(commandName string, operands []string, flagSet *flag.FlagSet) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", strayPositionalError(operands))
	if len(operands) == 1 {
		if name, ok := shared.PrimaryIDFlagName(flagSet); ok {
			if hint, rendered := strayPositionalHint(commandName, name, operands[0], runtime.GOOS); rendered {
				fmt.Fprintf(os.Stderr, "Did you mean: %s\n", hint)
			}
		}
	}
	fmt.Fprintln(os.Stderr, "For help:")
	fmt.Fprintf(os.Stderr, "  %s --help\n", commandName)
}
