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
	// screenshots list --paginate is likewise a real flag scoped to --set-id;
	// TestScreenshotsListBySetIDUsageErrors keeps it rejected without --set-id.
	tests := []struct {
		path []string
		flag string
	}{
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
