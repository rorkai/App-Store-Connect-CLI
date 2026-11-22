package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// Apple's 409 on PATCH /v1/reviewSubmissions/{id} with submitted=true for an
// app that has no approved version and a submission without one, captured
// live on 2026-11-22 (app ID replaced).
const reviewSubmissionNeedsVersionBody = `{"errors":[{"id":"55fd262d-edd0-42fe-8986-c7e82a3b1818","status":"409","code":"ENTITY_ERROR.RELATIONSHIP.REQUIRED","title":"Relationship is required","detail":"App app-1 must have an approved appStoreVersions for at least one of platforms [IOS], or an appStoreVersions must be included in this review submission.","source":{"pointer":"/data/relationships/appStoreVersionForReview"}}]}`

func TestReviewSubmissionsSubmitNeedsVersionHintNamesTheVersionToAdd(t *testing.T) {
	tests := []struct {
		name     string
		versions string
		wantHint string
	}{
		{
			name:     "editable version exists",
			versions: `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"PREPARE_FOR_SUBMISSION"}}]}`,
			wantHint: "Hint: App app-1 has no approved IOS version yet, so App Store Connect reviews this submission only together with an App Store version. Add version 1.0 (PREPARE_FOR_SUBMISSION) with `asc review items add --submission sub-1 --item-type appStoreVersions --item-id version-1`, then submit again.\n",
		},
		{
			name:     "no editable version",
			versions: `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"platform":"IOS","versionString":"1.0","appVersionState":"WAITING_FOR_REVIEW"}}]}`,
			wantHint: "Hint: App app-1 has no approved IOS version yet, so App Store Connect reviews this submission only together with an App Store version. Create one with `asc versions create --app app-1 --version VERSION --platform IOS`, add it with `asc review items add --submission sub-1 --item-type appStoreVersions --item-id VERSION_ID`, then submit again.\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.PATCH("/v1/reviewSubmissions/sub-1"), asctest.JSON(http.StatusConflict, reviewSubmissionNeedsVersionBody))
			reg.Register(asctest.GET("/v1/reviewSubmissions/sub-1", "include=app"),
				asctest.JSON(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"sub-1","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`))
			reg.Register(asctest.GET("/v1/apps/app-1/appStoreVersions", "filter[platform]=IOS", "limit=200"), asctest.JSON(http.StatusOK, test.versions))

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "review", "submissions-submit", "--id", "sub-1", "--confirm")

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "must have an approved appStoreVersions") {
				t.Fatalf("stderr %q dropped Apple's detail", stderr)
			}
			if !strings.HasSuffix(stderr, test.wantHint) || strings.Count(stderr, "Hint:") != 1 {
				t.Fatalf("stderr = %q, want it to end with the one hint %q", stderr, test.wantHint)
			}
		})
	}
}
