package cmdtest

import (
	"strings"
	"testing"
)

func TestAnalyticsRankedAmbiguousFlagsRemainRejected(t *testing.T) {
	// These high-volume pairs need extra resource resolution or a missing
	// subcommand, so treating them as spelling aliases would change semantics.
	// screenshots upload --locale is intentionally absent: it is a real
	// app-scoped selector that resolves the localization, not an alias.
	tests := []struct {
		path []string
		flag string
	}{
		{path: []string{"screenshots", "list"}, flag: "paginate"},
		{path: []string{"localizations", "update"}, flag: "localization-id"},
		{path: []string{"profiles", "list"}, flag: "bundle-id"},
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
