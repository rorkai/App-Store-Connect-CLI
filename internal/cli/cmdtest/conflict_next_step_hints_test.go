package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestConflictNamesReadCommandForCurrentState(t *testing.T) {
	const conflictBody = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"The request cannot be fulfilled because of the state of another resource.","detail":"Apple conflict detail."}]}`

	const reviewItemBody = `{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"sub-1"}},"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`

	tests := []struct {
		args         []string
		lookupTester bool
		request      asctest.Matcher
		wantHint     string
	}{
		{
			args:     []string{"review", "items", "add", "--submission", "sub-1", "--item-type", "appStoreVersions", "--item-id", "version-1"},
			request:  asctest.POST("/v1/reviewSubmissionItems").JSONBody(reviewItemBody),
			wantHint: "Hint: Check the submission's state and items with `asc review submissions-get --id SUBMISSION_ID --include items`. If the item is already on it, `--if-exists skip` accepts it.",
		},
		{
			args:     []string{"review", "items-add", "--submission", "sub-1", "--item-type", "appStoreVersions", "--item-id", "version-1"},
			request:  asctest.POST("/v1/reviewSubmissionItems").JSONBody(reviewItemBody),
			wantHint: "Hint: Check the submission's state and items with `asc review submissions-get --id SUBMISSION_ID --include items`. If the item is already on it, `--if-exists skip` accepts it.",
		},
		{
			args:     []string{"review", "submissions-submit", "--id", "sub-1", "--confirm"},
			request:  asctest.PATCH("/v1/reviewSubmissions/sub-1").JSONBody(`{"data":{"type":"reviewSubmissions","id":"sub-1","attributes":{"submitted":true}}}`),
			wantHint: "Hint: Check the submission's state and items with `asc review submissions-get --id SUBMISSION_ID --include items`, and what blocks review with `asc review doctor --app APP_ID`.",
		},
		{
			args:     []string{"localizations", "update", "--id", "loc-1", "--description", "Description"},
			request:  asctest.PATCH("/v1/appStoreVersionLocalizations/loc-1").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"description":"Description"}}}`),
			wantHint: "Hint: Check the version's state with `asc versions view --version-id VERSION_ID`, or for app-info localizations `asc localizations list --app APP_ID --type app-info --app-info-fields state`.",
		},
		{
			args:     []string{"testflight", "testers", "add", "--email", "tester@example.com", "--build-id", "build-1"},
			request:  asctest.POST("/v1/betaTesters").JSONBody(`{"data":{"type":"betaTesters","attributes":{"email":"tester@example.com"},"relationships":{"builds":{"data":[{"type":"builds","id":"build-1"}]}}}}`),
			wantHint: "Hint: Check the tester's current state with `asc testflight testers list --app APP_ID --email EMAIL`.",
		},
		{
			args:         []string{"testflight", "testers", "invite", "--app", "app-1", "--email", "tester@example.com"},
			lookupTester: true,
			request:      asctest.POST("/v1/betaTesterInvitations").JSONBody(`{"data":{"type":"betaTesterInvitations","relationships":{"app":{"data":{"type":"apps","id":"app-1"}},"betaTester":{"data":{"type":"betaTesters","id":"tester-1"}}}}}`),
			wantHint:     "Hint: Check the tester's current state with `asc testflight testers list --app APP_ID --email EMAIL`.",
		},
		{
			args:     []string{"video-previews", "set-poster-frame", "--id", "preview-1", "--time-code", "00:00:05:00"},
			request:  asctest.PATCH("/v1/appPreviews/preview-1").JSONBody(`{"data":{"type":"appPreviews","id":"preview-1","attributes":{"previewFrameTimeCode":"00:00:05:00"}}}`),
			wantHint: "Hint: Apple accepts a poster frame only after the preview's video finishes processing. Check its `videoDeliveryState` with `asc video-previews list --version-localization VERSION_LOCALIZATION_ID` and retry once it is COMPLETE.",
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.args[:3], " "), func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Install()
			if test.lookupTester {
				reg.Register(asctest.GET("/v1/betaTesters", "filter[apps]=app-1", "filter[email]=tester@example.com"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"betaTesters","id":"tester-1","attributes":{"email":"tester@example.com"}}],"links":{"self":"/v1/betaTesters"}}`))
			}
			reg.Register(test.request, asctest.JSON(http.StatusConflict, conflictBody))

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
