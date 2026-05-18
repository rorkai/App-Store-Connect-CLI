package actors

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestActorsCommandShape(t *testing.T) {
	cmd := ActorsCommand()
	if cmd == nil {
		t.Fatal("expected actors command")
		return
	}
	if cmd.Name != "actors" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := ActorsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestActorsValidationErrors(t *testing.T) {
	t.Run("list missing id", func(t *testing.T) {
		cmd := ActorsListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		cmd := ActorsGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestNormalizeActorFields(t *testing.T) {
	got, err := normalizeActorFields("actorType,userEmail")
	if err != nil {
		t.Fatalf("expected valid fields, got err=%v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(got))
	}

	if _, err := normalizeActorFields("badField"); err == nil {
		t.Fatal("expected error for invalid field")
	}
}
