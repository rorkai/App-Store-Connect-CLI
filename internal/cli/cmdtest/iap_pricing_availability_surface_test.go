package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPricingAvailabilitySetAcceptsSpacedBoolValue(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseAvailabilities").JSONBody(`{"data":{"type":"inAppPurchaseAvailabilities","attributes":{"availableInNewTerritories":false},"relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000001"}},"availableTerritories":{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"CAN"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`))

	stdout, stderr, err := runCommand(t, []string{"iap", "pricing", "availability", "set", "--iap-id", "9000000001", "--available-in-new-territories", "false", "--territories", "USA,CAN", "--output", "json"})
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
