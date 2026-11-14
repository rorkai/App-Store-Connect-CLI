package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPAvailabilitySetNormalizesTerritories(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchaseAvailabilities").JSONBody(`{"data":{"type":"inAppPurchaseAvailabilities","attributes":{"availableInNewTerritories":false},"relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"8000000001"}},"availableTerritories":{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchaseAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`))

	if _, _, err := runCommand(t, []string{
		"iap", "pricing", "availability", "set",
		"--iap-id", "8000000001",
		"--territories", "US,France",
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
