package crashes

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestListCommand_MissingApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := NewListCommand(shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestListCommand_InvalidLimit(t *testing.T) {
	cmd := NewListCommand(shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{"--limit", "201", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error for --limit, got %v", err)
	}
}

func TestListCommand_InvalidSort(t *testing.T) {
	cmd := NewListCommand(shared.ListCommandConfig{Name: "list"})

	if err := cmd.FlagSet.Parse([]string{"--sort", "invalid", "--app", "123"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected sort validation error, got %v", err)
	}
}

func TestListCommand_DefaultsNameAndUsageFunc(t *testing.T) {
	cmd := NewListCommand(shared.ListCommandConfig{})
	if cmd.Name != "crashes" {
		t.Fatalf("expected default name crashes, got %q", cmd.Name)
	}
	if cmd.UsageFunc == nil {
		t.Fatal("expected default usage func")
	}
	if cmd.FlagSet.Lookup("build") != nil {
		t.Fatal("expected removed --build alias to be unregistered")
	}
}
