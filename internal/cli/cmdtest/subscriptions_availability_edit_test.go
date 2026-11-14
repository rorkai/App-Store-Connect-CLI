package cmdtest

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsAvailabilityEditNormalizesTerritories(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.POST("/v1/subscriptionAvailabilities").JSONBody(`{"data":{"type":"subscriptionAvailabilities","attributes":{"availableInNewTerritories":false},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"8000000001"}},"availableTerritories":{"data":[{"type":"territories","id":"USA"},{"type":"territories","id":"FRA"}]}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"subscriptionAvailabilities","id":"avail-1","attributes":{"availableInNewTerritories":false}}}`))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	if err := root.Parse([]string{
		"subscriptions", "pricing", "availability", "edit",
		"--subscription-id", "8000000001",
		"--territories", "US,France",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}
