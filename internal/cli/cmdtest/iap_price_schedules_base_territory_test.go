package cmdtest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPriceSchedulesCreateSendsNormalizedBaseTerritory(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/inAppPurchasePriceSchedules").JSONBody(`{"data":{"type":"inAppPurchasePriceSchedules","relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000003"}},"baseTerritory":{"data":{"type":"territories","id":"USA"}},"manualPrices":{"data":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}","attributes":{"startDate":"2026-03-01"},"relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"9000000003"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"PP1"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "create",
		"--iap-id", "9000000003",
		"--base-territory", "US",
		"--prices", "PP1:2026-03-01",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"sched-1"`) {
		t.Fatalf("expected schedule output, got %q", stdout)
	}
}

func TestIAPPriceSchedulesCreateResolvesTiersWithNormalizedBaseTerritory(t *testing.T) {
	setupAuth(t)
	t.Setenv("HOME", t.TempDir())

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000003/pricePoints", "filter[territory]=USA", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"inAppPurchasePricePoints","id":"iap-pp-1","attributes":{"customerPrice":"0.99","proceeds":"0.70"}}],"links":{"next":""}}`))
	reg.Register(asctest.POST("/v1/inAppPurchasePriceSchedules").JSONBody(`{"data":{"type":"inAppPurchasePriceSchedules","relationships":{"inAppPurchase":{"data":{"type":"inAppPurchases","id":"9000000003"}},"baseTerritory":{"data":{"type":"territories","id":"USA"}},"manualPrices":{"data":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}"}]}}},"included":[{"type":"inAppPurchasePrices","id":"${local-manual-price-1}","attributes":{"startDate":"2026-03-01"},"relationships":{"inAppPurchaseV2":{"data":{"type":"inAppPurchases","id":"9000000003"}},"inAppPurchasePricePoint":{"data":{"type":"inAppPurchasePricePoints","id":"iap-pp-1"}}}}]}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"inAppPurchasePriceSchedules","id":"sched-1","attributes":{}}}`))

	_, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "create",
		"--iap-id", "9000000003",
		"--base-territory", "United States",
		"--tier", "1",
		"--start-date", "2026-03-01",
		"--refresh",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
