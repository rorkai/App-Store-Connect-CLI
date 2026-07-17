package promotedpurchases

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
)

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

func TestScopedPromotedPurchasesCommandsUseScopedPaths(t *testing.T) {
	cmd := scopedPromotedPurchasesCommandForTest()

	for _, name := range []string{"list", "view", "update", "delete", "link"} {
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

func TestScopedPromotedPurchasesShortUsageShowsAlternativeFlows(t *testing.T) {
	cmd := scopedPromotedPurchasesCommandForTest()

	listCmd := findDirectSubcommand(cmd, "list")
	if listCmd == nil {
		t.Fatal("expected list subcommand")
	}
	if !strings.Contains(listCmd.ShortUsage, "(--app APP_ID | --next URL)") {
		t.Fatalf("list ShortUsage should show app and next alternatives, got %q", listCmd.ShortUsage)
	}

	linkCmd := findDirectSubcommand(cmd, "link")
	if linkCmd == nil {
		t.Fatal("expected link subcommand")
	}
	if !strings.Contains(linkCmd.ShortUsage, "--promoted-purchase-id PROMO_ID[,PROMO_ID...] | --clear --confirm") {
		t.Fatalf("link ShortUsage should show link and clear alternatives, got %q", linkCmd.ShortUsage)
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

func TestFilterPromotedPurchaseIncludedPreservesRetainedResourcesAndOrder(t *testing.T) {
	included := json.RawMessage(`[{"type":"inAppPurchases","id":"iap-2","attributes":{"versions":["v2"],"name":"Second"}},{"type":"subscriptions","id":"subscription-1","attributes":{"versions":["subscription-version"]}},{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"First","versions":["v1"]}}]`)
	retained := map[string]struct{}{
		promotedPurchaseProductResourceKey(promotedPurchaseScope{productType: promotedPurchaseProductTypeInAppPurchase, productID: "iap-1"}): {},
		promotedPurchaseProductResourceKey(promotedPurchaseScope{productType: promotedPurchaseProductTypeInAppPurchase, productID: "iap-2"}): {},
	}

	got, err := filterPromotedPurchaseIncluded(included, retained)
	if err != nil {
		t.Fatalf("filterPromotedPurchaseIncluded() error: %v", err)
	}
	want := `[{"type":"inAppPurchases","id":"iap-2","attributes":{"versions":["v2"],"name":"Second"}},{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"First","versions":["v1"]}}]`
	if string(got) != want {
		t.Fatalf("included = %s, want exact retained attributes and order %s", got, want)
	}
}

func TestFilterPromotedPurchaseIncludedNilAndNullAreNoOps(t *testing.T) {
	tests := []struct {
		name     string
		included json.RawMessage
	}{
		{name: "nil"},
		{name: "null", included: json.RawMessage("null")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterPromotedPurchaseIncluded(tt.included, nil)
			if err != nil {
				t.Fatalf("filterPromotedPurchaseIncluded() error: %v", err)
			}
			if string(got) != string(tt.included) {
				t.Fatalf("included = %q, want unchanged %q", got, tt.included)
			}
		})
	}
}

func TestFilterPromotedPurchaseIncludedRejectsMissingIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		included json.RawMessage
	}{
		{name: "missing type", included: json.RawMessage(`[{"id":"iap-1","attributes":{"name":"Premium"}}]`)},
		{name: "missing id", included: json.RawMessage(`[{"type":"inAppPurchases","attributes":{"name":"Premium"}}]`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := filterPromotedPurchaseIncluded(tt.included, nil)
			if err == nil || !strings.Contains(err.Error(), "missing type or id") {
				t.Fatalf("error = %v, want missing type or id", err)
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
