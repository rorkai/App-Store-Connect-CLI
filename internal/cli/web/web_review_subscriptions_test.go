package web

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

const reviewSubscriptionsListRequest = "GET /iris/v1/apps/app-1/subscriptionGroups"

func TestWebReviewSubscriptionsListCommandOutputsJSON(t *testing.T) {
	fake := stubReviewSubscriptions(t, premiumGroupBody(t, monthlySubscription("READY_TO_SUBMIT", true)))

	stdout, _, err := runReviewSubscriptionsCommand(t, WebReviewSubscriptionsListCommand, "--app", "app-1", "--output", "json")
	if err != nil {
		t.Fatalf("exec error: %v", err)
	}
	fake.assertRequests(t, reviewSubscriptionsListRequest)

	var payload reviewSubscriptionsListOutput
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
	}
	if payload.AppID != "app-1" {
		t.Fatalf("expected app-1, got %#v", payload)
	}
	if payload.AttachedCount != 1 || len(payload.Subscriptions) != 1 {
		t.Fatalf("unexpected list output: %#v", payload)
	}
	if payload.Subscriptions[0].ID != "sub-1" || !payload.Subscriptions[0].SubmitWithNextAppStoreVersion {
		t.Fatalf("unexpected subscription output: %#v", payload.Subscriptions[0])
	}
}

func TestWebReviewSubscriptionsListCommandTableIncludesGroupID(t *testing.T) {
	fake := stubReviewSubscriptions(t, premiumGroupBody(t, monthlySubscription("READY_TO_SUBMIT", true)))

	stdout, _, err := runReviewSubscriptionsCommand(t, WebReviewSubscriptionsListCommand, "--app", "app-1", "--output", "table")
	if err != nil {
		t.Fatalf("exec error: %v", err)
	}
	fake.assertRequests(t, reviewSubscriptionsListRequest)

	if !strings.Contains(stdout, "Group ID") {
		t.Fatalf("expected table header to include Group ID, got %q", stdout)
	}
	if !strings.Contains(stdout, "group-1") {
		t.Fatalf("expected table output to include group ID, got %q", stdout)
	}
	if !strings.Contains(stdout, "Premium") {
		t.Fatalf("expected table output to include group reference name, got %q", stdout)
	}
}

func TestReviewSubscriptionAttachSkipReasonReadyToSubmitDoesNotClaimAlreadyAttached(t *testing.T) {
	reason := reviewSubscriptionAttachSkipReason(webcore.ReviewSubscription{State: "READY_TO_SUBMIT"})
	if strings.Contains(reason, "already attached") {
		t.Fatalf("expected READY_TO_SUBMIT skip reason to avoid already-attached wording, got %q", reason)
	}
	if !strings.Contains(reason, "READY_TO_SUBMIT") {
		t.Fatalf("expected READY_TO_SUBMIT skip reason to mention current state, got %q", reason)
	}
}

func TestFindReviewSubscriptionBoundsDuplicateIDDiagnostic(t *testing.T) {
	selector := "subscription-id-" + strings.Repeat("9", shared.AmbiguousDiagnosticTextLimit) + "-selector-tail"
	providerProductID := strings.Repeat("界", shared.AmbiguousDiagnosticTextLimit) + "-product-tail\x1b[31m"
	providerName := strings.Repeat("名", shared.AmbiguousDiagnosticTextLimit) + "-name-tail"

	_, err := findReviewSubscription([]webcore.ReviewSubscription{
		{ID: selector, ProductID: providerProductID, Name: providerName},
		{ID: selector, ProductID: "com.example.second", Name: "Second"},
	}, selector)
	if err == nil {
		t.Fatal("expected duplicate subscription ID to fail")
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity diagnostic must remain valid UTF-8: %q", message)
	}
	for _, unsafe := range []string{"-selector-tail", "-product-tail", "-name-tail", "\x1b"} {
		if strings.Contains(message, unsafe) {
			t.Fatalf("provider text must be bounded and terminal-safe: %q", message)
		}
	}

	var structured *shared.AmbiguousSelectionError
	if !errors.As(err, &structured) {
		t.Fatalf("expected structured ambiguity error, got %T: %v", err, err)
	}
	if structured.DisplayTextLimit != shared.AmbiguousDiagnosticTextLimit {
		t.Fatalf("display text limit = %d, want %d", structured.DisplayTextLimit, shared.AmbiguousDiagnosticTextLimit)
	}
	if len(structured.Candidates) != 2 || structured.Candidates[0].ID != selector || structured.Candidates[0].Label != providerProductID || structured.Candidates[0].Extra != providerName {
		t.Fatalf("structured ambiguity must retain exact provider values: %#v", structured.Candidates)
	}
}

func TestFindReviewSubscriptionGroupMatchesReferenceName(t *testing.T) {
	got, err := findReviewSubscriptionGroup([]webcore.ReviewSubscription{
		{GroupID: "group-1", GroupReferenceName: "Premium", ID: "sub-1"},
		{GroupID: "group-1", GroupReferenceName: "Premium", ID: "sub-2"},
	}, "premium")
	if err != nil {
		t.Fatalf("findReviewSubscriptionGroup() error = %v", err)
	}
	if got == nil || got.GroupID != "group-1" || got.GroupReferenceName != "Premium" {
		t.Fatalf("unexpected group match: %#v", got)
	}
}

func TestFindReviewSubscriptionGroupRejectsAmbiguousReferenceName(t *testing.T) {
	_, err := findReviewSubscriptionGroup([]webcore.ReviewSubscription{
		{GroupID: "group-1", GroupReferenceName: "Premium", ID: "sub-1"},
		{GroupID: "group-2", GroupReferenceName: "Premium", ID: "sub-2"},
	}, "Premium")
	if err == nil {
		t.Fatal("expected ambiguous group name to fail")
	}
	if !strings.Contains(err.Error(), `2 subscription groups match "Premium" by name; pass --group-id with one of:`) {
		t.Fatalf("expected ambiguity diagnostic, got %q", err)
	}
}

func TestFindReviewSubscriptionGroupBoundsProviderDiagnostic(t *testing.T) {
	selector := strings.Repeat("名", shared.AmbiguousDiagnosticTextLimit) + "-selector-tail"
	firstID := "group-recovery-id-" + strings.Repeat("9", shared.AmbiguousDiagnosticTextLimit) + "-id-tail\x1b[31m"
	secondID := "group-2"

	_, err := findReviewSubscriptionGroup([]webcore.ReviewSubscription{
		{GroupID: firstID, GroupReferenceName: selector, ID: "sub-1"},
		{GroupID: secondID, GroupReferenceName: selector, ID: "sub-2"},
	}, selector)
	if err == nil {
		t.Fatal("expected ambiguous group name to fail")
	}
	message := err.Error()
	if !utf8.ValidString(message) {
		t.Fatalf("ambiguity diagnostic must remain valid UTF-8: %q", message)
	}
	for _, unsafe := range []string{"-selector-tail", "-id-tail", "\x1b"} {
		if strings.Contains(message, unsafe) {
			t.Fatalf("provider text must be bounded and terminal-safe: %q", message)
		}
	}

	var structured *shared.AmbiguousSelectionError
	if !errors.As(err, &structured) {
		t.Fatalf("expected structured ambiguity error, got %T: %v", err, err)
	}
	if structured.DisplayTextLimit != shared.AmbiguousDiagnosticTextLimit {
		t.Fatalf("display text limit = %d, want %d", structured.DisplayTextLimit, shared.AmbiguousDiagnosticTextLimit)
	}
	if len(structured.Candidates) != 2 || structured.Candidates[0].ID != firstID || structured.Candidates[0].Extra != selector {
		t.Fatalf("structured ambiguity must retain exact provider values: %#v", structured.Candidates)
	}
}

func TestFindReviewSubscriptionGroupRejectsEmptySelector(t *testing.T) {
	_, err := findReviewSubscriptionGroup(nil, "   ")
	if err == nil {
		t.Fatal("expected empty group selector to fail")
	}
	if !strings.Contains(err.Error(), "group selector is required") {
		t.Fatalf("expected empty-selector diagnostic, got %q", err)
	}
}

func TestWithReviewSelectorDiagnosticPreservesRemoteFailure(t *testing.T) {
	err := withReviewSelectorDiagnostic(&webcore.APIError{Status: http.StatusUnauthorized}, "--subscription-id")
	if err == nil {
		t.Fatal("expected remote failure")
	}
	if _, ok := shared.DiagnosticFromError(err); ok {
		t.Fatalf("remote failure unexpectedly received selector diagnostic: %v", err)
	}
}

func TestWebReviewSubscriptionsRejectAmbiguousSelectorBeforeMutation(t *testing.T) {
	tests := []struct {
		name          string
		command       func() *ffcli.Command
		args          []string
		body          func(t *testing.T) string
		wantMessage   string
		wantParameter string
	}{
		{
			name:    "AttachGroupRejectsAmbiguousSelectorBeforeMutation",
			command: WebReviewSubscriptionsAttachGroupCommand,
			args:    []string{"--group-id", "Premium"},
			body: func(t *testing.T) string {
				return reviewSubscriptionGroupsBody(t,
					reviewSubscriptionGroupFixture{ID: "group-1", ReferenceName: "Premium", Subscriptions: []reviewSubscriptionFixture{{ID: "sub-1", State: "READY_TO_SUBMIT"}}},
					reviewSubscriptionGroupFixture{ID: "group-2", ReferenceName: "premium", Subscriptions: []reviewSubscriptionFixture{{ID: "sub-2", State: "READY_TO_SUBMIT"}}},
				)
			},
			wantMessage:   `2 subscription groups match "Premium" by name; pass --group-id with one of:`,
			wantParameter: "--group-id",
		},
		{
			name:    "AttachRejectsAmbiguousSelectorBeforeMutation",
			command: WebReviewSubscriptionsAttachCommand,
			args:    []string{"--subscription-id", "Monthly"},
			body: func(t *testing.T) string {
				return premiumGroupBody(t,
					reviewSubscriptionFixture{ID: "sub-1", ProductID: "com.example.monthly.one", Name: "Monthly", State: "READY_TO_SUBMIT"},
					reviewSubscriptionFixture{ID: "sub-2", ProductID: "com.example.monthly.two", Name: "monthly", State: "READY_TO_SUBMIT"},
				)
			},
			wantMessage:   `2 subscriptions match "Monthly" by name; pass --subscription-id with one of:`,
			wantParameter: "--subscription-id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stubReviewSubscriptions(t, tt.body(t))

			_, _, err := runReviewSubscriptionsCommand(t, tt.command, append([]string{"--app", "app-1", "--confirm"}, tt.args...)...)
			if err == nil {
				t.Fatal("expected ambiguous selector error")
			}
			if !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("expected ambiguity diagnostic, got %v", err)
			}
			diagnostic, ok := shared.DiagnosticFromError(err)
			if !ok || diagnostic.Code != shared.DiagnosticInvalidInput || diagnostic.Parameter != tt.wantParameter {
				t.Fatalf("diagnostic = %+v, found=%t, want invalid_input for %s", diagnostic, ok, tt.wantParameter)
			}
			fake.assertRequests(t, reviewSubscriptionsListRequest)
		})
	}
}

func TestWebReviewSubscriptionsSingleMutationReportsRefreshedState(t *testing.T) {
	tests := []struct {
		name              string
		command           func() *ffcli.Command
		selector          string
		initialAttached   bool
		refreshedAttached bool
		wantOperation     string
		wantChanged       bool
		wantSubmissionID  string
		wantMutation      string
		wantLabels        []string
	}{
		{
			name:              "AttachCommandRefreshesState",
			command:           WebReviewSubscriptionsAttachCommand,
			selector:          "Monthly",
			refreshedAttached: true,
			wantOperation:     "attach",
			wantChanged:       true,
			wantSubmissionID:  "submission-1",
			wantMutation:      "POST sub-1",
			wantLabels:        []string{"Loading review subscriptions", "Attaching subscription to next app version", "Refreshing review subscriptions"},
		},
		{
			name:             "AttachCommandOnlyMarksChangedWhenRefreshShowsAttached",
			command:          WebReviewSubscriptionsAttachCommand,
			selector:         "sub-1",
			wantOperation:    "attach",
			wantSubmissionID: "submission-1",
			wantMutation:     "POST sub-1",
			wantLabels:       []string{"Loading review subscriptions", "Attaching subscription to next app version", "Refreshing review subscriptions"},
		},
		{
			name:            "RemoveCommandRefreshesState",
			command:         WebReviewSubscriptionsRemoveCommand,
			selector:        "sub-1",
			initialAttached: true,
			wantOperation:   "remove",
			wantChanged:     true,
			wantMutation:    "DELETE sub-1",
			wantLabels:      []string{"Loading review subscriptions", "Removing subscription from next app version", "Refreshing review subscriptions"},
		},
		{
			name:              "RemoveCommandOnlyMarksChangedWhenRefreshShowsDetached",
			command:           WebReviewSubscriptionsRemoveCommand,
			selector:          "sub-1",
			initialAttached:   true,
			refreshedAttached: true,
			wantOperation:     "remove",
			wantMutation:      "DELETE sub-1",
			wantLabels:        []string{"Loading review subscriptions", "Removing subscription from next app version", "Refreshing review subscriptions"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stubReviewSubscriptions(t,
				premiumGroupBody(t, monthlySubscription("READY_TO_SUBMIT", tt.initialAttached)),
				premiumGroupBody(t, monthlySubscription("READY_TO_SUBMIT", tt.refreshedAttached)),
			)

			stdout, _, err := runReviewSubscriptionsCommand(t, tt.command, "--app", "app-1", "--subscription-id", tt.selector, "--confirm", "--output", "json")
			if err != nil {
				t.Fatalf("exec error: %v", err)
			}

			var payload reviewSubscriptionMutationOutput
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
			}
			if payload.Operation != tt.wantOperation || payload.Changed != tt.wantChanged || payload.SubmissionID != tt.wantSubmissionID {
				t.Fatalf("unexpected %s output: %#v", tt.wantOperation, payload)
			}
			if payload.Subscription.SubmitWithNextAppStoreVersion != tt.refreshedAttached {
				t.Fatalf("expected refreshed subscription attached=%t, got %#v", tt.refreshedAttached, payload.Subscription)
			}
			fake.assertRequests(t, reviewSubscriptionsListRequest, tt.wantMutation, reviewSubscriptionsListRequest)
			fake.assertLabels(t, tt.wantLabels...)
		})
	}
}

func TestWebReviewSubscriptionsMutationsRequireConfirm(t *testing.T) {
	cases := []struct {
		name    string
		command func() *ffcli.Command
		args    []string
	}{
		{name: "attach", command: WebReviewSubscriptionsAttachCommand, args: []string{"--subscription-id", "sub-1"}},
		{name: "attach-group", command: WebReviewSubscriptionsAttachGroupCommand, args: []string{"--group-id", "group-1"}},
		{name: "remove-group", command: WebReviewSubscriptionsRemoveGroupCommand, args: []string{"--group-id", "group-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, err := runReviewSubscriptionsCommand(t, tc.command, append([]string{"--app", "app-1"}, tc.args...)...)
			if err == nil {
				t.Fatal("expected missing confirm error")
			}
			if !strings.Contains(stderr, "--confirm is required") {
				t.Fatalf("expected confirm guidance in stderr, got %q", stderr)
			}
		})
	}
}

func TestWebReviewSubscriptionsAttachPreflightFailsFast(t *testing.T) {
	tests := []struct {
		name         string
		command      func() *ffcli.Command
		args         []string
		body         func(t *testing.T) string
		wantStderr   []string
		rejectStderr []string
	}{
		{
			name:    "AttachFailsFastForMissingMetadata",
			command: WebReviewSubscriptionsAttachCommand,
			args:    []string{"--subscription-id", "sub-1"},
			body: func(t *testing.T) string {
				return premiumGroupBody(t, monthlySubscription("MISSING_METADATA", false))
			},
			wantStderr: []string{
				"is MISSING_METADATA",
				`asc validate subscriptions --app "app-1"`,
				`asc subscriptions versions list --subscription-id "sub-1"`,
				`asc subscriptions versions create --subscription-id "sub-1"`,
				`asc subscriptions versions images upload --version-id "VERSION_ID" --file "./image.png"`,
			},
			rejectStderr: []string{"asc subscriptions images create"},
		},
		{
			name:    "AttachFailsFastForNonReadyState",
			command: WebReviewSubscriptionsAttachCommand,
			args:    []string{"--subscription-id", "sub-1"},
			body: func(t *testing.T) string {
				return premiumGroupBody(t, monthlySubscription("DEVELOPER_ACTION_NEEDED", false))
			},
			wantStderr: []string{"DEVELOPER_ACTION_NEEDED", "READY_TO_SUBMIT"},
		},
		{
			name:    "AttachGroupFailsFastWhenNoSubscriptionsAreReadyToSubmit",
			command: WebReviewSubscriptionsAttachGroupCommand,
			args:    []string{"--group-id", "group-1"},
			body: func(t *testing.T) string {
				return premiumGroupBody(t,
					monthlySubscription("DEVELOPER_ACTION_NEEDED", false),
					reviewSubscriptionFixture{ID: "sub-2", ProductID: "com.example.annual", Name: "Annual", State: "APPROVED"},
				)
			},
			wantStderr: []string{"no READY_TO_SUBMIT subscriptions"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stubReviewSubscriptions(t, tt.body(t))

			_, stderr, err := runReviewSubscriptionsCommand(t, tt.command, append([]string{"--app", "app-1", "--confirm"}, tt.args...)...)
			if err == nil {
				t.Fatal("expected attach preflight error")
			}
			var reported shared.ReportedError
			if !errors.As(err, &reported) {
				t.Fatalf("expected ReportedError, got %T: %v", err, err)
			}
			fake.assertRequests(t, reviewSubscriptionsListRequest)
			for _, want := range tt.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected preflight stderr containing %q, got %q", want, stderr)
				}
			}
			for _, reject := range tt.rejectStderr {
				if strings.Contains(stderr, reject) {
					t.Fatalf("preflight stderr must not contain %q, got %q", reject, stderr)
				}
			}
			fake.assertLabels(t, "Loading review subscriptions")
		})
	}
}

func TestWebReviewSubscriptionsGroupMutationsReportRefreshedState(t *testing.T) {
	const (
		missingMetadataReason = "state is MISSING_METADATA; run `asc validate subscriptions` and upload missing assets first"
		attachLabel           = "Attaching subscription group to next app version"
		removeLabel           = "Removing subscription group from next app version"
	)
	tests := []struct {
		name          string
		command       func() *ffcli.Command
		groupSelector string
		initial       func(t *testing.T) string
		refreshed     func(t *testing.T) string
		wantOperation string
		wantChanged   []string
		wantSkipped   []string
		wantMutations []string
		wantLabels    []string
	}{
		{
			name:          "AttachGroupResolvesReferenceName",
			command:       WebReviewSubscriptionsAttachGroupCommand,
			groupSelector: "Premium",
			initial: func(t *testing.T) string {
				return premiumGroupBody(t, monthlySubscription("READY_TO_SUBMIT", false))
			},
			refreshed: func(t *testing.T) string {
				return premiumGroupBody(t, monthlySubscription("READY_TO_SUBMIT", true))
			},
			wantOperation: "attach-group",
			wantChanged:   []string{"sub-1"},
			wantMutations: []string{"POST sub-1"},
			wantLabels:    []string{"Loading review subscriptions", attachLabel, "Refreshing review subscriptions"},
		},
		{
			name:          "AttachGroupCommandRefreshesReadySubscriptions",
			command:       WebReviewSubscriptionsAttachGroupCommand,
			groupSelector: "group-1",
			initial:       func(t *testing.T) string { return premiumTrioBody(t, false, false) },
			refreshed:     func(t *testing.T) string { return premiumTrioBody(t, true, true) },
			wantOperation: "attach-group",
			wantChanged:   []string{"sub-1", "sub-2"},
			wantSkipped:   []string{missingMetadataReason},
			wantMutations: []string{"POST sub-1", "POST sub-2"},
			wantLabels:    []string{"Loading review subscriptions", attachLabel, "Refreshing review subscriptions"},
		},
		{
			name:          "AttachGroupCommandOnlyCountsRefreshedAttachedSubscriptions",
			command:       WebReviewSubscriptionsAttachGroupCommand,
			groupSelector: "group-1",
			initial:       func(t *testing.T) string { return premiumTrioBody(t, false, false) },
			refreshed:     func(t *testing.T) string { return premiumTrioBody(t, true, false) },
			wantOperation: "attach-group",
			wantChanged:   []string{"sub-1"},
			wantSkipped:   []string{missingMetadataReason, "attach request completed but refreshed state still shows not attached"},
			wantMutations: []string{"POST sub-1", "POST sub-2"},
			wantLabels:    []string{"Loading review subscriptions", attachLabel, "Refreshing review subscriptions"},
		},
		{
			name:          "AttachGroupCommandIsIdempotentAfterReadySubscriptionsAlreadyAttached",
			command:       WebReviewSubscriptionsAttachGroupCommand,
			groupSelector: "group-1",
			initial:       func(t *testing.T) string { return premiumTrioBody(t, true, true) },
			refreshed:     func(t *testing.T) string { return premiumTrioBody(t, true, true) },
			wantOperation: "attach-group",
			wantSkipped:   []string{"already attached", "already attached", missingMetadataReason},
			wantLabels:    []string{"Loading review subscriptions", "Refreshing review subscriptions"},
		},
		{
			name:          "RemoveGroupCommandRefreshesAttachedSubscriptions",
			command:       WebReviewSubscriptionsRemoveGroupCommand,
			groupSelector: "group-1",
			initial:       func(t *testing.T) string { return premiumTrioBody(t, true, true) },
			refreshed:     func(t *testing.T) string { return premiumTrioBody(t, false, false) },
			wantOperation: "remove-group",
			wantChanged:   []string{"sub-1", "sub-2"},
			wantSkipped:   []string{"not attached"},
			wantMutations: []string{"DELETE sub-1", "DELETE sub-2"},
			wantLabels:    []string{"Loading review subscriptions", removeLabel, "Refreshing review subscriptions"},
		},
		{
			name:          "RemoveGroupCommandOnlyCountsRefreshedDetachedSubscriptions",
			command:       WebReviewSubscriptionsRemoveGroupCommand,
			groupSelector: "group-1",
			initial:       func(t *testing.T) string { return premiumTrioBody(t, true, true) },
			refreshed:     func(t *testing.T) string { return premiumTrioBody(t, false, true) },
			wantOperation: "remove-group",
			wantChanged:   []string{"sub-1"},
			wantSkipped:   []string{"not attached", "remove request completed but refreshed state still shows attached"},
			wantMutations: []string{"DELETE sub-1", "DELETE sub-2"},
			wantLabels:    []string{"Loading review subscriptions", removeLabel, "Refreshing review subscriptions"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := stubReviewSubscriptions(t, tt.initial(t), tt.refreshed(t))

			stdout, _, err := runReviewSubscriptionsCommand(t, tt.command, "--app", "app-1", "--group-id", tt.groupSelector, "--confirm", "--output", "json")
			if err != nil {
				t.Fatalf("exec error: %v", err)
			}

			var payload reviewSubscriptionGroupMutationOutput
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
			}
			if payload.Operation != tt.wantOperation || payload.GroupID != "group-1" || payload.ChangedCount != len(tt.wantChanged) || payload.SkippedCount != len(tt.wantSkipped) {
				t.Fatalf("unexpected %s output: %#v", tt.wantOperation, payload)
			}
			wantAttached := tt.wantOperation == "attach-group"
			changedIDs := []string{}
			for _, changed := range payload.Changed {
				if changed.SubmitWithNextAppStoreVersion != wantAttached {
					t.Fatalf("expected changed subscriptions attached=%t, got %#v", wantAttached, payload.Changed)
				}
				changedIDs = append(changedIDs, changed.ID)
			}
			if !slices.Equal(changedIDs, tt.wantChanged) {
				t.Fatalf("changed subscriptions = %v, want %v", changedIDs, tt.wantChanged)
			}
			reasons := []string{}
			for _, skipped := range payload.Skipped {
				reasons = append(reasons, skipped.Reason)
			}
			if !slices.Equal(reasons, tt.wantSkipped) {
				t.Fatalf("skip reasons = %q, want %q", reasons, tt.wantSkipped)
			}
			wantRequests := append([]string{reviewSubscriptionsListRequest}, tt.wantMutations...)
			fake.assertRequests(t, append(wantRequests, reviewSubscriptionsListRequest)...)
			fake.assertLabels(t, tt.wantLabels...)
		})
	}
}

func TestCollectReviewSubscriptionGroupChangesMarksNotFoundAfterRefresh(t *testing.T) {
	refreshedGroup := []webcore.ReviewSubscription{
		{ID: "sub-1", SubmitWithNextAppStoreVersion: true, SubmitWithNextAppStoreVersionKnown: true},
	}

	changed, skipped := collectReviewSubscriptionGroupChanges(
		refreshedGroup,
		[]string{" sub-1 ", "sub-missing"},
		true,
		reviewSubscriptionAttachUnchangedAfterRefreshReason(),
	)

	if len(changed) != 1 || changed[0].ID != "sub-1" {
		t.Fatalf("expected sub-1 to be marked changed, got %#v", changed)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected one skipped subscription, got %#v", skipped)
	}
	if skipped[0].Subscription.ID != "sub-missing" {
		t.Fatalf("expected missing subscription id in skip output, got %#v", skipped[0])
	}
	if skipped[0].Reason != "subscription was not found after refresh" {
		t.Fatalf("unexpected skip reason: %#v", skipped[0])
	}
}

func TestReviewSubscriptionChangedAfterRefreshRequiresKnownState(t *testing.T) {
	tests := []struct {
		name         string
		subscription webcore.ReviewSubscription
		wantAttached bool
		wantChanged  bool
	}{
		{name: "known attached", subscription: webcore.ReviewSubscription{SubmitWithNextAppStoreVersionKnown: true, SubmitWithNextAppStoreVersion: true}, wantAttached: true, wantChanged: true},
		{name: "known detached", subscription: webcore.ReviewSubscription{SubmitWithNextAppStoreVersionKnown: true}, wantAttached: false, wantChanged: true},
		{name: "unknown cannot prove attach", subscription: webcore.ReviewSubscription{}, wantAttached: true},
		{name: "unknown cannot prove remove", subscription: webcore.ReviewSubscription{}, wantAttached: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reviewSubscriptionChangedAfterRefresh(test.subscription, test.wantAttached); got != test.wantChanged {
				t.Fatalf("changed = %t, want %t", got, test.wantChanged)
			}
		})
	}
}

func TestCollectReviewSubscriptionGroupChangesClassifiesUnknownRefreshedState(t *testing.T) {
	changed, skipped := collectReviewSubscriptionGroupChanges(
		[]webcore.ReviewSubscription{{ID: "sub-1"}},
		[]string{"sub-1"},
		false,
		reviewSubscriptionRemoveUnchangedAfterRefreshReason(),
	)

	if len(changed) != 0 || len(skipped) != 1 {
		t.Fatalf("changed = %#v, skipped = %#v", changed, skipped)
	}
	if skipped[0].Reason != "next-version attachment state is unknown after refresh" {
		t.Fatalf("skip reason = %q", skipped[0].Reason)
	}
}

func TestBuildReviewSubscriptionGroupMutationRowsIncludeChangedAndSkippedDetails(t *testing.T) {
	payload := reviewSubscriptionGroupMutationOutput{
		AppID:        "app-1",
		GroupID:      "group-1",
		Operation:    "attach-group",
		ChangedCount: 2,
		SkippedCount: 1,
		Changed: []webcore.ReviewSubscription{
			{ID: "sub-1", ProductID: "com.example.monthly", State: "READY_TO_SUBMIT", SubmitWithNextAppStoreVersion: true},
			{ID: "sub-2", Name: "   ", ProductID: "   ", State: " ", SubmitWithNextAppStoreVersion: false},
		},
		Skipped: []reviewSubscriptionMutationSkip{
			{
				Subscription: webcore.ReviewSubscription{ID: "sub-3", Name: "Legacy"},
				Reason:       "state is MISSING_METADATA",
			},
		},
	}

	rows := buildReviewSubscriptionGroupMutationRows(payload)
	if len(rows) != 9 {
		t.Fatalf("expected 6 summary + 2 changed + 1 skipped rows, got %#v", rows)
	}
	if rows[4][2] != "2" || rows[5][2] != "1" {
		t.Fatalf("expected changed/skipped counts in summary rows, got %#v", rows[4:6])
	}
	if !strings.Contains(rows[6][2], "id=sub-1") || !strings.Contains(rows[6][2], "name=com.example.monthly") || !strings.Contains(rows[6][2], "attached=true") {
		t.Fatalf("expected changed row to include product-id fallback and attached state, got %#v", rows[6])
	}
	if !strings.Contains(rows[7][2], "id=sub-2") || !strings.Contains(rows[7][2], "name=sub-2") || !strings.Contains(rows[7][2], "state=n/a") {
		t.Fatalf("expected changed row to include subscription-id fallback and n/a state, got %#v", rows[7])
	}
	if !strings.Contains(rows[8][2], "id=sub-3") || !strings.Contains(rows[8][2], "name=Legacy") || !strings.Contains(rows[8][2], "reason=state is MISSING_METADATA") {
		t.Fatalf("expected skipped row details, got %#v", rows[8])
	}
}

func TestBuildReviewSubscriptionMutationRowsFallbacks(t *testing.T) {
	rows := buildReviewSubscriptionMutationRows(reviewSubscriptionMutationOutput{
		AppID:        "app-1",
		Operation:    "attach",
		Changed:      false,
		SubmissionID: "   ",
		Subscription: webcore.ReviewSubscription{
			ID:                                 "sub-1",
			ProductID:                          "   ",
			Name:                               "",
			GroupReferenceName:                 "",
			State:                              "",
			SubmitWithNextAppStoreVersion:      false,
			SubmitWithNextAppStoreVersionKnown: true,
			IsAppStoreReviewInProgress:         false,
		},
	})

	if len(rows) != 11 {
		t.Fatalf("expected 11 mutation rows, got %#v", rows)
	}
	if rows[3][2] != "n/a" {
		t.Fatalf("expected empty submission id to render as n/a, got %#v", rows[3])
	}
	if rows[6][2] != "sub-1" {
		t.Fatalf("expected subscription name fallback to subscription id, got %#v", rows[6])
	}
	if rows[9][2] != "false" || rows[10][2] != "false" {
		t.Fatalf("expected boolean fields to render false values, got %#v %#v", rows[9], rows[10])
	}
}

func TestWebReviewSubscriptionsListCommandAppSelection(t *testing.T) {
	tests := []struct {
		name        string
		envAppID    string
		args        []string
		wantRequest string
		wantAppID   string
	}{
		{name: "ResolvesAppFromEnv", envAppID: "123", wantRequest: "GET /iris/v1/apps/123/subscriptionGroups", wantAppID: "123"},
		{name: "ExplicitFlagWinsOverEnv", envAppID: "999", args: []string{"--app", "app-1"}, wantRequest: reviewSubscriptionsListRequest, wantAppID: "app-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", tt.envAppID)
			fake := stubReviewSubscriptions(t, `{"data": [], "included": []}`)

			stdout, _, err := runReviewSubscriptionsCommand(t, WebReviewSubscriptionsListCommand, append(tt.args, "--output", "json")...)
			if err != nil {
				t.Fatalf("exec error: %v", err)
			}
			fake.assertRequests(t, tt.wantRequest)

			var payload reviewSubscriptionsListOutput
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("failed to parse stdout JSON: %v\nstdout=%s", err, stdout)
			}
			if payload.AppID != tt.wantAppID {
				t.Fatalf("expected payload app id %s, got %#v", tt.wantAppID, payload)
			}
		})
	}
}

func TestWebReviewSubscriptionsListCommandMissingAppReportsFallbackAndDiagnostic(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	origResolveSession := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = origResolveSession })
	sessionCalls := 0
	resolveSessionFn = func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		sessionCalls++
		return nil, "", errors.New("session resolution must not run")
	}

	cmd := WebReviewSubscriptionsListCommand()
	if err := cmd.FlagSet.Parse(nil); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		runErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("error = %v, want flag.ErrHelp", runErr)
	}
	if runErr.Error() != "--app is required (or set ASC_APP_ID)" {
		t.Fatalf("error = %q, want exact missing-app message", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "Error: --app is required (or set ASC_APP_ID)\n" {
		t.Fatalf("stderr = %q, want exact missing-app diagnostic", stderr)
	}
	if sessionCalls != 0 {
		t.Fatalf("session resolver called %d time(s), want 0", sessionCalls)
	}

	diagnostic, ok := shared.DiagnosticFromError(runErr)
	if !ok {
		t.Fatalf("expected structured diagnostic, got %v", runErr)
	}
	if diagnostic.Code != shared.DiagnosticRequiredInputMissing || diagnostic.Parameter != "--app" {
		t.Fatalf("diagnostic = %+v, want required_input_missing for --app", diagnostic)
	}
}

func TestReviewSubscriptionGroupLabelFallsBackToGroupID(t *testing.T) {
	name := reviewSubscriptionGroupLabel(
		[]webcore.ReviewSubscription{
			{GroupReferenceName: "   "},
			{GroupReferenceName: "Premium"},
		},
		"group-1",
	)
	if name != "Premium" {
		t.Fatalf("expected first non-empty group label, got %q", name)
	}

	fallback := reviewSubscriptionGroupLabel(
		[]webcore.ReviewSubscription{
			{GroupReferenceName: "   "},
		},
		"  group-2  ",
	)
	if fallback != "group-2" {
		t.Fatalf("expected fallback group id when names missing, got %q", fallback)
	}
}

type reviewSubscriptionFixture struct {
	ID        string
	ProductID string
	Name      string
	State     string
	Attached  bool
}

type reviewSubscriptionGroupFixture struct {
	ID            string
	ReferenceName string
	Subscriptions []reviewSubscriptionFixture
}

func monthlySubscription(state string, attached bool) reviewSubscriptionFixture {
	return reviewSubscriptionFixture{ID: "sub-1", ProductID: "com.example.monthly", Name: "Monthly", State: state, Attached: attached}
}

func premiumGroupBody(t *testing.T, subscriptions ...reviewSubscriptionFixture) string {
	t.Helper()
	return reviewSubscriptionGroupsBody(t, reviewSubscriptionGroupFixture{ID: "group-1", ReferenceName: "Premium", Subscriptions: subscriptions})
}

func premiumTrioBody(t *testing.T, monthlyAttached, annualAttached bool) string {
	t.Helper()
	return premiumGroupBody(t,
		monthlySubscription("READY_TO_SUBMIT", monthlyAttached),
		reviewSubscriptionFixture{ID: "sub-2", ProductID: "com.example.annual", Name: "Annual", State: "READY_TO_SUBMIT", Attached: annualAttached},
		reviewSubscriptionFixture{ID: "sub-3", ProductID: "com.example.legacy", Name: "Legacy", State: "MISSING_METADATA"},
	)
}

func reviewSubscriptionGroupsBody(t *testing.T, groups ...reviewSubscriptionGroupFixture) string {
	t.Helper()
	data := []any{}
	included := []any{}
	for _, group := range groups {
		refs := []any{}
		for _, subscription := range group.Subscriptions {
			refs = append(refs, map[string]string{"type": "subscriptions", "id": subscription.ID})
			attributes := map[string]any{
				"state":                         subscription.State,
				"isAppStoreReviewInProgress":    false,
				"submitWithNextAppStoreVersion": subscription.Attached,
			}
			if subscription.ProductID != "" {
				attributes["productId"] = subscription.ProductID
			}
			if subscription.Name != "" {
				attributes["name"] = subscription.Name
			}
			included = append(included, map[string]any{"id": subscription.ID, "type": "subscriptions", "attributes": attributes})
		}
		data = append(data, map[string]any{
			"id":            group.ID,
			"type":          "subscriptionGroups",
			"attributes":    map[string]string{"referenceName": group.ReferenceName},
			"relationships": map[string]any{"subscriptions": map[string]any{"data": refs}},
		})
	}
	body, err := json.Marshal(map[string]any{"data": data, "included": included})
	if err != nil {
		t.Fatalf("marshal subscription groups fixture: %v", err)
	}
	return string(body)
}

type reviewSubscriptionsFake struct {
	requests []string
	labels   *[]string
}

// stubReviewSubscriptions answers the nth subscription group list with
// listBodies[n], repeating the last body, and records each request.
func stubReviewSubscriptions(t *testing.T, listBodies ...string) *reviewSubscriptionsFake {
	t.Helper()
	fixture := handlertest.New(t)
	fake := &reviewSubscriptionsFake{labels: stubWebProgressLabels(t)}
	lists := 0
	stubWebSessionTransport(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/iris/v1/apps/") && strings.HasSuffix(req.URL.Path, "/subscriptionGroups"):
			fake.requests = append(fake.requests, "GET "+req.URL.Path)
			body := listBodies[min(lists, len(listBodies)-1)]
			lists++
			return webJSONResponse(req, http.StatusOK, body), nil
		case req.Method == http.MethodPost && req.URL.Path == "/iris/v1/subscriptionSubmissions":
			var payload struct {
				Data struct {
					Relationships struct {
						Subscription struct {
							Data struct {
								ID string `json:"id"`
							} `json:"data"`
						} `json:"subscription"`
					} `json:"relationships"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				return nil, fixture.Errorf("decode attach request: %w", err)
			}
			id := payload.Data.Relationships.Subscription.Data.ID
			fake.requests = append(fake.requests, "POST "+id)
			body := `{"data":{"id":"submission-1","type":"subscriptionSubmissions","attributes":{"submitWithNextAppStoreVersion":true},"relationships":{"subscription":{"data":{"type":"subscriptions","id":"` + id + `"}}}}}`
			return webJSONResponse(req, http.StatusCreated, body), nil
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/iris/v1/subscriptionSubmissions/"):
			fake.requests = append(fake.requests, "DELETE "+strings.TrimPrefix(req.URL.Path, "/iris/v1/subscriptionSubmissions/"))
			return webJSONResponse(req, http.StatusNoContent, ""), nil
		default:
			return nil, fixture.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	return fake
}

func (f *reviewSubscriptionsFake) assertRequests(t *testing.T, want ...string) {
	t.Helper()
	if !slices.Equal(f.requests, want) {
		t.Fatalf("requests = %q, want %q", f.requests, want)
	}
}

func (f *reviewSubscriptionsFake) assertLabels(t *testing.T, want ...string) {
	t.Helper()
	if !slices.Equal(*f.labels, want) {
		t.Fatalf("expected labels %v, got %v", want, *f.labels)
	}
}

func runReviewSubscriptionsCommand(t *testing.T, command func() *ffcli.Command, args ...string) (string, string, error) {
	t.Helper()
	cmd := command()
	if err := cmd.FlagSet.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	var err error
	stdout, stderr := captureOutput(t, func() { err = cmd.Exec(context.Background(), nil) })
	return stdout, stderr, err
}
