package registry

import (
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestVisibleLeafCommandsPreferViewOverGet(t *testing.T) {
	paths := visibleLeafPaths(Subcommands("dev"))

	for _, path := range paths {
		if strings.HasSuffix(path, " get") {
			t.Fatalf("expected no visible canonical get leaf commands, found %q", path)
		}
	}

	mustContain := []string{
		"asc apps view",
		"asc age-rating view",
		"asc pricing availability edit",
		"asc app-setup availability edit",
	}
	for _, want := range mustContain {
		if !containsPath(paths, want) {
			t.Fatalf("expected visible leaf commands to include %q", want)
		}
	}

	mustNotContain := []string{
		"asc apps get",
		"asc age-rating set",
		"asc pricing availability set",
		"asc app-setup availability set",
	}
	for _, banned := range mustNotContain {
		if containsPath(paths, banned) {
			t.Fatalf("expected visible leaf commands to omit %q", banned)
		}
	}
}

func TestDeprecatedAliasesRetainLegacyPaths(t *testing.T) {
	subs := Subcommands("dev")

	legacyPaths := []string{
		"asc apps get",
		"asc age-rating set",
		"asc pricing availability set",
		"asc app-setup availability set",
	}
	for _, legacyPath := range legacyPaths {
		cmd := findCommandByPath(subs, legacyPath)
		if cmd == nil {
			t.Fatalf("expected legacy path %q to still exist as an alias", legacyPath)
		}
		if !isDeprecatedCompatibilityAlias(cmd) {
			t.Fatalf("expected legacy path %q to be deprecated compatibility alias, got short help %q", legacyPath, cmd.ShortHelp)
		}
	}
}

func TestHelpHidesDeprecatedLegacyVerbs(t *testing.T) {
	subs := Subcommands("dev")

	tests := []struct {
		path           string
		mustContain    []string
		mustNotContain []string
	}{
		{
			path:           "asc apps",
			mustContain:    []string{"view"},
			mustNotContain: []string{"get"},
		},
		{
			path:           "asc age-rating",
			mustContain:    []string{"view", "edit"},
			mustNotContain: []string{"get", "set"},
		},
		{
			path:           "asc pricing availability",
			mustContain:    []string{"edit"},
			mustNotContain: []string{"set"},
		},
		{
			path:           "asc app-setup availability",
			mustContain:    []string{"edit"},
			mustNotContain: []string{"set"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			cmd := findCommandByPath(subs, tt.path)
			if cmd == nil {
				t.Fatalf("expected command %q", tt.path)
			}

			usage := cmd.UsageFunc(cmd)
			for _, want := range tt.mustContain {
				if !usageListsSubcommand(usage, want) {
					t.Fatalf("expected help for %q to contain %q, got %q", tt.path, want, usage)
				}
			}
			for _, banned := range tt.mustNotContain {
				if usageListsSubcommand(usage, banned) {
					t.Fatalf("expected help for %q to omit %q, got %q", tt.path, banned, usage)
				}
			}
		})
	}
}

func TestHelpRetainsCanonicalCommandsThatMentionLegacyAliases(t *testing.T) {
	subs := Subcommands("dev")

	tests := []struct {
		path        string
		subcommands []string
	}{
		{
			path:        "asc web auth",
			subcommands: []string{"login", "status", "capabilities", "logout"},
		},
		{
			path:        "asc web apps",
			subcommands: []string{"create", "availability"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			cmd := findCommandByPath(subs, tt.path)
			if cmd == nil {
				t.Fatalf("expected command %q", tt.path)
			}

			usage := cmd.UsageFunc(cmd)
			for _, want := range tt.subcommands {
				if !usageListsSubcommand(usage, want) {
					t.Fatalf("expected help for %q to contain %q, got %q", tt.path, want, usage)
				}
			}
		})
	}
}

func visibleLeafPaths(rootSubcommands []*ffcli.Command) []string {
	paths := make([]string, 0)
	for _, root := range rootSubcommands {
		collectVisibleLeafPaths(root, "asc", &paths)
	}
	return paths
}

func collectVisibleLeafPaths(cmd *ffcli.Command, prefix string, paths *[]string) {
	if cmd == nil || isDeprecatedCompatibilityAlias(cmd) {
		return
	}

	path := strings.TrimSpace(prefix + " " + strings.TrimSpace(cmd.Name))
	if len(cmd.Subcommands) == 0 {
		*paths = append(*paths, path)
		return
	}

	for _, sub := range cmd.Subcommands {
		collectVisibleLeafPaths(sub, path, paths)
	}
}

func findCommandByPath(rootSubcommands []*ffcli.Command, want string) *ffcli.Command {
	for _, root := range rootSubcommands {
		if cmd := findCommandByPathRecursive(root, "asc", want); cmd != nil {
			return cmd
		}
	}
	return nil
}

func findCommandByPathRecursive(cmd *ffcli.Command, prefix, want string) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	path := strings.TrimSpace(prefix + " " + strings.TrimSpace(cmd.Name))
	if path == want {
		return cmd
	}

	for _, sub := range cmd.Subcommands {
		if found := findCommandByPathRecursive(sub, path, want); found != nil {
			return found
		}
	}

	return nil
}

func isDeprecatedCompatibilityAlias(cmd *ffcli.Command) bool {
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

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func usageListsSubcommand(usage, name string) bool {
	inSubcommands := false
	lines := strings.Split(usage, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "SUBCOMMANDS":
			inSubcommands = true
			continue
		case trimmed == "FLAGS" || trimmed == "":
			if inSubcommands {
				return false
			}
		}
		if !inSubcommands {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		if fields[0] == name {
			return true
		}
	}
	return false
}
