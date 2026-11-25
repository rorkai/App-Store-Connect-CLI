package subscriptions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReadSubscriptionPriceTiersInputRejectsInvalidTierDocument(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{`[{"subscriptionPricePointId":"point"}]`, "minQuantity is required"},
		{`[{"minQuantity":2,"pricePointId":"point"}]`, "unknown field"},
		{`[{"minQuantity":2,"subscriptionPricePointId":""}]`, "subscriptionPricePointId is required"},
		{`[{"minQuantity":1,"subscriptionPricePointId":"point"}]`, "minQuantity must be at least 2"},
		{`[{"minQuantity":10,"subscriptionPricePointId":"point"},{"minQuantity":2,"subscriptionPricePointId":"point-2"}]`, "ascending order"},
		{`[{"minQuantity":2,"subscriptionPricePointId":"point"},{"minQuantity":2,"subscriptionPricePointId":"point-2"}]`, "distinct"},
		{`[{"minQuantity":2,"subscriptionPricePointId":"p2"},{"minQuantity":3,"subscriptionPricePointId":"p3"},{"minQuantity":4,"subscriptionPricePointId":"p4"},{"minQuantity":5,"subscriptionPricePointId":"p5"},{"minQuantity":6,"subscriptionPricePointId":"p6"}]`, "at most four"},
		{`[] {}`, "one JSON value"},
	} {
		t.Run(test.want, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tiers.json")
			if err := os.WriteFile(path, []byte(test.input), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readSubscriptionPriceTiersInput(shared.DefaultDeps(), path)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want %q", err, test.want)
			}
		})
	}
}

func TestReadSubscriptionPriceTiersInputPreservesExplicitEmptyAndLargeQuantity(t *testing.T) {
	for _, input := range []string{`[]`, `null`, `[{"minQuantity":250,"subscriptionPricePointId":"point"}]`} {
		t.Run(input, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tiers.json")
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			tiers, err := readSubscriptionPriceTiersInput(shared.DefaultDeps(), path)
			if err != nil {
				t.Fatal(err)
			}
			if tiers == nil {
				t.Fatal("explicit document was omitted")
			}
			if input == `[]` && *tiers == nil {
				t.Fatal("empty array became null")
			}
			if input == `null` && *tiers != nil {
				t.Fatalf("null became %+v", *tiers)
			}
			if strings.Contains(input, "250") && (len(*tiers) != 1 || (*tiers)[0].MinQuantity != 250) {
				t.Fatalf("tiers=%+v", tiers)
			}
		})
	}
}
