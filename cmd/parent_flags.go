package cmd

import (
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// groupFlagsBeforeSubcommandError rejects flags set on a group command when
// ffcli then dispatched to a subcommand. ffcli parses those flags into the
// group's FlagSet and runs the subcommand with its own unset copy, so the
// value would otherwise be dropped silently. Root flags are global and exempt.
func groupFlagsBeforeSubcommandError(root *ffcli.Command) error {
	type group struct {
		command *ffcli.Command
		path    string
	}
	var groups []group
	path := root.Name
	command := root
	for {
		rest := command.FlagSet.Args()
		if len(rest) == 0 {
			break
		}
		child := findDirectSubcommand(command, rest[0])
		if child == nil {
			break
		}
		if command != root {
			groups = append(groups, group{command: command, path: path})
		}
		command = child
		path += " " + child.Name
	}

	var parts []string
	for _, g := range groups {
		var moveAfter, groupOnly []string
		g.command.FlagSet.Visit(func(f *flag.Flag) {
			if command.FlagSet.Lookup(f.Name) != nil {
				moveAfter = append(moveAfter, "--"+f.Name)
			} else {
				groupOnly = append(groupOnly, "--"+f.Name)
			}
		})
		if len(moveAfter) > 0 {
			parts = append(parts, fmt.Sprintf("%s must be passed after the subcommand name (%s [flags])", strings.Join(moveAfter, ", "), path))
		}
		if len(groupOnly) > 0 {
			verb := "are"
			if len(groupOnly) == 1 {
				verb = "is"
			}
			parts = append(parts, fmt.Sprintf("%s %s only valid for %s", strings.Join(groupOnly, ", "), verb, g.path))
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return shared.WithDiagnostic(shared.NewReportedUsageError(shared.UsageErrorInvalidValue, strings.Join(parts, "; ")), shared.DiagnosticInvalidInput, "")
}
