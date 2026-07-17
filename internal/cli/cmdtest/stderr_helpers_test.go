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
		if isDeprecatedCommandWarning(trimmed) {
			continue
		}
		kept = append(kept, trimmed)
	}

	return strings.Join(kept, "\n")
}

func isDeprecatedCommandWarning(line string) bool {
	if strings.HasPrefix(line, "Warning: `--") &&
		strings.Contains(line, " is deprecated. Use `--") &&
		strings.HasSuffix(line, "`.") {
		return true
	}

	const warningPrefix = "Warning: `asc "
	const deprecationMarker = "` is deprecated by App Store Connect API 4.4.1. "
	if !strings.HasPrefix(line, warningPrefix) {
		return false
	}
	_, guidance, found := strings.Cut(line, deprecationMarker)
	if !found {
		return false
	}
	if strings.HasPrefix(guidance, "Use `asc ") && strings.HasSuffix(guidance, "`.") {
		return true
	}

	return strings.HasPrefix(guidance, "No one-command replacement exists. Reconcile each locale through `asc ") &&
		strings.Contains(guidance, "` list/create/update/delete commands with a ") &&
		strings.HasSuffix(guidance, " version ID.")
}

func TestStripDeprecatedCommandWarnings(t *testing.T) {
	t.Parallel()

	unrelatedWarning := "Warning: request is deprecated. No one-command replacement exists."
	unrelatedReplacementWarning := "Warning: request is deprecated. Use a new endpoint."
	stderr := strings.Join([]string{
		"Warning: `--id` is deprecated. Use `--localization-id`.",
		"Warning: `--id` as a build selector is deprecated. Use `--build-id`.",
		"Warning: `asc iap localizations update` is deprecated by App Store Connect API 4.4.1. Use `asc iap versions localizations update`.",
		"Warning: `asc subscriptions localizations sync` is deprecated by App Store Connect API 4.4.1. No one-command replacement exists. Reconcile each locale through `asc subscriptions versions localizations` list/create/update/delete commands with a subscription version ID.",
		unrelatedWarning,
		unrelatedReplacementWarning,
		"Error: request failed",
		"",
	}, "\n")

	want := strings.Join([]string{unrelatedWarning, unrelatedReplacementWarning, "Error: request failed"}, "\n")
	if got := stripDeprecatedCommandWarnings(stderr); got != want {
		t.Fatalf("stripDeprecatedCommandWarnings() = %q, want %q", got, want)
	}
}
