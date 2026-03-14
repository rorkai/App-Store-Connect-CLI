package cmdtest

import (
	"strings"
	"testing"
)

func assertOnlyDeprecatedCommandWarnings(t *testing.T, stderr string) {
	t.Helper()

	if got := stripDeprecatedCommandWarnings(stderr); got != "" {
		t.Fatalf("expected empty stderr apart from deprecation warnings, got %q", stderr)
	}
}

func stripDeprecatedCommandWarnings(stderr string) string {
	if strings.TrimSpace(stderr) == "" {
		return ""
	}

	lines := strings.Split(stderr, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "Warning: ") && strings.Contains(trimmed, " is deprecated. Use ") {
			continue
		}
		kept = append(kept, trimmed)
	}

	return strings.Join(kept, "\n")
}
