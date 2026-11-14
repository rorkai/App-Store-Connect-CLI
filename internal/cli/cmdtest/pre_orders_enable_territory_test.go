package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestPreOrdersEnableNormalizesTerritories(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appAvailabilityV2"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}},"links":{}}`))
	reg.Register(asctest.GET("/v2/appAvailabilities/availability-1/territoryAvailabilities", "fields[territoryAvailabilities]=available,releaseDate,preOrderEnabled,preOrderPublishDate,contentStatuses,territory", "include=territory", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"territoryAvailabilities","id":"ta-us","relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},
				{"type":"territoryAvailabilities","id":"ta-fr","relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}
			],
			"links":{"next":""}
		}`))
	for _, id := range []string{"ta-us", "ta-fr"} {
		reg.Register(asctest.PATCH("/v1/territoryAvailabilities/"+id).JSONBody(`{"data":{"type":"territoryAvailabilities","id":"`+id+`","attributes":{"available":true,"preOrderEnabled":true,"releaseDate":"2026-06-01"}}}`),
			asctest.JSON(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"patched","attributes":{"available":true,"preOrderEnabled":true,"releaseDate":"2026-06-01"}}}`))
	}

	if _, _, err := runCommand(t, []string{
		"pre-orders", "enable",
		"--app", "app-1",
		"--territory", "US,France",
		"--release-date", "2026-06-01",
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
