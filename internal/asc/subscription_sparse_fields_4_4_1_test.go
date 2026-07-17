package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestSubscriptionSparseFields441OpenAPILedger(t *testing.T) {
	t.Parallel()

	type addition struct {
		parameter string
		value     string
	}
	want := map[string][]addition{
		"/v1/subscriptionAppStoreReviewScreenshots/{id}":             {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptionGroupLocalizations/{id}":                    {{"fields[subscriptionGroups]", "versions"}},
		"/v1/subscriptionImages/{id}":                                {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptionLocalizations/{id}":                         {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptionOfferCodes/{id}":                            {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptionPricePoints/{id}":                           {{"fields[subscriptionPricePoints]", "adjustedEqualizations"}},
		"/v1/subscriptionPromotionalOffers/{id}":                     {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptionGroups/{id}/subscriptionGroupLocalizations": {{"fields[subscriptionGroups]", "versions"}},
		"/v1/subscriptionOfferCodes/{id}/prices":                     {{"fields[subscriptionPricePoints]", "adjustedEqualizations"}},
		"/v1/subscriptionPromotionalOffers/{id}/prices":              {{"fields[subscriptionPricePoints]", "adjustedEqualizations"}},
		"/v1/subscriptions/{id}/appStoreReviewScreenshot":            {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptions/{id}/images":                              {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptions/{id}/introductoryOffers":                  {{"fields[subscriptions]", "versions"}, {"fields[subscriptionPricePoints]", "adjustedEqualizations"}},
		"/v1/subscriptions/{id}/offerCodes":                          {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptions/{id}/promotedPurchase":                    {{"fields[subscriptions]", "versions"}, {"fields[inAppPurchases]", "versions"}},
		"/v1/subscriptions/{id}/promotionalOffers":                   {{"fields[subscriptions]", "versions"}},
		"/v1/subscriptions/{id}/subscriptionLocalizations":           {{"fields[subscriptions]", "versions"}},
	}
	if len(want) != 17 {
		t.Fatalf("ledger paths = %d, want 17", len(want))
	}

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	schemaPath := filepath.Join(filepath.Dir(sourceFile), "..", "..", "docs", "openapi", "latest.json")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read OpenAPI schema: %v", err)
	}
	var schema struct {
		Paths map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("decode OpenAPI schema: %v", err)
	}

	for path, additions := range want {
		pathJSON, ok := schema.Paths[path]
		if !ok {
			t.Errorf("missing GET %s", path)
			continue
		}
		var pathItem struct {
			Get struct {
				Parameters []struct {
					Name   string `json:"name"`
					Schema struct {
						Items struct {
							Enum []string `json:"enum"`
						} `json:"items"`
					} `json:"schema"`
				} `json:"parameters"`
			} `json:"get"`
		}
		if err := json.Unmarshal(pathJSON, &pathItem); err != nil {
			t.Errorf("decode GET %s: %v", path, err)
			continue
		}
		for _, addition := range additions {
			found := false
			for _, parameter := range pathItem.Get.Parameters {
				if parameter.Name != addition.parameter {
					continue
				}
				found = slices.Contains(parameter.Schema.Items.Enum, addition.value)
				break
			}
			if !found {
				t.Errorf("GET %s missing %s=%s", path, addition.parameter, addition.value)
			}
		}
	}
}

func TestSubscriptionSparseFields441ExactQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		wantQuery url.Values
		call      func(*Client) error
	}{
		{
			name: "review screenshot detail subscription versions",
			path: "/v1/subscriptionAppStoreReviewScreenshots/shot-1",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionAppStoreReviewScreenshot(
					context.Background(), "shot-1",
					WithSubscriptionAppStoreReviewScreenshotSubscriptionFields([]string{"versions"}),
					WithSubscriptionAppStoreReviewScreenshotInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "group localization detail group versions",
			path: "/v1/subscriptionGroupLocalizations/group-loc-1",
			wantQuery: url.Values{
				"fields[subscriptionGroups]": {"versions"},
				"include":                    {"subscriptionGroup"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupLocalization(
					context.Background(), "group-loc-1",
					WithSubscriptionGroupLocalizationGroupFields([]string{"versions"}),
					WithSubscriptionGroupLocalizationInclude([]string{"subscriptionGroup"}),
				)
				return err
			},
		},
		{
			name: "image detail subscription versions",
			path: "/v1/subscriptionImages/image-1",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionImage(
					context.Background(), "image-1",
					WithSubscriptionImageSubscriptionFields([]string{"versions"}),
					WithSubscriptionImageInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "localization detail subscription versions",
			path: "/v1/subscriptionLocalizations/loc-1",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionLocalization(
					context.Background(), "loc-1",
					WithSubscriptionLocalizationSubscriptionFields([]string{"versions"}),
					WithSubscriptionLocalizationInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "offer code detail subscription versions",
			path: "/v1/subscriptionOfferCodes/code-1",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionOfferCode(
					context.Background(), "code-1",
					WithSubscriptionOfferCodeSubscriptionFields([]string{"versions"}),
					WithSubscriptionOfferCodeInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "price point detail adjusted equalizations",
			path: "/v1/subscriptionPricePoints/point-1",
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"adjustedEqualizations"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionPricePoint(
					context.Background(), "point-1",
					WithSubscriptionPricePointFields([]string{"adjustedEqualizations"}),
				)
				return err
			},
		},
		{
			name: "promotional offer detail subscription versions",
			path: "/v1/subscriptionPromotionalOffers/promo-1",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionPromotionalOffer(
					context.Background(), "promo-1",
					WithSubscriptionPromotionalOfferSubscriptionFields([]string{"versions"}),
					WithSubscriptionPromotionalOfferInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "group localizations group versions",
			path: "/v1/subscriptionGroups/group-1/subscriptionGroupLocalizations",
			wantQuery: url.Values{
				"fields[subscriptionGroups]": {"versions"},
				"include":                    {"subscriptionGroup"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupLocalizations(
					context.Background(), "group-1",
					WithSubscriptionGroupLocalizationsGroupFields([]string{"versions"}),
					WithSubscriptionGroupLocalizationsInclude([]string{"subscriptionGroup"}),
				)
				return err
			},
		},
		{
			name: "offer code prices adjusted equalizations",
			path: "/v1/subscriptionOfferCodes/code-1/prices",
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"adjustedEqualizations"},
				"include":                         {"subscriptionPricePoint"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionOfferCodePrices(
					context.Background(), "code-1",
					WithSubscriptionOfferCodePricesPricePointFields([]string{"adjustedEqualizations"}),
					WithSubscriptionOfferCodePricesInclude([]string{"subscriptionPricePoint"}),
				)
				return err
			},
		},
		{
			name: "promotional offer prices adjusted equalizations",
			path: "/v1/subscriptionPromotionalOffers/promo-1/prices",
			wantQuery: url.Values{
				"fields[subscriptionPricePoints]": {"adjustedEqualizations"},
				"include":                         {"subscriptionPricePoint"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionPromotionalOfferPrices(
					context.Background(), "promo-1",
					WithSubscriptionPromotionalOfferPricesPricePointFields([]string{"adjustedEqualizations"}),
					WithSubscriptionPromotionalOfferPricesInclude([]string{"subscriptionPricePoint"}),
				)
				return err
			},
		},
		{
			name: "subscription screenshot relationship subscription versions",
			path: "/v1/subscriptions/sub-1/appStoreReviewScreenshot",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionAppStoreReviewScreenshotForSubscription(
					context.Background(), "sub-1",
					WithSubscriptionAppStoreReviewScreenshotSubscriptionFields([]string{"versions"}),
					WithSubscriptionAppStoreReviewScreenshotInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "subscription images subscription versions",
			path: "/v1/subscriptions/sub-1/images",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionImages(
					context.Background(), "sub-1",
					WithSubscriptionImagesSubscriptionFields([]string{"versions"}),
					WithSubscriptionImagesInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "introductory offers dual sparse fields",
			path: "/v1/subscriptions/sub-1/introductoryOffers",
			wantQuery: url.Values{
				"fields[subscriptions]":           {"versions"},
				"fields[subscriptionPricePoints]": {"adjustedEqualizations"},
				"include":                         {"subscription,subscriptionPricePoint"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionIntroductoryOffers(
					context.Background(), "sub-1",
					WithSubscriptionIntroductoryOffersSubscriptionFields([]string{"versions"}),
					WithSubscriptionIntroductoryOffersPricePointFields([]string{"adjustedEqualizations"}),
					WithSubscriptionIntroductoryOffersInclude([]string{"subscription", "subscriptionPricePoint"}),
				)
				return err
			},
		},
		{
			name: "subscription offer codes subscription versions",
			path: "/v1/subscriptions/sub-1/offerCodes",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionOfferCodes(
					context.Background(), "sub-1",
					WithSubscriptionOfferCodesSubscriptionFields([]string{"versions"}),
					WithSubscriptionOfferCodesInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "subscription promotional offers subscription versions",
			path: "/v1/subscriptions/sub-1/promotionalOffers",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionPromotionalOffers(
					context.Background(), "sub-1",
					WithSubscriptionPromotionalOffersSubscriptionFields([]string{"versions"}),
					WithSubscriptionPromotionalOffersInclude([]string{"subscription"}),
				)
				return err
			},
		},
		{
			name: "subscription localizations subscription versions",
			path: "/v1/subscriptions/sub-1/subscriptionLocalizations",
			wantQuery: url.Values{
				"fields[subscriptions]": {"versions"},
				"include":               {"subscription"},
			},
			call: func(c *Client) error {
				_, err := c.GetSubscriptionLocalizations(
					context.Background(), "sub-1",
					WithSubscriptionLocalizationsSubscriptionFields([]string{"versions"}),
					WithSubscriptionLocalizationsInclude([]string{"subscription"}),
				)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodGet {
					t.Errorf("method = %s, want GET", req.Method)
				}
				if req.URL.Path != test.path {
					t.Errorf("path = %q, want %q", req.URL.Path, test.path)
				}
				if got, want := req.URL.Query().Encode(), test.wantQuery.Encode(); got != want {
					t.Errorf("query = %q, want %q", got, want)
				}
			}, jsonResponse(http.StatusOK, `{"data":null}`))
			if err := test.call(client); err != nil {
				t.Fatalf("call error: %v", err)
			}
		})
	}
}

func TestSubscriptionSparseFields441NextURLConflictsBeforeHTTP(t *testing.T) {
	t.Parallel()

	const next = "https://api.appstoreconnect.apple.com/v1/subscriptions/sub-1/images?cursor=next"
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "localizations",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionLocalizations(context.Background(), "sub-1", WithSubscriptionLocalizationsNextURL(next), WithSubscriptionLocalizationsSubscriptionFields([]string{"versions"}))
				return err
			},
		},
		{
			name: "images",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionImages(context.Background(), "sub-1", WithSubscriptionImagesNextURL(next), WithSubscriptionImagesSubscriptionFields([]string{"versions"}))
				return err
			},
		},
		{
			name: "introductory offers",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionIntroductoryOffers(context.Background(), "sub-1", WithSubscriptionIntroductoryOffersNextURL(next), WithSubscriptionIntroductoryOffersPricePointFields([]string{"adjustedEqualizations"}))
				return err
			},
		},
		{
			name: "promotional offers",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionPromotionalOffers(context.Background(), "sub-1", WithSubscriptionPromotionalOffersNextURL(next), WithSubscriptionPromotionalOffersSubscriptionFields([]string{"versions"}))
				return err
			},
		},
		{
			name: "promotional offer prices",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionPromotionalOfferPrices(context.Background(), "promo-1", WithSubscriptionPromotionalOfferPricesNextURL(next), WithSubscriptionPromotionalOfferPricesPricePointFields([]string{"adjustedEqualizations"}))
				return err
			},
		},
		{
			name: "offer codes",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionOfferCodes(context.Background(), "sub-1", WithSubscriptionOfferCodesNextURL(next), WithSubscriptionOfferCodesSubscriptionFields([]string{"versions"}))
				return err
			},
		},
		{
			name: "offer code prices",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionOfferCodePrices(context.Background(), "code-1", WithSubscriptionOfferCodePricesNextURL(next), WithSubscriptionOfferCodePricesPricePointFields([]string{"adjustedEqualizations"}))
				return err
			},
		},
		{
			name: "group localizations",
			call: func(c *Client) error {
				_, err := c.GetSubscriptionGroupLocalizations(context.Background(), "group-1", WithSubscriptionGroupLocalizationsNextURL(next), WithSubscriptionGroupLocalizationsGroupFields([]string{"versions"}))
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requests := 0
			client := newTestClient(t, func(*http.Request) { requests++ }, jsonResponse(http.StatusOK, `{"data":null}`))
			err := test.call(client)
			if err == nil || !strings.Contains(err.Error(), "next URL cannot be combined with query options") {
				t.Fatalf("error = %v, want next URL conflict", err)
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
		})
	}
}
