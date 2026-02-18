package asc

import (
	"context"
	"net/http"
	"testing"
)

func TestPricingRelationshipEndpoints(t *testing.T) {
	t.Run("to-many relationships use LinkagesResponse", func(t *testing.T) {
		linkagesBody := `{"data":[],"links":{}}`

		cases := []struct {
			name     string
			call     func(*Client) error
			wantPath string
		}{
			{
				name: "app price schedule automatic prices",
				call: func(c *Client) error {
					_, err := c.GetAppPriceScheduleAutomaticPricesRelationships(context.Background(), "sch-1", WithLinkagesLimit(3))
					return err
				},
				wantPath: "/v1/appPriceSchedules/sch-1/relationships/automaticPrices",
			},
			{
				name: "app price schedule manual prices",
				call: func(c *Client) error {
					_, err := c.GetAppPriceScheduleManualPricesRelationships(context.Background(), "sch-1", WithLinkagesLimit(3))
					return err
				},
				wantPath: "/v1/appPriceSchedules/sch-1/relationships/manualPrices",
			},
			{
				name: "app price point equalizations",
				call: func(c *Client) error {
					_, err := c.GetAppPricePointEqualizationsRelationships(context.Background(), "pp-1", WithLinkagesLimit(3))
					return err
				},
				wantPath: "/v3/appPricePoints/pp-1/relationships/equalizations",
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
					if req.URL.Query().Get("limit") != "3" {
						t.Fatalf("expected limit=3, got %q", req.URL.Query().Get("limit"))
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
		toOneBody := `{"data":{"type":"territories","id":"USA"},"links":{}}`

		response := jsonResponse(http.StatusOK, toOneBody)
		client := newTestClient(t, func(req *http.Request) {
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/appPriceSchedules/sch-1/relationships/baseTerritory" {
				t.Fatalf("expected path /v1/appPriceSchedules/sch-1/relationships/baseTerritory, got %s", req.URL.Path)
			}
			if len(req.URL.Query()) != 0 {
				t.Fatalf("expected no query params, got %q", req.URL.RawQuery)
			}
			assertAuthorized(t, req)
		}, response)

		if _, err := client.GetAppPriceScheduleBaseTerritoryRelationship(context.Background(), "sch-1"); err != nil {
			t.Fatalf("GetAppPriceScheduleBaseTerritoryRelationship() error: %v", err)
		}
	})
}
