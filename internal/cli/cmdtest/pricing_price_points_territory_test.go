package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPricingPricePointsNormalizesTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "filter[territory]=USA"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appPricePoints","id":"pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}}],"links":{"next":""}}`))

	if _, _, err := runCommand(t, []string{
		"pricing", "price-points",
		"--app", "app-1",
		"--territory", "United States",
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
