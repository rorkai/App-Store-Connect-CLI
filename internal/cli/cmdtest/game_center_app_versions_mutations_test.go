package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestGameCenterAppVersionsMutationValidationErrors(t *testing.T) {
	runGameCenterUsageCases(t, []gameCenterUsageCase{
		{[]string{"game-center", "app-versions", "create"}, "--app-store-version-id is required"},
		{[]string{"game-center", "app-versions", "update", "--enabled", "true"}, "--id is required"},
		{[]string{"game-center", "app-versions", "update", "--id", "GC_APP_VERSION_ID"}, "at least one update flag is required (--enabled)"},
	})
}

func TestGameCenterAppVersionsUpdateInvalidEnabled(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"game-center", "app-versions", "update", "--id", "GC_APP_VERSION_ID", "--enabled", "maybe"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --enabled must be 'true' or 'false'") {
		t.Fatalf("expected stderr to contain invalid --enabled error, got %q", stderr)
	}
}
