package reviews

import (
	"context"
	"errors"
	"flag"
	"testing"
)

func TestReviewSubmissionsListCommand_InvalidState(t *testing.T) {
	cmd := ReviewSubmissionsListCommand()
	t.Setenv("ASC_APP_ID", "test-app")

	err := cmd.ParseAndRun(context.Background(), []string{"--state", "NOT_A_REAL_STATE"})
	if err == nil {
		t.Fatal("expected usage error for invalid --state, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid --state, got: %v", err)
	}
}
