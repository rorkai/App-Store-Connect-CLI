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
	for _, want := range []string{"inAppPurchaseVersions", "subscriptionVersions", "subscriptionGroupVersions", "appStoreVersionExperimentsV2"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("item types %q do not contain %q", joined, want)
		}
	}
	if strings.Contains(joined, "appStoreVersionExperimentTreatments") {
		t.Fatalf("item types %q must not advertise unsupported experiment treatments", joined)
	}
	if strings.Contains(joined, "appCustomProductPages") {
		t.Fatalf("item types %q must not advertise unsupported custom product pages", joined)
	}
}

func TestReviewItemsAddRejectsDeprecatedExperimentTreatment(t *testing.T) {
	_, err := normalizeReviewSubmissionItemType("appStoreVersionExperimentTreatments")
	if err == nil || !strings.Contains(err.Error(), "deprecated and no longer supported") {
		t.Fatalf("error = %v, want treatment deprecation guidance", err)
	}
}

func TestReviewItemsAddRejectsDeprecatedCustomProductPage(t *testing.T) {
	_, err := normalizeReviewSubmissionItemType("appCustomProductPages")
	if err == nil || !strings.Contains(err.Error(), "app custom product page version ID") || !strings.Contains(err.Error(), "appCustomProductPageVersions") {
		t.Fatalf("error = %v, want custom product page version migration guidance", err)
	}
}

func TestReviewItemsAddRejectsUnsupportedLegacyTypesBeforeAuth(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		want     string
	}{
		{name: "custom product page", itemType: "appCustomProductPages", want: "appCustomProductPageVersions"},
		{name: "experiment treatment", itemType: "appStoreVersionExperimentTreatments", want: "experiment treatments cannot be added"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := SetReviewItemsClientFactory(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			defer restore()

			err := ReviewItemsAddCommand().ParseAndRun(context.Background(), []string{
				"--submission", "submission-1",
				"--item-type", test.itemType,
				"--item-id", "item-1",
			})
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want usage error containing %q", err, test.want)
			}
			if factoryCalled {
				t.Fatal("client factory called before legacy type validation")
			}
		})
	}
}

func TestReviewItemsViewIsDeprecatedBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
	}{
		{name: "legacy", command: ReviewItemsGetCommand},
		{name: "nested", command: func() *ffcli.Command {
			return reviewItemsGetCommand("view", "review items view", `asc review items view --id "ITEM_ID"`)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := SetReviewItemsClientFactory(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			defer restore()

			err := test.command().ParseAndRun(context.Background(), []string{"--id", "item-1"})
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "has no item-detail GET") || !strings.Contains(err.Error(), "review items list --submission") {
				t.Fatalf("error = %v, want deprecated item-detail guidance", err)
			}
			if factoryCalled {
				t.Fatal("client factory called for unsupported item-detail GET")
			}
		})
	}
}

func TestReviewItemsUpdateValidatesSchemaFlagsBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "deprecated state", args: []string{"--id", "item-1", "--state", "READY_FOR_REVIEW"}, want: "--state is deprecated"},
		{name: "explicit empty state", args: []string{"--id", "item-1", "--state", ""}, want: "--state is deprecated"},
		{name: "missing update", args: []string{"--id", "item-1"}, want: "at least one of --resolved, --removed, --clear-resolved, or --clear-removed is required"},
		{name: "invalid resolved", args: []string{"--id", "item-1", "--resolved", "yes"}, want: "--resolved must be true or false"},
		{name: "invalid removed", args: []string{"--id", "item-1", "--removed", "no"}, want: "--removed must be true or false"},
		{name: "resolved conflict", args: []string{"--id", "item-1", "--resolved", "false", "--clear-resolved"}, want: "--resolved cannot be combined with --clear-resolved"},
		{name: "removed conflict", args: []string{"--id", "item-1", "--removed", "true", "--clear-removed"}, want: "--removed cannot be combined with --clear-removed"},
		{name: "positional", args: []string{"--id", "item-1", "--resolved", "true", "unexpected"}, want: "unexpected positional arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := SetReviewItemsClientFactory(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			defer restore()

			err := ReviewItemsUpdateCommand().ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want usage error containing %q", err, test.want)
			}
			if factoryCalled {
				t.Fatal("client factory called before update validation")
			}
		})
	}
}

func TestReviewSubmissionItemIDsRejectOpaqueNextConflictsAndPositionals(t *testing.T) {
	const next = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/relationships/items?cursor=next"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "owner", args: []string{"--next", next, "--id", "sub-1"}, want: "--id"},
		{name: "empty owner", args: []string{"--next", next, "--id", ""}, want: "--id"},
		{name: "zero limit", args: []string{"--next", next, "--limit", "0"}, want: "--limit"},
		{name: "positional", args: []string{"--id", "sub-1", "unexpected"}, want: "unexpected positional arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := SetReviewSubmissionsClientFactory(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			defer restore()

			err := ReviewSubmissionsItemsIDsCommand().ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want usage error containing %q", err, test.want)
			}
			if factoryCalled {
				t.Fatal("client factory called before linkage validation")
			}
		})
	}
}

func TestReviewSubmissionMutationsRejectPositionalsBeforeAuth(t *testing.T) {
	tests := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "create", command: ReviewSubmissionsCreateCommand, args: []string{"--app", "app-1", "unexpected"}},
		{name: "update", command: ReviewSubmissionsUpdateCommand, args: []string{"--id", "sub-1", "--canceled", "true", "unexpected"}},
		{name: "submit", command: ReviewSubmissionsSubmitCommand, args: []string{"--id", "sub-1", "--confirm", "unexpected"}},
		{name: "cancel", command: ReviewSubmissionsCancelCommand, args: []string{"--id", "sub-1", "--confirm", "unexpected"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			restore := SetReviewSubmissionsClientFactory(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})
			defer restore()

			err := test.command().ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "unexpected positional arguments") {
				t.Fatalf("error = %v, want positional usage error", err)
			}
			if factoryCalled {
				t.Fatal("client factory called before positional validation")
			}
		})
	}
}
