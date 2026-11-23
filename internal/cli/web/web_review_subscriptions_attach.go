package web

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// The attach commands add a subscription's version to the app's draft review
// submission through the App Store Connect API, the same request as
// `asc review items add --item-type subscriptionVersions`. They need the API
// key, not a web session.

// reviewSubscriptionSubmittableVersionStates are the subscription version
// states App Store Connect accepts as a review submission item.
var reviewSubscriptionSubmittableVersionStates = []asc.SubscriptionVersionState{
	asc.SubscriptionVersionStatePrepareForSubmission,
	asc.SubscriptionVersionStateReadyForReview,
	asc.SubscriptionVersionStateRejected,
	asc.SubscriptionVersionStateDeveloperRejected,
}

// reviewSubscriptionAttachWebFlags are the web session flags the attach
// commands still accept from before they used the API key. Setting one only
// prints a deprecation warning.
var reviewSubscriptionAttachWebFlags = []string{
	"apple-id", "two-factor-code-command", "two-factor-method", "two-factor-phone-id", "provider-id", "public-provider-id",
}

// bindDeprecatedReviewSubscriptionAttachWebFlags keeps the web session flags
// accepted, with help that says they have no effect.
func bindDeprecatedReviewSubscriptionAttachWebFlags(fs *flag.FlagSet) {
	bindWebSessionFlags(fs)
	for _, name := range reviewSubscriptionAttachWebFlags {
		if f := fs.Lookup(name); f != nil {
			f.Usage = "Deprecated: has no effect; this command uses the App Store Connect API key"
		}
	}
}

func warnDeprecatedReviewSubscriptionAttachWebFlags(d *shared.Deps, fs *flag.FlagSet, operation string) {
	fs.Visit(func(f *flag.Flag) {
		if slices.Contains(reviewSubscriptionAttachWebFlags, f.Name) {
			fmt.Fprintf(d.IO.ErrOut, "Warning: --%s is deprecated for `asc web review subscriptions %s` and has no effect: the command uses the App Store Connect API key, not a web session. Drop the flag; it will be removed in a future release.\n", f.Name, operation)
		}
	})
}

// reviewSubscriptionDraftItem is a subscription version already on a draft.
type reviewSubscriptionDraftItem struct {
	SubmissionID string
	ItemID       string
}

// reviewSubscriptionDrafts holds the app's READY_FOR_REVIEW review
// submissions and the subscription versions already on them.
type reviewSubscriptionDrafts struct {
	IDs       []string
	Platforms map[string]asc.Platform
	Items     map[string]reviewSubscriptionDraftItem
}

// loadAPIReviewSubscriptions lists every subscription of the app with its
// group, in the shape the web review commands print.
func loadAPIReviewSubscriptions(ctx context.Context, client *asc.Client, appID string) ([]webcore.ReviewSubscription, error) {
	firstGroups, err := client.GetSubscriptionGroups(ctx, appID, asc.Limit[asc.SubscriptionGroupsOption](200))
	if err != nil {
		return nil, fmt.Errorf("list subscription groups: %w", err)
	}
	subscriptions := []webcore.ReviewSubscription{}
	err = asc.PaginateEach(ctx, firstGroups, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		return client.GetSubscriptionGroups(ctx, appID, asc.NextURL[asc.SubscriptionGroupsOption](next))
	}, func(page asc.PaginatedResponse) error {
		groups, ok := page.(*asc.SubscriptionGroupsResponse)
		if !ok {
			return fmt.Errorf("unexpected subscription groups page type %T", page)
		}
		for _, group := range groups.Data {
			groupID := strings.TrimSpace(group.ID)
			firstSubs, err := client.GetSubscriptions(ctx, groupID, asc.Limit[asc.SubscriptionsOption](200))
			if err != nil {
				return fmt.Errorf("list subscriptions in group %q: %w", groupID, err)
			}
			err = asc.PaginateEach(ctx, firstSubs, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
				return client.GetSubscriptions(ctx, groupID, asc.NextURL[asc.SubscriptionsOption](next))
			}, func(page asc.PaginatedResponse) error {
				subs, ok := page.(*asc.SubscriptionsResponse)
				if !ok {
					return fmt.Errorf("unexpected subscriptions page type %T", page)
				}
				for _, sub := range subs.Data {
					subscriptions = append(subscriptions, webcore.ReviewSubscription{
						ID:                 strings.TrimSpace(sub.ID),
						GroupID:            groupID,
						GroupReferenceName: strings.TrimSpace(group.Attributes.ReferenceName),
						ProductID:          strings.TrimSpace(sub.Attributes.ProductID),
						Name:               strings.TrimSpace(sub.Attributes.Name),
						State:              strings.TrimSpace(sub.Attributes.State),
					})
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("list subscriptions in group %q: %w", groupID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list subscription groups: %w", err)
	}
	return subscriptions, nil
}

// resolveReviewSubscriptionVersion returns the subscription's one version that
// can go on a review submission, or "" and the states it saw when there is none.
func resolveReviewSubscriptionVersion(ctx context.Context, client *asc.Client, subscriptionID string) (string, []string, error) {
	versions, err := client.GetSubscriptionVersions(ctx, subscriptionID, asc.Limit[asc.SubscriptionVersionsOption](200))
	if err != nil {
		return "", nil, fmt.Errorf("list versions of subscription %q: %w", subscriptionID, err)
	}
	selected := ""
	states := []string{}
	for _, version := range versions.Data {
		states = append(states, string(version.Attributes.State))
		if !slices.Contains(reviewSubscriptionSubmittableVersionStates, version.Attributes.State) {
			continue
		}
		if selected != "" && selected != version.ID {
			return "", nil, fmt.Errorf("subscription %q has more than one version that can be submitted; refusing to attach", subscriptionID)
		}
		selected = strings.TrimSpace(version.ID)
	}
	return selected, states, nil
}

// loadReviewSubscriptionDrafts lists the app's draft review submissions and
// the subscription versions already on them.
func loadReviewSubscriptionDrafts(ctx context.Context, client *asc.Client, appID string) (reviewSubscriptionDrafts, error) {
	drafts := reviewSubscriptionDrafts{Platforms: map[string]asc.Platform{}, Items: map[string]reviewSubscriptionDraftItem{}}
	submissions, err := shared.FetchAllReviewSubmissions(ctx, client, appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)}),
		asc.Limit[asc.ReviewSubmissionsOption](200),
	)
	if err != nil {
		return drafts, fmt.Errorf("list draft review submissions: %w", err)
	}
	for _, submission := range submissions {
		submissionID := strings.TrimSpace(submission.ID)
		if submissionID == "" || submission.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		drafts.IDs = append(drafts.IDs, submissionID)
		drafts.Platforms[submissionID] = submission.Attributes.Platform
		if err := collectReviewSubscriptionDraftItems(ctx, client, submissionID, drafts.Items); err != nil {
			return drafts, err
		}
	}
	return drafts, nil
}

func collectReviewSubscriptionDraftItems(ctx context.Context, client *asc.Client, submissionID string, into map[string]reviewSubscriptionDraftItem) error {
	first, err := client.GetReviewSubmissionItems(ctx, submissionID,
		asc.WithReviewSubmissionItemsInclude([]string{"subscriptionVersion"}),
		asc.Limit[asc.ReviewSubmissionItemsOption](200),
	)
	if err != nil {
		return fmt.Errorf("list items of review submission %q: %w", submissionID, err)
	}
	err = asc.PaginateEach(ctx, first, func(ctx context.Context, next string) (asc.PaginatedResponse, error) {
		return client.GetReviewSubmissionItems(ctx, submissionID, asc.NextURL[asc.ReviewSubmissionItemsOption](next))
	}, func(page asc.PaginatedResponse) error {
		items, ok := page.(*asc.ReviewSubmissionItemsResponse)
		if !ok {
			return fmt.Errorf("unexpected review submission items page type %T", page)
		}
		for _, item := range items.Data {
			if strings.EqualFold(strings.TrimSpace(item.Attributes.State), "REMOVED") || item.Relationships == nil || item.Relationships.SubscriptionVersion == nil {
				continue
			}
			if versionID := strings.TrimSpace(item.Relationships.SubscriptionVersion.Data.ID); versionID != "" {
				into[versionID] = reviewSubscriptionDraftItem{SubmissionID: submissionID, ItemID: strings.TrimSpace(item.ID)}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("list items of review submission %q: %w", submissionID, err)
	}
	return nil
}

// reviewSubscriptionDraftChoice is the draft items are added to, and whether
// this run created it.
type reviewSubscriptionDraftChoice struct {
	ID      string
	Created bool
}

// selectReviewSubscriptionDraft picks the draft to add items to: --submission
// when given, the only draft (on --platform when it is set), or a new one for
// --platform when there is none.
func selectReviewSubscriptionDraft(ctx context.Context, d *shared.Deps, client *asc.Client, appID string, drafts *reviewSubscriptionDrafts, opts reviewSubscriptionDraftFlags) (reviewSubscriptionDraftChoice, error) {
	if opts.SubmissionID != "" {
		if !slices.Contains(drafts.IDs, opts.SubmissionID) {
			return reviewSubscriptionDraftChoice{}, d.UsageError("--submission must identify a READY_FOR_REVIEW review submission of --app")
		}
		return reviewSubscriptionDraftChoice{ID: opts.SubmissionID}, nil
	}
	candidates := drafts.IDs
	if opts.PlatformSet && len(drafts.IDs) > 0 {
		candidates = slices.DeleteFunc(slices.Clone(drafts.IDs), func(id string) bool { return drafts.Platforms[id] != opts.Platform })
		if len(candidates) == 0 {
			listed := make([]string, 0, len(drafts.IDs))
			for _, id := range drafts.IDs {
				listed = append(listed, fmt.Sprintf("%s (%s)", id, drafts.Platforms[id]))
			}
			return reviewSubscriptionDraftChoice{}, d.UsageErrorf("--platform %s does not match the app's READY_FOR_REVIEW review submission %s; drop --platform or pick one with --submission", opts.Platform, strings.Join(listed, ", "))
		}
	}
	switch len(candidates) {
	case 1:
		return reviewSubscriptionDraftChoice{ID: candidates[0]}, nil
	case 0:
		created, err := client.CreateReviewSubmission(ctx, appID, opts.Platform)
		if err != nil {
			return reviewSubscriptionDraftChoice{}, fmt.Errorf("create review submission for app %q: %w", appID, err)
		}
		createdID := strings.TrimSpace(created.Data.ID)
		if createdID == "" {
			return reviewSubscriptionDraftChoice{}, fmt.Errorf("create review submission for app %q: response has no id", appID)
		}
		drafts.IDs = append(drafts.IDs, createdID)
		drafts.Platforms[createdID] = opts.Platform
		fmt.Fprintf(d.IO.ErrOut, "Created review submission %s (%s) for app %s; it is a draft and is not submitted for review.\n", createdID, opts.Platform, appID)
		return reviewSubscriptionDraftChoice{ID: createdID, Created: true}, nil
	default:
		return reviewSubscriptionDraftChoice{}, d.UsageErrorf("multiple READY_FOR_REVIEW review submissions; select one with --submission: %s", strings.Join(candidates, ", "))
	}
}

// withCreatedDraftCleanup cancels a draft this run created when adding its
// first item failed, so no empty review submission is left behind. Only that
// draft is touched; canceling never submits anything.
func withCreatedDraftCleanup(ctx context.Context, client *asc.Client, draft reviewSubscriptionDraftChoice, addErr error) error {
	if !draft.Created {
		return addErr
	}
	if _, err := client.CancelReviewSubmission(ctx, draft.ID); err != nil {
		return fmt.Errorf("%w (review submission %s, which this run created, is still an empty draft; cancel it with `asc review submissions-cancel --id %q --confirm`: %w)", addErr, draft.ID, draft.ID, err)
	}
	return fmt.Errorf("%w (canceled review submission %s, which this run created)", addErr, draft.ID)
}

// reviewSubscriptionNotReadyError marks an attach that cannot proceed until
// the subscription changes, as the web flow's preflight did.
func reviewSubscriptionNotReadyError(err error, parameter string) error {
	return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, parameter)
}

func reviewSubscriptionMissingMetadataError(appID, subscriptionID string) error {
	return &shared.HintedError{
		Err:  reviewSubscriptionNotReadyError(fmt.Errorf("web review subscriptions attach: subscription %q is MISSING_METADATA, so App Store Connect will not accept it on a review submission; nothing was sent", subscriptionID), "--subscription-id"),
		Hint: fmt.Sprintf("Run `asc validate subscriptions --app %q` to see what is missing, then retry.", appID),
	}
}

// reviewSubscriptionAPIClient returns the API client, and tells a caller who
// only has the web session these commands used before what changed.
func reviewSubscriptionAPIClient(d *shared.Deps, operation string) (*asc.Client, error) {
	client, err := d.Client()
	if err != nil {
		return nil, &shared.HintedError{
			Err:  fmt.Errorf("web review subscriptions %s: %w", operation, err),
			Hint: "`asc web review subscriptions " + operation + "` now uses the App Store Connect API key, not the web session. Set up a key with `asc auth login`, then retry.",
		}
	}
	return client, nil
}

func reviewSubscriptionNoVersionError(operation, subscriptionID string, states []string) error {
	seen := "none"
	if len(states) > 0 {
		seen = strings.Join(states, ", ")
	}
	return &shared.HintedError{
		Err: reviewSubscriptionNotReadyError(fmt.Errorf("web review subscriptions %s: subscription %q has no version that can be added to a review submission (version states: %s)", operation, subscriptionID, seen), "--subscription-id"),
		Hint: fmt.Sprintf(
			"A version in WAITING_FOR_REVIEW or IN_REVIEW is already submitted. Create a new one with `asc subscriptions versions create --subscription-id %q`, then retry.",
			subscriptionID,
		),
	}
}

// reviewSubscriptionDraftFlags are the validated --submission and --platform.
type reviewSubscriptionDraftFlags struct {
	SubmissionID string
	Platform     asc.Platform
	PlatformSet  bool
}

func bindReviewSubscriptionAttachFlags(fs *flag.FlagSet) (submissionID, platform *string) {
	submissionID = fs.String("submission", "", "Review submission ID (required when the app has several READY_FOR_REVIEW review submissions)")
	platform = fs.String("platform", "IOS", "Platform of the review submission: picks the app's draft on that platform, or creates one when the app has none: IOS, MAC_OS, TV_OS, VISION_OS")
	return submissionID, platform
}

func validateReviewSubscriptionAttachDraftFlags(d *shared.Deps, fs *flag.FlagSet, submissionID, platform string) (reviewSubscriptionDraftFlags, error) {
	trimmedSubmissionID := strings.TrimSpace(submissionID)
	if submissionID != "" && trimmedSubmissionID == "" {
		return reviewSubscriptionDraftFlags{}, d.UsageError("--submission must not be blank")
	}
	normalized, err := shared.NormalizeAppStoreVersionPlatform(platform)
	if err != nil {
		return reviewSubscriptionDraftFlags{}, shared.WithDiagnostic(d.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--platform")
	}
	platformSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "platform" {
			platformSet = true
		}
	})
	if platformSet && trimmedSubmissionID != "" {
		return reviewSubscriptionDraftFlags{}, d.UsageError("--platform only applies when a new review submission is created; drop it when --submission is set")
	}
	return reviewSubscriptionDraftFlags{SubmissionID: trimmedSubmissionID, Platform: asc.Platform(normalized), PlatformSet: platformSet}, nil
}

// newWebReviewSubscriptionsAttachCommand adds a subscription's version to the app's review submission.
func newWebReviewSubscriptionsAttachCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web review subscriptions attach", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	submissionID, platform := bindReviewSubscriptionAttachFlags(fs)
	confirm := fs.Bool("confirm", false, "Confirm the attach operation")
	bindDeprecatedReviewSubscriptionAttachWebFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach",
		ShortUsage: "asc web review subscriptions attach --app APP_ID --subscription-id SUB_ID [--submission SUBMISSION_ID] [--platform PLATFORM] --confirm [flags]",
		ShortHelp:  "Add a subscription to the app's review submission.",
		LongHelp: `Add a subscription's version to the app's draft (READY_FOR_REVIEW) review
submission through the App Store Connect API, like
` + "`asc review items add --item-type subscriptionVersions`" + `. It uses the API key,
not a web session. It stages the subscription for review; it does not submit
the review submission.

When the app has several draft review submissions, choose one with --submission.
When it has none, one is created for --platform. A subscription version already
on a draft is reported with changed=false and nothing is sent.

The web session flags are deprecated here and have no effect.

Examples:
  asc web review subscriptions attach --app "123456789" --subscription-id "SUB_ID" --confirm
  asc web review subscriptions attach --app "123456789" --subscription-id "com.example.monthly" --submission "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			trimmedSubscriptionID := strings.TrimSpace(*subscriptionID)
			switch {
			case trimmedAppID == "":
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			case trimmedSubscriptionID == "":
				return d.UsageError("--subscription-id is required")
			}
			draftFlags, err := validateReviewSubscriptionAttachDraftFlags(d, fs, *submissionID, *platform)
			if err != nil {
				return err
			}
			if !*confirm {
				return d.RequireConfirm(*confirm)
			}
			warnDeprecatedReviewSubscriptionAttachWebFlags(d, fs, "attach")

			client, err := reviewSubscriptionAPIClient(d, "attach")
			if err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			subscriptions, err := withWebSpinnerValueWith(d, "Loading subscriptions", func() ([]webcore.ReviewSubscription, error) {
				return loadAPIReviewSubscriptions(requestCtx, client, trimmedAppID)
			})
			if err != nil {
				return fmt.Errorf("web review subscriptions attach: %w", err)
			}
			selected, err := findReviewSubscription(subscriptions, trimmedSubscriptionID)
			if err != nil {
				return fmt.Errorf("subscription lookup for app %q failed: %w", trimmedAppID, withReviewSelectorDiagnostic(err, "--subscription-id"))
			}
			subscription := *selected
			if strings.EqualFold(subscription.State, "MISSING_METADATA") {
				return reviewSubscriptionMissingMetadataError(trimmedAppID, subscription.ID)
			}

			versionID, states, err := resolveReviewSubscriptionVersion(requestCtx, client, subscription.ID)
			if err != nil {
				return fmt.Errorf("web review subscriptions attach: %w", err)
			}
			if versionID == "" {
				return reviewSubscriptionNoVersionError("attach", subscription.ID, states)
			}

			drafts, err := loadReviewSubscriptionDrafts(requestCtx, client, trimmedAppID)
			if err != nil {
				return fmt.Errorf("web review subscriptions attach: %w", err)
			}
			payload := reviewSubscriptionMutationOutput{
				AppID:                 trimmedAppID,
				Operation:             "attach",
				SubscriptionVersionID: versionID,
			}
			existing, attached := drafts.Items[versionID]
			switch {
			case attached && draftFlags.SubmissionID != "" && existing.SubmissionID != draftFlags.SubmissionID:
				return d.UsageErrorf("subscription %q is already on review submission %s; drop --submission or remove it there first with `asc review items remove --id %q --confirm`", subscription.ID, existing.SubmissionID, existing.ItemID)
			case attached:
				payload.SubmissionID = existing.SubmissionID
				payload.ReviewSubmissionItemID = existing.ItemID
			default:
				draft, err := selectReviewSubscriptionDraft(requestCtx, d, client, trimmedAppID, &drafts, draftFlags)
				if err != nil {
					return err
				}
				itemID, err := withWebSpinnerValueWith(d, "Adding subscription to review submission", func() (string, error) {
					return createReviewSubscriptionItem(requestCtx, client, draft.ID, versionID)
				})
				if err != nil {
					return fmt.Errorf("web review subscriptions attach: %w", withCreatedDraftCleanup(requestCtx, client, draft, err))
				}
				payload.Changed = true
				payload.SubmissionID = draft.ID
				payload.ReviewSubmissionItemID = itemID
			}
			subscription.SubmitWithNextAppStoreVersion = true
			subscription.SubmitWithNextAppStoreVersionKnown = true
			payload.Subscription = subscription

			return d.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionMutationTable(d, payload) },
				func() error { return renderReviewSubscriptionMutationMarkdown(d, payload) },
			)
		},
	}
}

// createReviewSubscriptionItem adds a subscription version to a draft review
// submission and returns the new review item ID.
func createReviewSubscriptionItem(ctx context.Context, client *asc.Client, submissionID, versionID string) (string, error) {
	created, err := client.CreateReviewSubmissionItem(ctx, submissionID, asc.ReviewSubmissionItemTypeSubscriptionVersion, versionID)
	if err != nil {
		return "", fmt.Errorf("add subscription version %q to review submission %q: %w", versionID, submissionID, err)
	}
	itemID := strings.TrimSpace(created.Data.ID)
	if itemID == "" {
		return "", fmt.Errorf("add subscription version %q to review submission %q: response has no id", versionID, submissionID)
	}
	return itemID, nil
}

// newWebReviewSubscriptionsAttachGroupCommand adds every subscription of one group to the app's review submission.
func newWebReviewSubscriptionsAttachGroupCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("web review subscriptions attach-group", flag.ExitOnError)

	appID := fs.String("app", "", "App ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Subscription group ID or exact current name")
	submissionID, platform := bindReviewSubscriptionAttachFlags(fs)
	confirm := fs.Bool("confirm", false, "Confirm the attach-group operation")
	bindDeprecatedReviewSubscriptionAttachWebFlags(fs)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach-group",
		ShortUsage: "asc web review subscriptions attach-group --app APP_ID --group-id GROUP_ID_OR_NAME [--submission SUBMISSION_ID] [--platform PLATFORM] --confirm [flags]",
		ShortHelp:  "Add every subscription in one group to the app's review submission.",
		LongHelp: `Add the version of every subscription in one group to the app's draft
(READY_FOR_REVIEW) review submission through the App Store Connect API. It uses
the API key, not a web session, and does not submit the review submission.

Subscriptions already on a draft, in MISSING_METADATA, or without a version
that can be submitted are listed as skipped with the reason. The draft is chosen
as in ` + "`asc web review subscriptions attach`" + `.

The web session flags are deprecated here and have no effect.

Examples:
  asc web review subscriptions attach-group --app "123456789" --group-id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(d.ResolveAppID(*appID))
			trimmedGroupID := strings.TrimSpace(*groupID)
			switch {
			case trimmedAppID == "":
				return d.UsageError("--app is required (or set ASC_APP_ID)")
			case trimmedGroupID == "":
				return d.UsageError("--group-id is required")
			}
			draftFlags, err := validateReviewSubscriptionAttachDraftFlags(d, fs, *submissionID, *platform)
			if err != nil {
				return err
			}
			if !*confirm {
				return d.RequireConfirm(*confirm)
			}
			warnDeprecatedReviewSubscriptionAttachWebFlags(d, fs, "attach-group")

			client, err := reviewSubscriptionAPIClient(d, "attach-group")
			if err != nil {
				return err
			}
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			subscriptions, err := withWebSpinnerValueWith(d, "Loading subscriptions", func() ([]webcore.ReviewSubscription, error) {
				return loadAPIReviewSubscriptions(requestCtx, client, trimmedAppID)
			})
			if err != nil {
				return fmt.Errorf("web review subscriptions attach-group: %w", err)
			}
			selectedGroup, err := findReviewSubscriptionGroup(subscriptions, trimmedGroupID)
			if err != nil {
				return fmt.Errorf("subscription group lookup for app %q failed: %w", trimmedAppID, withReviewSelectorDiagnostic(err, "--group-id"))
			}
			trimmedGroupID = strings.TrimSpace(selectedGroup.GroupID)
			groupSubscriptions := findReviewSubscriptionsByGroup(subscriptions, trimmedGroupID)

			drafts, err := loadReviewSubscriptionDrafts(requestCtx, client, trimmedAppID)
			if err != nil {
				return fmt.Errorf("web review subscriptions attach-group: %w", err)
			}

			type pending struct {
				subscription webcore.ReviewSubscription
				versionID    string
			}
			toAttach := []pending{}
			skipped := []reviewSubscriptionMutationSkip{}
			for _, subscription := range groupSubscriptions {
				if strings.EqualFold(subscription.State, "MISSING_METADATA") {
					skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: fmt.Sprintf("state is MISSING_METADATA; run `asc validate subscriptions --app %q` to see what is missing", trimmedAppID)})
					continue
				}
				versionID, states, err := resolveReviewSubscriptionVersion(requestCtx, client, subscription.ID)
				if err != nil {
					return fmt.Errorf("web review subscriptions attach-group: %w", err)
				}
				if versionID == "" {
					skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: fmt.Sprintf("no version that can be submitted (version states: %s)", strings.Join(states, ", "))})
					continue
				}
				if existing, ok := drafts.Items[versionID]; ok {
					subscription.SubmitWithNextAppStoreVersion, subscription.SubmitWithNextAppStoreVersionKnown = true, true
					skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: "already on review submission " + existing.SubmissionID})
					continue
				}
				toAttach = append(toAttach, pending{subscription: subscription, versionID: versionID})
			}

			payload := reviewSubscriptionGroupMutationOutput{
				AppID:              trimmedAppID,
				GroupID:            trimmedGroupID,
				GroupReferenceName: reviewSubscriptionGroupLabel(groupSubscriptions, trimmedGroupID),
				Operation:          "attach-group",
			}
			if len(toAttach) > 0 {
				draft, err := selectReviewSubscriptionDraft(requestCtx, d, client, trimmedAppID, &drafts, draftFlags)
				if err != nil {
					return err
				}
				draftID := draft.ID
				payload.SubmissionID = draftID
				err = withWebSpinnerWith(d, "Adding subscription group to review submission", func() error {
					for _, item := range toAttach {
						if _, err := createReviewSubscriptionItem(requestCtx, client, draftID, item.versionID); err != nil {
							return fmt.Errorf("subscription %q: %w", item.subscription.ID, err)
						}
						item.subscription.SubmitWithNextAppStoreVersion, item.subscription.SubmitWithNextAppStoreVersionKnown = true, true
						payload.Changed = append(payload.Changed, item.subscription)
					}
					return nil
				})
				if err != nil {
					if len(payload.Changed) > 0 {
						err = fmt.Errorf("%w (already added to review submission %s: %s)", err, draftID, strings.Join(reviewSubscriptionIDs(payload.Changed), ", "))
					} else {
						err = withCreatedDraftCleanup(requestCtx, client, draft, err)
					}
					return fmt.Errorf("web review subscriptions attach-group: %w", err)
				}
			} else if !slices.ContainsFunc(skipped, func(skip reviewSubscriptionMutationSkip) bool { return skip.Subscription.SubmitWithNextAppStoreVersion }) {
				reasons := make([]string, 0, len(skipped))
				for _, skip := range skipped {
					reasons = append(reasons, fmt.Sprintf("%s: %s", skip.Subscription.ID, skip.Reason))
				}
				return reviewSubscriptionNotReadyError(fmt.Errorf("web review subscriptions attach-group: group %q has no subscription that can be added to a review submission (%s)", trimmedGroupID, strings.Join(reasons, "; ")), "--group-id")
			}
			payload.ChangedCount = len(payload.Changed)
			payload.Skipped = skipped
			payload.SkippedCount = len(skipped)

			return d.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionGroupMutationTable(d, payload) },
				func() error { return renderReviewSubscriptionGroupMutationMarkdown(d, payload) },
			)
		},
	}
}

func reviewSubscriptionIDs(subscriptions []webcore.ReviewSubscription) []string {
	ids := make([]string, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		ids = append(ids, subscription.ID)
	}
	return ids
}
