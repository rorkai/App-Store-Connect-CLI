package eula

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &stdout, ErrOut: &stderr}}, &stdout, &stderr
}

func TestEULACommandShape(t *testing.T) {
	t.Parallel()

	d, _, _ := newTestDeps()
	cmd := NewEULACommand(d)
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
	if got := NewEULACommand(d); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestEULAValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	t.Run("get missing id and app", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newEULAGetCommand(d)
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if got, want := stderr.String(), "Error: --id or --app is required (or set ASC_APP_ID)\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("get id and app are mutually exclusive", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newEULAGetCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--id", "EULA_ID", "--app", "APP_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if got, want := stderr.String(), "Error: --id and --app are mutually exclusive\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("list missing app", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newEULAListCommand(d)
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if got, want := stderr.String(), "Error: --app is required (or set ASC_APP_ID)\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("create missing app", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newEULACreateCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--agreement-text", "Terms", "--territory", "USA"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if got, want := stderr.String(), "Error: --app is required (or set ASC_APP_ID)\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("update missing id", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newEULAUpdateCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--agreement-text", "Terms"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if got, want := stderr.String(), "Error: --id is required\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		d, stdout, stderr := newTestDeps()
		cmd := newEULADeleteCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--id", "EULA_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
		if got, want := stderr.String(), "Error: --confirm is required\n"; got != want {
			t.Fatalf("stderr = %q, want %q", got, want)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
	})
}
