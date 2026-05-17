package nominations

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestNominationsCommandShape(t *testing.T) {
	cmd := NominationsCommand()
	if cmd == nil {
		t.Fatal("expected nominations command")
		return
	}
	if cmd.Name != "nominations" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 5 {
		t.Fatalf("expected 5 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := NominationsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestNominationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	t.Run("list missing status", func(t *testing.T) {
		cmd := NominationsListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("get missing id", func(t *testing.T) {
		cmd := NominationsGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("create missing submitted flag", func(t *testing.T) {
		cmd := NominationsCreateCommand()
		args := []string{
			"--app", "APP_ID",
			"--name", "Launch",
			"--type", "APP_LAUNCH",
			"--description", "Desc",
			"--publish-start-date", "2026-02-01T08:00:00Z",
		}
		if err := cmd.FlagSet.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("update missing submitted and archived", func(t *testing.T) {
		cmd := NominationsUpdateCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "NOM_ID", "--notes", "updated"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		cmd := NominationsDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--id", "NOM_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestNominationHelpers(t *testing.T) {
	if _, err := normalizeNominationType("bad"); err == nil {
		t.Fatal("expected invalid type error")
	}
	if got, err := normalizeNominationType("app_launch"); err != nil || got != "APP_LAUNCH" {
		t.Fatalf("expected APP_LAUNCH, got %q err=%v", got, err)
	}

	if _, err := normalizeNominationPublishDate("--publish-start-date", "bad", true); err == nil {
		t.Fatal("expected date parsing error")
	}

	ids := []string{"app-1", "app-2"}
	rel := buildNominationRelationshipList(asc.ResourceTypeApps, ids)
	if rel == nil || len(rel.Data) != 2 {
		t.Fatalf("expected relationship list with 2 items, got %#v", rel)
	}
}
