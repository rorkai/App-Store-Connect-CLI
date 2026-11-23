package submit

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	companionsVersionItem = `{"type":"reviewSubmissionItems","id":"item-v","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}`
	companionsSubItem     = `{"type":"reviewSubmissionItems","id":"item-s","relationships":{"appStoreVersion":{"data":null},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"sub-ver-1"}}}}`
	companionsGroupItem   = `{"type":"reviewSubmissionItems","id":"item-g","relationships":{"appStoreVersion":{"data":null},"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"group-ver-1"}}}}`
)

func companionsItemsResponse(items string) (*http.Response, error) {
	return submitJSONResponse(http.StatusOK, `{"data":[`+items+`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items"}}`)
}

// TestSummarizeReviewSubmissionItemsAcceptsStagedSubscriptionItems covers the
// draft `asc web review subscriptions attach --review-items` builds for a first
// subscription: the app version plus its subscription and subscription group
// versions. App Store Connect reviews those with the version, so they must not
// make the draft look unrelated; every other item still does.
func TestSummarizeReviewSubmissionItemsAcceptsStagedSubscriptionItems(t *testing.T) {
	const (
		versionItem = `{"type":"reviewSubmissionItems","id":"item-v","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}`
		subItem     = `{"type":"reviewSubmissionItems","id":"item-s","relationships":{"appStoreVersion":{"data":null},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"sub-ver-1"}}}}`
		groupItem   = `{"type":"reviewSubmissionItems","id":"item-g","relationships":{"appStoreVersion":{"data":null},"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"group-ver-1"}}}}`
		eventItem   = `{"type":"reviewSubmissionItems","id":"item-e","relationships":{"appStoreVersion":{"data":null},"appEvent":{"data":{"type":"appEvents","id":"event-1"}}}}`
		bothItem    = `{"type":"reviewSubmissionItems","id":"item-b","relationships":{"appStoreVersion":{"data":null},"subscriptionVersion":{"data":{"type":"subscriptionVersions","id":"sub-ver-2"}},"subscriptionGroupVersion":{"data":{"type":"subscriptionGroupVersions","id":"group-ver-2"}}}}`
	)
	tests := []struct {
		name       string
		items      string
		wantTarget bool
		wantOther  bool
	}{
		{name: "version with staged subscription and group versions", items: versionItem + "," + subItem + "," + groupItem, wantTarget: true},
		{name: "an app event still blocks", items: versionItem + "," + subItem + "," + eventItem, wantTarget: true, wantOther: true},
		{name: "subscription items without the version are not the target", items: subItem + "," + groupItem},
		{name: "an item holding both a subscription and a group version still blocks", items: versionItem + "," + bothItem, wantTarget: true, wantOther: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fields, include string
			client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				fields = req.URL.Query().Get("fields[reviewSubmissionItems]")
				include = req.URL.Query().Get("include")
				return submitJSONResponse(http.StatusOK, `{"data":[`+tt.items+`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/reviewSubmissions/submission-1/items"}}`)
			}))

			summary, err := summarizeReviewSubmissionItems(context.Background(), client, "submission-1", "version-1")
			if err != nil {
				t.Fatalf("summarizeReviewSubmissionItems() error: %v", err)
			}
			if want := "appStoreVersion,subscriptionGroupVersion,subscriptionVersion"; fields != want || include != want {
				t.Fatalf("fields=%q include=%q, want both %q so staged subscription items are recognized", fields, include, want)
			}
			if summary.hasTargetVersion != tt.wantTarget || summary.hasOtherItems != tt.wantOther {
				t.Fatalf("summary = %+v, want target %t other %t", summary, tt.wantTarget, tt.wantOther)
			}
		})
	}
}

// TestReviewSubmissionHoldsOnlyVersionMatchesReviewSubmitReuse keeps
// `asc review status` advice in step with the drafts `asc review submit`
// reuses: a draft with only subscription items and no app version is skipped
// by review submit, so status must not call it reusable either.
func TestReviewSubmissionHoldsOnlyVersionMatchesReviewSubmitReuse(t *testing.T) {
	tests := []struct {
		name  string
		items string
		want  bool
	}{
		{name: "empty draft", items: "", want: true},
		{name: "version with staged subscription items", items: companionsVersionItem + "," + companionsSubItem + "," + companionsGroupItem, want: true},
		{name: "subscription items without the version", items: companionsSubItem + "," + companionsGroupItem, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newSubmitTestClient(t, submitRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return companionsItemsResponse(tt.items)
			}))
			got, _, err := ReviewSubmissionHoldsOnlyVersion(context.Background(), client, "submission-1", "version-1")
			if err != nil {
				t.Fatalf("ReviewSubmissionHoldsOnlyVersion() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ReviewSubmissionHoldsOnlyVersion() = %t, want %t", got, tt.want)
			}
		})
	}
}

// TestPrepareReviewSubmissionForCreateNamesStagedSubscriptionItemsOnReuse
// checks that reusing a draft returns its subscription items by type, ID and
// review item, so review submit can name them before it sends them.
func TestPrepareReviewSubmissionForCreateNamesStagedSubscriptionItemsOnReuse(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
			return companionsItemsResponse(companionsVersionItem + "," + companionsSubItem + "," + companionsGroupItem)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	}))

	got, err := prepareReviewSubmissionForCreate(context.Background(), io.Discard, client, "app-1", "IOS", "version-1", nil)
	if err != nil {
		t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
	}
	if got.reuseSubmissionID != "submission-1" || got.versionItemID != "item-v" {
		t.Fatalf("expected submission-1 reused with version item item-v, got %#v", got)
	}
	if want := "subscriptionVersions sub-ver-1 (review item item-s), subscriptionGroupVersions group-ver-1 (review item item-g)"; DescribeReviewItems(got.stagedItems) != want {
		t.Fatalf("staged items = %q, want %q", DescribeReviewItems(got.stagedItems), want)
	}
}

// TestPrepareReviewSubmissionForCreatePrefersTheVersionsDraft lists an empty
// draft before the draft that holds the version and staged subscription items.
// Picking the empty one would make --dry-run name no items while the real run
// later falls back to the version's draft and submits them, so the version's
// draft must win and the empty draft must not even be inspected.
func TestPrepareReviewSubmissionForCreatePrefersTheVersionsDraft(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[`+
				`{"type":"reviewSubmissions","id":"empty-draft","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}},`+
				`{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"appStoreVersionForReview":{"data":{"type":"appStoreVersions","id":"version-1"}}}}`+
				`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
			return companionsItemsResponse(companionsVersionItem + "," + companionsSubItem)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	}))

	got, err := prepareReviewSubmissionForCreate(context.Background(), io.Discard, client, "app-1", "IOS", "version-1", nil)
	if err != nil {
		t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
	}
	if got.reuseSubmissionID != "submission-1" || len(got.stagedItems) != 1 {
		t.Fatalf("expected the version's draft submission-1 with its staged item, got %#v", got)
	}
}

// TestPrepareReviewSubmissionForCreatePrefersTheDraftHoldingTheVersionWithoutALink
// covers drafts whose appStoreVersionForReview link is empty. An empty draft
// listed first must not win over a later draft whose items hold the version:
// the real run would add the version to the empty draft, get a 409, recover
// into the version's draft and submit its staged items, while --dry-run named
// none.
func TestPrepareReviewSubmissionForCreatePrefersTheDraftHoldingTheVersionWithoutALink(t *testing.T) {
	client := newSubmitTestClient(t, submitRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/app-1/reviewSubmissions":
			return submitJSONResponse(http.StatusOK, `{"data":[`+
				`{"type":"reviewSubmissions","id":"empty-draft","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}},`+
				`{"type":"reviewSubmissions","id":"submission-1","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}`+
				`],"links":{"self":"https://api.appstoreconnect.apple.com/v1/apps/app-1/reviewSubmissions"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/empty-draft/items":
			return companionsItemsResponse("")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/submission-1/items":
			return companionsItemsResponse(companionsVersionItem + "," + companionsSubItem)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.RequestURI())
			return submitJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		}
	}))

	got, err := prepareReviewSubmissionForCreate(context.Background(), io.Discard, client, "app-1", "IOS", "version-1", nil)
	if err != nil {
		t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
	}
	if got.reuseSubmissionID != "submission-1" || !got.reuseSubmissionHasVersion || len(got.stagedItems) != 1 {
		t.Fatalf("expected the version's draft submission-1 with its staged item, got %#v", got)
	}
}

// TestEmitUnnamedReviewItemsSingular checks the notice for one item reads
// "1 item ... sends it".
func TestEmitUnnamedReviewItemsSingular(t *testing.T) {
	var got []string
	emitUnnamedReviewItems(func(m string) { got = append(got, m) }, "draft-1", "version-1", []asc.ReviewSubmissionReceiptItem{
		{Type: "subscriptionVersions", ID: "sub-ver-1", ReviewSubmissionItemID: "item-s"},
	}, false)
	want := "Review submission draft-1 holds 1 item this command did not name; submitting it sends it to App Review with version version-1."
	if len(got) != 2 || got[0] != want {
		t.Fatalf("messages = %q, want first %q", got, want)
	}
}
