package subscriptions

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestFetchSubscriptionPriceDeriveCandidatesBatchesTerritoriesAndPaginates(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	calls := 0
	http.DefaultTransport = resolvedPricesRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			if req.URL.Path != "/v1/subscriptions/target-sub/pricePoints" {
				t.Fatalf("first path = %q", req.URL.Path)
			}
			query := req.URL.Query()
			if query.Get("filter[territory]") != "SWE,USA" || query.Get("filter[planType]") != "UPFRONT" {
				t.Fatalf("unexpected first-page filters: %s", req.URL.RawQuery)
			}
			if query.Get("fields[subscriptionPricePoints]") != "customerPrice,territory" || query.Get("include") != "territory" || query.Get("limit") != "8000" {
				t.Fatalf("unexpected first-page sparse fields/limit: %s", req.URL.RawQuery)
			}
			return resolvedPricesJSONResponse(`{"data":[{"type":"subscriptionPricePoints","id":"pp-899","attributes":{"customerPrice":"8.99"},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}],"links":{"next":"/v1/subscriptions/target-sub/pricePoints?cursor=next"}}`), nil
		case 2:
			if req.URL.Path != "/v1/subscriptions/target-sub/pricePoints" || req.URL.Query().Get("cursor") != "next" {
				t.Fatalf("unexpected continuation request: %s", req.URL.String())
			}
			return resolvedPricesJSONResponse(`{"data":[{"type":"subscriptionPricePoints","id":"pp-89","attributes":{"customerPrice":"89"},"relationships":{"territory":{"data":{"type":"territories","id":"SWE"}}}}],"links":{}}`), nil
		default:
			t.Fatalf("unexpected extra request: %s", req.URL.String())
			return nil, nil
		}
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}

	candidates, err := fetchSubscriptionPriceDeriveCandidatesByTerritory(context.Background(), shared.DefaultDeps(), client, "target-sub", []string{"USA", "SWE", "USA"})
	if err != nil {
		t.Fatalf("fetchSubscriptionPriceDeriveCandidatesByTerritory() error = %v", err)
	}
	if calls != 2 || len(candidates) != 2 || len(candidates["SWE"]) != 1 || candidates["SWE"][0].PricePointID != "pp-89" || len(candidates["USA"]) != 1 || candidates["USA"][0].PricePointID != "pp-899" {
		t.Fatalf("unexpected paginated candidates: calls=%d candidates=%+v", calls, candidates)
	}
}

func TestFetchSubscriptionPriceDeriveCandidatesRequiresTerritoryRelationship(t *testing.T) {
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = resolvedPricesRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return resolvedPricesJSONResponse(`{"data":[{"type":"subscriptionPricePoints","id":"pp-89","attributes":{"customerPrice":"89"}}],"links":{}}`), nil
	})
	client, err := asc.NewClientFromPEM("KEY123", "issuer", introImportTestPrivateKeyPEM(t))
	if err != nil {
		t.Fatalf("NewClientFromPEM() error: %v", err)
	}

	_, err = fetchSubscriptionPriceDeriveCandidatesByTerritory(context.Background(), shared.DefaultDeps(), client, "target-sub", []string{"SWE"})
	if err == nil || !strings.Contains(err.Error(), `target price point "pp-89" is missing its territory relationship`) {
		t.Fatalf("expected missing territory relationship error, got %v", err)
	}
}
