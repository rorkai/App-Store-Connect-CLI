package web

import (
	"fmt"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestExistingMonthlyAvailabilityOnlyRejectsConfirmedMissingTerritory(t *testing.T) {
	unloaded := webcore.SubscriptionPlanAvailability{
		ID:                         "plan-monthly",
		PlanType:                   "MONTHLY",
		AvailableTerritoriesLoaded: false,
	}
	if availabilityExcludesTerritory(unloaded, "NOR") {
		t.Fatal("unloaded relationship must not be treated as confirmed missing")
	}

	capped := unloaded
	capped.AvailableTerritoriesLoaded = true
	capped.AvailableTerritories = make([]string, 200)
	for i := range capped.AvailableTerritories {
		capped.AvailableTerritories[i] = fmt.Sprintf("T%03d", i)
	}
	if availabilityExcludesTerritory(capped, "NOR") {
		t.Fatal("territory relationship at the response cap must not be treated as complete")
	}

	loaded := unloaded
	loaded.AvailableTerritoriesLoaded = true
	loaded.AvailableTerritories = []string{"DEU"}
	if !availabilityExcludesTerritory(loaded, "NOR") {
		t.Fatal("loaded relationship should confirm the territory is missing")
	}
}
