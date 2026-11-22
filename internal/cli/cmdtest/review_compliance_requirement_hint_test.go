package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// reviewComplianceRequirementBody is the shape of Apple's 409 when the app
// still owes a compliance answer: the version is not reviewable and the
// associated errors on its appInfos repeat the reason once per open
// requirement (IDs replaced).
const reviewComplianceRequirementBody = `{"errors":[{"status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"appStoreVersions with id 'version-1' is not in valid state.","detail":"This resource cannot be reviewed, please check associated errors to see why.","meta":{"associatedErrors":{"/v1/appInfos/info-1":[{"status":"409","code":"STATE_ERROR","detail":"Cannot submit version due to unfulfilled compliance requirement."},{"status":"409","code":"STATE_ERROR","detail":"Cannot submit version due to unfulfilled compliance requirement."}]}}}]}`

const reviewComplianceRequirementHint = "Hint: App Store Connect does not review the app until every compliance requirement has an answer. List the App Compliance requirements, such as the China mainland Security Assessment and AI Generated or Synthetic Content declarations, with `asc web apps compliance list --app APP_ID` and answer them with `asc web apps compliance set`; list the App Store Regulations & Permits declarations with `asc web apps declarations list --app APP_ID`.\n"

func TestReviewComplianceRequirementConflictNamesTheRequirementReads(t *testing.T) {
	tests := []struct {
		name     string
		register func(*asctest.Registry)
		args     []string
	}{
		{
			name: "review items update --resolved",
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.PATCH("/v1/reviewSubmissionItems/item-1"), asctest.JSON(http.StatusConflict, reviewComplianceRequirementBody))
			},
			args: []string{"review", "items", "update", "--id", "item-1", "--resolved", "true"},
		},
		{
			name: "review submissions-submit",
			register: func(reg *asctest.Registry) {
				reg.Register(asctest.PATCH("/v1/reviewSubmissions/sub-1"), asctest.JSON(http.StatusConflict, reviewComplianceRequirementBody))
			},
			args: []string{"review", "submissions-submit", "--id", "sub-1", "--confirm"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			test.register(reg)

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), test.args...)

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "Cannot submit version due to unfulfilled compliance requirement.") {
				t.Fatalf("stderr %q dropped Apple's associated error", stderr)
			}
			if !strings.HasSuffix(stderr, reviewComplianceRequirementHint) || strings.Count(stderr, "Hint:") != 1 {
				t.Fatalf("stderr = %q, want it to end with the one hint %q", stderr, reviewComplianceRequirementHint)
			}
		})
	}
}
