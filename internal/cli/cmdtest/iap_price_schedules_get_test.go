package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestIAPPriceSchedulesGetRejectsInvalidInclude(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"iap", "pricing", "schedules", "view",
			"--schedule-id", "schedule-1",
			"--include", "unknown",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--include must be one of: baseTerritory, manualPrices, automaticPrices") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestIAPPriceSchedulesGetByIDWithIncludeOptions(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/inAppPurchasePriceSchedules/schedule-1", "fields[inAppPurchasePriceSchedules]=baseTerritory,manualPrices,automaticPrices", "fields[inAppPurchasePrices]=startDate,endDate,manual,inAppPurchasePricePoint,territory", "fields[territories]=currency", "include=baseTerritory,manualPrices,automaticPrices", "limit[automaticPrices]=25", "limit[manualPrices]=25"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"inAppPurchasePriceSchedules","id":"schedule-1"}}`))

	stdout, stderr, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "view",
		"--schedule-id", "schedule-1",
		"--include", "baseTerritory,manualPrices,automaticPrices",
		"--schedule-fields", "baseTerritory,manualPrices,automaticPrices",
		"--territory-fields", "currency",
		"--price-fields", "startDate,endDate,manual,inAppPurchasePricePoint,territory",
		"--manual-prices-limit", "25",
		"--automatic-prices-limit", "25",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"schedule-1"`) {
		t.Fatalf("expected schedule id in output, got %q", stdout)
	}
}

func TestIAPPriceSchedulesGetUsesCanonicalErrorPrefix(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v2/inAppPurchases/9000000001/iapPriceSchedule"),
		asctest.JSON(http.StatusInternalServerError, `{"errors":[{"status":"500","code":"INTERNAL_ERROR","title":"boom"}]}`))

	_, _, err := runCommand(t, []string{
		"iap", "pricing", "schedules", "view",
		"--iap-id", "9000000001",
	})
	if err == nil {
		t.Fatal("expected fetch error")
	}
	if !strings.Contains(err.Error(), "iap pricing schedules view: failed to fetch:") {
		t.Fatalf("expected canonical error prefix, got %q", err.Error())
	}
	if strings.Contains(err.Error(), "iap price-schedules get:") {
		t.Fatalf("expected legacy error prefix to be removed, got %q", err.Error())
	}
}
