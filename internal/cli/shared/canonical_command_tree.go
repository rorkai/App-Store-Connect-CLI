package shared

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// PrepareCanonicalCommandTree preserves migration guidance for removed get/set
// paths without mutating canonical command names or user-facing text.
func PrepareCanonicalCommandTree(root *ffcli.Command, editPaths map[string]struct{}) *ffcli.Command {
	if root == nil || isDeprecatedCompatibilityAliasCommand(root) {
		return root
	}

	rootName := strings.TrimSpace(root.Name)
	if rootName == "" {
		return root
	}

	removedChildren := make(map[string]map[string]string)
	var walk func(parent, current *ffcli.Command, path string)
	walk = func(parent, current *ffcli.Command, path string) {
		if current == nil || isDeprecatedCompatibilityAliasCommand(current) {
			return
		}

		for _, sub := range current.Subcommands {
			if sub == nil {
				continue
			}
			childName := strings.TrimSpace(sub.Name)
			if childName == "" {
				continue
			}
			walk(current, sub, strings.TrimSpace(path+" "+childName))
		}

		if len(current.Subcommands) != 0 || parent == nil {
			return
		}
		legacyName, ok := legacyVerbForCanonicalLeaf(path, current.Name, editPaths)
		if !ok || findSubcommandByName(parent, legacyName) != nil {
			return
		}

		parentPath := strings.TrimSpace(strings.TrimSuffix(path, " "+strings.TrimSpace(current.Name)))
		if parentPath == "" {
			return
		}
		if removedChildren[parentPath] == nil {
			removedChildren[parentPath] = make(map[string]string)
		}
		removedChildren[parentPath][legacyName] = path
	}

	walk(nil, root, "asc "+rootName)
	if len(removedChildren) == 0 {
		return root
	}
	wrapRemovedViewEditCommandExecs(root, "asc "+rootName, removedChildren)
	wrapUsageFuncsToHideDeprecatedAliases(root)
	return root
}

func wrapRemovedViewEditCommandExecs(cmd *ffcli.Command, path string, removedChildren map[string]map[string]string) {
	if cmd == nil {
		return
	}

	if replacements := removedChildren[path]; len(replacements) > 0 {
		originalExec := cmd.Exec
		if originalExec == nil {
			originalExec = func(context.Context, []string) error { return flag.ErrHelp }
		}

		cmd.Exec = func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				legacyName := strings.TrimSpace(args[0])
				if replacement, ok := replacements[legacyName]; ok {
					fmt.Fprintf(os.Stderr, "Error: `%s %s` was removed. Use `%s` instead.\n", path, legacyName, replacement)
					return flag.ErrHelp
				}
			}
			return originalExec(ctx, args)
		}
	}

	for _, sub := range cmd.Subcommands {
		if sub == nil || isDeprecatedCompatibilityAliasCommand(sub) {
			continue
		}
		childName := strings.TrimSpace(sub.Name)
		if childName == "" {
			continue
		}
		wrapRemovedViewEditCommandExecs(sub, strings.TrimSpace(path+" "+childName), removedChildren)
	}
}

func legacyVerbForCanonicalLeaf(path, currentName string, editPaths map[string]struct{}) (string, bool) {
	switch strings.TrimSpace(currentName) {
	case "view":
		return "get", true
	case "edit":
		if _, ok := editPaths[replaceLastPathSegment(path, "set")]; ok {
			return "set", true
		}
	}
	return "", false
}

func findSubcommandByName(cmd *ffcli.Command, name string) *ffcli.Command {
	if cmd == nil {
		return nil
	}
	trimmed := strings.TrimSpace(name)
	for _, sub := range cmd.Subcommands {
		if sub != nil && strings.TrimSpace(sub.Name) == trimmed {
			return sub
		}
	}
	return nil
}

func replaceLastPathSegment(path, newName string) string {
	trimmed := strings.TrimSpace(path)
	lastSpace := strings.LastIndex(trimmed, " ")
	if lastSpace == -1 {
		return strings.TrimSpace(newName)
	}
	return strings.TrimSpace(trimmed[:lastSpace+1] + newName)
}

func renameFlagSetLastToken(fs *flag.FlagSet, oldName, newName string) {
	if fs == nil {
		return
	}

	output := fs.Output()
	usage := fs.Usage
	name := strings.TrimSpace(fs.Name())
	switch {
	case name == "":
		name = newName
	case name == oldName:
		name = newName
	case strings.HasSuffix(name, " "+oldName):
		name = strings.TrimSuffix(name, " "+oldName) + " " + newName
	default:
		name = newName
	}

	fs.Init(name, fs.ErrorHandling())
	if output != nil {
		fs.SetOutput(output)
	}
	if usage != nil {
		fs.Usage = usage
	}
}

func isDeprecatedCompatibilityAliasCommand(cmd *ffcli.Command) bool {
	if cmd == nil {
		return false
	}
	shortHelp := strings.ToLower(strings.TrimSpace(cmd.ShortHelp))
	longHelp := strings.ToLower(strings.TrimSpace(cmd.LongHelp))
	return strings.HasPrefix(shortHelp, "deprecated:") ||
		strings.HasPrefix(shortHelp, "compatibility alias") ||
		strings.HasPrefix(longHelp, "deprecated compatibility alias") ||
		strings.HasPrefix(longHelp, "compatibility alias")
}

func wrapUsageFuncsToHideDeprecatedAliases(cmd *ffcli.Command) {
	if cmd == nil {
		return
	}
	cmd.UsageFunc = wrapUsageFuncToHideDeprecatedAliases(cmd.UsageFunc)
	for _, sub := range cmd.Subcommands {
		wrapUsageFuncsToHideDeprecatedAliases(sub)
	}
}

func wrapUsageFuncToHideDeprecatedAliases(base func(*ffcli.Command) string) func(*ffcli.Command) string {
	if base == nil {
		base = DefaultUsageFunc
	}
	return func(cmd *ffcli.Command) string {
		if cmd == nil {
			return ""
		}
		clone := *cmd
		if len(cmd.Subcommands) > 0 {
			visible := make([]*ffcli.Command, 0, len(cmd.Subcommands))
			for _, sub := range cmd.Subcommands {
				if sub == nil || isDeprecatedCompatibilityAliasCommand(sub) {
					continue
				}
				visible = append(visible, sub)
			}
			clone.Subcommands = visible
		}
		return base(&clone)
	}
}
