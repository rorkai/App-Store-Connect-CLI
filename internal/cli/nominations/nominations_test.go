package nominations

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestNominationsValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	t.Run("list missing status", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newNominationsListCommand(d)
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		assertUsageOutput(t, stdout, stderr, "Error: --status is required\n")
	})

	t.Run("get missing id", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newNominationsGetCommand(d)
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		assertUsageOutput(t, stdout, stderr, "Error: --id is required\n")
	})

	t.Run("create missing submitted flag", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newNominationsCreateCommand(d)
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
		assertUsageOutput(t, stdout, stderr, "Error: --submitted is required\n")
	})

	t.Run("update missing submitted and archived", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newNominationsUpdateCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--id", "NOM_ID", "--notes", "updated"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		assertUsageOutput(t, stdout, stderr, "Error: --submitted or --archived is required by Apple's live API\n")
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newNominationsDeleteCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--id", "NOM_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		assertUsageOutput(t, stdout, stderr, "Error: --confirm is required to delete\n")
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

func newTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}}, &stdout, &stderr
}

func assertUsageOutput(t *testing.T, stdout, stderr *bytes.Buffer, wantStderr string) {
	t.Helper()
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); got != wantStderr {
		t.Fatalf("stderr = %q, want %q", got, wantStderr)
	}
}
