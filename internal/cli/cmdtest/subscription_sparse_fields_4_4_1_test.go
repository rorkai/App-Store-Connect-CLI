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
			name:      "group localization detail",
			args:      []string{"subscriptions", "groups", "localizations", "view", "--id", "group-loc-1", "--group-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionGroupLocalizations/group-loc-1",
			wantQuery: url.Values{"fields[subscriptionGroups]": {"versions"}, "include": {"subscriptionGroup"}},
		},
		{
			name:      "image detail",
			args:      []string{"subscriptions", "images", "view", "--id", "image-1", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionImages/image-1",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "localization detail",
			args:      []string{"subscriptions", "localizations", "view", "--id", "loc-1", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionLocalizations/loc-1",
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
			name:      "group localizations",
			args:      []string{"subscriptions", "groups", "localizations", "list", "--group-id", "group-1", "--group-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations",
			wantQuery: url.Values{"fields[subscriptionGroups]": {"versions"}, "include": {"subscriptionGroup"}},
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
			name:      "subscription images",
			args:      []string{"subscriptions", "images", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/images",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "introductory offers",
			args:      []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--price-point-fields", "adjustedEqualizations", "--output", "json"},
			path:      "/v1/subscriptions/123456789/introductoryOffers",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "fields[subscriptionPricePoints]": {"adjustedEqualizations"}, "include": {"subscription,subscriptionPricePoint"}},
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
			name:      "subscription promotional offers",
			args:      []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/promotionalOffers",
			wantQuery: url.Values{"fields[subscriptions]": {"versions"}, "include": {"subscription"}},
		},
		{
			name:      "subscription localizations",
			args:      []string{"subscriptions", "localizations", "list", "--subscription-id", "123456789", "--subscription-fields", "versions", "--output", "json"},
			path:      "/v1/subscriptions/123456789/subscriptionLocalizations",
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
		{"group localization detail", []string{"subscriptions", "groups", "localizations", "view", "--id", "loc-1", "--group-fields", "bogus"}, "--group-fields must be one of"},
		{"image detail", []string{"subscriptions", "images", "view", "--id", "image-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"localization detail", []string{"subscriptions", "localizations", "view", "--id", "loc-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"offer code detail", []string{"subscriptions", "offers", "offer-codes", "view", "--offer-code-id", "code-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"price point detail", []string{"subscriptions", "pricing", "price-points", "view", "--price-point-id", "point-1", "--fields", "bogus"}, "--fields must be one of"},
		{"promotional offer detail", []string{"subscriptions", "offers", "promotional", "view", "--id", "promo-1", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"group localizations", []string{"subscriptions", "groups", "localizations", "list", "--group-id", "group-1", "--group-fields", "bogus"}, "--group-fields must be one of"},
		{"offer code prices", []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "code-1", "--price-point-fields", "bogus"}, "--price-point-fields must be one of"},
		{"promotional offer prices", []string{"subscriptions", "offers", "promotional", "prices", "--id", "promo-1", "--price-point-fields", "bogus"}, "--price-point-fields must be one of"},
		{"subscription screenshot relationship", []string{"subscriptions", "review", "app-store-screenshot", "view", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription images", []string{"subscriptions", "images", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"introductory subscription fields", []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"introductory price point fields", []string{"subscriptions", "offers", "introductory", "list", "--subscription-id", "123456789", "--price-point-fields", "bogus"}, "--price-point-fields must be one of"},
		{"subscription offer codes", []string{"subscriptions", "offers", "offer-codes", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription promoted purchase invalid iap fields", []string{"subscriptions", "promoted-purchases", "view", "--subscription-id", "123456789", "--iap-fields", "bogus"}, "--iap-fields must be one of"},
		{"subscription promoted purchase invalid subscription fields", []string{"subscriptions", "promoted-purchases", "view", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription promoted purchase selector conflict", []string{"subscriptions", "promoted-purchases", "view", "--subscription-id", "123456789", "--promoted-purchase-id", "promo-1"}, "--promoted-purchase-id and --subscription-id are mutually exclusive"},
		{"subscription promotional offers", []string{"subscriptions", "offers", "promotional", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription localizations", []string{"subscriptions", "localizations", "list", "--subscription-id", "123456789", "--subscription-fields", "bogus"}, "--subscription-fields must be one of"},
		{"subscription fields reject price point field", []string{"subscriptions", "images", "view", "--id", "image-1", "--subscription-fields", "adjustedEqualizations"}, "--subscription-fields must be one of"},
		{"group fields reject subscription field", []string{"subscriptions", "groups", "localizations", "view", "--id", "loc-1", "--group-fields", "productId"}, "--group-fields must be one of"},
		{"price point fields reject subscription field", []string{"subscriptions", "offers", "offer-codes", "prices", "--offer-code-id", "code-1", "--price-point-fields", "versions"}, "--price-point-fields must be one of"},
		{"explicit empty", []string{"subscriptions", "images", "view", "--id", "image-1", "--subscription-fields", ""}, "--subscription-fields must not be empty"},
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
	const next = "https://api.appstoreconnect.apple.com/v1/subscriptions/123456789/images?cursor=next"
	tests := []struct {
		name string
		args []string
	}{
		{"group localizations", []string{"subscriptions", "groups", "localizations", "list", "--next", next, "--group-fields", "versions"}},
		{"offer code prices", []string{"subscriptions", "offers", "offer-codes", "prices", "--next", next, "--price-point-fields", "adjustedEqualizations"}},
		{"promotional offer prices", []string{"subscriptions", "offers", "promotional", "prices", "--next", next, "--price-point-fields", "adjustedEqualizations"}},
		{"images", []string{"subscriptions", "images", "list", "--next", next, "--subscription-fields", "versions"}},
		{"introductory offers subscription", []string{"subscriptions", "offers", "introductory", "list", "--next", next, "--subscription-fields", "versions"}},
		{"introductory offers price point", []string{"subscriptions", "offers", "introductory", "list", "--next", next, "--price-point-fields", "adjustedEqualizations"}},
		{"offer codes", []string{"subscriptions", "offers", "offer-codes", "list", "--next", next, "--subscription-fields", "versions"}},
		{"promotional offers", []string{"subscriptions", "offers", "promotional", "list", "--next", next, "--subscription-fields", "versions"}},
		{"localizations", []string{"subscriptions", "localizations", "list", "--next", next, "--subscription-fields", "versions"}},
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
			if !strings.Contains(stderr, "--next cannot be combined") {
				t.Fatalf("stderr = %q", stderr)
			}
			if clientFactoryCalled {
				t.Fatal("client factory called before validation")
			}
		})
	}
}
