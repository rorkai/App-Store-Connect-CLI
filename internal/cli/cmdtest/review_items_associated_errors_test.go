package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestReviewItemsAddIncludesAssociatedErrorsInReturnedError(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
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

	stdout, _, runErr := runCommand(t, []string{"review", "items-add", "--submission", "submission-1", "--item-type", "appStoreVersions", "--item-id", "version-1"})

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
