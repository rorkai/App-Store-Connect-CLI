package asc

import (
	"testing"
)

func TestOutputRegistryNotEmpty(t *testing.T) {
	if len(outputRegistry) == 0 {
		t.Fatal("output registry is empty; init() may not have run")
	}
}

func TestOutputRegistryAllHandlersNonNil(t *testing.T) {
	for typ, fn := range outputRegistry {
		if fn == nil {
			t.Errorf("nil handler registered for type %s", typ)
		}
	}
}

func TestOutputRegistryExpectedTypeCount(t *testing.T) {
	// Total registered types across both registries should be ~471.
	total := len(outputRegistry) + len(directRenderRegistry)
	const minExpected = 460
	if total < minExpected {
		t.Errorf("expected at least %d registered types, got %d (rows: %d, direct: %d)",
			minExpected, total, len(outputRegistry), len(directRenderRegistry))
	}
}

func TestDirectRenderRegistryAllHandlersNonNil(t *testing.T) {
	for typ, fn := range directRenderRegistry {
		if fn == nil {
			t.Errorf("nil handler registered for type %s", typ)
		}
	}
}

func TestRenderByRegistryFallbackToJSON(t *testing.T) {
	// Unregistered type should fall back to JSON without error.
	type unregistered struct {
		Value string `json:"value"`
	}
	output := captureStdout(t, func() error {
		return renderByRegistry(&unregistered{Value: "test"}, RenderTable)
	})
	if output == "" {
		t.Fatal("expected JSON fallback output")
	}
	if !contains(output, "test") {
		t.Fatalf("expected JSON output to contain 'test', got: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
