package cmdtest

import (
	"io"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func resetDefaultOutput(t *testing.T) {
	t.Helper()
	shared.ResetDefaultOutputFormat()
	t.Cleanup(shared.ResetDefaultOutputFormat)
}

func findCommand(root *ffcli.Command, path ...string) *ffcli.Command {
	cmd := root
	for _, name := range path {
		var next *ffcli.Command
		for _, sub := range cmd.Subcommands {
			if sub.Name == name {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		cmd = next
	}
	return cmd
}

func TestDefaultOutputEnvSetsFlagDefault(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")

	root := RootCommand("1.2.3")
	cmd := findCommand(root, "categories", "list")
	if cmd == nil {
		t.Fatal("expected categories list command")
	}

	outputFlag := cmd.FlagSet.Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected --output flag")
	}
	if got := outputFlag.DefValue; got != "table" {
		t.Fatalf("expected default output to be table, got %q", got)
	}
}

func TestDefaultOutputEnvOverriddenByExplicitFlag(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "table")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"categories", "list", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	cmd := findCommand(root, "categories", "list")
	if cmd == nil {
		t.Fatal("expected categories list command")
	}

	outputFlag := cmd.FlagSet.Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected --output flag")
	}
	if got := outputFlag.Value.String(); got != "json" {
		t.Fatalf("expected parsed output to be json, got %q", got)
	}
}
