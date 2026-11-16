package shared

import (
	"flag"
	"testing"
)

// Command trees are rebuilt on every RunWithDeps call, so an alias must be
// hidden from help without being added to the process-wide registry, which
// never forgets a flag.
func TestBindFlagAliasHidesWithoutGlobalRegistration(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("group-id", "", "Group ID")

	hiddenCommandHelpRegistry.RLock()
	before := len(hiddenCommandHelpRegistry.flags)
	hiddenCommandHelpRegistry.RUnlock()

	BindFlagAlias(fs, "group", "group-id")

	hiddenCommandHelpRegistry.RLock()
	after := len(hiddenCommandHelpRegistry.flags)
	hiddenCommandHelpRegistry.RUnlock()
	if after != before {
		t.Fatalf("hidden-flag registry grew from %d to %d", before, after)
	}
	visible := VisibleHelpFlags(fs)
	if len(visible) != 1 || visible[0].Name != "group-id" {
		t.Fatalf("visible help flags = %v, want only --group-id", visible)
	}
}
