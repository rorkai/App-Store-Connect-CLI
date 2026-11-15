package apps

import (
	"strings"
	"testing"
)

func TestAppsCommandRegistersPublished(t *testing.T) {
	d, _ := newAppsTestDeps()
	cmd := NewAppsCommand(d)
	for _, subcommand := range cmd.Subcommands {
		if subcommand.Name == "published" {
			if !strings.Contains(cmd.LongHelp, "asc apps published") {
				t.Fatalf("apps help does not mention published: %q", cmd.LongHelp)
			}
			return
		}
	}
	t.Fatal("expected apps published subcommand")
}

func TestAppsPublishedCommandDefaultsToJSON(t *testing.T) {
	d, _ := newAppsTestDeps()
	cmd := newAppsPublishedCommand(d)

	output := cmd.FlagSet.Lookup("output")
	if output == nil {
		t.Fatal("expected --output flag")
	}
	if output.DefValue != "json" {
		t.Fatalf("default output = %q, want json", output.DefValue)
	}
}
