package promotedpurchases

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

func TestPromotedPurchasesCommandConstructors(t *testing.T) {
	top := PromotedPurchasesCommand()
	if top == nil {
		t.Fatal("expected promoted-purchases command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}
}

func TestPromotedPurchasesListValidation(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	cmd := PromotedPurchasesListCommand()
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := cmd.Exec(context.Background(), nil); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestScopedPromotedPurchasesDetailCommandsUseScopedPaths(t *testing.T) {
	cmd := scopedPromotedPurchasesCommandForTest()

	for _, name := range []string{"view", "update", "delete"} {
		t.Run(name, func(t *testing.T) {
			subcommand := findDirectSubcommand(cmd, name)
			if subcommand == nil {
				t.Fatalf("expected %q subcommand", name)
			}
			for label, text := range map[string]string{
				"short usage": subcommand.ShortUsage,
				"long help":   subcommand.LongHelp,
			} {
				if !strings.Contains(text, "asc iap promoted-purchases "+name) {
					t.Fatalf("%s should use scoped path, got %q", label, text)
				}
				if strings.Contains(text, "asc promoted-purchases "+name) {
					t.Fatalf("%s leaked generic path: %q", label, text)
				}
			}
		})
	}
}

func TestScopedPromotedPurchasesDetailCommandsUseScopedErrorPrefixes(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")

	tests := []struct {
		name string
		args []string
	}{
		{name: "view", args: []string{"--promoted-purchase-id", "PROMO_ID"}},
		{name: "update", args: []string{"--promoted-purchase-id", "PROMO_ID", "--enabled", "true"}},
		{name: "delete", args: []string{"--promoted-purchase-id", "PROMO_ID", "--confirm"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := scopedPromotedPurchasesCommandForTest()
			subcommand := findDirectSubcommand(cmd, tt.name)
			if subcommand == nil {
				t.Fatalf("expected %q subcommand", tt.name)
			}
			if err := subcommand.FlagSet.Parse(tt.args); err != nil {
				t.Fatalf("parse error: %v", err)
			}

			err := subcommand.Exec(context.Background(), nil)
			if err == nil {
				t.Fatal("expected auth error")
			}
			wantPrefix := "iap promoted-purchases " + tt.name + ":"
			if !strings.HasPrefix(err.Error(), wantPrefix) {
				t.Fatalf("expected error prefix %q, got %q", wantPrefix, err.Error())
			}
		})
	}
}

func scopedPromotedPurchasesCommandForTest() *ffcli.Command {
	cmd := PromotedPurchasesCommand()
	cmd.ShortUsage = "asc iap promoted-purchases <subcommand> [flags]"
	ConfigureScopedPromotedPurchasesCommand(cmd, ScopedPromotedPurchasesCommandConfig{
		PathPrefix:      "asc iap promoted-purchases",
		ProductType:     promotedPurchaseProductTypeInAppPurchase,
		ProductSingular: "in-app purchase",
		ProductPlural:   "in-app purchases",
	})
	return cmd
}
