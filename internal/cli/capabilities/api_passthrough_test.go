package capabilities

import (
	"slices"
	"testing"
)

func TestRawAPIRequestCapabilityIsCLISupported(t *testing.T) {
	for _, capability := range capabilityRows() {
		if capability.Capability != "Raw authenticated API requests" {
			continue
		}
		if capability.Status != statusCLISupported {
			t.Fatalf("status = %q, want %q", capability.Status, statusCLISupported)
		}
		if !slices.Contains(capability.Commands, "asc api") {
			t.Fatalf("commands = %v, want asc api", capability.Commands)
		}
		return
	}
	t.Fatal("Raw authenticated API requests capability entry not found")
}
