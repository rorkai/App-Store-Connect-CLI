package cmd

import (
	"slices"
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

// commandPathAliases maps, per command group, the paths callers guess from
// App Store Connect resource names (`app-infos`, `betaGroups`) or from a
// neighbouring group (`testflight builds`) to the full command path asc uses
// for the same resource. A target may sit in another part of the tree. Tests
// reject a key that shadows a real subcommand and a target that does not
// resolve.
var commandPathAliases = map[string]map[string][]string{
	"asc": {
		"app-info":              {"apps", "info"},
		"app-infos":             {"apps", "info"},
		"availability":          {"pricing", "availability"},
		"submissions":           {"review", "submissions"},
		"version-localizations": {"localizations"},
	},
	"asc apps": {
		"versions": {"versions"},
	},
	"asc builds": {
		"beta-groups": {"builds", "groups"},
	},
	"asc testflight": {
		"beta-groups":  {"testflight", "groups"},
		"beta-testers": {"testflight", "testers"},
		"builds":       {"builds"},
	},
	"asc versions": {
		"localizations": {"localizations"},
	},
}

// rootHelpToken is the `git help <command>` spelling of `<command> --help`.
const rootHelpToken = "help"

// resolveCommandAliases rewrites a guessed command to the command it means, so
// `asc versions get` runs `asc versions view`, `asc app-infos list` runs `asc
// apps info list`, and `asc help builds list` runs `asc builds list --help`. It only
// rewrites a token that would otherwise be an unknown command: the token must
// sit where a subcommand of a command group belongs and match no subcommand of
// that group. A read verb must name a leaf of that group; a path alias
// replaces the whole command path typed so far, so it applies only when no
// flag sits between those command tokens. The walk follows analyzeInvocation
// and stops at `--`, at an unknown flag, and at a leaf's own arguments. The
// aliases stay out of help, and parsing, diagnostics and telemetry see only the
// canonical command.
func resolveCommandAliases(root *ffcli.Command, args []string) []string {
	if root == nil {
		return args
	}
	resolved := args
	copied := false
	edit := func() {
		if !copied {
			resolved = slices.Clone(args)
			copied = true
		}
	}
	current := root
	path := []string{root.Name}
	pathStart := -1
	contiguous := true
	help := false
	index := 0
walk:
	for index < len(resolved) {
		token := resolved[index]
		switch token {
		case "":
			index++
			continue
		case "--":
			break walk
		}
		if sub := findDirectSubcommand(current, token); sub != nil {
			if pathStart < 0 {
				pathStart = index
			}
			current = sub
			path = append(path, sub.Name)
			index++
			continue
		}
		if isHelpToken(token) {
			index++
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			next, consumed := consumeFlagToken(current.FlagSet, token, resolved, index)
			if !consumed {
				break
			}
			if pathStart >= 0 {
				contiguous = false
			}
			index = next
			continue
		}
		if current == root && pathStart < 0 && !help && strings.EqualFold(token, rootHelpToken) {
			edit()
			resolved = slices.Delete(resolved, index, index+1)
			help = true
			continue
		}
		if target := readVerbTarget(current, token); target != nil {
			edit()
			resolved[index] = target.Name
			current = target
			path = append(path, target.Name)
			index++
			continue
		}
		if target, ok := commandPathAliases[strings.Join(path, " ")][strings.ToLower(token)]; ok && contiguous {
			start := index
			if pathStart >= 0 {
				start = pathStart
			}
			edit()
			resolved = slices.Concat(resolved[:start], target, resolved[index+1:])
			path = append([]string{root.Name}, target...)
			pathStart = start
			index = start + len(target)
			// A lazily built root holds only the command the caller named, so
			// a target elsewhere in the tree may not be materialized yet; the
			// caller rebuilds the root for the rewritten arguments and walks
			// again.
			if current = resolveCommandPath(root, target); current == nil {
				break
			}
			continue
		}
		break
	}
	if help {
		edit()
		// `--help` goes after the command's own flags but before `--`, which
		// would make it a positional.
		end := len(resolved)
		if terminator := slices.Index(resolved, "--"); terminator >= 0 {
			end = terminator
		}
		resolved = slices.Insert(resolved, end, "--help")
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

// commandAliasTarget names what an unknown child token of group runs instead,
// or "" when no command alias applies. Hint tables use it to drop entries that
// the alias makes unreachable.
func commandAliasTarget(groupPath string, group *ffcli.Command, token string) string {
	if target := readVerbTarget(group, token); target != nil {
		return groupPath + " " + target.Name
	}
	if target, ok := commandPathAliases[groupPath][strings.ToLower(token)]; ok {
		return "asc " + strings.Join(target, " ")
	}
	return ""
}

// topLevelCommand names the top-level command args select, or "" for none.
func topLevelCommand(root *ffcli.Command, args []string) string {
	parts := strings.Fields(getCommandName(root, args))
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
