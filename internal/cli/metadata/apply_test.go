package metadata

import "testing"

func TestMetadataApplyCommandUsesApplyFlagSetName(t *testing.T) {
	cmd := newMetadataApplyCommand(metadataTestDeps())
	if cmd.FlagSet == nil {
		t.Fatal("expected apply command flag set")
	}
	if got := cmd.FlagSet.Name(); got != "metadata apply" {
		t.Fatalf("expected metadata apply flag set name, got %q", got)
	}
}
