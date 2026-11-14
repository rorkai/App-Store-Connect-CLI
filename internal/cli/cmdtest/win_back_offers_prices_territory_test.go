package cmdtest

import (
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestWinBackOffersPricesNormalizesTerritories(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/winBackOffers/offer-1/prices", "filter[territory]=USA,FRA"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "offers", "win-back", "prices",
		"--id", "offer-1",
		"--territory", "US,France",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output")
	}
}

func TestWinBackOffersPricesNormalizesCommaContainingTerritoryNames(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/winBackOffers/offer-1/prices", "filter[territory]=MDA,BOL"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	stdout, stderr, err := runCommand(t, []string{
		"subscriptions", "offers", "win-back", "prices",
		"--id", "offer-1",
		"--territory", "Moldova, Republic of,Bolivia, Plurinational State of",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output")
	}
}
