package asc

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestTerritoryAvailabilitiesRowsIncludesContentStatuses(t *testing.T) {
	body := []byte(`{
		"data": [{
			"type": "territoryAvailabilities",
			"id": "ta-bra",
			"attributes": {
				"available": false,
				"contentStatuses": ["BRAZIL_GAMBLING_NOT_VERIFIED"]
			}
		}]
	}`)

	var resp TerritoryAvailabilitiesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode territory availabilities: %v", err)
	}

	if got := resp.Data[0].Attributes.ContentStatuses; !slices.Contains(got, "BRAZIL_GAMBLING_NOT_VERIFIED") {
		t.Fatalf("expected decoded Brazil gambling status, got %v", got)
	}

	headers, rows := territoryAvailabilitiesRows(&resp)
	if !slices.Contains(headers, "Content Statuses") {
		t.Fatalf("expected Content Statuses header, got %v", headers)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if got := rows[0][4]; got != "BRAZIL_GAMBLING_NOT_VERIFIED" {
		t.Fatalf("expected Brazil gambling status in output, got %q", got)
	}
}
