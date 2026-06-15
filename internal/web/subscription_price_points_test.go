package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveSubscriptionPricePointMatchesEquivalentDecimal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("filter[territory]") != "NOR" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"type":"subscriptionPricePoints","id":"point-1","attributes":{"customerPrice":"5.00"},"relationships":{"territory":{"data":{"type":"territories","id":"NOR"}}}}]}`))
	}))
	t.Cleanup(server.Close)

	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}
	point, err := client.ResolveSubscriptionPricePoint(context.Background(), "sub-1", "nor", "5")
	if err != nil {
		t.Fatalf("ResolveSubscriptionPricePoint() error = %v", err)
	}
	if point.ID != "point-1" {
		t.Fatalf("point.ID = %q", point.ID)
	}
}
