package reviews

import (
	"context"
	"errors"
	"flag"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestReviewSubmissionsListCommand_InvalidState(t *testing.T) {
	d, _, _ := newReviewTestDeps(nil)
	cmd := newReviewSubmissionsListCommand(d)
	t.Setenv("ASC_APP_ID", "test-app")

	err := cmd.ParseAndRun(context.Background(), []string{"--state", "NOT_A_REAL_STATE"})
	if err == nil {
		t.Fatal("expected usage error for invalid --state, got nil")
	}
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp for invalid --state, got: %v", err)
	}
	diagnostic, ok := shared.DiagnosticFromError(err)
	if !ok {
		t.Fatal("expected structured diagnostic for invalid --state")
	}
	if diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != "--state" {
		t.Fatalf("diagnostic = %+v, want code %q parameter %q", diagnostic, shared.DiagnosticInvalidInput, "--state")
	}
}
