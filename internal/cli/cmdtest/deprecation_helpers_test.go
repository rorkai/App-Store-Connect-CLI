package cmdtest

import (
	"strings"
	"testing"
)

const (
	feedbackRootDeprecationWarning                         = "Warning: `asc feedback` is deprecated. Use `asc testflight feedback list`."
	crashesRootDeprecationWarning                          = "Warning: `asc crashes` is deprecated. Use `asc testflight crashes list`."
	betaAppLocalizationsListDeprecationWarning             = "Warning: `asc beta-app-localizations list` is deprecated. Use `asc testflight app-localizations list`."
	preReleaseLinksDeprecationWarning                      = "Warning: `asc testflight pre-release relationships view` is deprecated. Use `asc testflight pre-release links view`."
	subscriptionsLocalizationsCreateDeprecationWarning     = "Warning: `asc subscriptions localizations create` is deprecated by App Store Connect API 4.4.1. Use `asc subscriptions versions localizations create --version-id \"SUBSCRIPTION_VERSION_ID\" --name \"NAME\" --locale \"LOCALE\"`."
	subscriptionsLocalizationsSyncDeprecationWarning       = "Warning: `asc subscriptions localizations sync` is deprecated by App Store Connect API 4.4.1. No one-command replacement exists. Reconcile each locale through `asc subscriptions versions localizations` list/create/update/delete commands with a subscription version ID."
	subscriptionsGroupsLocalizationsSyncDeprecationWarning = "Warning: `asc subscriptions groups localizations sync` is deprecated by App Store Connect API 4.4.1. No one-command replacement exists. Reconcile each locale through `asc subscriptions groups versions localizations` list/create/update/delete commands with a subscription group version ID."
)

func requireStderrContainsWarning(t *testing.T, stderr, warning string) {
	t.Helper()
	if !strings.Contains(stderr, warning) {
		t.Fatalf("expected stderr to contain warning %q, got %q", warning, stderr)
	}
}

func assertOnlyCommandDeprecationWarning(t *testing.T, stderr, warning string) {
	t.Helper()
	if got := stripCommandDeprecationWarning(t, stderr, warning); got != "" {
		t.Fatalf("expected only command deprecation warning %q, got additional stderr %q", warning, got)
	}
}

func stripCommandDeprecationWarning(t *testing.T, stderr, warning string) string {
	t.Helper()

	found := 0
	kept := make([]string, 0)
	for _, line := range strings.Split(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if trimmed == warning {
			found++
			continue
		}
		kept = append(kept, trimmed)
	}
	if found != 1 {
		t.Fatalf("expected stderr to contain warning %q exactly once, found %d in %q", warning, found, stderr)
	}
	return strings.Join(kept, "\n")
}

func TestStripCommandDeprecationWarningPreservesAdditionalWarnings(t *testing.T) {
	additional := "Warning: `asc subscriptions localizations sync` is deprecated by App Store Connect API 4.4.1. No one-command replacement exists. Reconcile each locale through `asc subscriptions versions localizations` list/create/update/delete commands with a subscription version ID."
	stderr := subscriptionsLocalizationsCreateDeprecationWarning + "\n" + additional + "\n"

	if got := stripCommandDeprecationWarning(t, stderr, subscriptionsLocalizationsCreateDeprecationWarning); got != additional {
		t.Fatalf("stripCommandDeprecationWarning() = %q, want additional warning %q", got, additional)
	}
}
