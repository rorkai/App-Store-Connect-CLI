package cmd

import (
	"slices"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// readVerbAliases lists, per read verb callers guess, the subcommand names that
// verb means, in preference order. Callers coming from other CLIs and REST
// habits try `get` for the command asc names `view`, `get` or `view` for the
// few groups whose single-resource read is `info`, and the shell's `ls` for
// `list`.
var readVerbAliases = map[string][]string{
	"get":  {"view", "info"},
	"view": {"info"},
	"ls":   {"list"},
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
		"build-uploads":         {"builds", "uploads"},
		"submissions":           {"review", "submissions"},
		"version-localizations": {"localizations"},
	},
	"asc apps": {
		"availability": {"pricing", "availability"},
		"versions":     {"versions"},
	},
	"asc builds": {
		"beta-groups": {"builds", "groups"},
		"betagroups":  {"builds", "groups"},
	},
	"asc subscriptions": {
		"availability":        {"subscriptions", "pricing", "availability"},
		"intro-offers":        {"subscriptions", "offers", "introductory"},
		"introductory-offers": {"subscriptions", "offers", "introductory"},
		// 5.0.0 removed the v1 subscription localization commands; their v2
		// replacements are version-scoped and share the localization IDs.
		"localizations": {"subscriptions", "versions", "localizations"},
		"prices":        {"subscriptions", "pricing", "prices"},
	},
	"asc telemetry": {
		"off": {"telemetry", "disable"},
		"on":  {"telemetry", "enable"},
	},
	"asc testflight": {
		"beta-groups":  {"testflight", "groups"},
		"beta-testers": {"testflight", "testers"},
		"builds":       {"builds"},
	},
	// Adding a build to a group is adding the group to the build; builds
	// add-groups also takes the group as --group-id.
	"asc testflight groups builds": {
		"add": {"builds", "add-groups"},
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
	var parent *ffcli.Command
	lastCommandIndex := -1
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
			parent, current, lastCommandIndex = current, sub, index
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
				// A group given flags it does not define, with no subcommand,
				// runs the leaf callers mean by it when that leaf defines every
				// flag that follows (`apps info --app X`, `versions --app X`).
				if view := groupViewForFlags(current, strings.Join(path, " "), resolved[index:]); view != nil {
					edit()
					resolved = slices.Insert(resolved, index, view.Name)
					parent, current, lastCommandIndex = current, view, index
					path = append(path, view.Name)
					index++
					continue
				}
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
			parent, current, lastCommandIndex = current, target, index
			path = append(path, target.Name)
			index++
			continue
		}
		// asc spells some verbs as one hyphenated child (`review
		// details-create`, `review submissions-get`); callers split them.
		if index+1 < len(resolved) {
			if target := compoundChild(current, token, resolved[index+1]); target != nil {
				edit()
				resolved = slices.Concat(resolved[:index], []string{target.Name}, resolved[index+2:])
				parent, current, lastCommandIndex = current, target, index
				path = append(path, target.Name)
				index++
				continue
			}
		}
		if parent != nil && lastCommandIndex == index-1 {
			if target := compoundChild(parent, current.Name, token); target != nil {
				edit()
				resolved = slices.Concat(resolved[:index-1], []string{target.Name}, resolved[index+1:])
				current = target
				path = append(path[:len(path)-1], target.Name)
				lastCommandIndex = index - 1
				continue
			}
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
			parent, lastCommandIndex = resolveCommandPath(root, target[:len(target)-1]), index-1
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

// compoundChild returns the leaf of group named first-second, the hyphenated
// verb a caller split into two words, or first-get for a guessed read verb
// (`review details view` runs `review details-get`). It returns nil when the
// group has no such leaf, or when first is a real child of the group.
func compoundChild(group *ffcli.Command, first, second string) *ffcli.Command {
	if group == nil || strings.HasPrefix(first, "-") || strings.HasPrefix(second, "-") {
		return nil
	}
	first, second = strings.ToLower(first), strings.ToLower(second)
	// Also the verb-noun order (`builds groups add` for `builds add-groups`).
	candidates := []string{first + "-" + second, second + "-" + first}
	// A guessed read verb names the leaf its aliases mean: `ls` the
	// first-list leaf, `get` and `view` the single-resource read.
	if targets, ok := readVerbAliases[second]; ok && slices.Contains(targets, "list") {
		candidates = append(candidates, first+"-list")
	} else if ok || second == "view" {
		candidates = append(candidates, first+"-get", first+"-view")
	}
	for _, name := range candidates {
		if target := findDirectSubcommand(group, name); target != nil && len(target.Subcommands) == 0 {
			return target
		}
	}
	return nil
}

// groupFlagsLeaf names, per command group, the leaf a caller means when they
// give the group flags and no subcommand: the single-resource read for a
// resource an app has one of, the list for a collection.
var groupFlagsLeaf = map[string]string{
	"asc apps info":                         "view",
	"asc pricing availability":              "view",
	"asc app-setup availability":            "view",
	"asc apps info relationships":           "view",
	"asc versions":                          "list",
	"asc builds":                            "list",
	"asc builds beta-app-review-submission": "view",
	"asc builds pre-release-version":        "view",
	"asc testflight groups":                 "list",
	"asc testflight groups builds":          "list",
	"asc review submissions":                "list",
}

// groupViewForFlags returns the leaf groupFlagsLeaf names for group when args,
// the rest of the command line after the group, hold only flags that leaf
// defines. A group that takes the flags itself never reaches here, and a
// positional argument or a flag the leaf does not define leaves the command as
// written.
func groupViewForFlags(group *ffcli.Command, groupPath string, args []string) *ffcli.Command {
	name, ok := groupFlagsLeaf[groupPath]
	if group == nil || !ok || len(args) == 0 {
		return nil
	}
	leaf := findDirectSubcommand(group, name)
	if leaf == nil || len(leaf.Subcommands) > 0 || leaf.FlagSet == nil {
		return nil
	}
	for index := 0; index < len(args); {
		token := args[index]
		if token == "--" || isHelpToken(token) || !strings.HasPrefix(token, "-") || token == "-" {
			return nil
		}
		next, consumed := consumeFlagToken(leaf.FlagSet, token, args, index)
		if !consumed {
			return nil
		}
		// A boolean given a spaced true or false, as the leaf accepts it.
		if name, hasInlineValue := splitFlagToken(strings.TrimLeft(token, "-")); !hasInlineValue && next < len(args) && isBoolFlag(leaf.FlagSet.Lookup(name)) {
			if _, err := strconv.ParseBool(strings.TrimSpace(args[next])); err == nil {
				next++
			}
		}
		index = next
	}
	return leaf
}
