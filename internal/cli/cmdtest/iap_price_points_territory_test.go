package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPricePointsListNormalizesTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/pricePoints", "filter[territory]=FRA"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchasePricePoints","id":"pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}}],"links":{"next":""}}`))

	if _, _, err := runCommand(t, []string{
		"iap", "pricing", "price-points", "list",
		"--iap-id", "9000000001",
		"--territory", "France",
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
