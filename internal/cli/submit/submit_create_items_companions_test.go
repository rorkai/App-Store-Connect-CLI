package submit

import (
	"context"
	"net/http"
	"testing"
)

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
