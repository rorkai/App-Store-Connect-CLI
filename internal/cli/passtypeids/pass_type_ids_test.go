package passtypeids

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestPassTypeIDsCommandShape(t *testing.T) {
	cmd := PassTypeIDsCommand()
	if cmd == nil {
		t.Fatal("expected pass-type-ids command")
		return
	}
	if cmd.Name != "pass-type-ids" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 6 {
		t.Fatalf("expected 6 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := PassTypeIDsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestPassTypeIDsValidationErrors(t *testing.T) {
	t.Run("get missing pass-type-id", func(t *testing.T) {
		cmd := PassTypeIDsGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("create missing identifier", func(t *testing.T) {
		cmd := PassTypeIDsCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "Example"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("update missing name", func(t *testing.T) {
		cmd := PassTypeIDsUpdateCommand()
		if err := cmd.FlagSet.Parse([]string{"--pass-type-id", "PASS_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		cmd := PassTypeIDsDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--pass-type-id", "PASS_ID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("certificates list missing pass-type-id", func(t *testing.T) {
		cmd := PassTypeIDCertificatesListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestPassTypeIDHelpers(t *testing.T) {
	if got, err := normalizePassTypeIDInclude("certificates"); err != nil || len(got) != 1 {
		t.Fatalf("expected valid include, got %v err=%v", got, err)
	}
	if _, err := normalizePassTypeIDInclude("bad"); err == nil {
		t.Fatal("expected include validation error")
	}

	if _, err := normalizePassTypeIDFields("invalid", "--fields"); err == nil {
		t.Fatal("expected fields validation error")
	}
}
