package merchantids

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMerchantIDsCommandShape(t *testing.T) {
	cmd := MerchantIDsCommand()
	if cmd == nil {
		t.Fatal("expected merchant-ids command")
		return
	}
	if cmd.Name != "merchant-ids" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 6 {
		t.Fatalf("expected 6 subcommands, got %d", len(cmd.Subcommands))
	}
	if got := MerchantIDsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
}

func TestMerchantIDsValidationErrors(t *testing.T) {
	t.Run("list invalid limit", func(t *testing.T) {
		cmd := MerchantIDsListCommand()
		if err := cmd.FlagSet.Parse([]string{"--limit", "300"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if err == nil || errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected non-ErrHelp error, got %v", err)
		}
	})

	t.Run("get missing merchant-id", func(t *testing.T) {
		cmd := MerchantIDsGetCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("create missing identifier", func(t *testing.T) {
		cmd := MerchantIDsCreateCommand()
		if err := cmd.FlagSet.Parse([]string{"--name", "Example"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("update conflicting flags", func(t *testing.T) {
		cmd := MerchantIDsUpdateCommand()
		if err := cmd.FlagSet.Parse([]string{"--merchant-id", "MID", "--name", "Example", "--clear-name"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		cmd := MerchantIDsDeleteCommand()
		if err := cmd.FlagSet.Parse([]string{"--merchant-id", "MID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("certificates list missing merchant-id", func(t *testing.T) {
		cmd := MerchantIDsCertificatesListCommand()
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
}

func TestMerchantIDSelectionHelpers(t *testing.T) {
	if got, err := shared.NormalizeSelection("name,identifier", merchantIDFieldsList(), "--fields"); err != nil || len(got) != 2 {
		t.Fatalf("expected valid normalized selection, got %v err=%v", got, err)
	}
	if _, err := shared.NormalizeSelection("badField", merchantIDFieldsList(), "--fields"); err == nil {
		t.Fatal("expected invalid field error")
	}
}
