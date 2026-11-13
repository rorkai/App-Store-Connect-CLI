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
				name: "subscription prices",
				call: func(c *Client) error {
					_, err := c.GetSubscriptionPricesRelationships(context.Background(), "sub-1", WithLinkagesLimit(5))
					return err
				},
				wantPath: "/v1/subscriptions/sub-1/relationships/prices",
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
