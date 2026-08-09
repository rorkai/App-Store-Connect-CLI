package capabilities

import (
	"slices"
	"testing"
)

func TestDeveloperPortalBundleIDCapabilitiesAreDiscoverable(t *testing.T) {
	for _, capability := range capabilityRows() {
		if capability.Capability != "Developer Portal-only Bundle ID capabilities" {
			continue
		}
		if capability.Status != statusWebSession {
			t.Fatalf("status = %q, want %q", capability.Status, statusWebSession)
		}
		if !slices.Contains(capability.Commands, "asc web bundle-ids capabilities enable") {
			t.Fatalf("missing enable command: %+v", capability.Commands)
		}
		if !slices.Contains(capability.APIResources, "PRIVATE_CLOUD_COMPUTE") {
			t.Fatalf("missing PCC resource: %+v", capability.APIResources)
		}
		return
	}
	t.Fatal("Developer Portal-only Bundle ID capability catalog entry not found")
}
