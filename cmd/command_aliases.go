package cmd

import (
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// readVerbAliases lists, per read verb callers guess, the subcommand names that
// verb means, in preference order. Callers coming from other CLIs and REST
// habits try `get` for the command asc names `view`, and `get` or `view` for
// the few groups whose single-resource read is `info`.
var readVerbAliases = map[string][]string{
	"get":  {"view", "info"},
	"view": {"info"},
}

// resolveCommandAliases rewrites a guessed read verb to the subcommand it
// means, so `asc versions get` runs `asc versions view`. It only rewrites a
// token that would otherwise be an unknown command: the token must sit where
// a subcommand of a command group belongs, match no subcommand of that group,
// and name a target that exists there as a leaf. It walks the arguments as
// analyzeInvocation does and stops at `--`, at an unknown flag, and at a
// leaf's own arguments. The aliases stay out of help, and parsing,
// diagnostics and telemetry see only the canonical command.
func resolveCommandAliases(root *ffcli.Command, args []string) []string {
	if root == nil {
		return args
	}
	resolved := args
	copied := false
	current := root
	for index := 0; index < len(args); {
		token := args[index]
		if token == "" {
			index++
			continue
		}
		if token == "--" {
			break
		}
		if sub := findDirectSubcommand(current, token); sub != nil {
			current = sub
			index++
			continue
		}
		if isHelpToken(token) {
			index++
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			next, consumed := consumeFlagToken(current.FlagSet, token, args, index)
			if !consumed {
				break
			}
			index = next
			continue
		}
		target := readVerbTarget(current, token)
		if target == nil {
			break
		}
		if !copied {
			resolved = append([]string(nil), args...)
			copied = true
		}
		resolved[index] = target.Name
		current = target
		index++
	}
	return resolved
}

// readVerbTarget returns the leaf subcommand of group that a guessed read verb
// stands for, or nil when the token is not such a verb for this group.
func readVerbTarget(group *ffcli.Command, token string) *ffcli.Command {
	if group == nil || len(group.Subcommands) == 0 {
		return nil
	}
	for _, name := range readVerbAliases[strings.ToLower(token)] {
		if target := findDirectSubcommand(group, name); target != nil {
			if len(target.Subcommands) > 0 {
				return nil
			}
			return target
		}
	}
	return nil
}
