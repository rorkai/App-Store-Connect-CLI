package testflight

import (
	"context"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestContextWithBuildGroupMembershipTimeoutUsesBulkDefault(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	ctx, cancel := contextWithBuildGroupMembershipTimeout(context.Background(), &shared.Deps{ResolveTimeout: shared.ResolveTimeoutSetting})
	defer cancel()
	deadline, ok := ctx.Deadline()
	remaining := time.Until(deadline)
	if !ok || remaining < 299*time.Second || remaining > 301*time.Second {
		t.Fatalf("expected five-minute bulk timeout, remaining=%v, present=%t", remaining, ok)
	}
}

func TestContextWithBuildGroupMembershipTimeoutHonorsASCOverride(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "90s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")

	ctx, cancel := contextWithBuildGroupMembershipTimeout(context.Background(), &shared.Deps{ResolveTimeout: shared.ResolveTimeoutSetting})
	defer cancel()
	deadline, ok := ctx.Deadline()
	remaining := time.Until(deadline)
	if !ok || remaining < 89*time.Second || remaining > 91*time.Second {
		t.Fatalf("expected ASC_TIMEOUT override near 90s, remaining=%v, present=%t", remaining, ok)
	}
}
