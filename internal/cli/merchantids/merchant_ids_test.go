package merchantids

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMerchantIDsValidationErrors(t *testing.T) {
	d := &shared.Deps{IO: &shared.IO{In: strings.NewReader(""), Out: io.Discard, ErrOut: io.Discard}}

	t.Run("list invalid limit", func(t *testing.T) {
		cmd := newMerchantIDsListCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--limit", "300"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := cmd.Exec(context.Background(), nil)
		if err == nil || !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})

	t.Run("get missing merchant-id", func(t *testing.T) {
		cmd := newMerchantIDsGetCommand(d)
		if err := cmd.FlagSet.Parse([]string{}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("create missing identifier", func(t *testing.T) {
		cmd := newMerchantIDsCreateCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--name", "Example"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("update conflicting flags", func(t *testing.T) {
		cmd := newMerchantIDsUpdateCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--merchant-id", "MID", "--name", "Example", "--clear-name"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("delete missing confirm", func(t *testing.T) {
		cmd := newMerchantIDsDeleteCommand(d)
		if err := cmd.FlagSet.Parse([]string{"--merchant-id", "MID"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	t.Run("certificates list missing merchant-id", func(t *testing.T) {
		cmd := newMerchantIDsCertificatesListCommand(d)
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
