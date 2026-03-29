package shared

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type reviewSubmissionsSharedRoundTripFunc func(*http.Request) (*http.Response, error)

func (f reviewSubmissionsSharedRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newReviewSubmissionsSharedTestClient(t *testing.T, transport reviewSubmissionsSharedRoundTripFunc) *asc.Client {
	t.Helper()

	keyPath := filepath.Join(t.TempDir(), "key.p8")
	writeECDSAPEM(t, keyPath)

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}

	return client
}

func reviewSubmissionsSharedJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func TestCompareRFC3339DateStringsParsesOffsets(t *testing.T) {
	t.Parallel()

	older := "2026-02-20T01:00:00+01:00"
	newer := "2026-02-20T00:30:00Z"

	if cmp := CompareRFC3339DateStrings(newer, older); cmp <= 0 {
		t.Fatalf("expected %q to be newer than %q, got %d", newer, older, cmp)
	}
	if cmp := CompareRFC3339DateStrings(older, newer); cmp >= 0 {
		t.Fatalf("expected %q to be older than %q, got %d", older, newer, cmp)
	}
}

func TestShouldPreferLatestReviewSubmissionPrefersActiveSubmissionWithoutSubmittedDate(t *testing.T) {
	t.Parallel()

	current := asc.ReviewSubmissionResource{
		ID: "sub-ready",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmissionState: asc.ReviewSubmissionStateReadyForReview,
			SubmittedDate:   "",
		},
	}
	best := asc.ReviewSubmissionResource{
		ID: "sub-complete",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmissionState: asc.ReviewSubmissionStateComplete,
			SubmittedDate:   "2026-03-16T10:00:00Z",
		},
	}

	if !ShouldPreferLatestReviewSubmission(current, best) {
		t.Fatal("expected active ready-for-review submission to win")
	}
}

func TestShouldPreferLatestReviewSubmissionBreaksTiesByID(t *testing.T) {
	t.Parallel()

	current := asc.ReviewSubmissionResource{
		ID: "sub-2",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmittedDate: "2026-02-20T00:00:00Z",
		},
	}
	best := asc.ReviewSubmissionResource{
		ID: "sub-1",
		Attributes: asc.ReviewSubmissionAttributes{
			SubmittedDate: "2026-02-20T00:00:00Z",
		},
	}

	if !ShouldPreferLatestReviewSubmission(current, best) {
		t.Fatal("expected larger ID to win deterministic tie-break")
	}
}

func TestNormalizeReviewSubmissionStates_AcceptsCaseAndWhitespace(t *testing.T) {
	t.Parallel()

	input := []string{" ready_for_review ", "complete"}
	got, err := NormalizeReviewSubmissionStates(input)
	if err != nil {
		t.Fatalf("NormalizeReviewSubmissionStates() error: %v", err)
	}
	if len(got) != len(input) {
		t.Fatalf("expected %d states, got %d", len(input), len(got))
	}
	for i := range input {
		if got[i] != input[i] {
			t.Fatalf("expected state %d to remain %q, got %q", i, input[i], got[i])
		}
	}
}

func TestNormalizeReviewSubmissionStates_RejectsUnknownValue(t *testing.T) {
	t.Parallel()

	_, err := NormalizeReviewSubmissionStates([]string{"NOT_A_STATE"})
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
	if !strings.Contains(err.Error(), "--state must be one of:") {
		t.Fatalf("expected allowed states guidance, got %v", err)
	}
}

func TestFetchAllAppStoreVersions_PaginatesAllPages(t *testing.T) {
	t.Parallel()

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=next-page"
	requestCount := 0
	client := newReviewSubmissionsSharedTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected first request method GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
				t.Fatalf("expected appStoreVersions path, got %s", req.URL.Path)
			}
			if got := req.URL.Query().Get("limit"); got != "1" {
				t.Fatalf("expected limit=1 on first request, got %q", got)
			}
			return reviewSubmissionsSharedJSONResponse(http.StatusOK, `{
				"data": [{"type":"appStoreVersions","id":"version-1"}],
				"links": {"next":"`+nextURL+`"}
			}`)
		case 2:
			if req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
				t.Fatalf("expected appStoreVersions next path, got %s", req.URL.Path)
			}
			if got := req.URL.Query().Get("cursor"); got != "next-page" {
				t.Fatalf("expected cursor=next-page on paginated request, got %q", got)
			}
			return reviewSubmissionsSharedJSONResponse(http.StatusOK, `{
				"data": [{"type":"appStoreVersions","id":"version-2"}],
				"links": {}
			}`)
		default:
			t.Fatalf("unexpected extra request #%d: %s %s", requestCount, req.Method, req.URL.RequestURI())
			return nil, nil
		}
	})

	got, err := FetchAllAppStoreVersions(context.Background(), client, "app-1", asc.WithAppStoreVersionsLimit(1))
	if err != nil {
		t.Fatalf("FetchAllAppStoreVersions() error: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 app store versions, got %d", len(got))
	}
	if got[0].ID != "version-1" || got[1].ID != "version-2" {
		t.Fatalf("unexpected version IDs: %#v", got)
	}
}

func TestFetchAllReviewSubmissions_PaginatesAllPages(t *testing.T) {
	t.Parallel()

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=next-page"
	requestCount := 0
	client := newReviewSubmissionsSharedTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet {
				t.Fatalf("expected first request method GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/apps/app-1/reviewSubmissions" {
				t.Fatalf("expected reviewSubmissions path, got %s", req.URL.Path)
			}
			if got := req.URL.Query().Get("limit"); got != "1" {
				t.Fatalf("expected limit=1 on first request, got %q", got)
			}
			return reviewSubmissionsSharedJSONResponse(http.StatusOK, `{
				"data": [{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"COMPLETE"}}],
				"links": {"next":"`+nextURL+`"}
			}`)
		case 2:
			if req.URL.Path != "/v1/apps/app-1/reviewSubmissions" {
				t.Fatalf("expected reviewSubmissions next path, got %s", req.URL.Path)
			}
			if got := req.URL.Query().Get("cursor"); got != "next-page" {
				t.Fatalf("expected cursor=next-page on paginated request, got %q", got)
			}
			return reviewSubmissionsSharedJSONResponse(http.StatusOK, `{
				"data": [{"type":"reviewSubmissions","id":"submission-2","attributes":{"state":"IN_REVIEW"}}],
				"links": {}
			}`)
		default:
			t.Fatalf("unexpected extra request #%d: %s %s", requestCount, req.Method, req.URL.RequestURI())
			return nil, nil
		}
	})

	got, err := FetchAllReviewSubmissions(context.Background(), client, "app-1", asc.WithReviewSubmissionsLimit(1))
	if err != nil {
		t.Fatalf("FetchAllReviewSubmissions() error: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests, got %d", requestCount)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 review submissions, got %d", len(got))
	}
	if got[0].ID != "submission-1" || got[1].ID != "submission-2" {
		t.Fatalf("unexpected submission IDs: %#v", got)
	}
}

func TestFetchAllReviewSubmissions_ReturnsPaginationErrors(t *testing.T) {
	t.Parallel()

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions?cursor=next-page"
	requestCount := 0
	client := newReviewSubmissionsSharedTestClient(t, func(req *http.Request) (*http.Response, error) {
		requestCount++
		switch requestCount {
		case 1:
			return reviewSubmissionsSharedJSONResponse(http.StatusOK, `{
				"data": [{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"COMPLETE"}}],
				"links": {"next":"`+nextURL+`"}
			}`)
		case 2:
			return reviewSubmissionsSharedJSONResponse(http.StatusInternalServerError, `{"errors":[{"status":"500","detail":"boom"}]}`)
		default:
			t.Fatalf("unexpected extra request #%d: %s %s", requestCount, req.Method, req.URL.RequestURI())
			return nil, nil
		}
	})

	_, err := FetchAllReviewSubmissions(context.Background(), client, "app-1", asc.WithReviewSubmissionsLimit(1))
	if err == nil {
		t.Fatal("expected pagination error, got nil")
	}
	if !strings.Contains(err.Error(), "page 2:") {
		t.Fatalf("expected page-aware pagination error, got %v", err)
	}
}
