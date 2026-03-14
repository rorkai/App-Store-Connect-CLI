package cmdtest

import (
	"strings"
	"testing"
)

const (
	feedbackRootDeprecationWarning             = "Warning: `asc feedback` is deprecated. Use `asc testflight feedback list`."
	crashesRootDeprecationWarning              = "Warning: `asc crashes` is deprecated. Use `asc testflight crashes list`."
	betaAppLocalizationsListDeprecationWarning = "Warning: `asc beta-app-localizations list` is deprecated. Use `asc testflight app-localizations list`."
	preReleaseLinksDeprecationWarning          = "Warning: `asc testflight pre-release relationships view` is deprecated. Use `asc testflight pre-release links view`."
)

func requireStderrContainsWarning(t *testing.T, stderr, warning string) {
	t.Helper()
	if !strings.Contains(stderr, warning) {
		t.Fatalf("expected stderr to contain warning %q, got %q", warning, stderr)
	}
}
