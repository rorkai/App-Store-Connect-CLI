package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestConflictNamesReadCommandForCurrentState(t *testing.T) {
	const conflictBody = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Apple conflict detail."}]}`

	tests := []struct {
		args     []string
		wantHint string
	}{
		{
			args:     []string{"review", "items", "add", "--submission", "sub-1", "--item-type", "appStoreVersions", "--item-id", "version-1"},
			wantHint: "Hint: Check the submission's state and items with `asc review submissions-get --id SUBMISSION_ID --include items`. If the item is already on it, `--if-exists skip` accepts it.",
		},
		{
			args:     []string{"review", "items-add", "--submission", "sub-1", "--item-type", "appStoreVersions", "--item-id", "version-1"},
			wantHint: "Hint: Check the submission's state and items with `asc review submissions-get --id SUBMISSION_ID --include items`. If the item is already on it, `--if-exists skip` accepts it.",
		},
		{
			args:     []string{"review", "submissions-submit", "--id", "sub-1", "--confirm"},
			wantHint: "Hint: Check the submission's state and items with `asc review submissions-get --id SUBMISSION_ID --include items`, and what blocks review with `asc review doctor --app APP_ID`.",
		},
		{
			args:     []string{"localizations", "update", "--id", "loc-1", "--description", "Description"},
			wantHint: "Hint: Check the version's state with `asc versions view --version-id VERSION_ID`, or for app-info localizations `asc localizations list --app APP_ID --type app-info --app-info-fields state`.",
		},
		{
			args:     []string{"testflight", "testers", "add", "--email", "tester@example.com", "--build-id", "build-1"},
			wantHint: "Hint: Check the tester's current state with `asc testflight testers list --app APP_ID --email EMAIL`.",
		},
		{
			args:     []string{"testflight", "testers", "invite", "--app", "app-1", "--email", "tester@example.com"},
			wantHint: "Hint: Check the tester's current state with `asc testflight testers list --app APP_ID --email EMAIL`.",
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args[:3], " "), func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method == http.MethodGet && req.URL.Path == "/v1/betaTesters" {
					return jsonResponse(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}],"links":{"self":"/v1/betaTesters"}}`)
				}
				if req.Method == http.MethodGet {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
				}
				return jsonResponse(http.StatusConflict, conflictBody)
			}))

			code, stdout, stderr := runASCForExitCode(t, test.args...)

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "Apple conflict detail.") {
				t.Fatalf("stderr %q dropped Apple's detail", stderr)
			}
			if got := strings.Count(stderr, test.wantHint+"\n"); got != 1 {
				t.Fatalf("stderr has %d copies of %q, want 1:\n%s", got, test.wantHint, stderr)
			}
		})
	}
}
