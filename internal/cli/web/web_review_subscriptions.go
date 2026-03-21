package web

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

type reviewSubscriptionsListOutput struct {
	AppID         string                       `json:"appId"`
	AttachedCount int                          `json:"attachedCount"`
	Subscriptions []webcore.ReviewSubscription `json:"subscriptions"`
}

type reviewSubscriptionMutationOutput struct {
	AppID        string                     `json:"appId"`
	Operation    string                     `json:"operation"`
	Changed      bool                       `json:"changed"`
	SubmissionID string                     `json:"submissionId,omitempty"`
	Subscription webcore.ReviewSubscription `json:"subscription"`
}

type reviewSubscriptionMutationSkip struct {
	Subscription webcore.ReviewSubscription `json:"subscription"`
	Reason       string                     `json:"reason"`
}

type reviewSubscriptionGroupMutationOutput struct {
	AppID              string                           `json:"appId"`
	GroupID            string                           `json:"groupId"`
	GroupReferenceName string                           `json:"groupReferenceName,omitempty"`
	Operation          string                           `json:"operation"`
	ChangedCount       int                              `json:"changedCount"`
	SkippedCount       int                              `json:"skippedCount"`
	Changed            []webcore.ReviewSubscription     `json:"changedSubscriptions,omitempty"`
	Skipped            []reviewSubscriptionMutationSkip `json:"skippedSubscriptions,omitempty"`
}

func reviewSubscriptionValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "n/a"
	}
	return trimmed
}

func reviewSubscriptionName(subscription webcore.ReviewSubscription) string {
	switch {
	case strings.TrimSpace(subscription.Name) != "":
		return strings.TrimSpace(subscription.Name)
	case strings.TrimSpace(subscription.ProductID) != "":
		return strings.TrimSpace(subscription.ProductID)
	default:
		return strings.TrimSpace(subscription.ID)
	}
}

func reviewSubscriptionBool(value bool) string {
	return strconv.FormatBool(value)
}

func reviewSubscriptionState(subscription webcore.ReviewSubscription) string {
	return strings.ToUpper(strings.TrimSpace(subscription.State))
}

func reviewSubscriptionAttachPreflight(appID string, subscription webcore.ReviewSubscription) error {
	state := reviewSubscriptionState(subscription)
	if state == "READY_TO_SUBMIT" {
		return nil
	}

	subscriptionID := strings.TrimSpace(subscription.ID)
	if subscriptionID == "" {
		subscriptionID = "SUB_ID"
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(
		os.Stderr,
		"Attach preflight: subscription %q (%s) is %s, so Apple will not attach it to the next app version review yet.\n",
		reviewSubscriptionName(subscription),
		reviewSubscriptionValue(subscriptionID),
		state,
	)
	fmt.Fprintf(
		os.Stderr,
		"Hint: run `asc validate subscriptions --app \"%s\"` to inspect readiness.\n",
		reviewSubscriptionValue(strings.TrimSpace(appID)),
	)
	fmt.Fprintln(os.Stderr, "Hint: Apple only allows this attach flow after the subscription reaches READY_TO_SUBMIT.")
	switch state {
	case "MISSING_METADATA":
		fmt.Fprintln(os.Stderr, "Hint: Check localizations, pricing coverage, and the App Store review screenshot.")
		fmt.Fprintf(
			os.Stderr,
			"Hint: In live testing, a subscription promotional image also mattered even though App Store Connect surfaces it as a recommendation. Upload one with `asc subscriptions images create --subscription-id \"%s\" --file \"./image.png\"` if it is missing.\n",
			reviewSubscriptionValue(subscriptionID),
		)
	case "":
		fmt.Fprintln(os.Stderr, "Hint: Refresh App Store Connect or rerun `asc web review subscriptions list` to confirm the current readiness state before retrying.")
	default:
		fmt.Fprintln(os.Stderr, "Hint: Complete the outstanding App Store Connect action for this subscription, then retry once it reaches READY_TO_SUBMIT.")
	}

	return shared.NewReportedError(
		fmt.Errorf(
			"web review subscriptions attach: subscription %q is %s; Apple only allows attach once it reaches READY_TO_SUBMIT",
			subscriptionID,
			state,
		),
	)
}

func countAttachedReviewSubscriptions(subscriptions []webcore.ReviewSubscription) int {
	count := 0
	for _, subscription := range subscriptions {
		if subscription.SubmitWithNextAppStoreVersion {
			count++
		}
	}
	return count
}

func buildReviewSubscriptionGroupMutationRows(payload reviewSubscriptionGroupMutationOutput) [][]string {
	rows := [][]string{
		{"Summary", "App ID", reviewSubscriptionValue(payload.AppID)},
		{"Summary", "Group ID", reviewSubscriptionValue(payload.GroupID)},
		{"Summary", "Group", reviewSubscriptionValue(payload.GroupReferenceName)},
		{"Summary", "Operation", reviewSubscriptionValue(payload.Operation)},
		{"Summary", "Changed Count", strconv.Itoa(payload.ChangedCount)},
		{"Summary", "Skipped Count", strconv.Itoa(payload.SkippedCount)},
	}
	for i, subscription := range payload.Changed {
		rows = append(rows, []string{
			"Changed",
			fmt.Sprintf("Subscription %d", i+1),
			fmt.Sprintf(
				"id=%s name=%s state=%s attached=%s",
				reviewSubscriptionValue(subscription.ID),
				reviewSubscriptionValue(reviewSubscriptionName(subscription)),
				reviewSubscriptionValue(subscription.State),
				reviewSubscriptionBool(subscription.SubmitWithNextAppStoreVersion),
			),
		})
	}
	for i, skipped := range payload.Skipped {
		rows = append(rows, []string{
			"Skipped",
			fmt.Sprintf("Subscription %d", i+1),
			fmt.Sprintf(
				"id=%s name=%s reason=%s",
				reviewSubscriptionValue(skipped.Subscription.ID),
				reviewSubscriptionValue(reviewSubscriptionName(skipped.Subscription)),
				reviewSubscriptionValue(skipped.Reason),
			),
		})
	}
	return rows
}

func renderReviewSubscriptionGroupMutationTable(payload reviewSubscriptionGroupMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderTable(headers, buildReviewSubscriptionGroupMutationRows(payload))
	return nil
}

func renderReviewSubscriptionGroupMutationMarkdown(payload reviewSubscriptionGroupMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderMarkdown(headers, buildReviewSubscriptionGroupMutationRows(payload))
	return nil
}

func buildReviewSubscriptionsListTableRows(subscriptions []webcore.ReviewSubscription) [][]string {
	rows := make([][]string, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		rows = append(rows, []string{
			reviewSubscriptionValue(subscription.GroupID),
			reviewSubscriptionValue(subscription.GroupReferenceName),
			reviewSubscriptionValue(subscription.ID),
			reviewSubscriptionValue(subscription.ProductID),
			reviewSubscriptionValue(reviewSubscriptionName(subscription)),
			reviewSubscriptionValue(subscription.State),
			reviewSubscriptionBool(subscription.SubmitWithNextAppStoreVersion),
			reviewSubscriptionBool(subscription.IsAppStoreReviewInProgress),
		})
	}
	return rows
}

func renderReviewSubscriptionsListTable(payload reviewSubscriptionsListOutput) error {
	headers := []string{"Group ID", "Group", "Subscription ID", "Product ID", "Name", "State", "Next Version", "Review In Progress"}
	asc.RenderTable(headers, buildReviewSubscriptionsListTableRows(payload.Subscriptions))
	return nil
}

func renderReviewSubscriptionsListMarkdown(payload reviewSubscriptionsListOutput) error {
	headers := []string{"Group ID", "Group", "Subscription ID", "Product ID", "Name", "State", "Next Version", "Review In Progress"}
	asc.RenderMarkdown(headers, buildReviewSubscriptionsListTableRows(payload.Subscriptions))
	return nil
}

func buildReviewSubscriptionMutationRows(payload reviewSubscriptionMutationOutput) [][]string {
	return [][]string{
		{"Mutation", "App ID", reviewSubscriptionValue(payload.AppID)},
		{"Mutation", "Operation", reviewSubscriptionValue(payload.Operation)},
		{"Mutation", "Changed", reviewSubscriptionBool(payload.Changed)},
		{"Mutation", "Submission ID", reviewSubscriptionValue(payload.SubmissionID)},
		{"Subscription", "Subscription ID", reviewSubscriptionValue(payload.Subscription.ID)},
		{"Subscription", "Product ID", reviewSubscriptionValue(payload.Subscription.ProductID)},
		{"Subscription", "Name", reviewSubscriptionValue(reviewSubscriptionName(payload.Subscription))},
		{"Subscription", "Group", reviewSubscriptionValue(payload.Subscription.GroupReferenceName)},
		{"Subscription", "State", reviewSubscriptionValue(payload.Subscription.State)},
		{"Subscription", "Next Version", reviewSubscriptionBool(payload.Subscription.SubmitWithNextAppStoreVersion)},
		{"Subscription", "Review In Progress", reviewSubscriptionBool(payload.Subscription.IsAppStoreReviewInProgress)},
	}
}

func renderReviewSubscriptionMutationTable(payload reviewSubscriptionMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderTable(headers, buildReviewSubscriptionMutationRows(payload))
	return nil
}

func renderReviewSubscriptionMutationMarkdown(payload reviewSubscriptionMutationOutput) error {
	headers := []string{"Section", "Field", "Value"}
	asc.RenderMarkdown(headers, buildReviewSubscriptionMutationRows(payload))
	return nil
}

func findReviewSubscription(subscriptions []webcore.ReviewSubscription, subscriptionID string) (*webcore.ReviewSubscription, bool) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	for i := range subscriptions {
		if strings.TrimSpace(subscriptions[i].ID) == subscriptionID {
			match := subscriptions[i]
			return &match, true
		}
	}
	return nil, false
}

func findReviewSubscriptionsByGroup(subscriptions []webcore.ReviewSubscription, groupID string) []webcore.ReviewSubscription {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil
	}
	filtered := make([]webcore.ReviewSubscription, 0)
	for _, subscription := range subscriptions {
		if strings.TrimSpace(subscription.GroupID) == groupID {
			filtered = append(filtered, subscription)
		}
	}
	return filtered
}

func reviewSubscriptionGroupLabel(subscriptions []webcore.ReviewSubscription, groupID string) string {
	for _, subscription := range subscriptions {
		if strings.TrimSpace(subscription.GroupReferenceName) != "" {
			return strings.TrimSpace(subscription.GroupReferenceName)
		}
	}
	return strings.TrimSpace(groupID)
}

func reviewSubscriptionAttachSkipReason(subscription webcore.ReviewSubscription) string {
	switch state := reviewSubscriptionState(subscription); state {
	case "MISSING_METADATA":
		return "state is MISSING_METADATA; run `asc validate subscriptions` and upload missing assets first"
	case "READY_TO_SUBMIT":
		return "state is READY_TO_SUBMIT but the refreshed review state still shows not attached"
	case "":
		return "state is unknown; Apple only allows attach from READY_TO_SUBMIT"
	default:
		return fmt.Sprintf("state is %s; Apple only allows attach from READY_TO_SUBMIT", state)
	}
}

func reviewSubscriptionAttachUnchangedAfterRefreshReason() string {
	return "attach request completed but refreshed state still shows not attached"
}

func reviewSubscriptionRemoveUnchangedAfterRefreshReason() string {
	return "remove request completed but refreshed state still shows attached"
}

func collectReviewSubscriptionGroupChanges(
	refreshedGroup []webcore.ReviewSubscription,
	subscriptionIDs []string,
	wantAttached bool,
	unchangedReason string,
) ([]webcore.ReviewSubscription, []reviewSubscriptionMutationSkip) {
	changed := make([]webcore.ReviewSubscription, 0, len(subscriptionIDs))
	skipped := make([]reviewSubscriptionMutationSkip, 0)
	for _, subscriptionID := range subscriptionIDs {
		refreshed, ok := findReviewSubscription(refreshedGroup, subscriptionID)
		if !ok {
			skipped = append(skipped, reviewSubscriptionMutationSkip{
				Subscription: webcore.ReviewSubscription{ID: strings.TrimSpace(subscriptionID)},
				Reason:       "subscription was not found after refresh",
			})
			continue
		}
		if refreshed.SubmitWithNextAppStoreVersion == wantAttached {
			changed = append(changed, *refreshed)
			continue
		}
		skipped = append(skipped, reviewSubscriptionMutationSkip{
			Subscription: *refreshed,
			Reason:       unchangedReason,
		})
	}
	return changed, skipped
}

func reviewSubscriptionGroupAttachPreflight(appID, groupID string, subscriptions []webcore.ReviewSubscription) error {
	readyCount := 0
	attachedCount := 0
	for _, subscription := range subscriptions {
		if subscription.SubmitWithNextAppStoreVersion {
			attachedCount++
		}
		if reviewSubscriptionState(subscription) == "READY_TO_SUBMIT" && !subscription.SubmitWithNextAppStoreVersion {
			readyCount++
		}
	}
	if readyCount > 0 || attachedCount > 0 {
		return nil
	}

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(
		os.Stderr,
		"Attach preflight: subscription group %q (%s) has no READY_TO_SUBMIT subscriptions for first-review attachment yet.\n",
		reviewSubscriptionGroupLabel(subscriptions, groupID),
		reviewSubscriptionValue(groupID),
	)
	fmt.Fprintf(
		os.Stderr,
		"Hint: run `asc validate subscriptions --app \"%s\"` to inspect readiness.\n",
		reviewSubscriptionValue(strings.TrimSpace(appID)),
	)
	fmt.Fprintln(os.Stderr, "Hint: Apple only allows attach after the relevant subscriptions reach READY_TO_SUBMIT.")
	fmt.Fprintln(os.Stderr, "Hint: In live testing, a subscription promotional image also mattered in addition to localization, pricing coverage, and the App Store review screenshot.")

	return shared.NewReportedError(
		fmt.Errorf(
			"web review subscriptions attach-group: group %q has no READY_TO_SUBMIT subscriptions to attach",
			groupID,
		),
	)
}

func loadReviewSubscriptionsWithLabel(ctx context.Context, client *webcore.Client, appID, label string) ([]webcore.ReviewSubscription, error) {
	return withWebSpinnerValue(label, func() ([]webcore.ReviewSubscription, error) {
		return client.ListReviewSubscriptions(ctx, appID)
	})
}

// WebReviewSubscriptionsCommand returns the app-version subscription attach helpers.
func WebReviewSubscriptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review subscriptions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscriptions",
		ShortUsage: "asc web review subscriptions <subcommand> [flags]",
		ShortHelp:  "[experimental] Inspect and mutate review-attached subscriptions.",
		LongHelp: `EXPERIMENTAL / UNOFFICIAL / DISCOURAGED

Inspect and mutate subscription selection for the next app version review.
This uses private Apple web-session /iris endpoints and may break without notice.

Subcommands:
  list    List subscriptions and their next-version attach state
  attach  Attach one subscription to the next app version review
  attach-group  Attach all READY_TO_SUBMIT subscriptions in one group
  remove  Remove one subscription from the next app version review
  remove-group  Remove all attached subscriptions in one group

` + webWarningText,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			WebReviewSubscriptionsListCommand(),
			WebReviewSubscriptionsAttachCommand(),
			WebReviewSubscriptionsAttachGroupCommand(),
			WebReviewSubscriptionsRemoveCommand(),
			WebReviewSubscriptionsRemoveGroupCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// WebReviewSubscriptionsListCommand lists review-scoped subscriptions for an app.
func WebReviewSubscriptionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review subscriptions list", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc web review subscriptions list --app APP_ID [flags]",
		ShortHelp:  "[experimental] List subscriptions and next-version attach state.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			if trimmedAppID == "" {
				return shared.UsageError("--app is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			subscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Loading review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions list")
			}

			payload := reviewSubscriptionsListOutput{
				AppID:         trimmedAppID,
				AttachedCount: countAttachedReviewSubscriptions(subscriptions),
				Subscriptions: subscriptions,
			}
			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionsListTable(payload) },
				func() error { return renderReviewSubscriptionsListMarkdown(payload) },
			)
		},
	}
}

// WebReviewSubscriptionsAttachCommand attaches a subscription to the next app version review.
func WebReviewSubscriptionsAttachCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review subscriptions attach", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	confirm := fs.Bool("confirm", false, "Confirm the attach operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach",
		ShortUsage: "asc web review subscriptions attach --app APP_ID --subscription-id SUB_ID --confirm [flags]",
		ShortHelp:  "[experimental] Attach a subscription to the next app version review.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			trimmedSubscriptionID := strings.TrimSpace(*subscriptionID)
			switch {
			case trimmedAppID == "":
				return shared.UsageError("--app is required")
			case trimmedSubscriptionID == "":
				return shared.UsageError("--subscription-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			subscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Loading review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions attach")
			}
			selected, ok := findReviewSubscription(subscriptions, trimmedSubscriptionID)
			if !ok {
				return fmt.Errorf("subscription %q was not found for app %q", trimmedSubscriptionID, trimmedAppID)
			}

			payload := reviewSubscriptionMutationOutput{
				AppID:        trimmedAppID,
				Operation:    "attach",
				Changed:      false,
				Subscription: *selected,
			}
			if !selected.SubmitWithNextAppStoreVersion {
				if err := reviewSubscriptionAttachPreflight(trimmedAppID, *selected); err != nil {
					return err
				}

				submission, err := withWebSpinnerValue("Attaching subscription to next app version", func() (webcore.ReviewSubscriptionSubmission, error) {
					return client.CreateSubscriptionSubmission(requestCtx, trimmedSubscriptionID)
				})
				if err != nil {
					return withWebAuthHint(err, "web review subscriptions attach")
				}

				refreshedSubscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Refreshing review subscriptions")
				if err != nil {
					return withWebAuthHint(err, "web review subscriptions attach")
				}
				refreshed, ok := findReviewSubscription(refreshedSubscriptions, trimmedSubscriptionID)
				if !ok {
					return fmt.Errorf("subscription %q was not found for app %q after attach", trimmedSubscriptionID, trimmedAppID)
				}
				payload.SubmissionID = strings.TrimSpace(submission.ID)
				payload.Changed = refreshed.SubmitWithNextAppStoreVersion
				payload.Subscription = *refreshed
			}

			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionMutationTable(payload) },
				func() error { return renderReviewSubscriptionMutationMarkdown(payload) },
			)
		},
	}
}

// WebReviewSubscriptionsAttachGroupCommand attaches every READY_TO_SUBMIT subscription in one group.
func WebReviewSubscriptionsAttachGroupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review subscriptions attach-group", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	groupID := fs.String("group-id", "", "Subscription group ID")
	confirm := fs.Bool("confirm", false, "Confirm the attach-group operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "attach-group",
		ShortUsage: "asc web review subscriptions attach-group --app APP_ID --group-id GROUP_ID --confirm [flags]",
		ShortHelp:  "[experimental] Attach all READY_TO_SUBMIT subscriptions in one group.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			trimmedGroupID := strings.TrimSpace(*groupID)
			switch {
			case trimmedAppID == "":
				return shared.UsageError("--app is required")
			case trimmedGroupID == "":
				return shared.UsageError("--group-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			subscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Loading review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions attach-group")
			}
			groupSubscriptions := findReviewSubscriptionsByGroup(subscriptions, trimmedGroupID)
			if len(groupSubscriptions) == 0 {
				return fmt.Errorf("subscription group %q was not found for app %q", trimmedGroupID, trimmedAppID)
			}
			if err := reviewSubscriptionGroupAttachPreflight(trimmedAppID, trimmedGroupID, groupSubscriptions); err != nil {
				return err
			}

			skipped := make([]reviewSubscriptionMutationSkip, 0)
			attachIDs := make([]string, 0)
			for _, subscription := range groupSubscriptions {
				switch {
				case subscription.SubmitWithNextAppStoreVersion:
					skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: "already attached"})
				case reviewSubscriptionState(subscription) != "READY_TO_SUBMIT":
					skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: reviewSubscriptionAttachSkipReason(subscription)})
				default:
					attachIDs = append(attachIDs, strings.TrimSpace(subscription.ID))
				}
			}

			if len(attachIDs) > 0 {
				err = withWebSpinner("Attaching subscription group to next app version", func() error {
					for _, subscriptionID := range attachIDs {
						if _, err := client.CreateSubscriptionSubmission(requestCtx, subscriptionID); err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					return withWebAuthHint(err, "web review subscriptions attach-group")
				}
			}

			refreshedSubscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Refreshing review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions attach-group")
			}
			refreshedGroup := findReviewSubscriptionsByGroup(refreshedSubscriptions, trimmedGroupID)

			changed, postRefreshSkipped := collectReviewSubscriptionGroupChanges(
				refreshedGroup,
				attachIDs,
				true,
				reviewSubscriptionAttachUnchangedAfterRefreshReason(),
			)
			skipped = append(skipped, postRefreshSkipped...)

			payload := reviewSubscriptionGroupMutationOutput{
				AppID:              trimmedAppID,
				GroupID:            trimmedGroupID,
				GroupReferenceName: reviewSubscriptionGroupLabel(refreshedGroup, trimmedGroupID),
				Operation:          "attach-group",
				ChangedCount:       len(changed),
				SkippedCount:       len(skipped),
				Changed:            changed,
				Skipped:            skipped,
			}

			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionGroupMutationTable(payload) },
				func() error { return renderReviewSubscriptionGroupMutationMarkdown(payload) },
			)
		},
	}
}

// WebReviewSubscriptionsRemoveCommand removes a subscription from the next app version review.
func WebReviewSubscriptionsRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review subscriptions remove", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	subscriptionID := fs.String("subscription-id", "", "Subscription ID")
	confirm := fs.Bool("confirm", false, "Confirm the remove operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove",
		ShortUsage: "asc web review subscriptions remove --app APP_ID --subscription-id SUB_ID --confirm [flags]",
		ShortHelp:  "[experimental] Remove a subscription from the next app version review.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			trimmedSubscriptionID := strings.TrimSpace(*subscriptionID)
			switch {
			case trimmedAppID == "":
				return shared.UsageError("--app is required")
			case trimmedSubscriptionID == "":
				return shared.UsageError("--subscription-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			subscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Loading review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions remove")
			}
			selected, ok := findReviewSubscription(subscriptions, trimmedSubscriptionID)
			if !ok {
				return fmt.Errorf("subscription %q was not found for app %q", trimmedSubscriptionID, trimmedAppID)
			}

			payload := reviewSubscriptionMutationOutput{
				AppID:        trimmedAppID,
				Operation:    "remove",
				Changed:      false,
				Subscription: *selected,
			}
			if selected.SubmitWithNextAppStoreVersion {
				err = withWebSpinner("Removing subscription from next app version", func() error {
					return client.DeleteSubscriptionSubmission(requestCtx, trimmedSubscriptionID)
				})
				if err != nil {
					return withWebAuthHint(err, "web review subscriptions remove")
				}

				refreshedSubscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Refreshing review subscriptions")
				if err != nil {
					return withWebAuthHint(err, "web review subscriptions remove")
				}
				refreshed, ok := findReviewSubscription(refreshedSubscriptions, trimmedSubscriptionID)
				if !ok {
					return fmt.Errorf("subscription %q was not found for app %q after remove", trimmedSubscriptionID, trimmedAppID)
				}
				payload.Changed = !refreshed.SubmitWithNextAppStoreVersion
				payload.Subscription = *refreshed
			}

			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionMutationTable(payload) },
				func() error { return renderReviewSubscriptionMutationMarkdown(payload) },
			)
		},
	}
}

// WebReviewSubscriptionsRemoveGroupCommand removes all attached subscriptions in one group.
func WebReviewSubscriptionsRemoveGroupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("web review subscriptions remove-group", flag.ExitOnError)

	appID := fs.String("app", "", "App ID")
	groupID := fs.String("group-id", "", "Subscription group ID")
	confirm := fs.Bool("confirm", false, "Confirm the remove-group operation")
	authFlags := bindWebSessionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-group",
		ShortUsage: "asc web review subscriptions remove-group --app APP_ID --group-id GROUP_ID --confirm [flags]",
		ShortHelp:  "[experimental] Remove all attached subscriptions in one group.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedAppID := strings.TrimSpace(*appID)
			trimmedGroupID := strings.TrimSpace(*groupID)
			switch {
			case trimmedAppID == "":
				return shared.UsageError("--app is required")
			case trimmedGroupID == "":
				return shared.UsageError("--group-id is required")
			case !*confirm:
				return shared.UsageError("--confirm is required")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			session, err := resolveWebSessionForCommand(requestCtx, authFlags)
			if err != nil {
				return err
			}
			client := newWebClientFn(session)

			subscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Loading review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions remove-group")
			}
			groupSubscriptions := findReviewSubscriptionsByGroup(subscriptions, trimmedGroupID)
			if len(groupSubscriptions) == 0 {
				return fmt.Errorf("subscription group %q was not found for app %q", trimmedGroupID, trimmedAppID)
			}

			skipped := make([]reviewSubscriptionMutationSkip, 0)
			removeIDs := make([]string, 0)
			for _, subscription := range groupSubscriptions {
				if subscription.SubmitWithNextAppStoreVersion {
					removeIDs = append(removeIDs, strings.TrimSpace(subscription.ID))
					continue
				}
				skipped = append(skipped, reviewSubscriptionMutationSkip{Subscription: subscription, Reason: "not attached"})
			}

			if len(removeIDs) > 0 {
				err = withWebSpinner("Removing subscription group from next app version", func() error {
					for _, subscriptionID := range removeIDs {
						if err := client.DeleteSubscriptionSubmission(requestCtx, subscriptionID); err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					return withWebAuthHint(err, "web review subscriptions remove-group")
				}
			}

			refreshedSubscriptions, err := loadReviewSubscriptionsWithLabel(requestCtx, client, trimmedAppID, "Refreshing review subscriptions")
			if err != nil {
				return withWebAuthHint(err, "web review subscriptions remove-group")
			}
			refreshedGroup := findReviewSubscriptionsByGroup(refreshedSubscriptions, trimmedGroupID)

			changed, postRefreshSkipped := collectReviewSubscriptionGroupChanges(
				refreshedGroup,
				removeIDs,
				false,
				reviewSubscriptionRemoveUnchangedAfterRefreshReason(),
			)
			skipped = append(skipped, postRefreshSkipped...)

			payload := reviewSubscriptionGroupMutationOutput{
				AppID:              trimmedAppID,
				GroupID:            trimmedGroupID,
				GroupReferenceName: reviewSubscriptionGroupLabel(refreshedGroup, trimmedGroupID),
				Operation:          "remove-group",
				ChangedCount:       len(changed),
				SkippedCount:       len(skipped),
				Changed:            changed,
				Skipped:            skipped,
			}

			return shared.PrintOutputWithRenderers(
				payload,
				*output.Output,
				*output.Pretty,
				func() error { return renderReviewSubscriptionGroupMutationTable(payload) },
				func() error { return renderReviewSubscriptionGroupMutationMarkdown(payload) },
			)
		},
	}
}
