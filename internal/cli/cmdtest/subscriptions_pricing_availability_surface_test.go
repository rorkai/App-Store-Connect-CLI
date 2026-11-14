package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsPricingAvailabilityEditAcceptsSpacedTrueBoolValue(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/subscriptionAvailabilities").JSONBody(`{"data":{"type":"subscriptionAvailabilities","attributes":{"availableInNewTerritories":true},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"availableTerritories":{"data":[{"type":"territories","id":"USA"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":true}}}`))

	stdout, stderr, err := runCommand(t, []string{"subscriptions", "pricing", "availability", "edit", "--subscription-id", "8000000001", "--available-in-new-territories", "true", "--territories", "USA", "--output", "json"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"avail-1"`) {
		t.Fatalf("expected availability response, got %q", stdout)
	}
}
