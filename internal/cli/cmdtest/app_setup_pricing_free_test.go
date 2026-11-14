package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppSetupPricingSetFreeResolvesBaseTerritoryAndPaginatedPricePoint(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appPriceSchedule"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appPriceSchedules","id":"sched-current"}}`))
	reg.Register(asctest.GET("/v1/appPriceSchedules/sched-current/baseTerritory"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"territories","id":"CAN"}}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "filter[territory]=CAN", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appPricePoints","id":"pp-paid","attributes":{"customerPrice":"0.99"}}
			],
			"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appPricePoints?cursor=AQ&limit=200&filter%5Bterritory%5D=CAN"}
		}`))
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "cursor=AQ", "filter[territory]=CAN", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appPricePoints","id":"pp-free","attributes":{"customerPrice":"0.00"}}
			],
			"links":{"next":""}
		}`))
	reg.Register(asctest.POST("/v1/appPriceSchedules").JSONBody(`{"data":{"type":"appPriceSchedules","relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"baseTerritory":{"data":{"type":"territories","id":"CAN"}},"manualPrices":{"data":[{"type":"appPrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"appPrices","id":"${local-manual-price-1}","attributes":{"startDate":"2026-03-01"},"relationships":{"appPricePoint":{"data":{"type":"appPricePoints","id":"pp-free"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appPriceSchedules","id":"sched-new","attributes":{}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"app-setup", "pricing", "set",
		"--app", "app-1",
		"--free",
		"--start-date", "2026-03-01",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	pricePointsCalls := 0
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/apps/app-1/appPricePoints" {
			pricePointsCalls++
		}
	}
	if pricePointsCalls != 2 {
		t.Fatalf("expected 2 paginated price-point lookups, got %d", pricePointsCalls)
	}
	if !strings.Contains(stdout, `"id":"sched-new"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}
