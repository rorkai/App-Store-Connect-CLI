package submit

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestSummarizeReviewSubmissionItemsRejectsRepeatedPaginationURL(t *testing.T) {
	pageReads := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		pageReads++
		return submitJSONResponse(http.StatusOK, `{
			"data": [],
			"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items", "next": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items?cursor=same"}
		}`)
	}))

	_, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "version-1")
	if !errors.Is(err, asc.ErrRepeatedPaginationURL) {
		t.Fatalf("expected repeated-pagination error, got %v", err)
	}
	if pageReads != 2 {
		t.Fatalf("review submission item pages read = %d, want 2", pageReads)
	}
}

func TestPrepareReviewSubmissionForCreateRejectsInvalidInputs(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
	}))

	tests := []struct {
		name      string
		ctx       context.Context
		client    *asc.Client
		appID     string
		platform  string
		versionID string
	}{
		{name: "nil context", client: client, appID: "app-1", platform: "IOS", versionID: "version-1"},
		{name: "nil client", ctx: context.Background(), appID: "app-1", platform: "IOS", versionID: "version-1"},
		{name: "empty app", ctx: context.Background(), client: client, platform: "IOS", versionID: "version-1"},
		{name: "empty platform", ctx: context.Background(), client: client, appID: "app-1", versionID: "version-1"},
		{name: "empty version", ctx: context.Background(), client: client, appID: "app-1", platform: "IOS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := prepareReviewSubmissionForCreate(tt.ctx, tt.client, tt.appID, tt.platform, tt.versionID, nil); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPrepareReviewSubmissionForCreateTreatsWhitespaceNextAsTerminal(t *testing.T) {
	requests := 0
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return submitJSONResponse(http.StatusOK, `{"data":[],"links":{"self":"/v1/apps/app-1/reviewSubmissions","next":"  \t  "}}`)
	}))

	prepared, err := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1", nil)
	if err != nil {
		t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
	}
	if prepared != (submitCreateReviewSubmissionPreparation{}) {
		t.Fatalf("preparation = %#v, want empty", prepared)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

// respondLikeAppStoreConnectItems replays what the live API actually does for
// GET /v1/reviewSubmissions/{id}/items, captured on 2026-08-16 against submission
// 3f07316f-1770-4675-96db-07b0eb857f55:
//
//   - ?fields[reviewSubmissionItems]=appStoreVersion  -> the item carries "links" ONLY. There is no
//     "relationships" key at all, because fields[] is a sparse-FIELDSET selector and does not ask
//     the API to materialise relationship linkage.
//   - ?include=appStoreVersion                        -> the item carries
//     relationships.appStoreVersion.data.id.
//
// Stubbing that difference is the whole point. A stub that returns relationships unconditionally
// cannot distinguish a request that asks for linkage from one that does not, so it reports a fix as
// working when the real API would still withhold the linkage.
func respondLikeAppStoreConnectItems(req *http.Request) (*http.Response, error) {
	for _, value := range req.URL.Query()["include"] {
		if value == "appStoreVersion" {
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "item-1",
					"attributes": {"state": "READY_FOR_REVIEW"},
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-1"}
						}
					}
				}],
				"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items"},
				"meta": {"paging": {"total": 1, "limit": 200}}
			}`)
		}
	}

	return submitJSONResponse(http.StatusOK, `{
		"data": [{
			"type": "reviewSubmissionItems",
			"id": "item-1",
			"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissionItems/item-1"}
		}],
		"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items"},
		"meta": {"paging": {"total": 1, "limit": 200}}
	}`)
}

// TestSummarizeReviewSubmissionItemsIncludesVersionRelationship is the regression test for the
// second half of the "does not contain target version" bug.
//
// Requesting fields[reviewSubmissionItems]=appStoreVersion was not enough: the API answered with an
// item that had no relationships at all, so reviewSubmissionItemVersionID read an empty id,
// hasTargetVersion stayed false, and a correctly prepared submission was rejected. Only
// include=appStoreVersion makes App Store Connect materialise the linkage.
//
// This test drives the stub above, which withholds linkage exactly as the live API does, so it fails
// against a query that only sets fields[] and passes once include is sent.
func TestSummarizeReviewSubmissionItemsIncludesVersionRelationship(t *testing.T) {
	var sawInclude bool
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/reviewSubmissions/submission-1/items" {
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
		for _, value := range req.URL.Query()["include"] {
			if value == "appStoreVersion" {
				sawInclude = true
			}
		}

		return respondLikeAppStoreConnectItems(req)
	}))

	summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "version-1")
	if err != nil {
		t.Fatalf("summarizeReviewSubmissionItems() error: %v", err)
	}

	if !sawInclude {
		t.Fatal("expected the items query to send include=appStoreVersion")
	}
	if !summary.hasTargetVersion {
		t.Fatal("expected the summary to find the target version once linkage is requested")
	}
	if summary.hasOtherItems {
		t.Fatal("the only item is the target version, so it must not count as unrelated")
	}
}

// TestSummarizeReviewSubmissionItemsFieldsAloneCannotResolveVersion pins WHY the previous fix was
// insufficient, so nobody re-introduces it by swapping include back for fields. It asserts the
// captured link-only payload (what fields[] alone returns) is unusable, which is the exact condition
// that produced "review submission %s does not contain target version %s" in production.
func TestSummarizeReviewSubmissionItemsFieldsAloneCannotResolveVersion(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Strip include to simulate a query that only sets fields[reviewSubmissionItems].
		query := req.URL.Query()
		query.Del("include")
		req.URL.RawQuery = query.Encode()

		return respondLikeAppStoreConnectItems(req)
	}))

	summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "version-1")
	if err != nil {
		t.Fatalf("summarizeReviewSubmissionItems() error: %v", err)
	}

	if summary.hasTargetVersion {
		t.Fatal("an item with no relationship linkage cannot prove it is the target version")
	}
}
