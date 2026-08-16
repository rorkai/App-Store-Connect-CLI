package submit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func runSubmissionLocalizationPreflight(
	ctx context.Context,
	client *asc.Client,
	appID, versionID, platform string,
	requestTimeout time.Duration,
	errorPrefix, retryCommand string,
) error {
	localizationsCtx, localizationsCancel := submitPreflightRequestContext(ctx, requestTimeout)
	localizations, err := client.GetAppStoreVersionLocalizations(localizationsCtx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	localizationsCancel()
	if err != nil {
		return submissionPreflightWrap(errorPrefix, fmt.Errorf("failed to fetch version localizations for preflight: %w", err))
	}
	if len(localizations.Data) == 0 {
		fmt.Fprintln(os.Stderr, "Submit preflight failed: no app store version localizations found for this version.")
		return submissionPreflightWrap(
			errorPrefix,
			shared.WithDiagnostic(
				shared.NewValidationError(errors.New("submit preflight failed")),
				shared.DiagnosticStateNotReady,
				"",
			),
		)
	}

	updateCtx, updateCancel := submitPreflightRequestContext(ctx, requestTimeout)
	requireWhatsNew, err := isAppUpdate(updateCtx, client, appID, platform)
	updateCancel()
	if err != nil {
		return submissionPreflightWrap(errorPrefix, fmt.Errorf("failed to determine whether version is an app update for preflight: %w", err))
	}

	opts := shared.SubmitReadinessOptions{
		RequireWhatsNew: requireWhatsNew,
	}

	issues := shared.SubmitReadinessIssuesByLocaleWithOptions(localizations.Data, opts)
	if len(issues) == 0 {
		return nil
	}

	fmt.Fprintln(os.Stderr, "Submit preflight failed: submission-blocking localization fields are missing:")
	for _, issue := range issues {
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", issue.Locale, strings.Join(issue.MissingFields, ", "))
	}
	fmt.Fprintf(os.Stderr, "Fix these with `asc metadata push` or `asc apps info edit` before retrying `%s`.\n", normalizeSubmissionRetryCommand(retryCommand))
	return submissionPreflightWrap(
		errorPrefix,
		shared.WithDiagnostic(
			shared.NewValidationError(errors.New("submit preflight failed")),
			shared.DiagnosticStateNotReady,
			"",
		),
	)
}

// isAppUpdate returns true if the target platform has ever been released,
// meaning this submission is an update and whatsNew is required. Checks for
// READY_FOR_SALE as well as removed-from-sale states, since apps that were
// previously published then removed are still considered updates by Apple.
func isAppUpdate(ctx context.Context, client *asc.Client, appID, platform string) (bool, error) {
	return shared.AppUpdateRequiresWhatsNew(ctx, client, appID, platform)
}

func runSubmissionSubscriptionPreflight(ctx context.Context, client *asc.Client, appID string, requestTimeout time.Duration, retryCommand string) {
	groups, warning := fetchSubscriptionPreflightGroups(ctx, client, appID, requestTimeout)
	if warning != "" {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintf(os.Stderr, "Warning: subscription preflight could not check subscriptions: %s.\n", warning)
		return
	}
	if len(groups) == 0 {
		return
	}

	var readyToSubmit []string
	var missingMetadata []string
	var skippedGroups []string

	for _, group := range groups {
		groupID := strings.TrimSpace(group.ID)
		if groupID == "" {
			continue
		}
		groupLabel := subscriptionPreflightGroupLabel(group)

		subs, warning := fetchSubscriptionPreflightSubscriptions(ctx, client, groupID, requestTimeout)
		if warning != "" {
			skippedGroups = append(skippedGroups, fmt.Sprintf("%s: %s", groupLabel, warning))
			continue
		}

		for _, sub := range subs {
			state := strings.ToUpper(strings.TrimSpace(sub.Attributes.State))
			label := strings.TrimSpace(sub.Attributes.Name)
			if label == "" {
				label = strings.TrimSpace(sub.Attributes.ProductID)
			}
			if label == "" {
				label = sub.ID
			}

			switch state {
			case "READY_TO_SUBMIT":
				readyToSubmit = append(readyToSubmit, label)
			case "MISSING_METADATA":
				missingMetadata = append(missingMetadata, label)
			}
		}
	}

	if len(missingMetadata) > 0 {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Warning: the following subscriptions are MISSING_METADATA and will not be included in review:")
		for _, name := range missingMetadata {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
		fmt.Fprintln(os.Stderr, "Run `asc validate subscriptions` for details on what's missing.")
	}

	if len(readyToSubmit) > 0 {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Warning: the following subscriptions are READY_TO_SUBMIT but are not automatically included in this submission:")
		for _, name := range readyToSubmit {
			fmt.Fprintf(os.Stderr, "  - %s\n", name)
		}
		fmt.Fprintf(
			os.Stderr,
			"If this is their first review, run `asc web review subscriptions list --app \"APP_ID\"` to find the relevant IDs, then attach the group with `asc web review subscriptions attach-group --app \"APP_ID\" --group-id \"GROUP_ID\" --confirm` (or use `attach --subscription-id \"SUB_ID\"` for one subscription) before retrying `%s`.\n",
			normalizeSubmissionRetryCommand(retryCommand),
		)
		fmt.Fprintln(os.Stderr, "For subsequent reviews, use `asc subscriptions versions list --subscription-id \"SUB_ID\"` (or `asc subscriptions versions create --subscription-id \"SUB_ID\"` when a new version is needed), then add that version with `asc review items add --submission \"SUBMISSION_ID\" --item-type subscriptionVersions --item-id \"VERSION_ID\"`.")
	}

	if len(skippedGroups) > 0 {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Warning: some subscription groups could not be fully checked during preflight:")
		for _, skipped := range skippedGroups {
			fmt.Fprintf(os.Stderr, "  - %s\n", skipped)
		}
		fmt.Fprintln(os.Stderr, "The warnings above only cover the groups that could be checked.")
	}
}

func submissionPreflightWrap(prefix string, err error) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(prefix) == "" {
		return err
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

func normalizeSubmissionRetryCommand(retryCommand string) string {
	trimmed := strings.TrimSpace(retryCommand)
	if trimmed == "" {
		return "asc review submit"
	}
	return trimmed
}

func fetchSubscriptionPreflightGroups(ctx context.Context, client *asc.Client, appID string, requestTimeout time.Duration) ([]asc.Resource[asc.SubscriptionGroupAttributes], string) {
	firstCtx, firstCancel := submitPreflightRequestContext(ctx, requestTimeout)
	resp, err := client.GetSubscriptionGroups(firstCtx, appID, asc.WithSubscriptionGroupsLimit(200))
	firstCancel()
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscription groups")
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := submitPreflightRequestContext(ctx, requestTimeout)
		defer pageCancel()
		return client.GetSubscriptionGroups(pageCtx, appID, asc.WithSubscriptionGroupsNextURL(nextURL))
	})
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscription groups")
	}

	typed, ok := paginated.(*asc.SubscriptionGroupsResponse)
	if !ok {
		return nil, fmt.Sprintf("received unexpected subscription groups response type %T", paginated)
	}
	return typed.Data, ""
}

func fetchSubscriptionPreflightSubscriptions(ctx context.Context, client *asc.Client, groupID string, requestTimeout time.Duration) ([]asc.Resource[asc.SubscriptionAttributes], string) {
	firstCtx, firstCancel := submitPreflightRequestContext(ctx, requestTimeout)
	resp, err := client.GetSubscriptions(firstCtx, groupID, asc.WithSubscriptionsLimit(200))
	firstCancel()
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscriptions for this group")
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := submitPreflightRequestContext(ctx, requestTimeout)
		defer pageCancel()
		return client.GetSubscriptions(pageCtx, groupID, asc.WithSubscriptionsNextURL(nextURL))
	})
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscriptions for this group")
	}

	typed, ok := paginated.(*asc.SubscriptionsResponse)
	if !ok {
		return nil, fmt.Sprintf("received unexpected subscriptions response type %T", paginated)
	}
	return typed.Data, ""
}

func submitPreflightRequestContext(ctx context.Context, requestTimeout time.Duration) (context.Context, context.CancelFunc) {
	if requestTimeout > 0 {
		return shared.ContextWithTimeoutDuration(shared.ContextWithoutTimeout(ctx), requestTimeout)
	}
	if ctx != nil {
		if _, ok := ctx.Deadline(); ok {
			return ctx, func() {}
		}
	}
	return shared.ContextWithTimeout(ctx)
}

func subscriptionPreflightGroupLabel(group asc.Resource[asc.SubscriptionGroupAttributes]) string {
	name := strings.TrimSpace(group.Attributes.ReferenceName)
	id := strings.TrimSpace(group.ID)
	switch {
	case name != "" && id != "":
		return fmt.Sprintf("%s (%s)", name, id)
	case name != "":
		return name
	case id != "":
		return id
	default:
		return "(unknown group)"
	}
}

func subscriptionPreflightSkipReason(err error, resourceLabel string) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("App Store Connect timed out while loading %s", resourceLabel)
	}
	if errors.Is(err, asc.ErrForbidden) || asc.IsUnauthorized(err) {
		return fmt.Sprintf("this App Store Connect account cannot read %s", resourceLabel)
	}
	if asc.IsRetryable(err) {
		return fmt.Sprintf("App Store Connect was temporarily unavailable while loading %s", resourceLabel)
	}
	if asc.IsNotFound(err) {
		return fmt.Sprintf("App Store Connect reported %s as not found", resourceLabel)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return fmt.Sprintf("App Store Connect could not be reached while loading %s", resourceLabel)
	}
	return fmt.Sprintf("failed to load %s: %v", resourceLabel, err)
}
