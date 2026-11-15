package versions

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestVersionsReleaseCommand_MissingVersionID(t *testing.T) {
	d, _ := newVersionsTestDeps(nil)
	cmd := newVersionsReleaseCommand(d)

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if err := cmd.Exec(context.Background(), []string{}); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --version-id is missing, got %v", err)
	}
}

func TestVersionsReleaseCommand_FlagDefinitions(t *testing.T) {
	d, _ := newVersionsTestDeps(nil)
	cmd := newVersionsReleaseCommand(d)
	expectedFlags := []string{"version-id", "confirm", "output", "pretty"}
	for _, name := range expectedFlags {
		if cmd.FlagSet.Lookup(name) == nil {
			t.Errorf("expected flag --%s to be defined", name)
		}
	}
}

func TestVersionsReleaseCommand_DefaultOutputJSON(t *testing.T) {
	d, _ := newVersionsTestDeps(nil)
	cmd := newVersionsReleaseCommand(d)
	f := cmd.FlagSet.Lookup("output")
	if f == nil {
		t.Fatal("expected --output flag to be defined")
		return
	}
	if f.DefValue != "json" {
		t.Errorf("expected --output default to be 'json', got %q", f.DefValue)
	}
}
