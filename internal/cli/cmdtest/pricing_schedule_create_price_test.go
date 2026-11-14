package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// registerUSAPricePointScheduleCreate stubs the USA price-point lookup that
// resolves --price 0.99 to pp-099 and the schedule create that uses it.
func registerUSAPricePointScheduleCreate(reg *asctest.Registry) {
	reg.Register(asctest.GET("/v1/apps/app-1/appPricePoints", "filter[territory]=USA", "limit=200"),
		asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"appPricePoints","id":"pp-099","attributes":{"customerPrice":"0.99"}}
			],
			"links":{"next":""}
		}`))
	reg.Register(asctest.POST("/v1/appPriceSchedules").JSONBody(`{"data":{"type":"appPriceSchedules","relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"baseTerritory":{"data":{"type":"territories","id":"USA"}},"manualPrices":{"data":[{"type":"appPrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"appPrices","id":"${local-manual-price-1}","attributes":{"startDate":"2026-03-01"},"relationships":{"appPricePoint":{"data":{"type":"appPricePoints","id":"pp-099"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appPriceSchedules","id":"sched-1","attributes":{}}}`))
}

func TestPricingScheduleCreateResolvesPriceUsingNumericMatch(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerUSAPricePointScheduleCreate(reg)

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "schedule", "create",
		"--app", "app-1",
		"--price", "0.990",
		"--base-territory", "USA",
		"--start-date", "2026-03-01",
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
	if pricePointsCalls != 1 {
		t.Fatalf("expected one price-point lookup call, got %d", pricePointsCalls)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}

func TestPricingScheduleCreateNormalizesBaseTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	registerUSAPricePointScheduleCreate(reg)

	if _, _, err := runCommand(t, []string{
		"pricing", "schedule", "create",
		"--app", "app-1",
		"--price", "0.99",
		"--base-territory", "United States",
		"--start-date", "2026-03-01",
	}); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
