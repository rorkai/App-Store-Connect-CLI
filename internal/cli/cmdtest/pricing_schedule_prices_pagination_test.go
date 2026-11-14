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

func TestPricingScheduleManualPricesLimit(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appPriceSchedules/schedule-1/manualPrices", "limit=175"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appPrices","id":"manual-price-175"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "schedule", "manual-prices",
		"--schedule", "schedule-1",
		"--limit", "175",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"manual-price-175"`) {
		t.Fatalf("expected manual price output, got %q", stdout)
	}
}

func TestPricingScheduleManualPricesPaginate(t *testing.T) {
	setupAuth(t)

	const secondURL = "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/manualPrices?cursor=Mg"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appPriceSchedules/schedule-1/manualPrices", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appPrices","id":"manual-price-1"}],"links":{"next":"`+secondURL+`"}}`))
	reg.Register(asctest.GET("/v1/appPriceSchedules/schedule-1/manualPrices", "cursor=Mg"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appPrices","id":"manual-price-2"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "schedule", "manual-prices",
		"--schedule", "schedule-1",
		"--paginate",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"manual-price-1"`) || !strings.Contains(stdout, `"id":"manual-price-2"`) {
		t.Fatalf("expected paginated manual price output, got %q", stdout)
	}
}

func TestPricingScheduleAutomaticPricesNext(t *testing.T) {
	setupAuth(t)

	const nextURL = "https://api.appstoreconnect.apple.com/v1/appPriceSchedules/schedule-1/automaticPrices?cursor=Mg"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appPriceSchedules/schedule-1/automaticPrices", "cursor=Mg"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appPrices","id":"automatic-price-next"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "schedule", "automatic-prices",
		"--next", nextURL,
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"automatic-price-next"`) {
		t.Fatalf("expected automatic price output, got %q", stdout)
	}
}

func TestPricingScheduleManualPricesRejectsInvalidLimit(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"pricing", "schedule", "manual-prices",
			"--schedule", "schedule-1",
			"--limit", "201",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: pricing schedule manual-prices: --limit must be between 1 and 200") {
		t.Fatalf("expected invalid limit usage error, got %q", stderr)
	}
}
