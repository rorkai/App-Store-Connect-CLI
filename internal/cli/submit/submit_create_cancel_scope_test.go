package submit

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TestPrepareReviewSubmissionForCreateDoesNotCancelSubmissionForAnotherVersion
// proves that preparation never withdraws a review submission holding another
// version's review items.
func TestPrepareReviewSubmissionForCreateDoesNotCancelSubmissionForAnotherVersion(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "other-version-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"},
					"relationships": {
						"appStoreVersionForReview": {
							"data": {"type": "appStoreVersions", "id": "version-2"}
						}
					}
				}],
				"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/other-version-submission/items":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissionItems",
					"id": "other-version-item",
					"relationships": {
						"appStoreVersion": {
							"data": {"type": "appStoreVersions", "id": "version-2"}
						}
					}
				}],
				"links": {"self": "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/other-version-submission/items"}
			}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got, err := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1", nil)
		if err != nil {
			t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
		}
		if got.reuseSubmissionID != "" {
			t.Fatalf("expected no reusable submission, got %#v", got)
		}
	})

	wantRequests := []string{
		"GET /v1/apps/app-1/reviewSubmissions?filter%5Bplatform%5D=IOS&filter%5Bstate%5D=READY_FOR_REVIEW&include=appStoreVersionForReview&limit=200",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("unexpected requests: got %v want %v", requests, wantRequests)
	}
	if !strings.Contains(stderr, "Skipped stale review submission other-version-submission") {
		t.Fatalf("expected skip diagnostic naming the submission, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc submit cancel") {
		t.Fatalf("expected skip diagnostic to point at explicit cancellation, got %q", stderr)
	}
}

// TestPrepareReviewSubmissionForCreateDoesNotCancelUnprovenSubmission proves
// that a failed item lookup blocks reuse without falling back to cancellation.
func TestPrepareReviewSubmissionForCreateDoesNotCancelUnprovenSubmission(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.RequestURI())

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{
				"data": [{
					"type": "reviewSubmissions",
					"id": "unproven-submission",
					"attributes": {"state": "READY_FOR_REVIEW", "platform": "IOS"}
				}],
				"links": {"self": "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}
			}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/unproven-submission/items":
			return submitJSONResponse(http.StatusBadRequest, `{"errors":[{"status":"400","title":"Invalid request"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
		}
	}))

	stderr := captureSubmitStderr(t, func() {
		got, err := prepareReviewSubmissionForCreate(context.Background(), client, "app-1", "IOS", "version-1", nil)
		if err == nil {
			t.Fatal("expected unproven submission inspection to fail")
		}
		if got.reuseSubmissionID != "" {
			t.Fatalf("expected unproven submission not to be reused, got %#v", got)
		}
	})

	for _, request := range requests {
		if strings.HasPrefix(request, http.MethodPatch) {
			t.Fatalf("expected no cancellation request, got %v", requests)
		}
	}
	if stderr != "" {
		t.Fatalf("expected preparation failure to return through the error channel, got stderr %q", stderr)
	}
}
