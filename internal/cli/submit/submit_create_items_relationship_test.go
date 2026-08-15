package submit

import (
	"context"
	"net/http"
	"testing"
)

// TestSummarizeReviewSubmissionItemsRequestsVersionRelationship proves the items query asks for the
// appStoreVersion relationship.
//
// App Store Connect returns review submission items with no relationship linkage unless the request
// asks for it. Without that ask, every item reads as an empty version id, hasTargetVersion can never
// become true, and a correctly prepared submission is rejected with "does not contain target
// version". The other tests in this package do not catch it because their stubbed responses always
// include relationships, which the real API only does on request.
func TestSummarizeReviewSubmissionItemsRequestsVersionRelationship(t *testing.T) {
	var itemFields string
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/reviewSubmissions/submission-1/items" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		itemFields = req.URL.Query().Get("fields[reviewSubmissionItems]")

		return submitJSONResponse(http.StatusOK, `{
			"data": [{
				"type": "reviewSubmissionItems",
				"id": "item-1",
				"relationships": {
					"appStoreVersion": {
						"data": {"type": "appStoreVersions", "id": "version-1"}
					}
				}
			}]
		}`)
	}))

	summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "version-1")
	if err != nil {
		t.Fatalf("summarizeReviewSubmissionItems() error: %v", err)
	}

	if itemFields != "appStoreVersion" {
		t.Fatalf("expected fields[reviewSubmissionItems]=appStoreVersion, got %q", itemFields)
	}
	if !summary.hasTargetVersion {
		t.Fatal("expected the summary to find the target version")
	}
}

// TestSummarizeReviewSubmissionItemsWithoutRelationshipsIsNotTarget pins the behaviour the fix
// depends on: an item that arrives without relationship linkage counts as unrelated rather than as
// the target version, so the guard stays honest when the API declines to expand it.
func TestSummarizeReviewSubmissionItemsWithoutRelationshipsIsNotTarget(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return submitJSONResponse(http.StatusOK, `{
			"data": [{
				"type": "reviewSubmissionItems",
				"id": "item-1",
				"attributes": {"state": "READY_FOR_REVIEW"}
			}]
		}`)
	}))

	summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "version-1")
	if err != nil {
		t.Fatalf("summarizeReviewSubmissionItems() error: %v", err)
	}

	if summary.hasTargetVersion {
		t.Fatal("expected an item with no relationships not to count as the target version")
	}
	if !summary.hasOtherItems {
		t.Fatal("expected an item with no relationships to count as an unrelated item")
	}
}
