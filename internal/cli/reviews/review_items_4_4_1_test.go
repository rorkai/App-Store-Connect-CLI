package reviews

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestReviewItemsListOptionsValidate441Selections(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "review item fields", args: []string{"bad", "", "", "", ""}, want: "--fields must be one of"},
		{name: "include", args: []string{"", "bad", "", "", ""}, want: "--include must be one of"},
		{name: "iap fields", args: []string{"", "", "bad", "", ""}, want: "--iap-version-fields must be one of"},
		{name: "subscription fields", args: []string{"", "", "", "bad", ""}, want: "--subscription-version-fields must be one of"},
		{name: "group fields", args: []string{"", "", "", "", "bad"}, want: "--subscription-group-version-fields must be one of"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := reviewItemsListOptions(0, "", test.args[0], test.args[1], test.args[2], test.args[3], test.args[4])
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestReviewItemsListOptionsRejectNextConflicts(t *testing.T) {
	_, err := reviewItemsListOptions(0, "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/items?cursor=next", "subscriptionVersion", "", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "--next cannot be combined") {
		t.Fatalf("error = %v, want next conflict", err)
	}
}

func TestReviewItemsListRejectsPositionalArgsBeforeAuth(t *testing.T) {
	cmd := ReviewItemsListCommand()
	err := cmd.ParseAndRun(context.Background(), []string{"unexpected"})
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestReviewItemTypeListIncludes441VersionTypes(t *testing.T) {
	joined := strings.Join(reviewSubmissionItemTypeList(), ",")
	for _, want := range []string{"inAppPurchaseVersions", "subscriptionVersions", "subscriptionGroupVersions"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("item types %q do not contain %q", joined, want)
		}
	}
}
