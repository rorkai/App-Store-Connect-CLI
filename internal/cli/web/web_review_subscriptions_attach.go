package web

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// With --review-items, attach and attach-group add the subscription's version
// to the app's draft review submission through the App Store Connect API, the
// same request as `asc review items add --item-type subscriptionVersions`. They
// use the API key, not a web session. Without the flag they keep the
// deprecated web-session flow.

// reviewSubscriptionSubmittableVersionStates are the subscription and
// subscription group version states App Store Connect accepts as a review
// submission item.
var reviewSubscriptionSubmittableVersionStates = []asc.SubscriptionVersionState{
	asc.SubscriptionVersionStatePrepareForSubmission,
	asc.SubscriptionVersionStateReadyForReview,
	asc.SubscriptionVersionStateRejected,
	asc.SubscriptionVersionStateDeveloperRejected,
}

// reviewSubscriptionApprovedVersionStates mean a version passed review once.
var reviewSubscriptionApprovedVersionStates = []string{"ACCEPTED", "APPROVED", "REPLACED_WITH_NEW_VERSION"}

// reviewSubscriptionAppVersionStates are the App Store version states that
// can go on a review submission.
var reviewSubscriptionAppVersionStates = []string{
	"DEVELOPER_REJECTED", "INVALID_BINARY", "METADATA_REJECTED", "PREPARE_FOR_SUBMISSION", "READY_FOR_REVIEW", "REJECTED",
}

// reviewSubscriptionWebSessionFlags belong to the web-session flow only.
var reviewSubscriptionWebSessionFlags = []string{
	"apple-id", "two-factor-code-command", "two-factor-method", "two-factor-phone-id", "provider-id", "public-provider-id",
}

const reviewSubscriptionWebFlowDeprecation = "Warning: `asc web review subscriptions %s` through the web session is deprecated; add --review-items to add the subscription to the app's review submission with the App Store Connect API key instead. The web-session flow may be removed in a future major release.\n"

// reviewSubscriptionAttachFlags are the flags of the review-items path.
type reviewSubscriptionAttachFlags struct {
	reviewItems  *bool
	submissionID *string
	platform     *string
}

func bindReviewSubscriptionAttachFlags(fs *flag.FlagSet) reviewSubscriptionAttachFlags {
	return reviewSubscriptionAttachFlags{
		reviewItems:  fs.Bool("review-items", false, "Add the subscription's version to the app's review submission with the App Store Connect API key instead of the deprecated web-session flow"),
		submissionID: fs.String("submission", "", "With --review-items: review submission ID (required when the app has several READY_FOR_REVIEW review submissions)"),
		platform:     fs.String("platform", "IOS", "With --review-items: platform of the review submission; picks the app's draft on that platform, or creates one when the app has none: IOS, MAC_OS, TV_OS, VISION_OS"),
	}
}

// reviewSubscriptionDraftFlags are the validated --submission and --platform.
type reviewSubscriptionDraftFlags struct {
	SubmissionID string
	Platform     asc.Platform
	PlatformSet  bool
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// prepareWebSubscriptionAttach checks the flags of the web-session flow and
// prints its deprecation warning.
func prepareWebSubscriptionAttach(d *shared.Deps, fs *flag.FlagSet, operation string) error {
	if flagWasSet(fs, "submission") || flagWasSet(fs, "platform") {
		return d.UsageError("--submission and --platform require --review-items")
	}
	fmt.Fprintf(d.IO.ErrOut, reviewSubscriptionWebFlowDeprecation, operation)
	return nil
}

func validateReviewSubscriptionAttachDraftFlags(d *shared.Deps, fs *flag.FlagSet, flags reviewSubscriptionAttachFlags) (reviewSubscriptionDraftFlags, error) {
	for _, name := range reviewSubscriptionWebSessionFlags {
		if flagWasSet(fs, name) {
			return reviewSubscriptionDraftFlags{}, d.UsageErrorf("--%s belongs to the web-session flow; drop it with --review-items", name)
		}
	}
	submissionID := *flags.submissionID
	trimmedSubmissionID := strings.TrimSpace(submissionID)
	if submissionID != "" && trimmedSubmissionID == "" {
		return reviewSubscriptionDraftFlags{}, d.UsageError("--submission must not be blank")
	}
	normalized, err := shared.NormalizeAppStoreVersionPlatform(*flags.platform)
	if err != nil {
		return reviewSubscriptionDraftFlags{}, shared.WithDiagnostic(d.UsageError(err.Error()), shared.DiagnosticInvalidInput, "--platform")
	}
	platformSet := flagWasSet(fs, "platform")
	if platformSet && trimmedSubmissionID != "" {
		return reviewSubscriptionDraftFlags{}, d.UsageError("--platform only applies when a new review submission is created; drop it when --submission is set")
	}
	return reviewSubscriptionDraftFlags{SubmissionID: trimmedSubmissionID, Platform: asc.Platform(normalized), PlatformSet: platformSet}, nil
}

// reviewSubscriptionReviewItem is one item on the review submission that the
// command added or found there.
type reviewSubscriptionReviewItem struct {
	ItemType               string `json:"itemType"`
	ResourceID             string `json:"resourceId"`
	ReviewSubmissionItemID string `json:"reviewSubmissionItemId"`
	SubscriptionID         string `json:"subscriptionId,omitempty"`
	Added                  bool   `json:"added"`
}

// reviewSubscriptionReviewItemsOutput is the --review-items receipt. Its
// subscriptions carry only what the App Store Connect API returned; the web
// listing's attachment fields are not read on this path.
type reviewSubscriptionReviewItemsOutput struct {
	AppID                   string                           `json:"appId"`
	Operation               string                           `json:"operation"`
	GroupID                 string                           `json:"groupId,omitempty"`
	GroupReferenceName      string                           `json:"groupReferenceName,omitempty"`
	Changed                 bool                             `json:"changed"`
	ReviewSubmissionID      string                           `json:"reviewSubmissionId,omitempty"`
	ReviewSubmissionCreated bool                             `json:"reviewSubmissionCreated"`
	Subscription            *webcore.ReviewSubscription      `json:"subscription,omitempty"`
	ChangedCount            *int                             `json:"changedCount,omitempty"`
	SkippedCount            *int                             `json:"skippedCount,omitempty"`
	ChangedSubscriptions    []webcore.ReviewSubscription     `json:"changedSubscriptions,omitempty"`
	Skipped                 []reviewSubscriptionMutationSkip `json:"skippedSubscriptions,omitempty"`
	Items                   []reviewSubscriptionReviewItem   `json:"items"`
}

func buildReviewSubscriptionReviewItemsRows(payload reviewSubscriptionReviewItemsOutput) [][]string {
	rows := [][]string{
		{"Mutation", "App ID", valueOrNA(payload.AppID)},
		{"Mutation", "Operation", valueOrNA(payload.Operation)},
		{"Mutation", "Changed", reviewSubscriptionBool(payload.Changed)},
		{"Mutation", "Review Submission ID", valueOrNA(payload.ReviewSubmissionID)},
		{"Mutation", "Review Submission Created", reviewSubscriptionBool(payload.ReviewSubmissionCreated)},
	}
	if payload.GroupID != "" {
		rows = append(rows, []string{"Group", "Group ID", payload.GroupID})
	}
	for _, item := range payload.Items {
		rows = append(rows, []string{"Item", item.ItemType, fmt.Sprintf("%s (item %s, added %t)", item.ResourceID, valueOrNA(item.ReviewSubmissionItemID), item.Added)})
	}
	for _, skip := range payload.Skipped {
		rows = append(rows, []string{"Skipped", valueOrNA(skip.Subscription.ID), skip.Reason})
	}
	return rows
}

func printReviewSubscriptionReviewItemsOutput(d *shared.Deps, payload reviewSubscriptionReviewItemsOutput, format string, pretty bool) error {
	headers := []string{"Section", "Field", "Value"}
	return d.PrintOutputWithRenderers(payload, format, pretty,
		func() error {
			asc.RenderTableTo(d.IO.Out, headers, buildReviewSubscriptionReviewItemsRows(payload))
			return nil
		},
		func() error {
			asc.RenderMarkdownTo(d.IO.Out, headers, buildReviewSubscriptionReviewItemsRows(payload))
			return nil
		},
	)
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

// reviewSubscriptionDraftItem is a resource already on a draft.
type reviewSubscriptionDraftItem struct {
	SubmissionID string
	ItemID       string
}

// reviewSubscriptionDrafts holds the app's READY_FOR_REVIEW review
// submissions and the items already on them, keyed by itemType/resourceID.
type reviewSubscriptionDrafts struct {
	IDs       []string
	Platforms map[string]asc.Platform
	Items     map[string]reviewSubscriptionDraftItem
}

func reviewItemKey(itemType asc.ReviewSubmissionItemType, resourceID string) string {
	return string(itemType) + "/" + resourceID
}

// firstOfType returns the resource ID of one item of itemType on the draft.
func (drafts reviewSubscriptionDrafts) firstOfType(submissionID string, itemType asc.ReviewSubmissionItemType) (string, bool) {
	prefix := string(itemType) + "/"
	keys := make([]string, 0)
	for key, item := range drafts.Items {
		if item.SubmissionID == submissionID && strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return "", false
	}
	slices.Sort(keys)
	return strings.TrimPrefix(keys[0], prefix), true
}

// loadReviewSubscriptionDrafts lists the app's draft review submissions and
// the app versions, subscription group versions and subscription versions on them.
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
		asc.WithReviewSubmissionItemsInclude([]string{"appStoreVersion", "subscriptionGroupVersion", "subscriptionVersion"}),
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
			if strings.EqualFold(strings.TrimSpace(item.Attributes.State), "REMOVED") || item.Relationships == nil {
				continue
			}
			for itemType, rel := range map[asc.ReviewSubmissionItemType]*asc.Relationship{
				asc.ReviewSubmissionItemTypeAppStoreVersion:          item.Relationships.AppStoreVersion,
				asc.ReviewSubmissionItemTypeSubscriptionGroupVersion: item.Relationships.SubscriptionGroupVersion,
				asc.ReviewSubmissionItemTypeSubscriptionVersion:      item.Relationships.SubscriptionVersion,
			} {
				if rel == nil || strings.TrimSpace(rel.Data.ID) == "" {
					continue
				}
				into[reviewItemKey(itemType, strings.TrimSpace(rel.Data.ID))] = reviewSubscriptionDraftItem{SubmissionID: submissionID, ItemID: strings.TrimSpace(item.ID)}
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
// when given; for a first subscription, the one draft that already holds an
// app version; the only draft (on --platform when it is set); or a new one for
// --platform when there is none.
func selectReviewSubscriptionDraft(ctx context.Context, d *shared.Deps, client *asc.Client, appID string, drafts *reviewSubscriptionDrafts, opts reviewSubscriptionDraftFlags, first bool) (reviewSubscriptionDraftChoice, error) {
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
	if first {
		withAppVersion := slices.DeleteFunc(slices.Clone(candidates), func(id string) bool {
			_, ok := drafts.firstOfType(id, asc.ReviewSubmissionItemTypeAppStoreVersion)
			return !ok
		})
		if len(withAppVersion) == 1 {
			return reviewSubscriptionDraftChoice{ID: withAppVersion[0]}, nil
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

// reviewSubscriptionNotReadyError marks an attach that cannot proceed until
// the subscription or app changes, as the web flow's preflight did.
func reviewSubscriptionNotReadyError(err error, parameter string) error {
	return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticStateNotReady, parameter)
}

func reviewSubscriptionMissingMetadataError(appID, subscriptionID string) error {
	return &shared.HintedError{
		Err:  reviewSubscriptionNotReadyError(fmt.Errorf("web review subscriptions attach: subscription %q is MISSING_METADATA, so App Store Connect will not accept it on a review submission; nothing was sent", subscriptionID), "--subscription-id"),
		Hint: fmt.Sprintf("Run `asc validate subscriptions --app %q` to see what is missing, then retry.", appID),
	}
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

// reviewSubscriptionAPIClient returns the API client, and tells a caller who
// only has a web session what --review-items needs.
func reviewSubscriptionAPIClient(d *shared.Deps, operation string) (*asc.Client, error) {
	client, err := d.Client()
	if err != nil {
		return nil, &shared.HintedError{
			Err:  fmt.Errorf("web review subscriptions %s: %w", operation, err),
			Hint: "`asc web review subscriptions " + operation + " --review-items` uses the App Store Connect API key, not the web session. Set up a key with `asc auth login`, or drop --review-items to use the web session.",
		}
	}
	return client, nil
}

// appHasApprovedSubscription reports whether the app passed subscription
// review before. Until it has, App Store Connect reviews its first
// subscription together with an app version.
func appHasApprovedSubscription(subscriptions []webcore.ReviewSubscription) bool {
	return slices.ContainsFunc(subscriptions, func(subscription webcore.ReviewSubscription) bool {
		return strings.EqualFold(subscription.State, "APPROVED")
	})
}

// reviewSubscriptionPlannedItem is an item the command will add or found.
type reviewSubscriptionPlannedItem struct {
	itemType       asc.ReviewSubmissionItemType
	resourceID     string
	subscriptionID string
}

// planFirstSubscriptionCompanions returns the app version and, when the group
// has never been approved, the subscription group version that must be on the
// same draft as a first subscription.
func planFirstSubscriptionCompanions(ctx context.Context, client *asc.Client, appID, groupID string, drafts reviewSubscriptionDrafts, draftID string) ([]reviewSubscriptionPlannedItem, error) {
	planned := []reviewSubscriptionPlannedItem{}
	if versionID, ok := drafts.firstOfType(draftID, asc.ReviewSubmissionItemTypeAppStoreVersion); ok {
		planned = append(planned, reviewSubscriptionPlannedItem{itemType: asc.ReviewSubmissionItemTypeAppStoreVersion, resourceID: versionID})
	} else {
		platform := drafts.Platforms[draftID]
		versions, err := client.GetAppStoreVersions(ctx, appID,
			asc.WithAppStoreVersionsPlatforms([]string{string(platform)}),
			asc.WithAppStoreVersionsVersionStates(reviewSubscriptionAppVersionStates),
			asc.Limit[asc.AppStoreVersionsOption](200),
		)
		if err != nil {
			return nil, fmt.Errorf("list app versions of app %q: %w", appID, err)
		}
		switch len(versions.Data) {
		case 0:
			return nil, &shared.HintedError{
				Err:  reviewSubscriptionNotReadyError(fmt.Errorf("app %s has no approved subscription yet, so App Store Connect reviews this one with an app version, and the app has no %s version that can be submitted; nothing was sent", appID, platform), "--app"),
				Hint: fmt.Sprintf("Create one with `asc versions create --app %q --platform %s --version VERSION`, then retry.", appID, platform),
			}
		case 1:
		default:
			return nil, fmt.Errorf("app %s has more than one %s version that can be submitted; refusing to choose", appID, platform)
		}
		versionID := strings.TrimSpace(versions.Data[0].ID)
		if other, ok := drafts.Items[reviewItemKey(asc.ReviewSubmissionItemTypeAppStoreVersion, versionID)]; ok && other.SubmissionID != draftID {
			return nil, fmt.Errorf("app version %s is on review submission %s; add the subscription there with --submission %s", versionID, other.SubmissionID, other.SubmissionID)
		}
		planned = append(planned, reviewSubscriptionPlannedItem{itemType: asc.ReviewSubmissionItemTypeAppStoreVersion, resourceID: versionID})
	}

	groupVersions, err := client.GetSubscriptionGroupVersions(ctx, groupID, asc.Limit[asc.SubscriptionGroupVersionsOption](200))
	if err != nil {
		return nil, fmt.Errorf("list versions of subscription group %q: %w", groupID, err)
	}
	selected := ""
	for _, version := range groupVersions.Data {
		state := strings.TrimSpace(version.Attributes.State)
		if slices.Contains(reviewSubscriptionApprovedVersionStates, state) {
			return planned, nil
		}
		if slices.Contains(reviewSubscriptionSubmittableVersionStates, asc.SubscriptionVersionState(state)) {
			selected = strings.TrimSpace(version.ID)
		}
	}
	if selected == "" {
		return nil, &shared.HintedError{
			Err:  reviewSubscriptionNotReadyError(fmt.Errorf("subscription group %q has never been approved and has no version that can be submitted; nothing was sent", groupID), "--group-id"),
			Hint: fmt.Sprintf("Create one with `asc subscriptions groups versions create --group-id %q`, then retry.", groupID),
		}
	}
	if other, ok := drafts.Items[reviewItemKey(asc.ReviewSubmissionItemTypeSubscriptionGroupVersion, selected)]; ok && other.SubmissionID != draftID {
		return nil, fmt.Errorf("subscription group version %s is on review submission %s; add the subscription there with --submission %s", selected, other.SubmissionID, other.SubmissionID)
	}
	return append(planned, reviewSubscriptionPlannedItem{itemType: asc.ReviewSubmissionItemTypeSubscriptionGroupVersion, resourceID: selected}), nil
}

// applyReviewSubscriptionItems adds the planned items that are not on the
// draft yet, in order. When one fails, it removes the items this run added
// and cancels the draft if this run created it; nothing else is touched and
// nothing is submitted.
func applyReviewSubscriptionItems(ctx context.Context, client *asc.Client, drafts reviewSubscriptionDrafts, draft reviewSubscriptionDraftChoice, planned []reviewSubscriptionPlannedItem) ([]reviewSubscriptionReviewItem, error) {
	items := make([]reviewSubscriptionReviewItem, 0, len(planned))
	for _, plan := range planned {
		if existing, ok := drafts.Items[reviewItemKey(plan.itemType, plan.resourceID)]; ok && existing.SubmissionID == draft.ID {
			items = append(items, reviewSubscriptionReviewItem{ItemType: string(plan.itemType), ResourceID: plan.resourceID, ReviewSubmissionItemID: existing.ItemID, SubscriptionID: plan.subscriptionID})
			continue
		}
		created, err := client.CreateReviewSubmissionItem(ctx, draft.ID, plan.itemType, plan.resourceID)
		if err == nil && strings.TrimSpace(created.Data.ID) == "" {
			err = fmt.Errorf("response has no id")
		}
		if err != nil {
			err = fmt.Errorf("add %s %q to review submission %q: %w", plan.itemType, plan.resourceID, draft.ID, err)
			return nil, rollbackReviewSubscriptionItems(ctx, client, draft, items, err)
		}
		items = append(items, reviewSubscriptionReviewItem{ItemType: string(plan.itemType), ResourceID: plan.resourceID, ReviewSubmissionItemID: strings.TrimSpace(created.Data.ID), SubscriptionID: plan.subscriptionID, Added: true})
	}
	return items, nil
}

func rollbackReviewSubscriptionItems(ctx context.Context, client *asc.Client, draft reviewSubscriptionDraftChoice, items []reviewSubscriptionReviewItem, addErr error) error {
	notes := []string{}
	for _, item := range slices.Backward(items) {
		if !item.Added {
			continue
		}
		if err := client.DeleteReviewSubmissionItem(ctx, item.ReviewSubmissionItemID); err != nil {
			notes = append(notes, fmt.Sprintf("could not remove review item %s (%s %s), which this run added: %v; remove it with `asc review items remove --id %q --confirm`", item.ReviewSubmissionItemID, item.ItemType, item.ResourceID, err, item.ReviewSubmissionItemID))
			continue
		}
		notes = append(notes, fmt.Sprintf("removed review item %s (%s %s), which this run added", item.ReviewSubmissionItemID, item.ItemType, item.ResourceID))
	}
	if draft.Created {
		if _, err := client.CancelReviewSubmission(ctx, draft.ID); err != nil {
			notes = append(notes, fmt.Sprintf("review submission %s, which this run created, is still a draft; cancel it with `asc review submissions-cancel --id %q --confirm`: %v", draft.ID, draft.ID, err))
		} else {
			notes = append(notes, fmt.Sprintf("canceled review submission %s, which this run created", draft.ID))
		}
	}
	if len(notes) == 0 {
		return addErr
	}
	return fmt.Errorf("%w (%s)", addErr, strings.Join(notes, "; "))
}

func noteFirstSubscription(d *shared.Deps, appID string, planned []reviewSubscriptionPlannedItem) {
	parts := []string{}
	for _, plan := range planned {
		if plan.itemType != asc.ReviewSubmissionItemTypeSubscriptionVersion {
			parts = append(parts, fmt.Sprintf("%s %s", plan.itemType, plan.resourceID))
		}
	}
	if len(parts) > 0 {
		fmt.Fprintf(d.IO.ErrOut, "App %s has no approved subscription yet, so its first subscription goes on the review submission with %s.\n", appID, strings.Join(parts, " and "))
	}
}

// reviewSubscriptionAttachRequest is a validated attach or attach-group call.
type reviewSubscriptionAttachRequest struct {
	AppID          string
	SubscriptionID string
	GroupID        string
	Draft          reviewSubscriptionDraftFlags
	Output         string
	Pretty         bool
}

// runReviewSubscriptionAttachWithReviewItems is `attach --review-items`.
func runReviewSubscriptionAttachWithReviewItems(ctx context.Context, d *shared.Deps, req reviewSubscriptionAttachRequest) error {
	client, err := reviewSubscriptionAPIClient(d, "attach")
	if err != nil {
		return err
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	subscriptions, err := withWebSpinnerValueWith(d, "Loading subscriptions", func() ([]webcore.ReviewSubscription, error) {
		return loadAPIReviewSubscriptions(requestCtx, client, req.AppID)
	})
	if err != nil {
		return fmt.Errorf("web review subscriptions attach: %w", err)
	}
	selected, err := findReviewSubscription(subscriptions, req.SubscriptionID)
	if err != nil {
		return fmt.Errorf("subscription lookup for app %q failed: %w", req.AppID, withReviewSelectorDiagnostic(err, "--subscription-id"))
	}
	subscription := *selected
	if strings.EqualFold(subscription.State, "MISSING_METADATA") {
		return reviewSubscriptionMissingMetadataError(req.AppID, subscription.ID)
	}
	versionID, states, err := resolveReviewSubscriptionVersion(requestCtx, client, subscription.ID)
	if err != nil {
		return fmt.Errorf("web review subscriptions attach: %w", err)
	}
	if versionID == "" {
		return reviewSubscriptionNoVersionError("attach", subscription.ID, states)
	}
	drafts, err := loadReviewSubscriptionDrafts(requestCtx, client, req.AppID)
	if err != nil {
		return fmt.Errorf("web review subscriptions attach: %w", err)
	}
	first := !appHasApprovedSubscription(subscriptions)

	var draft reviewSubscriptionDraftChoice
	if existing, ok := drafts.Items[reviewItemKey(asc.ReviewSubmissionItemTypeSubscriptionVersion, versionID)]; ok {
		if req.Draft.SubmissionID != "" && existing.SubmissionID != req.Draft.SubmissionID {
			return d.UsageErrorf("subscription %q is already on review submission %s; drop --submission or remove it there first with `asc review items remove --id %q --confirm`", subscription.ID, existing.SubmissionID, existing.ItemID)
		}
		draft = reviewSubscriptionDraftChoice{ID: existing.SubmissionID}
	}

	var planned []reviewSubscriptionPlannedItem
	plan := func(draftID string) error {
		planned = nil
		if first {
			companions, err := planFirstSubscriptionCompanions(requestCtx, client, req.AppID, subscription.GroupID, drafts, draftID)
			if err != nil {
				return err
			}
			planned = companions
		}
		planned = append(planned, reviewSubscriptionPlannedItem{itemType: asc.ReviewSubmissionItemTypeSubscriptionVersion, resourceID: versionID, subscriptionID: subscription.ID})
		return nil
	}
	preplanned := false
	if draft.ID == "" {
		// With no draft, plan before creating one, so a missing app version
		// creates nothing.
		if first && req.Draft.SubmissionID == "" && len(drafts.IDs) == 0 {
			drafts.Platforms[""] = req.Draft.Platform
			if err := plan(""); err != nil {
				return fmt.Errorf("web review subscriptions attach: %w", err)
			}
			preplanned = true
		}
		draft, err = selectReviewSubscriptionDraft(requestCtx, d, client, req.AppID, &drafts, req.Draft, first)
		if err != nil {
			return err
		}
	}
	if !preplanned {
		if err := plan(draft.ID); err != nil {
			return fmt.Errorf("web review subscriptions attach: %w", err)
		}
	}
	if first {
		noteFirstSubscription(d, req.AppID, planned)
	}
	items, err := withWebSpinnerValueWith(d, "Adding subscription to review submission", func() ([]reviewSubscriptionReviewItem, error) {
		return applyReviewSubscriptionItems(requestCtx, client, drafts, draft, planned)
	})
	if err != nil {
		return fmt.Errorf("web review subscriptions attach: %w", err)
	}
	payload := reviewSubscriptionReviewItemsOutput{
		AppID:                   req.AppID,
		Operation:               "attach",
		ReviewSubmissionID:      draft.ID,
		ReviewSubmissionCreated: draft.Created,
		Subscription:            &subscription,
		Items:                   items,
		Changed:                 slices.ContainsFunc(items, func(item reviewSubscriptionReviewItem) bool { return item.Added }),
	}
	return printReviewSubscriptionReviewItemsOutput(d, payload, req.Output, req.Pretty)
}

// runReviewSubscriptionAttachGroupWithReviewItems is `attach-group --review-items`.
func runReviewSubscriptionAttachGroupWithReviewItems(ctx context.Context, d *shared.Deps, req reviewSubscriptionAttachRequest) error {
	client, err := reviewSubscriptionAPIClient(d, "attach-group")
	if err != nil {
		return err
	}
	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	subscriptions, err := withWebSpinnerValueWith(d, "Loading subscriptions", func() ([]webcore.ReviewSubscription, error) {
		return loadAPIReviewSubscriptions(requestCtx, client, req.AppID)
	})
	if err != nil {
		return fmt.Errorf("web review subscriptions attach-group: %w", err)
	}
	selectedGroup, err := findReviewSubscriptionGroup(subscriptions, req.GroupID)
	if err != nil {
		return fmt.Errorf("subscription group lookup for app %q failed: %w", req.AppID, withReviewSelectorDiagnostic(err, "--group-id"))
	}
	groupID := strings.TrimSpace(selectedGroup.GroupID)
	groupSubscriptions := findReviewSubscriptionsByGroup(subscriptions, groupID)
	drafts, err := loadReviewSubscriptionDrafts(requestCtx, client, req.AppID)
	if err != nil {
		return fmt.Errorf("web review subscriptions attach-group: %w", err)
	}

	toAttach := []reviewSubscriptionPlannedItem{}
	attachedSubs := []webcore.ReviewSubscription{}
	skipped := []reviewSubscriptionMutationSkip{}
	for _, subscription := range groupSubscriptions {
		if strings.EqualFold(subscription.State, "MISSING_METADATA") {
			skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: fmt.Sprintf("state is MISSING_METADATA; run `asc validate subscriptions --app %q` to see what is missing", req.AppID)})
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
		if existing, ok := drafts.Items[reviewItemKey(asc.ReviewSubmissionItemTypeSubscriptionVersion, versionID)]; ok {
			skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: "already on review submission " + existing.SubmissionID})
			continue
		}
		toAttach = append(toAttach, reviewSubscriptionPlannedItem{itemType: asc.ReviewSubmissionItemTypeSubscriptionVersion, resourceID: versionID, subscriptionID: subscription.ID})
		attachedSubs = append(attachedSubs, subscription)
	}

	payload := reviewSubscriptionReviewItemsOutput{
		AppID:              req.AppID,
		Operation:          "attach-group",
		GroupID:            groupID,
		GroupReferenceName: reviewSubscriptionGroupLabel(groupSubscriptions, groupID),
		Skipped:            skipped,
		Items:              []reviewSubscriptionReviewItem{},
	}
	if len(toAttach) == 0 {
		if !slices.ContainsFunc(skipped, func(skip reviewSubscriptionMutationSkip) bool {
			return strings.HasPrefix(skip.Reason, "already on review submission")
		}) {
			reasons := make([]string, 0, len(skipped))
			for _, skip := range skipped {
				reasons = append(reasons, fmt.Sprintf("%s: %s", skip.Subscription.ID, skip.Reason))
			}
			return reviewSubscriptionNotReadyError(fmt.Errorf("web review subscriptions attach-group: group %q has no subscription that can be added to a review submission (%s)", groupID, strings.Join(reasons, "; ")), "--group-id")
		}
	} else {
		first := !appHasApprovedSubscription(subscriptions)
		var companions []reviewSubscriptionPlannedItem
		preplanned := false
		if first && req.Draft.SubmissionID == "" && len(drafts.IDs) == 0 {
			// With no draft, plan before creating one.
			drafts.Platforms[""] = req.Draft.Platform
			if companions, err = planFirstSubscriptionCompanions(requestCtx, client, req.AppID, groupID, drafts, ""); err != nil {
				return fmt.Errorf("web review subscriptions attach-group: %w", err)
			}
			preplanned = true
		}
		draft, err := selectReviewSubscriptionDraft(requestCtx, d, client, req.AppID, &drafts, req.Draft, first)
		if err != nil {
			return err
		}
		if first && !preplanned {
			if companions, err = planFirstSubscriptionCompanions(requestCtx, client, req.AppID, groupID, drafts, draft.ID); err != nil {
				return fmt.Errorf("web review subscriptions attach-group: %w", err)
			}
		}
		planned := append(companions, toAttach...)
		if first {
			noteFirstSubscription(d, req.AppID, planned)
		}
		items, err := withWebSpinnerValueWith(d, "Adding subscription group to review submission", func() ([]reviewSubscriptionReviewItem, error) {
			return applyReviewSubscriptionItems(requestCtx, client, drafts, draft, planned)
		})
		if err != nil {
			return fmt.Errorf("web review subscriptions attach-group: %w", err)
		}
		payload.ReviewSubmissionID = draft.ID
		payload.ReviewSubmissionCreated = draft.Created
		payload.Items = items
		payload.Changed = slices.ContainsFunc(items, func(item reviewSubscriptionReviewItem) bool { return item.Added })
		payload.ChangedSubscriptions = attachedSubs
	}
	changedCount, skippedCount := len(payload.ChangedSubscriptions), len(skipped)
	payload.ChangedCount, payload.SkippedCount = &changedCount, &skippedCount
	return printReviewSubscriptionReviewItemsOutput(d, payload, req.Output, req.Pretty)
}
