package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSubscriptionSparseFields441CommandQueries(t *testing.T) {
	setupAuth(t)

	tests := []struct {
		name      string
		args      []string
		path      string
		wantQuery url.Values
	}{
		{
			name:      "review screenshot detail",
			args:      []string{"subscriptions", "review", "screenshots", "view", "--screenshot-id", "shot-1", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionAppStoreReviewScreenshots/shot-1",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "offer code detail",
			args:      []string{"subscriptions", "offers", "offer-codes", "view", "--offer-code-id", "code-1", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionOfferCodes/code-1",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "price point detail",
			args:      []string{"subscriptions", "pricing", "price-points", "view", "--price-point-id", "point-1", "--fields", "adjustedEqualizations", "--output", "json"},
			path:      "/v1/subscriptionPricePoints/point-1",
			wantQuery: url.Values{"fields[subscriptionPricePoints]": {"adjustedEqualizations"}},
		},
		{
			name:      "promotional offer detail",
			args:      []string{"subscriptions", "offers", "promotional", "view", "--id", "promo-1", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionPromotionalOffers/promo-1",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "offer code prices",
			args:      []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "code-1", "--price-point-fields", "adjustedEqualizations", "--output", "json"},
			path:      "/v1/subscriptionOfferCodes/code-1/prices",
			wantQuery: url.Values{"fields[subscriptionPricePoints]": {"adjustedEqualizations"}, "include": {"subscriptionPricePoint"}},
		},
		{
			name:      "promotional offer prices",
			args:      []string{"subscriptions", "offers", "promotional", "prices", "--id", "promo-1", "--price-point-fields", "adjustedEqualizations", "--output", "json"},
			path:      "/v1/subscriptionPromotionalOffers/promo-1/prices",
			wantQuery: url.Values{"fields[subscriptionPricePoints]": {"adjustedEqualizations"}, "include": {"subscriptionPricePoint"}},
		},
		{
			name:      "subscription screenshot relationship",
			args:      []string{"subscriptions", "review", "app-store-screenshot", "view", "--subscription-id", "123456789", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/appStoreReviewScreenshot",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "introductory offers",
			args:      []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--price-point-fields", "adjustedEqualizations", "--output", "json"},
			path:      "/v1/subscriptions/123456789/introductoryOffers",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "fields[subscriptionPricePoints]": {"adjustedEqualizations"}, "include": {"subscription,subscriptionPricePoint"}},
		},
		{
			name:      "introductory offers territory and include",
			args:      []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--territory", "usa,United Kingdom", "--include", "territory,subscriptionPricePoint", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/introductoryOffers",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "filter[territory]": {"USA,GBR"}, "include": {"territory,subscriptionPricePoint,subscription"}},
		},
		{
			name:      "subscription offer codes",
			args:      []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/offerCodes",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name: "subscription promoted purchase",
			args: []string{
				"subscriptions", "promoted-purchases", "view",
				"--subscription-id", "123456789",
				"--iap-fields", "versions",
				"--subscription-fields", "versions",
				"--output", "json",
			},
			path: "/v1/subscriptions/123456789/promotedPurchase",
			wantQuery: url.Values{
				"fields[inAppPurchases]": {"versions"},
				"fields[subscriptions]":  {"versions"},
				"include":                {"inAppPurchaseV2,subscription"},
			},
		},
		{
			name: "subscription promoted purchase market fields",
			args: []string{
				"subscriptions", "promoted-purchases", "view",
				"--subscription-id", "123456789",
				"--subscription-fields", "marketSettings,multiSeatStatus",
				"--output", "json",
			},
			path: "/v1/subscriptions/123456789/promotedPurchase",
			wantQuery: url.Values{
				"fields[subscriptions]": {"marketSettings,multiSeatStatus"},
				"include":               {"subscription"},
			},
		},
		{
			name: "win-back offer prices adjusted equalizations",
			args: []string{
				"subscriptions", "offers", "win-back", "prices",
				"--id", "offer-1",
				"--include", "subscriptionPricePoint",
				"--price-point-fields", "adjustedEqualizations",
				"--output", "json",
			},
			path: "/v1/winBackOffers/offer-1/prices",
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"adjustedEqualizations"},
				"include":                         {"subscriptionPricePoint"},
			},
		},
		{
			name:      "subscription promotional offers",
			args:      []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/promotionalOffers",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			useSubscriptionVersionServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					reportSubscriptionVersionHandlerError(t, w, "request = %s %s, want GET %s", req.Method, req.URL.Path, test.path)
					return
				}
				if got, want := req.URL.Query().Encode(), test.wantQuery.Encode(); got != want {
					reportSubscriptionVersionHandlerError(t, w, "query = %q, want %q", got, want)
					return
				}
				body := `{"data":null}`
				if req.URL.Path == "/v1/subscriptions/123456789/appStoreReviewScreenshot" {
					body = `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1"}}`
				}
				writeSubscriptionVersionJSON(w, http.StatusOK, body)
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if strings.Contains(stderr, "Error:") {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestSubscriptionSparseFields441ValidationBeforeClient(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"review screenshot detail", []string{"subscriptions", "review", "screenshots", "view", "--screenshot-id", "shot-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"offer code detail", []string{"subscriptions", "offers", "offer-codes", "view", "--offer-code-id", "code-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"price point detail", []string{"subscriptions", "pricing", "price-points", "view", "--price-point-id", "point-1", "--fields", "bogus"}, "--fields must be one of"},
		{"promotional offer detail", []string{"subscriptions", "offers", "promotional", "view", "--id", "promo-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"offer code prices", []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "code-1", "--price-point-fields", "bogus"}, "--price-point-fields must be one of"},
		{"promotional offer prices", []string{"subscriptions", "offers", "promotional", "prices", "--id", "promo-1", "--price-point-fields", "bogus"}, "--price-point-fields must be one of"},
		{"subscription screenshot relationship", []string{"subscriptions", "review", "app-store-screenshot", "view", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"introductory subscription fields", []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"introductory price point fields", []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--price-point-fields", "bogus"}, "--price-point-fields must be one of"},
		{"introductory include", []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--include", "versions"}, "--include must be one of"},
		{"introductory territory", []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--territory", "USA,Narnia"}, `territory "Narnia" could not be mapped`},
		{"subscription offer codes", []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription promoted purchase invalid iap fields", []string{"subscriptions", "promoted-purchases", "view", "--subscription-id", "123456789", "--iap-fields", "bogus"}, "--iap-fields must be one of"},
		{"subscription promoted purchase invalid subscription fields", []string{"subscriptions", "promoted-purchases", "view", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription promoted purchase selector conflict", []string{"subscriptions", "promoted-purchases", "view", "--subscription-id", "123456789", "--promoted-purchase-id", "promo-1"}, "--promoted-purchase-id and --subscription-id are mutually exclusive"},
		{"subscription promotional offers", []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription fields reject price point field", []string{"subscriptions", "offers", "offer-codes", "view", "--offer-code-id", "code-1", "--subscription-fields", "adjustedEqualizations"}, "--subscription-fields must be one of"},
		{"price point fields reject subscription field", []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "code-1", "--price-point-fields", "versions"}, "--price-point-fields must be one of"},
		{"explicit empty", []string{"subscriptions", "offers", "offer-codes", "view", "--offer-code-id", "code-1", "--subscription-fields", ""}, "--subscription-fields must not be empty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clientFactoryCalled := false
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client factory must not run")
			})
			defer restore()

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			_, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("error = %v, want usage error", err)
				}
			})
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if clientFactoryCalled {
				t.Fatal("client factory called before validation")
			}
		})
	}
}

func TestSubscriptionSparseFields441NextConflictsBeforeClient(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/subscriptions/123456789/introductoryOffers?cursor=next"
	commands := []struct {
		name      string
		base      []string
		modifiers [][]string
	}{
		{
			name: "introductory offers", base: []string{"subscriptions", "offers", "introductory", "list"},
			modifiers: append(subscriptionNextModifierFamilies441(), []string{"--price-point-fields", "adjustedEqualizations"}, []string{"--territory", "USA"}, []string{"--include", "territory"}),
		},
		{
			name: "offer codes", base: []string{"subscriptions", "offers", "offer-codes", "list"},
			modifiers: subscriptionNextModifierFamilies441(),
		},
		{
			name: "promotional offers", base: []string{"subscriptions", "offers", "promotional", "list"},
			modifiers: subscriptionNextModifierFamilies441(),
		},
		{
			name: "offer code prices", base: []string{"subscriptions", "offers", "offer-codes", "prices"},
			modifiers: [][]string{{"--offer-code-id", "code-1"}, {"--limit", "10"}, {"--price-point-fields", "adjustedEqualizations"}},
		},
		{
			name: "promotional offer prices", base: []string{"subscriptions", "offers", "promotional", "prices"},
			modifiers: [][]string{{"--id", "promo-1"}, {"--limit", "10"}, {"--price-point-fields", "adjustedEqualizations"}},
		},
	}

	for _, command := range commands {
		for _, modifier := range command.modifiers {
			modifier := modifier
			t.Run(command.name+" "+modifier[0], func(t *testing.T) {
				clientFactoryCalled := false
				restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
					clientFactoryCalled = true
					return nil, errors.New("client factory must not run")
				})
				defer restore()

				args := append(append(append([]string{}, command.base...), "--next", next), modifier...)
				stdout, stderr := captureOutput(t, func() {
					if code := cmd.Run(args, "1.2.3"); code != cmd.ExitUsage {
						t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
					}
				})
				if strings.TrimSpace(stdout) != "" {
					t.Fatalf("stdout = %q, want empty", stdout)
				}
				if !strings.Contains(stderr, "--next cannot be combined with "+modifier[0]) {
					t.Fatalf("stderr = %q, want conflict for %s", stderr, modifier[0])
				}
				if clientFactoryCalled {
					t.Fatal("client factory called before validation")
				}
			})
		}
	}
}

func subscriptionNextModifierFamilies441() [][]string {
	return [][]string{
		{"--subscription-id", "123456789"},
		{"--app", "app-1"},
		{"--limit", "10"},
		{"--subscription-fields", "versions"},
	}
}
