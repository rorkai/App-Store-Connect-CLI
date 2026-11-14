package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsUpdateSendsReviewNote(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const note = "Same paywall structure, design may differ."

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/subscriptions/sub-1").JSONBody(`{"data":{"type":"subscriptions","id":"sub-1","attributes":{"reviewNote":"Same paywall structure, design may differ."}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"reviewNote":"`+note+`"}}}`))

	stdout, stderr, runErr := runCommand(t, []string{"subscriptions", "update", "--id", "sub-1", "--review-note", note})

	if runErr != nil {
		t.Fatalf("run error: %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"reviewNote":"`+note+`"`) {
		t.Fatalf("expected output to contain review note, got %q", stdout)
	}
}

func TestSubscriptionsUpdateRejectsEmptyReviewNoteExitCode(t *testing.T) {
	assertUsageExit(
		t,
		[]string{"subscriptions", "update", "--id", "sub-1", "--review-note", ""},
		"--review-note cannot be empty",
	)
}
