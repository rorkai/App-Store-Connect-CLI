package reviews

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
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
			d, _, _ := newReviewTestDeps(nil)
			_, err := reviewItemsListOptions(d, 0, "", test.args[0], test.args[1], test.args[2], test.args[3], test.args[4])
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestReviewItemsListOptionsRejectNextConflicts(t *testing.T) {
	d, _, _ := newReviewTestDeps(nil)
	_, err := reviewItemsListOptions(d, 0, "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/items?cursor=next", "subscriptionVersion", "", "", "", "")
	if err == nil || !strings.Contains(err.Error(), "--next cannot be combined") {
		t.Fatalf("error = %v, want next conflict", err)
	}
}

func TestReviewListNextConflictsUseFlagPresenceBeforeClientFactory(t *testing.T) {
	const itemsNext = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions/sub-1/items?cursor=next"
	const submissionsNext = "https://api.appstoreconnect.apple.com/v1/reviewSubmissions?cursor=next"

	tests := []struct {
		name    string
		command func(*shared.Deps) *ffcli.Command
		args    []string
		want    string
	}{
		{name: "items zero limit", command: newReviewItemsListCommand, args: []string{"--next", itemsNext, "--limit", "0"}, want: "--limit"},
		{name: "items empty fields", command: newReviewItemsListCommand, args: []string{"--next", itemsNext, "--fields", ""}, want: "--fields"},
		{name: "items whitespace include", command: newReviewItemsListCommand, args: []string{"--next", itemsNext, "--include", "   "}, want: "--include"},
		{name: "items empty iap fields", command: newReviewItemsListCommand, args: []string{"--next", itemsNext, "--iap-version-fields", ""}, want: "--iap-version-fields"},
		{name: "items whitespace subscription fields", command: newReviewItemsListCommand, args: []string{"--next", itemsNext, "--subscription-version-fields", "   "}, want: "--subscription-version-fields"},
		{name: "items empty group fields", command: newReviewItemsListCommand, args: []string{"--next", itemsNext, "--subscription-group-version-fields", ""}, want: "--subscription-group-version-fields"},
		{name: "submissions true global", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--global"}, want: "--global"},
		{name: "submissions false global", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--global=false"}, want: "--global"},
		{name: "submissions empty platform", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--platform", ""}, want: "--platform"},
		{name: "submissions whitespace state", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--state", "   "}, want: "--state"},
		{name: "submissions zero limit", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--limit", "0"}, want: "--limit"},
		{name: "submissions empty item fields", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--item-fields", ""}, want: "--item-fields"},
		{name: "submissions whitespace include", command: newReviewSubmissionsListCommand, args: []string{"--next", submissionsNext, "--include", "   "}, want: "--include"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			poisonFactory := func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			}
			d, _, _ := newReviewTestDeps(poisonFactory)

			err := test.command(d).ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want usage error containing %q", err, test.want)
			}
			if factoryCalled {
				t.Fatal("client factory called before opaque-next conflict validation")
			}
		})
	}
}

func TestReviewSubmissionsValidateIncludeBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*shared.Deps) *ffcli.Command
		args []string
	}{
		{
			name: "list invalid include",
			cmd:  newReviewSubmissionsListCommand,
			args: []string{"--global", "--app", "app-1", "--include", "invalid"},
		},
		{
			name: "detail invalid include",
			cmd:  newReviewSubmissionsGetCommand,
			args: []string{"--id", "submission-1", "--include", "invalid"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, _ := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := test.cmd(d).ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "--include must be one of") {
				t.Fatalf("error = %v, want include usage error", err)
			}
			if factoryCalled {
				t.Fatal("client factory called before include validation")
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

func TestReviewItemsAddRejectsRemovedItemTypesBeforeAuth(t *testing.T) {
	tests := []struct {
		name     string
		itemType string
		wantErr  string
	}{
		{
			name:     "custom product page",
			itemType: "appCustomProductPages",
			wantErr:  "--item-type appCustomProductPages is deprecated and no longer supported by App Store Connect; pass an app custom product page version ID with --item-type appCustomProductPageVersions",
		},
		{
			name:     "experiment treatment",
			itemType: "appStoreVersionExperimentTreatments",
			wantErr:  "--item-type appStoreVersionExperimentTreatments is deprecated and no longer supported by App Store Connect; experiment treatments cannot be added as review submission items",
		},
		{
			name:     "unknown",
			itemType: "nope",
			wantErr:  "--item-type must be one of: " + strings.Join(reviewSubmissionItemTypeList(), ", "),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, _ := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := newReviewItemsAddCommand(d).ParseAndRun(context.Background(), []string{
				"--submission", "submission-1",
				"--item-type", test.itemType,
				"--item-id", "item-1",
			})
			if err == nil || !errors.Is(err, flag.ErrHelp) || err.Error() != test.wantErr {
				t.Fatalf("error = %q, want %q with flag.ErrHelp", err, test.wantErr)
			}
			if factoryCalled {
				t.Fatal("client factory called before removed item type validation")
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
		{name: "missing update", args: []string{"--id", "item-1"}, want: "at least one update flag is required (--resolved, --removed, --clear-resolved, --clear-removed)"},
		{name: "invalid resolved", args: []string{"--id", "item-1", "--resolved", "yes"}, want: "--resolved must be true or false"},
		{name: "invalid removed", args: []string{"--id", "item-1", "--removed", "no"}, want: "--removed must be true or false"},
		{name: "resolved conflict", args: []string{"--id", "item-1", "--resolved", "false", "--clear-resolved"}, want: "--resolved cannot be combined with --clear-resolved"},
		{name: "removed conflict", args: []string{"--id", "item-1", "--removed", "true", "--clear-removed"}, want: "--removed cannot be combined with --clear-removed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, stderrBuffer := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := newReviewItemsUpdateCommand(d).ParseAndRun(context.Background(), test.args)
			stderr := stderrBuffer.String()
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(stderr, "Error: "+test.want) {
				t.Fatalf("error = %v, stderr = %q, want usage error printing %q", err, stderr, test.want)
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
		{name: "zero limit", args: []string{"--next", next, "--limit", "0"}, want: "--limit"},
		{name: "positional", args: []string{"--id", "sub-1", "unexpected"}, want: "unexpected positional arguments"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, _ := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := newReviewSubmissionsItemsIDsCommand(d).ParseAndRun(context.Background(), test.args)
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
		command func(*shared.Deps) *ffcli.Command
		args    []string
	}{
		{name: "create", command: newReviewSubmissionsCreateCommand, args: []string{"--app", "app-1", "unexpected"}},
		{name: "update", command: newReviewSubmissionsUpdateCommand, args: []string{"--id", "sub-1", "--canceled", "true", "unexpected"}},
		{name: "submit", command: newReviewSubmissionsSubmitCommand, args: []string{"--id", "sub-1", "--confirm", "unexpected"}},
		{name: "cancel", command: newReviewSubmissionsCancelCommand, args: []string{"--id", "sub-1", "--confirm", "unexpected"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalled := false
			d, _, _ := newReviewTestDeps(func() (*asc.Client, error) {
				factoryCalled = true
				return nil, errors.New("poison client factory called")
			})

			err := test.command(d).ParseAndRun(context.Background(), test.args)
			if err == nil || !errors.Is(err, flag.ErrHelp) || !strings.Contains(err.Error(), "unexpected positional arguments") {
				t.Fatalf("error = %v, want positional usage error", err)
			}
			if factoryCalled {
				t.Fatal("client factory called before positional validation")
			}
		})
	}
}
