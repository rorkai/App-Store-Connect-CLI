package cmdtest

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewItemsAddIncludesAssociatedErrorsInReturnedError(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.POST("/v1/reviewSubmissionItems").JSONBody(`{"data":{"type":"reviewSubmissionItems","relationships":{"reviewSubmission":{"data":{"type":"reviewSubmissions","id":"submission-1"}},"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`),
		asctest.JSON(http.StatusConflict, `{
		"errors": [{
			"status": "409",
			"code": "STATE_ERROR.ENTITY_STATE_INVALID",
			"title": "appStoreVersions with id 'version-1' is not in valid state.",
			"detail": "This resource cannot be reviewed, please check associated errors to see why.",
			"meta": {
				"associatedErrors": {
					"/v1/ageRatingDeclarations/age-rating-1": [
						{
							"code": "ENTITY_ERROR.ATTRIBUTE.REQUIRED",
							"detail": "You must provide a value for the attribute 'parentalControls' with this request"
						},
						{
							"code": "ENTITY_ERROR.ATTRIBUTE.REQUIRED",
							"detail": "You must provide a value for the attribute 'healthOrWellnessTopics' with this request"
						}
					]
				}
			}
		}]
	}`))

	stdout, _, runErr := runRootWithDeps(t, stubClient(t, reg), "review", "items-add", "--submission", "submission-1", "--item-type", "appStoreVersions", "--item-id", "version-1")

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	message := runErr.Error()
	if !strings.Contains(message, "review items-add: appStoreVersions with id 'version-1' is not in valid state.") {
		t.Fatalf("expected wrapped top-level message, got %q", message)
	}
	if !strings.Contains(message, "Associated errors for /v1/ageRatingDeclarations/age-rating-1:") {
		t.Fatalf("expected associated error heading, got %q", message)
	}
	if !strings.Contains(message, "parentalControls") {
		t.Fatalf("expected parentalControls detail, got %q", message)
	}
	if !strings.Contains(message, "healthOrWellnessTopics") {
		t.Fatalf("expected healthOrWellnessTopics detail, got %q", message)
	}
}

// reviewItemsAddNotReviewableBody is Apple's 409 for an App Store version that
// cannot be reviewed yet, captured live on the disposable app on 2026-11-22
// (two of its associated errors kept).
const reviewItemsAddNotReviewableBody = `{"errors":[{"id":"da02de37-d52d-459e-8c84-afc8126d6600","status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"appStoreVersions with id 'version-1' is not in valid state.","detail":"This resource cannot be reviewed, please check associated errors to see why.","meta":{"associatedErrors":{"/v1/apps/app-1":[{"id":"9f9090f6-3ea2-4966-bccf-b2b87f232da5","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.REQUIRED","title":"The provided entity is missing a required attribute","detail":"You must provide a value for the attribute 'contentRightsDeclaration' with this request","source":{"pointer":"/data/attributes/contentRightsDeclaration"}}],"/v1/appStoreReviewDetails/detail-1":[{"id":"54cfa550-e889-4d68-a935-da71ca7b97fa","status":"409","code":"ENTITY_ERROR.ATTRIBUTE.REQUIRED","title":"The provided entity is missing a required attribute","detail":"You must provide a value for the attribute 'demoAccountName' with this request","source":{"pointer":"/data/attributes/demoAccountName"}}]}}}]}`

// reviewItemsAddAssociatedBody wraps one associated error the way Apple's
// STATE_ERROR.ENTITY_STATE_INVALID does, with a detail App Store Connect sends.
func reviewItemsAddAssociatedBody(resource, detail string) string {
	return `{"errors":[{"status":"409","code":"STATE_ERROR.ENTITY_STATE_INVALID","title":"appStoreVersions with id 'version-1' is not in valid state.","detail":"This resource cannot be reviewed, please check associated errors to see why.","meta":{"associatedErrors":{"` + resource + `":[{"status":"409","code":"STATE_ERROR","detail":"` + detail + `"}]}}}]}`
}

func TestReviewItemsAddConflictHintNamesTheNextStep(t *testing.T) {
	submission := func(state string) string {
		return fmt.Sprintf(`{"data":{"type":"reviewSubmissions","id":"submission-1","attributes":{"platform":"IOS","state":%q},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}}`, state)
	}
	const submissionNotDraftBody = `{"errors":[{"status":"409","code":"STATE_ERROR","title":"Resource state is invalid.","detail":"reviewSubmission state does not allow adding more items."}]}`
	tests := []struct {
		name     string
		body     string
		state    string
		drafts   string
		wantHint string
	}{
		{
			name:     "version not reviewable",
			body:     reviewItemsAddNotReviewableBody,
			wantHint: "Hint: App Store Connect cannot review this item yet: fix the associated errors listed above, then add it again. `asc validate --app APP_ID --version-id version-1` lists what the version is missing.\n",
		},
		{
			name:     "unfulfilled compliance requirement",
			body:     reviewItemsAddAssociatedBody("/v1/appInfos/info-1", "Cannot submit version due to unfulfilled compliance requirement."),
			wantHint: " App Store Connect does not review the app until every compliance requirement has an answer. List the App Compliance requirements, such as the China mainland Security Assessment and AI Generated or Synthetic Content declarations, with `asc web apps compliance list --app APP_ID` and answer them with `asc web apps compliance set`; list the App Store Regulations & Permits declarations with `asc web apps declarations list --app APP_ID`.\n",
		},
		{
			name:     "version already on another submission",
			body:     reviewItemsAddAssociatedBody("/v1/appStoreVersions/version-1", "appStoreVersions with id 894111393 was already added to another reviewSubmission with id submission-0"),
			wantHint: " The item is already on submission submission-0: inspect it with `asc review submissions-get --id submission-0 --include items`, and submit that one or remove the item from it with `asc review items remove`.",
		},
		{
			name:     "asset placed on a version",
			body:     reviewItemsAddAssociatedBody("/appAssetLibraryImages/image-1", "Asset is locked by an embedded review submission."),
			wantHint: "Hint: This asset is placed on an app version, so App Store Connect reviews it with that version and does not take it as a separate item. Submit the version instead, for example with `asc review submit`, and the placed assets go with it.\n",
		},
		{
			name:     "submission no longer a draft, another draft exists",
			body:     submissionNotDraftBody,
			state:    "UNRESOLVED_ISSUES",
			drafts:   `{"data":[{"type":"reviewSubmissions","id":"submission-2","attributes":{"platform":"IOS","state":"READY_FOR_REVIEW"}}]}`,
			wantHint: "Hint: Submission submission-1 is UNRESOLVED_ISSUES, and App Store Connect adds items only to a draft submission (READY_FOR_REVIEW). Submission submission-2 is a draft for IOS: add the item there with `asc review items add --submission submission-2 --item-type appStoreVersions --item-id version-1`.\n",
		},
		{
			name:     "submission no longer a draft, no draft",
			body:     submissionNotDraftBody,
			state:    "WAITING_FOR_REVIEW",
			drafts:   `{"data":[]}`,
			wantHint: "Hint: Submission submission-1 is WAITING_FOR_REVIEW, and App Store Connect adds items only to a draft submission (READY_FOR_REVIEW). Create a draft with `asc review submissions-create --app app-1 --platform IOS`, then add the item to it.\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.POST("/v1/reviewSubmissionItems"), asctest.JSON(http.StatusConflict, test.body))
			if test.state != "" {
				reg.Register(asctest.GET("/v1/reviewSubmissions/submission-1", "include=app"), asctest.JSON(http.StatusOK, submission(test.state)))
			}
			if test.drafts != "" {
				reg.Register(asctest.GET("/v1/apps/app-1/reviewSubmissions", "filter[state]=READY_FOR_REVIEW", "filter[platform]=IOS"), asctest.JSON(http.StatusOK, test.drafts))
			}

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "review", "items", "add", "--submission", "submission-1", "--item-type", "appStoreVersions", "--item-id", "version-1")

			if code != cmd.ExitConflict {
				t.Fatalf("exit code = %d, want %d (stderr %q)", code, cmd.ExitConflict, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantHint) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantHint)
			}
			if got := strings.Count(stderr, "Hint:"); got != 1 {
				t.Fatalf("stderr has %d hints, want 1:\n%s", got, stderr)
			}
		})
	}
}
