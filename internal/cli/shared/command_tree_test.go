package shared

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestVisibleHelpFlagsFiltersHiddenFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = fs.String("visible", "", "visible flag")
	_ = fs.String("hidden", "", "hidden flag")
	HideFlagFromHelp(fs.Lookup("hidden"))

	visible := VisibleHelpFlags(fs)
	if len(visible) != 1 {
		t.Fatalf("expected 1 visible flag, got %d", len(visible))
	}
	if visible[0].Name != "visible" {
		t.Fatalf("expected visible flag to remain, got %q", visible[0].Name)
	}
}

func TestRewriteCommandTreePathRewritesRuntimeErrorPrefix(t *testing.T) {
	cmd := &ffcli.Command{
		Name:       "values",
		ShortUsage: "asc offer-codes values [flags]",
		Exec: func(ctx context.Context, args []string) error {
			return fmt.Errorf("offer-codes values: %w", errors.New("boom"))
		},
	}

	rewritten := RewriteCommandTreePath(cmd, "asc offer-codes", "asc subscriptions offer-codes")
	if rewritten == nil {
		t.Fatal("expected rewritten command")
	}

	err := rewritten.Exec(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "subscriptions offer-codes values: boom" {
		t.Fatalf("unexpected rewritten error: %q", got)
	}
}
