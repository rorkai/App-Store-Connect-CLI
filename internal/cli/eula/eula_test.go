package eula

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestEULACommandShape(t *testing.T) {
	cmd := EULACommand()
	if cmd == nil {
		t.Fatal("expected eula command")
		return
	}
	if cmd.Name != "eula" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 5 {
		t.Fatalf("expected 5 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := EULACommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestEULAValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	t.Run("get missing id and app", func(t *testing.T) {
		cmd := EULAGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("get id and app are mutually exclusive", func(t *testing.T) {
		cmd := EULAGetCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "EULA_ID", "--app", "APP_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("list missing app", func(t *testing.T) {
		cmd := EULAListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("create missing app", func(t *testing.T) {
		cmd := EULACreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--agreement-text", "Terms", "--territory", "USA"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("update missing id", func(t *testing.T) {
		cmd := EULAUpdateCommand()
		if err := cmd.FlagSet.Parse([]string{"--agreement-text", "Terms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		cmd := EULADeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "EULA_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}
