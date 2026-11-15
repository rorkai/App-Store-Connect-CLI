package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestGameCenterAppVersionsMutationValidationErrors(t *testing.T) {
	runGameCenterUsageCasesWithDeps(t, []gameCenterUsageCase{
		{[]string{"game-center", "app-versions", "create"}, "--app-store-version-id is required"},
		{[]string{"game-center", "app-versions", "update", "--enabled", "true"}, "--id is required"},
		{[]string{"game-center", "app-versions", "update", "--id", "GC_APP_VERSION_ID"}, "at least one update flag is required (--enabled)"},
	})
}

func TestGameCenterAppVersionsUpdateInvalidEnabled(t *testing.T) {
	stdout, stderr, err := runRootWithDeps(t, nil, "game-center", "app-versions", "update", "--id", "GC_APP_VERSION_ID", "--enabled", "maybe")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --enabled must be 'true' or 'false'") {
		t.Fatalf("expected stderr to contain invalid --enabled error, got %q", stderr)
	}
}
