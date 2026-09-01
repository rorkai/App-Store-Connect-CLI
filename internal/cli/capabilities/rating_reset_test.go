package capabilities

import (
	"slices"
	"strings"
	"testing"
)

func TestRatingResetCapabilityDocumentsExperimentalWebSessionSurface(t *testing.T) {
	for _, capability := range capabilityRows() {
		if capability.Capability != "App overview-rating reset" {
			continue
		}
		if capability.Status != statusPartial {
			t.Fatalf("status = %q, want %q", capability.Status, statusPartial)
		}
		if !slices.Contains(capability.Commands, "asc versions rating-reset") {
			t.Fatalf("commands = %v, want rating-reset", capability.Commands)
		}
		if !slices.Contains(capability.APIResources, "resetRatingsRequests") {
			t.Fatalf("resources = %v, want resetRatingsRequests", capability.APIResources)
		}
		if len(capability.Notes) == 0 || !strings.Contains(capability.Notes[0], "published OpenAPI specification") {
			t.Fatalf("notes = %v, want undocumented endpoint warning", capability.Notes)
		}
		return
	}
	t.Fatal("App overview-rating reset capability entry not found")
}
