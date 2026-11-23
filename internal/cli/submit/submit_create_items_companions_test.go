package submit

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
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
			got, err := ReviewSubmissionHoldsOnlyVersion(context.Background(), client, "submission-1", "version-1")
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
// checks that the reuse notice says the draft's subscription items go to
// review with the version, so the caller is not surprised they were submitted.
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

	stderr := captureSubmitStderr(t, func(w io.Writer) {
		got, err := prepareReviewSubmissionForCreate(context.Background(), w, client, "app-1", "IOS", "version-1", nil)
		if err != nil {
			t.Fatalf("prepareReviewSubmissionForCreate() error: %v", err)
		}
		if got.reuseSubmissionID != "submission-1" {
			t.Fatalf("expected submission-1 to be reused, got %#v", got)
		}
	})

	if want := "Reusing existing review submission submission-1 because the target version is already attached; its 2 subscription review items are submitted with the version."; !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
