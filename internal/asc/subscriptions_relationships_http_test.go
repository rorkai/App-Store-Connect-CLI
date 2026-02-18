package asc

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestSubscriptionRelationshipEndpoints(t *testing.T) {
	t.Run("to-many relationships use LinkagesResponse", func(t *testing.T) {
		linkagesBody := `{"data":[],"links":{}}`

		cases := []struct {
			name     string
			call     func(*Client) error
			wantPath string
		}{
			{
				name: "subscription availability available territories",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionAvailabilityAvailableTerritoriesRelationships(context.Background(), "avail-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionAvailabilities/avail-1/relationships/availableTerritories",
			},
			{
				name: "subscription group localizations",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionGroupSubscriptionGroupLocalizationsRelationships(context.Background(), "group-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionGroups/group-1/relationships/subscriptionGroupLocalizations",
			},
			{
				name: "subscription group subscriptions",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionGroupSubscriptionsRelationships(context.Background(), "group-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionGroups/group-1/relationships/subscriptions",
			},
			{
				name: "subscription offer code custom codes",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionOfferCodeCustomCodesRelationships(context.Background(), "oc-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionOfferCodes/oc-1/relationships/customCodes",
			},
			{
				name: "subscription offer code one-time use codes",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionOfferCodeOneTimeUseCodesRelationships(context.Background(), "oc-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionOfferCodes/oc-1/relationships/oneTimeUseCodes",
			},
			{
				name: "subscription offer code prices",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionOfferCodePricesRelationships(context.Background(), "oc-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionOfferCodes/oc-1/relationships/prices",
			},
			{
				name: "subscription price point equalizations",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPricePointEqualizationsRelationships(context.Background(), "pp-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionPricePoints/pp-1/relationships/equalizations",
			},
			{
				name: "subscription promotional offer prices",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPromotionalOfferPricesRelationships(context.Background(), "promo-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptionPromotionalOffers/promo-1/relationships/prices",
			},
			{
				name: "subscription images",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionImagesRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/images",
			},
			{
				name: "subscription introductory offers",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionIntroductoryOffersRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/introductoryOffers",
			},
			{
				name: "subscription offer codes",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionOfferCodesRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/offerCodes",
			},
			{
				name: "subscription price points",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPricePointsRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/pricePoints",
			},
			{
				name: "subscription prices",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPricesRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/prices",
			},
			{
				name: "subscription promotional offers",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPromotionalOffersRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/promotionalOffers",
			},
			{
				name: "subscription localizations",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionSubscriptionLocalizationsRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/subscriptionLocalizations",
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
					if req.URL.Query().Get("limit") != "5" {
						t.Fatalf("expected limit=5, got %q", req.URL.Query().Get("limit"))
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
				name: "subscription review screenshot",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionAppStoreReviewScreenshotRelationship(context.Background(), "sub-1")
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/appStoreReviewScreenshot",
			},
			{
				name: "subscription promoted purchase",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPromotedPurchaseRelationship(context.Background(), "sub-1")
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/promotedPurchase",
			},
			{
				name: "subscription availability",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionSubscriptionAvailabilityRelationship(context.Background(), "sub-1")
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/subscriptionAvailability",
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

	t.Run("delete relationships send RelationshipRequest payloads", func(t *testing.T) {
		response := jsonResponse(http.StatusNoContent, `{}`)

		t.Run("introductory offers", func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodDelete {
					t.Fatalf("expected DELETE, got %s", req.Method)
				}
				if req.URL.Path != "/v1/subscriptions/sub-1/relationships/introductoryOffers" {
					t.Fatalf("expected path /v1/subscriptions/sub-1/relationships/introductoryOffers, got %s", req.URL.Path)
				}
				var payload RelationshipRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}
				if len(payload.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(payload.Data))
				}
				if payload.Data[0].Type != ResourceTypeSubscriptionIntroductoryOffers || payload.Data[0].ID != "offer-1" {
					t.Fatalf("unexpected first relationship: %+v", payload.Data[0])
				}
				if payload.Data[1].Type != ResourceTypeSubscriptionIntroductoryOffers || payload.Data[1].ID != "offer-2" {
					t.Fatalf("unexpected second relationship: %+v", payload.Data[1])
				}
				assertAuthorized(t, req)
			}, response)

			if err := client.RemoveSubscriptionIntroductoryOffers(context.Background(), "sub-1", []string{"offer-1", "offer-2"}); err != nil {
				t.Fatalf("RemoveSubscriptionIntroductoryOffers() error: %v", err)
			}
		})

		t.Run("prices", func(t *testing.T) {
			client := newTestClient(t, func(req *http.Request) {
				if req.Method != http.MethodDelete {
					t.Fatalf("expected DELETE, got %s", req.Method)
				}
				if req.URL.Path != "/v1/subscriptions/sub-1/relationships/prices" {
					t.Fatalf("expected path /v1/subscriptions/sub-1/relationships/prices, got %s", req.URL.Path)
				}
				var payload RelationshipRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("failed to decode request: %v", err)
				}
				if len(payload.Data) != 2 {
					t.Fatalf("expected 2 relationship items, got %d", len(payload.Data))
				}
				if payload.Data[0].Type != ResourceTypeSubscriptionPrices || payload.Data[0].ID != "price-1" {
					t.Fatalf("unexpected first relationship: %+v", payload.Data[0])
				}
				if payload.Data[1].Type != ResourceTypeSubscriptionPrices || payload.Data[1].ID != "price-2" {
					t.Fatalf("unexpected second relationship: %+v", payload.Data[1])
				}
				assertAuthorized(t, req)
			}, response)

			if err := client.RemoveSubscriptionPrices(context.Background(), "sub-1", []string{"price-1", "price-2"}); err != nil {
				t.Fatalf("RemoveSubscriptionPrices() error: %v", err)
			}
		})
	})
}
