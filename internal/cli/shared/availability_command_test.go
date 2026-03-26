package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestMapTerritoryAvailabilityIDs(t *testing.T) {
	relationships := asc.TerritoryAvailabilityRelationships{
		Territory: asc.Relationship{
			Data: asc.ResourceData{
				Type: asc.ResourceTypeTerritories,
				ID:   "usa",
			},
		},
	}
	relationshipsJSON, err := json.Marshal(relationships)
	if err != nil {
		t.Fatalf("failed to marshal relationships: %v", err)
	}

	resp := &asc.TerritoryAvailabilitiesResponse{
		Data: []asc.Resource[asc.TerritoryAvailabilityAttributes]{
			{
				Type:          asc.ResourceTypeTerritoryAvailabilities,
				ID:            "ta-1",
				Relationships: relationshipsJSON,
			},
		},
	}

	ids, err := MapTerritoryAvailabilityIDs(resp)
	if err != nil {
		t.Fatalf("MapTerritoryAvailabilityIDs() error: %v", err)
	}
	if ids["USA"] != "ta-1" {
		t.Fatalf("expected territory USA to map to ta-1, got %q", ids["USA"])
	}
}

func TestMapTerritoryAvailabilityIDs_FallbackID(t *testing.T) {
	payload := `{"s":"6740467361","t":"USA"}`
	encoded := base64.RawStdEncoding.EncodeToString([]byte(payload))

	resp := &asc.TerritoryAvailabilitiesResponse{
		Data: []asc.Resource[asc.TerritoryAvailabilityAttributes]{
			{
				Type: asc.ResourceTypeTerritoryAvailabilities,
				ID:   encoded,
			},
		},
	}

	ids, err := MapTerritoryAvailabilityIDs(resp)
	if err != nil {
		t.Fatalf("MapTerritoryAvailabilityIDs() error: %v", err)
	}
	if ids["USA"] != encoded {
		t.Fatalf("expected territory USA to map to %q, got %q", encoded, ids["USA"])
	}
}

func TestMapTerritoryAvailabilityIDs_NilResponse(t *testing.T) {
	_, err := MapTerritoryAvailabilityIDs(nil)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
}

func TestContextWithAvailabilityTimeout_AllTerritoriesUsesBulkDefault(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	ctx, cancel := contextWithAvailabilityTimeout(context.Background(), true)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on bulk availability context")
	}

	got := time.Until(deadline)
	if got < 4*time.Minute+59*time.Second || got > 5*time.Minute+time.Second {
		t.Fatalf("expected bulk timeout near 5m, got %v", got)
	}
}

func TestContextWithAvailabilityTimeout_AllTerritoriesRespectsASCTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "45s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	ctx, cancel := contextWithAvailabilityTimeout(context.Background(), true)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on bulk availability context")
	}

	got := time.Until(deadline)
	if got < 44*time.Second || got > 46*time.Second {
		t.Fatalf("expected bulk timeout near 45s from ASC_TIMEOUT, got %v", got)
	}
}

func TestContextWithAvailabilityTimeout_SingleTerritoryUsesStandardTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "12s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	ctx, cancel := contextWithAvailabilityTimeout(context.Background(), false)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline on standard availability context")
	}

	got := time.Until(deadline)
	if got < 11*time.Second || got > 13*time.Second {
		t.Fatalf("expected standard timeout near 12s from ASC_TIMEOUT, got %v", got)
	}
}
