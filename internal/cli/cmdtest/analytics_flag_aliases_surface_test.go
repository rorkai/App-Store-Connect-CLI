package cmdtest

import (
	"strings"
	"testing"
)

func TestAnalyticsRankedStringAliasesAreAcceptedAndHidden(t *testing.T) {
	tests := []struct {
		path      []string
		alias     string
		canonical string
	}{
		{path: []string{"versions", "view"}, alias: "id", canonical: "version-id"},
		{path: []string{"versions", "attach-build"}, alias: "build", canonical: "build-id"},
		{path: []string{"localizations", "list"}, alias: "version-id", canonical: "version"},
		{path: []string{"subscriptions", "view"}, alias: "subscription-id", canonical: "id"},
		{path: []string{"testflight", "groups", "view"}, alias: "group-id", canonical: "id"},
		{path: []string{"iap", "localizations", "update"}, alias: "id", canonical: "localization-id"},
		{path: []string{"subscriptions", "review", "screenshots", "delete"}, alias: "id", canonical: "screenshot-id"},
		{path: []string{"versions", "update"}, alias: "id", canonical: "version-id"},
		{path: []string{"localizations", "update"}, alias: "version-id", canonical: "version"},
		{path: []string{"apps", "view"}, alias: "app", canonical: "id"},
		{path: []string{"builds", "list"}, alias: "app-id", canonical: "app"},
		{path: []string{"bundle-ids", "capabilities", "list"}, alias: "bundle-id", canonical: "bundle"},
	}

	root := RootCommand("1.2.3")
	for _, test := range tests {
		name := strings.Join(test.path, " ") + " --" + test.alias
		t.Run(name, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %q not found", strings.Join(test.path, " "))
			}
			if command.FlagSet.Lookup(test.canonical) == nil {
				t.Fatalf("canonical flag --%s not found", test.canonical)
			}
			if command.FlagSet.Lookup(test.alias) == nil {
				t.Fatalf("analytics-ranked alias --%s not found", test.alias)
			}

			usage := command.UsageFunc(command)
			if strings.Contains(usage, "\n  --"+test.alias+" ") {
				t.Fatalf("compatibility alias --%s should stay hidden from canonical help: %q", test.alias, usage)
			}
		})
	}
}

func TestAnalyticsRankedAmbiguousFlagsRemainRejected(t *testing.T) {
	// These high-volume pairs need extra resource resolution or a missing
	// subcommand, so treating them as spelling aliases would change semantics.
	tests := []struct {
		path []string
		flag string
	}{
		{path: []string{"screenshots", "upload"}, flag: "locale"},
		{path: []string{"screenshots", "list"}, flag: "app"},
		{path: []string{"screenshots", "list"}, flag: "paginate"},
		{path: []string{"localizations", "update"}, flag: "localization-id"},
		{path: []string{"profiles", "list"}, flag: "bundle-id"},
		{path: []string{"subscriptions", "localizations", "update"}, flag: "subscription-id"},
	}

	root := RootCommand("1.2.3")
	for _, test := range tests {
		name := strings.Join(test.path, " ") + " --" + test.flag
		t.Run(name, func(t *testing.T) {
			command := findSubcommand(root, test.path...)
			if command == nil {
				t.Fatalf("command %q not found", strings.Join(test.path, " "))
			}
			if command.FlagSet.Lookup(test.flag) != nil {
				t.Fatalf("ambiguous flag --%s must not be accepted as an alias", test.flag)
			}
		})
	}
}
