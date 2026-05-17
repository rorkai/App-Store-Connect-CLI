package routingcoverage

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestRoutingCoverageCommandShape(t *testing.T) {
	cmd := RoutingCoverageCommand()
	if cmd == nil {
		t.Fatal("expected routing-coverage command")
		return
	}
	if cmd.Name != "routing-coverage" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 4 {
		t.Fatalf("expected 4 subcommands, got %d", len(cmd.Subcommands))
	}
}

func TestRoutingCoverageGetCommand_MissingVersionID(t *testing.T) {
	cmd := RoutingCoverageGetCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestRoutingCoverageInfoCommand_MissingID(t *testing.T) {
	cmd := RoutingCoverageInfoCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", err)
	}
}

func TestRoutingCoverageCreateCommand_MissingRequiredFlags(t *testing.T) {
	t.Run("missing version-id", func(t *testing.T) {
		cmd := RoutingCoverageCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--file", "coverage.geojson"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		cmd := RoutingCoverageCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--version-id", "VERSION_ID"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestRoutingCoverageDeleteCommandValidation(t *testing.T) {
	t.Run("missing id", func(t *testing.T) {
		cmd := RoutingCoverageDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--confirm"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})

	t.Run("missing confirm", func(t *testing.T) {
		cmd := RoutingCoverageDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "COVERAGE_ID"}); err != nil {
			t.Fatalf("failed to parse flags: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected flag.ErrHelp, got %v", err)
		}
	})
}

func TestCommandWrapper(t *testing.T) {
	if got := RoutingCoverageCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}
