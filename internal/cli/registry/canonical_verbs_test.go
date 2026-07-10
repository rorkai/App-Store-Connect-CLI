package registry

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestRawSubcommandsDefineCanonicalViewEditLeaves(t *testing.T) {
	t.Parallel()

	editPaths := map[string]struct{}{
		"asc age-rating set":             {},
		"asc app-setup availability set": {},
		"asc pricing availability set":   {},
	}

	var legacyPaths []string
	var walk func(*ffcli.Command, string)
	walk = func(cmd *ffcli.Command, path string) {
		if cmd == nil || isCompatibilityAlias(cmd) {
			return
		}

		path = strings.TrimSpace(path + " " + cmd.Name)
		for _, sub := range cmd.Subcommands {
			walk(sub, path)
		}
		if len(cmd.Subcommands) != 0 {
			return
		}

		if strings.TrimSpace(cmd.Name) == "get" {
			legacyPaths = append(legacyPaths, path)
		}
		if _, ok := editPaths[path]; ok {
			legacyPaths = append(legacyPaths, path)
		}
		if cmd.FlagSet != nil {
			flagSetParts := strings.Fields(cmd.FlagSet.Name())
			if len(flagSetParts) > 0 {
				last := flagSetParts[len(flagSetParts)-1]
				if (cmd.Name == "view" && last == "get") || (cmd.Name == "edit" && last == "set") {
					legacyPaths = append(legacyPaths, path+" (flag set: "+cmd.FlagSet.Name()+")")
				}
			}
		}
	}

	for _, cmd := range rawSubcommands("test") {
		walk(cmd, "asc")
	}
	if len(legacyPaths) != 0 {
		t.Fatalf("raw command constructors still define legacy leaf verbs:\n%s", strings.Join(legacyPaths, "\n"))
	}
}

func isCompatibilityAlias(cmd *ffcli.Command) bool {
	shortHelp := strings.ToLower(strings.TrimSpace(cmd.ShortHelp))
	longHelp := strings.ToLower(strings.TrimSpace(cmd.LongHelp))
	return strings.HasPrefix(shortHelp, "deprecated:") ||
		strings.HasPrefix(shortHelp, "compatibility alias") ||
		strings.HasPrefix(longHelp, "deprecated compatibility alias") ||
		strings.HasPrefix(longHelp, "compatibility alias")
}
