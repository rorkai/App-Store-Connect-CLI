package cmdtest

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

const territoryAvailabilitiesFields = "fields[territoryAvailabilities]=available,releaseDate,preOrderEnabled,preOrderPublishDate,contentStatuses,territory"

func registerTerritoryAvailabilitiesRead(reg *asctest.Registry, body string) {
	reg.Register(asctest.GET("/v2/appAvailabilities/availability-1/territoryAvailabilities", territoryAvailabilitiesFields, "include=territory", "limit=200"),
		asctest.JSON(http.StatusOK, body))
}

func TestPricingAvailabilityEditNormalizesTerritories(t *testing.T) {
	setupAuth(t)

	territories := func(usAvailable, frAvailable bool) string {
		return fmt.Sprintf(`{
			"data":[
				{"type":"territoryAvailabilities","id":"ta-us","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},
				{"type":"territoryAvailabilities","id":"ta-fr","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}}
			],
			"links":{"next":""}
		}`, usAvailable, frAvailable)
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appAvailabilityV2"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`))
	registerTerritoryAvailabilitiesRead(reg, territories(false, false))
	for _, id := range []string{"ta-us", "ta-fr"} {
		reg.Register(asctest.PATCH("/v1/territoryAvailabilities/"+id).JSONBody(`{"data":{"type":"territoryAvailabilities","id":"`+id+`","attributes":{"available":true}}}`),
			asctest.JSON(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"patched","attributes":{"available":true}}}`))
	}
	registerTerritoryAvailabilitiesRead(reg, territories(true, true))

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "availability", "edit",
		"--app", "app-1",
		"--territory", "US,France",
		"--available", "true",
		"--available-in-new-territories", "false",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" && !strings.Contains(stderr, "Updating availability for") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected command output")
	}
}

func TestPricingAvailabilityEditSkipsNoOpWithoutNewTerritoriesGuard(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appAvailabilityV2"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`))
	registerTerritoryAvailabilitiesRead(reg, `{
		"data":[
			{"type":"territoryAvailabilities","id":"ta-us","attributes":{"available":true},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}}
		],
		"links":{"next":""}
	}`)

	stdout, stderr, err := runCommand(t, []string{
		"pricing", "availability", "edit",
		"--app", "app-1",
		"--territory", "US",
		"--available", "true",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	for _, req := range reg.Requests() {
		if req.Method == http.MethodPatch {
			t.Fatalf("expected no PATCH requests, got %s %s", req.Method, req.URL)
		}
	}
	if stdout == "" {
		t.Fatal("expected command output")
	}
	if !strings.Contains(stderr, "Updated 0 territories; 1 already matched") {
		t.Fatalf("expected no-op summary, got stderr %q", stderr)
	}
}

func TestPricingAvailabilityEditRejectsMismatchedNewTerritoriesGuardBeforeTerritoryRead(t *testing.T) {
	setupAuth(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appAvailabilityV2"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":true}}}`))

	_, _, err := runCommand(t, []string{
		"pricing", "availability", "edit",
		"--app", "app-1",
		"--territory", "US",
		"--available", "true",
		"--available-in-new-territories", "false",
	})
	if err == nil {
		t.Fatal("expected policy mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match the existing policy (current value: true)") {
		t.Fatalf("expected policy mismatch error, got %v", err)
	}

	if got := len(reg.Requests()); got != 1 {
		t.Fatalf("expected no territory reads or PATCH requests, got %d requests", got)
	}
}

func TestPricingAvailabilityEditContinuesAfterFailureAndVerifiesFinalState(t *testing.T) {
	setupAuth(t)

	territories := func(usa, fra, deu bool) string {
		return fmt.Sprintf(`{"data":[`+
			`{"type":"territoryAvailabilities","id":"ta-usa","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"USA"}}}},`+
			`{"type":"territoryAvailabilities","id":"ta-fra","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"FRA"}}}},`+
			`{"type":"territoryAvailabilities","id":"ta-deu","attributes":{"available":%t},"relationships":{"territory":{"data":{"type":"territories","id":"DEU"}}}}`+
			`],"links":{"next":""}}`, usa, fra, deu)
	}
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-1/appAvailabilityV2"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appAvailabilities","id":"availability-1","attributes":{"availableInNewTerritories":false}}}`))
	registerTerritoryAvailabilitiesRead(reg, territories(false, false, false))
	for _, id := range []string{"ta-usa", "ta-fra", "ta-deu"} {
		resp := asctest.JSON(http.StatusOK, `{"data":{"type":"territoryAvailabilities","id":"patched","attributes":{"available":true}}}`)
		if id == "ta-fra" {
			resp = asctest.JSON(http.StatusUnprocessableEntity, `{"errors":[{"status":"422","code":"ENTITY_ERROR","title":"Invalid"}]}`)
		}
		reg.Register(asctest.PATCH("/v1/territoryAvailabilities/"+id).JSONBody(`{"data":{"type":"territoryAvailabilities","id":"`+id+`","attributes":{"available":true}}}`), resp)
	}
	registerTerritoryAvailabilitiesRead(reg, territories(true, false, true))

	_, _, err := runCommand(t, []string{
		"pricing", "availability", "edit",
		"--app", "app-1",
		"--all-territories",
		"--available", "true",
	})
	if err == nil {
		t.Fatal("expected partial failure")
	}
	if !strings.Contains(err.Error(), "updated 2, skipped 0, failed 1 (FRA)") {
		t.Fatalf("expected verified partial-failure summary, got %v", err)
	}
}
