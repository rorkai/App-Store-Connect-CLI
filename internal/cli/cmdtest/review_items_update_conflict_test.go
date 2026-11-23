package cmdtest

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Apple's 409 for --resolved true on an item App Review did not reject,
// captured live on 2026-11-22 against a READY_FOR_REVIEW draft item.
const reviewItemResolveNotRejectedBody = `{"errors":[{"id":"3719fbb4-6ff5-4a00-a112-fd96f802878b","status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"Resource state is invalid.","detail":"Item can only be resolved when it is in REJECTED state."},{"id":"ca56ef79-f273-4356-974c-ab7ba03206a8","status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"Resource state is invalid.","detail":"Cannot resolve because of state of reviewSubmission."}]}`

func TestReviewItemsUpdateResolveConflictNamesWhichItemsTakeIt(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	reg := asctest.NewRegistry(t)
	reg.Register(asctest.PATCH("/v1/reviewSubmissionItems/item-1").JSONBody(`{"data":{"type":"reviewSubmissionItems","id":"item-1","attributes":{"resolved":true}}}`),
		asctest.JSON(http.StatusConflict, reviewItemResolveNotRejectedBody))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "review", "items", "update", "--id", "item-1", "--resolved", "true")

	if code != cmd.ExitConflict {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: review items update: Resource state is invalid.: Item can only be resolved when it is in REJECTED state.\n\n" +
		"Additional errors:\n  - Cannot resolve because of state of reviewSubmission.\n" +
		"Hint: Only items App Review rejected (state REJECTED) take --resolved. See each item's state with `asc review items list --submission SUBMISSION_ID`. When none is REJECTED there is nothing to resolve: edit the version if the fix needs it, then resubmit with `asc review submissions-submit --id SUBMISSION_ID --confirm`.\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
