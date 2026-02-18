package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestIAPRelationshipEndpoints(t *testing.T) {
	t.Run("to-many relationships use LinkagesResponse", func(t *testing.T) {
		linkagesBody := `{"data":[],"links":{}}`

		cases := []struct {
			name     string
			call     func(*Client) error
			wantPath string
		}{
			{
				name: "iap availability available territories",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseAvailabilityAvailableTerritoriesRelationships(context.Background(), "avail-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchaseAvailabilities/avail-1/relationships/availableTerritories",
			},
			{
				name: "iap offer code custom codes",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseOfferCodeCustomCodesRelationships(context.Background(), "oc-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchaseOfferCodes/oc-1/relationships/customCodes",
			},
			{
				name: "iap offer code one-time use codes",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseOfferCodeOneTimeUseCodesRelationships(context.Background(), "oc-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchaseOfferCodes/oc-1/relationships/oneTimeUseCodes",
			},
			{
				name: "iap offer code prices",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseOfferCodePricesRelationships(context.Background(), "oc-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchaseOfferCodes/oc-1/relationships/prices",
			},
			{
				name: "iap price point equalizations",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchasePricePointEqualizationsRelationships(context.Background(), "pp-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchasePricePoints/pp-1/relationships/equalizations",
			},
			{
				name: "iap price schedule automatic prices",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchasePriceScheduleAutomaticPricesRelationships(context.Background(), "sch-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchasePriceSchedules/sch-1/relationships/automaticPrices",
			},
			{
				name: "iap price schedule manual prices",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchasePriceScheduleManualPricesRelationships(context.Background(), "sch-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v1/inAppPurchasePriceSchedules/sch-1/relationships/manualPrices",
			},
			{
				name: "in-app purchase images",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseImagesRelationships(context.Background(), "iap-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/images",
			},
			{
				name: "in-app purchase localizations",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseInAppPurchaseLocalizationsRelationships(context.Background(), "iap-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/inAppPurchaseLocalizations",
			},
			{
				name: "in-app purchase offer codes",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseOfferCodesRelationships(context.Background(), "iap-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/offerCodes",
			},
			{
				name: "in-app purchase price points",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchasePricePointsRelationships(context.Background(), "iap-1", WithLinkagesLimit(4))
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/pricePoints",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				response := jsonResponse(http.StatusOK, linkagesBody)
				client := newTestClient(t, func(req *http.Request) {
					if req.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", req.Method)
					}
					if req.URL.Path != tc.wantPath {
						t.Fatalf("expected path %s, got %s", tc.wantPath, req.URL.Path)
					}
					if req.URL.Query().Get("limit") != "4" {
						t.Fatalf("expected limit=4, got %q", req.URL.Query().Get("limit"))
					}
					assertAuthorized(t, req)
				}, response)

				if err := tc.call(client); err != nil {
					t.Fatalf("request error: %v", err)
				}
			})
		}
	})

	t.Run("to-one relationships use ResourceData linkage", func(t *testing.T) {
		toOneBody := `{"data":{"type":"apps","id":"a1"},"links":{}}`

		cases := []struct {
			name     string
			call     func(*Client) error
			wantPath string
		}{
			{
				name: "iap price schedule base territory",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchasePriceScheduleBaseTerritoryRelationship(context.Background(), "sch-1")
					return err
				},
				wantPath: "/v1/inAppPurchasePriceSchedules/sch-1/relationships/baseTerritory",
			},
			{
				name: "in-app purchase review screenshot",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseAppStoreReviewScreenshotRelationship(context.Background(), "iap-1")
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/appStoreReviewScreenshot",
			},
			{
				name: "in-app purchase content",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseContentRelationship(context.Background(), "iap-1")
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/content",
			},
			{
				name: "in-app purchase price schedule",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseIapPriceScheduleRelationship(context.Background(), "iap-1")
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/iapPriceSchedule",
			},
			{
				name: "in-app purchase availability",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchaseInAppPurchaseAvailabilityRelationship(context.Background(), "iap-1")
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/inAppPurchaseAvailability",
			},
			{
				name: "in-app purchase promoted purchase",
				call: func(c *Client) error {
					_, err := c.GetInAppPurchasePromotedPurchaseRelationship(context.Background(), "iap-1")
					return err
				},
				wantPath: "/v2/inAppPurchases/iap-1/relationships/promotedPurchase",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				response := jsonResponse(http.StatusOK, toOneBody)
				client := newTestClient(t, func(req *http.Request) {
					if req.Method != http.MethodGet {
						t.Fatalf("expected GET, got %s", req.Method)
					}
					if req.URL.Path != tc.wantPath {
						t.Fatalf("expected path %s, got %s", tc.wantPath, req.URL.Path)
					}
					if len(req.URL.Query()) != 0 {
						t.Fatalf("expected no query params, got %q", req.URL.RawQuery)
					}
					assertAuthorized(t, req)
				}, response)

				if err := tc.call(client); err != nil {
					t.Fatalf("request error: %v", err)
				}
			})
		}
	})
}
