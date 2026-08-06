package capabilities

import (
	"strings"
	"testing"
)

func TestAssetLibraryIsNotInPublicCapabilities(t *testing.T) {
	for _, capability := range capabilityRows() {
		fields := []string{
			capability.Area,
			capability.Capability,
			strings.Join(capability.Commands, " "),
			strings.Join(capability.APIResources, " "),
			strings.Join(capability.Notes, " "),
			capability.NextAction,
		}
		row := strings.ToLower(strings.Join(fields, " "))
		if strings.Contains(row, "asset library") || strings.Contains(row, "creative asset") {
			t.Fatalf("Asset Library must remain absent from public capabilities, got %+v", capability)
		}
	}
}
