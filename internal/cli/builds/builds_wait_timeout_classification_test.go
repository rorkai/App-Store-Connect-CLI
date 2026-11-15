package builds

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestBuildsWaitTimeoutWithoutReportPendingIsValidationFailure(t *testing.T) {
	timeout := &buildsWaitTimeout{d: &shared.Deps{Now: time.Now}, timeout: time.Minute}
	err := timeout.processingTimeout("build-1")
	if !shared.IsValidationError(err) {
		t.Fatalf("error = %v, want a validation failure", err)
	}
}

func TestVersionBuildSelectorWaitTimeoutKeepsDeadline(t *testing.T) {
	waitTimeout := time.Minute
	selector := &VersionBuildSelector{timeout: &waitTimeout}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	err := selector.waitError(ctx, context.DeadlineExceeded, "build 1 to finish processing")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want wrapped context.DeadlineExceeded", err)
	}
}
