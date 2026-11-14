package asc

import (
	"context"
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
					_, err := c.GetSubscriptionPricesRelationships(context.Background(), "sub-1", Limit[LinkagesOption](5))
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
}
