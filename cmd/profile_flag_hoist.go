package cmd

import (
	"flag"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// rootProfileFlagName is the credential-profile selector owned by the root
// flag set. It stays bound exactly once so that SelectedProfile() has a single
// source, and so that the value indirection exclusion for root-owned selectors
// keeps applying to it.
const rootProfileFlagName = shared.RootProfileFlagName

// hoistRootProfileFlag relocates a root-owned `--profile` selector written
// after the command name into the leading root-flag run, so that
// `asc apps list --profile prod` selects the same credential as
// `asc --profile prod apps list`.
//
// The walk mirrors how the standard flag package parses each command's own
// flag set, and refuses to move anything whose meaning could change:
//
//   - a command that defines its own `profile` flag keeps it, so
//     `asc signing run --profile app.mobileprovision` is untouched;
//   - the walk stops at `--`, at the first positional argument (an empty token
//     included, because the standard flag package stops there too), at a
//     malformed `---flag` spelling, and at the first unknown flag, so every
//     existing diagnostic and positional payload keeps describing the
//     invocation as the operator wrote it;
//   - a separated value is carried as `--profile=VALUE`, so the root flag set
//     can never swallow a following command name instead;
//   - at a command group, a following token naming one of its subcommands is a
//     misplaced command name rather than a profile value, so the selector is
//     left in place and reported as missing its profile name. Use
//     `--profile=NAME` to select a profile whose name collides with a
//     subcommand.
//
// An explicitly empty `--profile=` is relocated unchanged: it clears the
// profile override exactly as it already does before the command name, so
// `--profile="$UNSET_VAR"` keeps meaning "no profile" in either position.
//
// Hoisted tokens are spliced in at the end of the leading root-flag run, which
// preserves the standard left-to-right "last value wins" precedence when the
// selector is written more than once.
func hoistRootProfileFlag(root *ffcli.Command, args []string) []string {
	if root == nil || root.FlagSet == nil || root.FlagSet.Lookup(rootProfileFlagName) == nil {
		return args
	}

	command := root
	commandPath := make([]string, 0, 4)
	if root.Name != "" {
		commandPath = append(commandPath, root.Name)
	}
	kept := make([]string, 0, len(args))
	hoisted := make([]string, 0, 1)
	boundary := -1

	for index := 0; index < len(args); index++ {
		token := args[index]
		if token == "--" {
			kept = append(kept, args[index:]...)
			break
		}
		if !strings.HasPrefix(token, "-") || token == "-" {
			if boundary < 0 {
				boundary = len(kept)
			}
			if token != "" {
				if subcommand := findDirectSubcommand(command, token); subcommand != nil {
					command = subcommand
					commandPath = append(commandPath, subcommand.Name)
					kept = append(kept, token)
					continue
				}
			}
			// The standard flag package stops parsing at the first positional
			// argument, so nothing after it is a flag this walk may move. An
			// empty token is that same boundary.
			kept = append(kept, args[index:]...)
			break
		}
		if !hasValidFlagPrefix(token) {
			kept = append(kept, args[index:]...)
			break
		}

		name, _, hasInlineValue := strings.Cut(strings.TrimLeft(token, "-"), "=")
		if item := commandFlag(command, name); item != nil {
			kept = append(kept, token)
			if hasInlineValue {
				continue
			}
			if isBoolFlag(item) {
				// Mirror normalizeSpacedBooleanFlags so a `--flag false` pair
				// stays one unit here too. Treating the value as positional
				// would end the walk and leave a later `--profile` misplaced.
				if consumesSpacedBooleanValue(command, commandPath, args, index) {
					index++
					kept = append(kept, args[index])
				}
				continue
			}
			if index+1 < len(args) {
				index++
				kept = append(kept, args[index])
			}
			continue
		}
		if command == root || name != rootProfileFlagName {
			// An unknown flag ends the walk: parsing reports it, and the
			// diagnostic must keep naming the command the operator wrote.
			kept = append(kept, args[index:]...)
			break
		}

		if hasInlineValue {
			hoisted = append(hoisted, token)
			continue
		}
		value, ok := hoistedProfileValue(command, args, index)
		if !ok {
			// Without a value to carry there is nothing to relocate: moving a
			// bare `--profile` to the root would consume whichever token
			// followed it. Leave the invocation as written so the unknown-flag
			// path reports the missing profile name.
			kept = append(kept, args[index:]...)
			break
		}
		index++
		hoisted = append(hoisted, "--"+rootProfileFlagName+"="+value)
	}

	if len(hoisted) == 0 {
		return args
	}
	if boundary < 0 {
		boundary = len(kept)
	}

	relocated := make([]string, 0, len(kept)+len(hoisted))
	relocated = append(relocated, kept[:boundary]...)
	relocated = append(relocated, hoisted...)
	relocated = append(relocated, kept[boundary:]...)
	return relocated
}

// consumesSpacedBooleanValue reports whether the token after a bare boolean
// flag is its value rather than a positional argument, using the same rules as
// normalizeSpacedBooleanFlags.
func consumesSpacedBooleanValue(command *ffcli.Command, commandPath, args []string, index int) bool {
	if index+1 >= len(args) || commandAcceptsPositionalPayload(commandPath) {
		return false
	}
	next := strings.TrimSpace(args[index+1])
	if _, err := strconv.ParseBool(next); err == nil {
		return true
	}
	return len(command.Subcommands) == 0 && !strings.HasPrefix(next, "-")
}

// hoistedProfileValue returns the token that carries the selector's value.
// A subcommand or another flag cannot be an implicit profile name; callers
// can still select a dash-prefixed name explicitly with --profile=NAME.
func hoistedProfileValue(command *ffcli.Command, args []string, index int) (string, bool) {
	next := index + 1
	if next >= len(args) {
		return "", false
	}
	if strings.HasPrefix(args[next], "-") || findDirectSubcommand(command, args[next]) != nil {
		return "", false
	}
	return args[next], true
}

// commandFlag returns the named flag defined by the command, or nil when the
// command defines no flag by that name.
func commandFlag(command *ffcli.Command, name string) *flag.Flag {
	if command == nil || command.FlagSet == nil || name == "" {
		return nil
	}
	return command.FlagSet.Lookup(name)
}
