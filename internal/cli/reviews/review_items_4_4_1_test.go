package reviews

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

func TestReviewListNextConflictsUseFlagPresenceBeforeClientFactory(t *testing.T) {
	const itemsNext = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/items?cursor=next"
	const submissionsNext = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions?cursor=next"

	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
		want    string
	}{
		{name: "items owner", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--submission", "sub-1"}, want: "--submission"},
		{name: "items empty owner", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--submission", ""}, want: "--submission"},
		{name: "items whitespace owner", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--submission", "   "}, want: "--submission"},
		{name: "items zero limit", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--limit", "0"}, want: "--limit"},
		{name: "items empty fields", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--fields", ""}, want: "--fields"},
		{name: "items whitespace include", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--include", "   "}, want: "--include"},
		{name: "items empty iap fields", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--iap-version-fields", ""}, want: "--iap-version-fields"},
		{name: "items whitespace subscription fields", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--subscription-version-fields", "   "}, want: "--subscription-version-fields"},
		{name: "items empty group fields", command: ReviewItemsListCommand, args: []string{"--next", itemsNext, "--subscription-group-version-fields", ""}, want: "--subscription-group-version-fields"},
		{name: "submissions app", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--app", "app-1"}, want: "--app"},
		{name: "submissions empty app", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--app", ""}, want: "--app"},
		{name: "submissions whitespace app", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--app", "   "}, want: "--app"},
		{name: "submissions true global", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--global"}, want: "--global"},
		{name: "submissions false global", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--global=false"}, want: "--global"},
		{name: "submissions empty platform", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--platform", ""}, want: "--platform"},
		{name: "submissions whitespace state", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--state", "   "}, want: "--state"},
		{name: "submissions zero limit", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--limit", "0"}, want: "--limit"},
		{name: "submissions empty item fields", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--item-fields", ""}, want: "--item-fields"},
		{name: "submissions whitespace include", command: ReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--include", "   "}, want: "--include"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			poisonFactory := func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			}
			restoreItems := SetReviewItemsClientFactory(poisonFactory)
			restoreSubmissions := SetReviewSubmissionsClientFactory(poisonFactory)
			defer restoreItems()
			defer restoreSubmissions()

			err := test.command().ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want usage error containing %q", err, test.want)
			}
			if factoryCalled {
				t.Fatal("client factory called before opaque-next conflict validation")
			}
		})
	}
}

func TestReviewItemsListRejectsPositionalArgsBeforeAuth(t *testing.T) {
	cmd := ReviewItemsListCommand()
	err := cmd.ParseAndRun(context.Background(), []string{"unexpected"})
	if err == nil || !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("error = %v, want usage error", err)
	}
}

func TestReviewSubmissionsValidateIncludeBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		cmd  *ffcli.Command
		args []string
	}{
		{
			name: "list invalid include",
			cmd:  ReviewSubmissionsListCommand(),
			args: []string{"--global", "--app", "app-1", "--include", "invalid"},
		},
		{
			name: "detail invalid include",
			cmd:  ReviewSubmissionsGetCommand(),
			args: []string{"--id", "submission-1", "--include", "invalid"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.cmd.ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--include must be one of") {
				t.Fatalf("error = %v, want include usage error", err)
			}
		})
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
