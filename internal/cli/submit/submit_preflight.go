package submit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

func runSubmissionLocalizationPreflight(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appID, versionID, platform string,
	requestTimeout time.Duration,
	errorPrefix, retryCommand string,
) error {
	localizationsCtx, localizationsCancel := submitPreflightRequestContext(ctx, d, requestTimeout)
	localizations, err := client.GetAppStoreVersionLocalizations(localizationsCtx, versionID, asc.Limit[asc.AppStoreVersionLocalizationsOption](200))
	localizationsCancel()
	if err != nil {
		return submissionPreflightWrap(errorPrefix, fmt.Errorf("failed to fetch version localizations for preflight: %w", err))
	}
	if len(localizations.Data) == 0 {
		fmt.Fprintln(d.IO.ErrOut, "Submit preflight failed: no app store version localizations found for this version.")
		return submissionPreflightWrap(
			errorPrefix,
			shared.WithDiagnostic(
				shared.NewValidationError(errors.New("submit preflight failed")),
				shared.DiagnosticStateNotReady,
				"",
			),
		)
	}

	updateCtx, updateCancel := submitPreflightRequestContext(ctx, d, requestTimeout)
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

	fmt.Fprintln(d.IO.ErrOut, "Submit preflight failed: submission-blocking localization fields are missing:")
	for _, issue := range issues {
		fmt.Fprintf(d.IO.ErrOut, "  - %s: %s\n", issue.Locale, strings.Join(issue.MissingFields, ", "))
	}
	fmt.Fprintf(d.IO.ErrOut, "Fix these with `asc metadata push` or `asc apps info edit` before retrying `%s`.\n", normalizeSubmissionRetryCommand(retryCommand))
	return submissionPreflightWrap(
		errorPrefix,
		shared.WithDiagnostic(
			shared.NewValidationError(errors.New("submit preflight failed")),
			shared.DiagnosticStateNotReady,
			"",
		),
	)
}

func runSubmissionAgeRatingPreflight(
	ctx context.Context,
	d *shared.Deps,
	client *asc.Client,
	appID, versionID string,
	requestTimeout time.Duration,
	retryCommand string,
) error {
	declaration, err := validatecli.FetchAgeRatingDeclaration(ctx, client, appID, versionID, requestTimeout)
	if err != nil {
		return fmt.Errorf("age rating preflight: %w", err)
	}

	checks := validation.AgeRatingChecks(declaration)
	if len(checks) == 0 {
		return nil
	}

	fmt.Fprintln(d.IO.ErrOut, "Submit preflight failed: the age rating declaration is incomplete:")
	var remediations []string
	for _, check := range checks {
		if check.Field != "" {
			fmt.Fprintf(d.IO.ErrOut, "  - %s: %s (%s)\n", check.Field, check.Message, check.ID)
		} else {
			fmt.Fprintf(d.IO.ErrOut, "  - %s (%s)\n", check.Message, check.ID)
		}
		if !slices.Contains(remediations, check.Remediation) {
			remediations = append(remediations, check.Remediation)
		}
	}
	for _, remediation := range remediations {
		fmt.Fprintf(d.IO.ErrOut, "%s before retrying `%s`.\n", remediation, normalizeSubmissionRetryCommand(retryCommand))
	}
	return shared.WithDiagnostic(
		shared.NewValidationError(errors.New("submit preflight failed")),
		shared.DiagnosticStateNotReady,
		"",
	)
}

// isAppUpdate returns true if the target platform has ever been released,
// meaning this submission is an update and whatsNew is required. Checks for
// live versions (READY_FOR_SALE or READY_FOR_DISTRIBUTION) as well as
// removed-from-sale states, since apps that were previously published then
// removed are still considered updates by Apple.
func isAppUpdate(ctx context.Context, client *asc.Client, appID, platform string) (bool, error) {
	return shared.AppUpdateRequiresWhatsNew(ctx, client, appID, platform)
}

func runSubmissionSubscriptionPreflight(ctx context.Context, d *shared.Deps, client *asc.Client, appID string, requestTimeout time.Duration, retryCommand string) {
	groups, warning := fetchSubscriptionPreflightGroups(ctx, d, client, appID, requestTimeout)
	if warning != "" {
		fmt.Fprintln(d.IO.ErrOut, "")
		fmt.Fprintf(d.IO.ErrOut, "Warning: subscription preflight could not check subscriptions: %s.\n", warning)
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

		subs, warning := fetchSubscriptionPreflightSubscriptions(ctx, d, client, groupID, requestTimeout)
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
		fmt.Fprintln(d.IO.ErrOut, "")
		fmt.Fprintln(d.IO.ErrOut, "Warning: the following subscriptions are MISSING_METADATA and will not be included in review:")
		for _, name := range missingMetadata {
			fmt.Fprintf(d.IO.ErrOut, "  - %s\n", name)
		}
		fmt.Fprintln(d.IO.ErrOut, "Run `asc validate subscriptions` for details on what's missing.")
	}

	if len(readyToSubmit) > 0 {
		fmt.Fprintln(d.IO.ErrOut, "")
		fmt.Fprintln(d.IO.ErrOut, "Warning: the following subscriptions are READY_TO_SUBMIT but are not automatically included in this submission:")
		for _, name := range readyToSubmit {
			fmt.Fprintf(d.IO.ErrOut, "  - %s\n", name)
		}
		fmt.Fprintf(
			d.IO.ErrOut,
			"To include them, find each version with `asc subscriptions versions list --subscription-id \"SUB_ID\"` (or `asc subscriptions versions create --subscription-id \"SUB_ID\"` when a new version is needed), then add it to the app's review submission with `asc review items add --submission \"SUBMISSION_ID\" --item-type subscriptionVersions --item-id \"VERSION_ID\"` before retrying `%s`; a first subscription goes in the same submission as the app version.\n",
			normalizeSubmissionRetryCommand(retryCommand),
		)
	}

	if len(skippedGroups) > 0 {
		fmt.Fprintln(d.IO.ErrOut, "")
		fmt.Fprintln(d.IO.ErrOut, "Warning: some subscription groups could not be fully checked during preflight:")
		for _, skipped := range skippedGroups {
			fmt.Fprintf(d.IO.ErrOut, "  - %s\n", skipped)
		}
		fmt.Fprintln(d.IO.ErrOut, "The warnings above only cover the groups that could be checked.")
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

func fetchSubscriptionPreflightGroups(ctx context.Context, d *shared.Deps, client *asc.Client, appID string, requestTimeout time.Duration) ([]asc.Resource[asc.SubscriptionGroupAttributes], string) {
	firstCtx, firstCancel := submitPreflightRequestContext(ctx, d, requestTimeout)
	resp, err := client.GetSubscriptionGroups(firstCtx, appID, asc.Limit[asc.SubscriptionGroupsOption](200))
	firstCancel()
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscription groups")
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := submitPreflightRequestContext(ctx, d, requestTimeout)
		defer pageCancel()
		return client.GetSubscriptionGroups(pageCtx, appID, asc.NextURL[asc.SubscriptionGroupsOption](nextURL))
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

func fetchSubscriptionPreflightSubscriptions(ctx context.Context, d *shared.Deps, client *asc.Client, groupID string, requestTimeout time.Duration) ([]asc.Resource[asc.SubscriptionAttributes], string) {
	firstCtx, firstCancel := submitPreflightRequestContext(ctx, d, requestTimeout)
	resp, err := client.GetSubscriptions(firstCtx, groupID, asc.Limit[asc.SubscriptionsOption](200))
	firstCancel()
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscriptions for this group")
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := submitPreflightRequestContext(ctx, d, requestTimeout)
		defer pageCancel()
		return client.GetSubscriptions(pageCtx, groupID, asc.NextURL[asc.SubscriptionsOption](nextURL))
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

func submitPreflightRequestContext(ctx context.Context, d *shared.Deps, requestTimeout time.Duration) (context.Context, context.CancelFunc) {
	if requestTimeout > 0 {
		return shared.ContextWithTimeoutDuration(shared.ContextWithoutTimeout(ctx), requestTimeout)
	}
	if ctx != nil {
		if _, ok := ctx.Deadline(); ok {
			return ctx, func() {}
		}
	}
	return d.Timeout(ctx)
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
