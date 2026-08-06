package assetlibrary

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestCommandIsBehaviorlessInternalSkeleton(t *testing.T) {
	cmd := Command()
	if cmd == nil {
		t.Fatal("expected command constructor to return a command")
	}
	if cmd.Name != "asset-library" {
		t.Fatalf("expected asset-library command name, got %q", cmd.Name)
	}
	if cmd.FlagSet == nil {
		t.Fatal("expected a flag set")
	}
	registeredFlags := 0
	cmd.FlagSet.VisitAll(func(*flag.Flag) {
		registeredFlags++
	})
	if registeredFlags != 0 {
		t.Fatalf("expected no speculative flags, got %d", registeredFlags)
	}
	if len(cmd.Subcommands) != 0 {
		t.Fatalf("expected no speculative subcommands, got %d", len(cmd.Subcommands))
	}
	if cmd.ShortUsage != "" || cmd.ShortHelp != "" || cmd.LongHelp != "" {
		t.Fatalf("expected no public help copy, got usage=%q short=%q long=%q", cmd.ShortUsage, cmd.ShortHelp, cmd.LongHelp)
	}
	if cmd.UsageFunc == nil {
		t.Fatal("expected repository-standard usage function")
	}
	if !errors.Is(cmd.Exec(context.Background(), nil), flag.ErrHelp) {
		t.Fatal("expected behaviorless command group to return flag.ErrHelp")
	}
}
