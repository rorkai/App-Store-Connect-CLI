package submit

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func SubmitCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc submit <subcommand> [flags]",
		ShortHelp:  "Submit builds for App Store review.",
		LongHelp:   `Submit builds for App Store review.`,
		UsageFunc:  shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubmitCreateCommand(),
			SubmitStatusCommand(),
			SubmitCancelCommand(),
			SubmitPreflightCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func SubmitCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	version := fs.String("version", "", "App Store version string")
	versionID := fs.String("version-id", "", "App Store version ID")
	buildID := fs.String("build", "", "Build ID to attach")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	confirm := fs.Bool("confirm", false, "Confirm submission (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc submit create [flags]",
		ShortHelp:  "Submit a build for App Store review.",
		LongHelp: `Submit a build for App Store review.

Examples:
  asc submit create --app "123456789" --version "1.0.0" --build "BUILD_ID" --confirm
  asc submit create --app "123456789" --version-id "VERSION_ID" --build "BUILD_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to submit for review")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*buildID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*version) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version or --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*version) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.UsageError("--version and --version-id are mutually exclusive")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit create: %w", err)
			}

			resolvedVersionID := strings.TrimSpace(*versionID)
			effectivePlatform := normalizedPlatform
			if resolvedVersionID == "" {
				resolveCtx, resolveCancel := shared.ContextWithTimeout(ctx)
				resolvedVersionID, err = shared.ResolveAppStoreVersionID(resolveCtx, client, resolvedAppID, strings.TrimSpace(*version), normalizedPlatform)
				resolveCancel()
				if err != nil {
					return fmt.Errorf("submit create: %w", err)
				}
			} else {
				versionCtx, versionCancel := shared.ContextWithTimeout(ctx)
				versionResp, versionErr := client.GetAppStoreVersion(versionCtx, resolvedVersionID)
				versionCancel()
				if versionErr != nil {
					return fmt.Errorf("submit create: failed to fetch version %q: %w", resolvedVersionID, versionErr)
				}

				effectivePlatform, err = shared.NormalizeAppStoreVersionPlatform(string(versionResp.Data.Attributes.Platform))
				if err != nil {
					return fmt.Errorf("submit create: version %q returned unsupported platform %q", resolvedVersionID, string(versionResp.Data.Attributes.Platform))
				}
			}

			if err := runSubmitCreateLocalizationPreflight(ctx, client, resolvedAppID, resolvedVersionID, effectivePlatform); err != nil {
				return err
			}

			runSubmitCreateSubscriptionPreflight(ctx, client, resolvedAppID)

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Attach build to version
			if err := client.AttachBuildToVersion(requestCtx, resolvedVersionID, strings.TrimSpace(*buildID)); err != nil {
				return fmt.Errorf("submit create: failed to attach build: %w", err)
			}

			// Cancel stale READY_FOR_REVIEW submissions to avoid orphans from prior failed attempts.
			canceledStaleSubmissionIDs := cancelStaleReviewSubmissions(requestCtx, client, resolvedAppID, effectivePlatform)

			// Use the new reviewSubmissions API (the old appStoreVersionSubmissions is deprecated)
			// Step 1: Create review submission for the app
			reviewSubmission, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(effectivePlatform))
			if err != nil {
				return fmt.Errorf("submit create: failed to create review submission: %w", err)
			}

			// Step 2: Add the app store version as a submission item.
			// If the version is already in another submission, recover by
			// submitting that existing submission instead. If the conflicting
			// submission is one we just canceled as stale, retry the add until
			// App Store Connect finishes detaching the version.
			submissionIDToSubmit, err := addVersionToSubmissionOrRecover(
				requestCtx,
				client,
				reviewSubmission.Data.ID,
				resolvedVersionID,
				canceledStaleSubmissionIDs,
			)
			if err != nil {
				cleanupEmptyReviewSubmission(requestCtx, client, reviewSubmission.Data.ID)
				printSubmissionErrorHints(err, resolvedAppID)
				return fmt.Errorf("submit create: failed to add version to submission: %w", err)
			}
			if submissionIDToSubmit != reviewSubmission.Data.ID {
				cleanupEmptyReviewSubmission(requestCtx, client, reviewSubmission.Data.ID)
			}

			// Step 3: Submit for review
			submitResp, err := client.SubmitReviewSubmission(requestCtx, submissionIDToSubmit)
			if err != nil {
				printSubmissionErrorHints(err, resolvedAppID)
				return fmt.Errorf("submit create: failed to submit for review: %w", err)
			}

			submittedDate := submitResp.Data.Attributes.SubmittedDate
			var createdDatePtr *string
			if submittedDate != "" {
				createdDatePtr = &submittedDate
			}
			result := &asc.AppStoreVersionSubmissionCreateResult{
				SubmissionID: submitResp.Data.ID,
				VersionID:    resolvedVersionID,
				BuildID:      strings.TrimSpace(*buildID),
				CreatedDate:  createdDatePtr,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func runSubmitCreateLocalizationPreflight(ctx context.Context, client *asc.Client, appID, versionID, platform string) error {
	localizationsCtx, localizationsCancel := shared.ContextWithTimeout(ctx)
	localizations, err := client.GetAppStoreVersionLocalizations(localizationsCtx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	localizationsCancel()
	if err != nil {
		return fmt.Errorf("submit create: failed to fetch version localizations for preflight: %w", err)
	}
	if len(localizations.Data) == 0 {
		fmt.Fprintln(os.Stderr, "Submit preflight failed: no app store version localizations found for this version.")
		return fmt.Errorf("submit create: submit preflight failed")
	}

	updateCtx, updateCancel := shared.ContextWithTimeout(ctx)
	requireWhatsNew, err := isAppUpdate(updateCtx, client, appID, platform)
	updateCancel()
	if err != nil {
		return fmt.Errorf("submit create: failed to determine whether version is an app update for preflight: %w", err)
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
	fmt.Fprintln(os.Stderr, "Fix these with `asc metadata push` or `asc apps info edit` before retrying submit create.")
	return fmt.Errorf("submit create: submit preflight failed")
}

// isAppUpdate returns true if the target platform has ever been released,
// meaning this submission is an update and whatsNew is required. Checks for
// READY_FOR_SALE as well as removed-from-sale states, since apps that were
// previously published then removed are still considered updates by Apple.
func isAppUpdate(ctx context.Context, client *asc.Client, appID, platform string) (bool, error) {
	opts := []asc.AppStoreVersionsOption{
		asc.WithAppStoreVersionsStates([]string{
			"READY_FOR_SALE",
			"DEVELOPER_REMOVED_FROM_SALE",
			"REMOVED_FROM_SALE",
		}),
		asc.WithAppStoreVersionsLimit(1),
	}
	if strings.TrimSpace(platform) != "" {
		opts = append(opts, asc.WithAppStoreVersionsPlatforms([]string{platform}))
	}

	versions, err := client.GetAppStoreVersions(ctx, appID, opts...)
	if err != nil {
		return false, err
	}
	return len(versions.Data) > 0, nil
}

func SubmitStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit status", flag.ExitOnError)

	submissionID := fs.String("id", "", "Submission ID")
	versionID := fs.String("version-id", "", "App Store version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc submit status [flags]",
		ShortHelp:  "Check submission status.",
		LongHelp: `Check submission status.

Examples:
  asc submit status --id "SUBMISSION_ID"
  asc submit status --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.UsageError("--id and --version-id are mutually exclusive")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit status: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedVersionID := strings.TrimSpace(*versionID)
			result := &asc.AppStoreVersionSubmissionStatusResult{}
			if resolvedSubmissionID := strings.TrimSpace(*submissionID); resolvedSubmissionID != "" {
				reviewSubmission, reviewErr := client.GetReviewSubmission(requestCtx, resolvedSubmissionID)
				if reviewErr != nil {
					if asc.IsNotFound(reviewErr) {
						return fmt.Errorf(
							"submit status: no review submission found for ID %q; retry with --version-id to inspect the App Store version state",
							resolvedSubmissionID,
						)
					}
					return fmt.Errorf("submit status: failed to fetch review submission %q: %w", resolvedSubmissionID, reviewErr)
				}

				applyReviewSubmissionStatus(result, &reviewSubmission.Data)
				resolvedVersionID, err = resolveReviewSubmissionVersionID(requestCtx, client, &reviewSubmission.Data)
				if err != nil {
					if !shouldIgnoreReviewSubmissionVersionLookupError(err) {
						return fmt.Errorf("submit status: %w", err)
					}
					resolvedVersionID = ""
				}
			} else {
				versionResp, versionErr := client.GetAppStoreVersion(requestCtx, resolvedVersionID, asc.WithAppStoreVersionInclude([]string{"app"}))
				if versionErr != nil {
					if asc.IsNotFound(versionErr) {
						return fmt.Errorf("submit status: no version found for ID %q", resolvedVersionID)
					}
					return fmt.Errorf("submit status: %w", versionErr)
				}
				applyVersionStatus(result, versionResp)

				if appID, appErr := resolveAppIDFromVersionResponse(versionResp); appErr == nil {
					reviewSubmission, reviewErr := findReviewSubmissionForVersion(requestCtx, client, appID, resolvedVersionID)
					if reviewErr != nil {
						if !shouldIgnoreReviewSubmissionVersionLookupError(reviewErr) {
							return fmt.Errorf("submit status: %w", reviewErr)
						}
					} else if reviewSubmission != nil {
						applyReviewSubmissionStatus(result, reviewSubmission)
						return shared.PrintOutput(result, *output.Output, *output.Pretty)
					}
				}

				legacySubmission, legacyErr := client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if legacyErr == nil {
					applyLegacySubmissionStatus(result, legacySubmission)
				} else if !asc.IsNotFound(legacyErr) {
					return fmt.Errorf("submit status: %w", legacyErr)
				}

				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			if resolvedVersionID != "" {
				versionResp, err := client.GetAppStoreVersion(requestCtx, resolvedVersionID)
				if err != nil {
					return fmt.Errorf("submit status: %w", err)
				}
				applyVersionStatus(result, versionResp)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

type submitStatusVersionRelationships struct {
	App *asc.Relationship `json:"app"`
}

func applyReviewSubmissionStatus(result *asc.AppStoreVersionSubmissionStatusResult, submission *asc.ReviewSubmissionResource) {
	if result == nil || submission == nil {
		return
	}
	result.ID = strings.TrimSpace(submission.ID)
	if submittedDate := strings.TrimSpace(submission.Attributes.SubmittedDate); submittedDate != "" {
		result.CreatedDate = &submittedDate
	}
	if state := strings.TrimSpace(string(submission.Attributes.SubmissionState)); state != "" {
		result.State = state
	}
}

func applyLegacySubmissionStatus(result *asc.AppStoreVersionSubmissionStatusResult, submission *asc.AppStoreVersionSubmissionResourceResponse) {
	if result == nil || submission == nil {
		return
	}
	result.ID = strings.TrimSpace(submission.Data.ID)
	result.CreatedDate = submission.Data.Attributes.CreatedDate
	if result.VersionID == "" && submission.Data.Relationships.AppStoreVersion != nil {
		result.VersionID = strings.TrimSpace(submission.Data.Relationships.AppStoreVersion.Data.ID)
	}
}

func applyVersionStatus(result *asc.AppStoreVersionSubmissionStatusResult, versionResp *asc.AppStoreVersionResponse) {
	if result == nil || versionResp == nil {
		return
	}
	result.VersionID = strings.TrimSpace(versionResp.Data.ID)
	result.VersionString = strings.TrimSpace(versionResp.Data.Attributes.VersionString)
	result.Platform = strings.TrimSpace(string(versionResp.Data.Attributes.Platform))
	if result.State == "" {
		result.State = shared.ResolveAppStoreVersionState(versionResp.Data.Attributes)
	}
}

func resolveAppIDFromVersionResponse(versionResp *asc.AppStoreVersionResponse) (string, error) {
	if versionResp == nil {
		return "", fmt.Errorf("version response is required")
	}
	if len(versionResp.Data.Relationships) == 0 {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionResp.Data.ID)
	}
	var relationships submitStatusVersionRelationships
	if err := json.Unmarshal(versionResp.Data.Relationships, &relationships); err != nil {
		return "", fmt.Errorf("failed to parse app store version relationships: %w", err)
	}
	if relationships.App == nil {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionResp.Data.ID)
	}
	appID := strings.TrimSpace(relationships.App.Data.ID)
	if appID == "" {
		return "", fmt.Errorf("app relationship missing for app store version %q", versionResp.Data.ID)
	}
	return appID, nil
}

func resolveReviewSubmissionVersionID(ctx context.Context, client *asc.Client, submission *asc.ReviewSubmissionResource) (string, error) {
	if submission == nil {
		return "", nil
	}
	if submission.Relationships != nil && submission.Relationships.AppStoreVersionForReview != nil {
		if versionID := strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID); versionID != "" {
			return versionID, nil
		}
	}
	return resolveReviewSubmissionVersionIDFromItems(ctx, client, strings.TrimSpace(submission.ID))
}

func resolveReviewSubmissionVersionIDFromItems(ctx context.Context, client *asc.Client, submissionID string) (string, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return "", nil
	}

	opts := []asc.ReviewSubmissionItemsOption{
		asc.WithReviewSubmissionItemsFields([]string{"appStoreVersion"}),
		asc.WithReviewSubmissionItemsLimit(200),
	}
	resp, err := client.GetReviewSubmissionItems(ctx, submissionID, opts...)
	if err != nil {
		return "", err
	}

	for {
		if versionID := reviewSubmissionVersionIDFromItems(resp.Data); versionID != "" {
			return versionID, nil
		}
		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return "", nil
		}
		resp, err = client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL))
		if err != nil {
			return "", err
		}
	}
}

func reviewSubmissionVersionIDFromItems(items []asc.ReviewSubmissionItemResource) string {
	for _, item := range items {
		if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
			continue
		}
		if versionID := strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID); versionID != "" {
			return versionID
		}
	}
	return ""
}

func findReviewSubmissionForVersion(ctx context.Context, client *asc.Client, appID, versionID string) (*asc.ReviewSubmissionResource, error) {
	appID = strings.TrimSpace(appID)
	versionID = strings.TrimSpace(versionID)
	if appID == "" || versionID == "" || client == nil {
		return nil, nil
	}

	resp, err := client.GetReviewSubmissions(
		ctx,
		appID,
		asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		asc.WithReviewSubmissionsLimit(200),
	)
	if err != nil {
		return nil, err
	}

	var candidates []asc.ReviewSubmissionResource
	for {
		for i := range resp.Data {
			submission := resp.Data[i]
			submissionVersionID, err := resolveReviewSubmissionVersionID(ctx, client, &submission)
			if err != nil {
				if !shouldIgnoreReviewSubmissionVersionLookupError(err) {
					return nil, err
				}
				continue
			}
			if submissionVersionID == versionID {
				candidates = append(candidates, submission)
			}
		}

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			break
		}
		resp, err = client.GetReviewSubmissions(ctx, appID, asc.WithReviewSubmissionsNextURL(nextURL))
		if err != nil {
			return nil, err
		}
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return reviewSubmissionSortKey(candidates[i]).less(reviewSubmissionSortKey(candidates[j]))
	})
	best := candidates[0]
	return &best, nil
}

func shouldIgnoreReviewSubmissionVersionLookupError(err error) bool {
	return asc.IsNotFound(err) || errors.Is(err, asc.ErrForbidden)
}

type reviewSubmissionCandidateKey struct {
	statePriority int
	submittedAt   time.Time
	hasSubmitted  bool
	id            string
}

func reviewSubmissionSortKey(submission asc.ReviewSubmissionResource) reviewSubmissionCandidateKey {
	submittedAt, hasSubmitted := parseReviewSubmissionSubmittedDate(submission.Attributes.SubmittedDate)
	return reviewSubmissionCandidateKey{
		statePriority: reviewSubmissionStatePriority(submission.Attributes.SubmissionState),
		submittedAt:   submittedAt,
		hasSubmitted:  hasSubmitted,
		id:            strings.TrimSpace(submission.ID),
	}
}

func (k reviewSubmissionCandidateKey) less(other reviewSubmissionCandidateKey) bool {
	if k.statePriority != other.statePriority {
		return k.statePriority > other.statePriority
	}
	if k.hasSubmitted != other.hasSubmitted {
		return k.hasSubmitted
	}
	if !k.submittedAt.Equal(other.submittedAt) {
		return k.submittedAt.After(other.submittedAt)
	}
	return k.id > other.id
}

func reviewSubmissionStatePriority(state asc.ReviewSubmissionState) int {
	switch state {
	case asc.ReviewSubmissionStateInReview:
		return 70
	case asc.ReviewSubmissionStateWaitingForReview:
		return 60
	case asc.ReviewSubmissionStateUnresolvedIssues:
		return 50
	case asc.ReviewSubmissionStateReadyForReview:
		return 40
	case asc.ReviewSubmissionStateCompleting:
		return 30
	case asc.ReviewSubmissionStateCanceling:
		return 20
	case asc.ReviewSubmissionStateComplete:
		return 10
	default:
		return 0
	}
}

func parseReviewSubmissionSubmittedDate(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func SubmitCancelCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit cancel", flag.ExitOnError)

	submissionID := fs.String("id", "", "Submission ID")
	versionID := fs.String("version-id", "", "App Store version ID")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "cancel",
		ShortUsage: "asc submit cancel [flags]",
		ShortHelp:  "Cancel a submission.",
		LongHelp: `Cancel a submission.

Examples:
  asc submit cancel --id "SUBMISSION_ID" --confirm
  asc submit cancel --version-id "VERSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to cancel a submission")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) == "" && strings.TrimSpace(*versionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-id is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) != "" && strings.TrimSpace(*versionID) != "" {
				return shared.UsageError("--id and --version-id are mutually exclusive")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("submit cancel: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedSubmissionID := strings.TrimSpace(*submissionID)
			if resolvedSubmissionID != "" {
				_, err := client.CancelReviewSubmission(requestCtx, resolvedSubmissionID)
				if err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no review submission found for ID %q", resolvedSubmissionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
			} else {
				resolvedVersionID := strings.TrimSpace(*versionID)

				// Resolve via legacy version submission lookup for backward compatibility.
				submissionResp, err := client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no legacy submission found for version %q", resolvedVersionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
				resolvedSubmissionID = strings.TrimSpace(submissionResp.Data.ID)
				if resolvedSubmissionID == "" {
					return fmt.Errorf("submit cancel: no legacy submission found for version %q", resolvedVersionID)
				}

				// Prefer the modern reviewSubmissions cancel endpoint when possible.
				_, err = client.CancelReviewSubmission(requestCtx, resolvedSubmissionID)
				if err == nil {
					result := &asc.AppStoreVersionSubmissionCancelResult{
						ID:        resolvedSubmissionID,
						Cancelled: true,
					}
					return shared.PrintOutput(result, *output.Output, *output.Pretty)
				}
				if !asc.IsNotFound(err) {
					return fmt.Errorf("submit cancel: %w", err)
				}

				// Fall back to the legacy delete endpoint for old submission flows.
				if err := client.DeleteAppStoreVersionSubmission(requestCtx, resolvedSubmissionID); err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no legacy submission found for ID %q", resolvedSubmissionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
			}

			result := &asc.AppStoreVersionSubmissionCancelResult{
				ID:        resolvedSubmissionID,
				Cancelled: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// runSubmitCreateSubscriptionPreflight checks whether the app has subscriptions
// that need attention before submission. This is advisory (warnings only) because
// the submit flow cannot include subscriptions in the review submission — they
// use a separate submission path.
func runSubmitCreateSubscriptionPreflight(ctx context.Context, client *asc.Client, appID string) {
	groups, warning := fetchSubscriptionPreflightGroups(ctx, client, appID)
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

		subs, warning := fetchSubscriptionPreflightSubscriptions(ctx, client, groupID)
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
		fmt.Fprintln(os.Stderr, "If this is their first review, you must submit them via the app version page in App Store Connect.")
		fmt.Fprintln(os.Stderr, "For subsequent reviews, use `asc subscriptions review submit --subscription-id \"SUB_ID\" --confirm`.")
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

func fetchSubscriptionPreflightGroups(ctx context.Context, client *asc.Client, appID string) ([]asc.Resource[asc.SubscriptionGroupAttributes], string) {
	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptionGroups(firstCtx, appID, asc.WithSubscriptionGroupsLimit(200))
	firstCancel()
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscription groups")
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
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

func fetchSubscriptionPreflightSubscriptions(ctx context.Context, client *asc.Client, groupID string) ([]asc.Resource[asc.SubscriptionAttributes], string) {
	firstCtx, firstCancel := shared.ContextWithTimeout(ctx)
	resp, err := client.GetSubscriptions(firstCtx, groupID, asc.WithSubscriptionsLimit(200))
	firstCancel()
	if err != nil {
		return nil, subscriptionPreflightSkipReason(err, "subscriptions for this group")
	}

	paginated, err := asc.PaginateAll(ctx, resp, func(_ context.Context, nextURL string) (asc.PaginatedResponse, error) {
		pageCtx, pageCancel := shared.ContextWithTimeout(ctx)
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

var submitCreateRecentlyCanceledRetryDelays = []time.Duration{
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2 * time.Second,
}

// alreadyAddedPattern matches Apple's error message when a version is already
// in another review submission. The capture group extracts the submission ID.
// Uses \S+ rather than a strict UUID pattern because the API spec defines
// ReviewSubmission.id as a generic string.
var alreadyAddedPattern = regexp.MustCompile(
	`(?i)already added to another reviewSubmission with id\s+(\S+)`,
)

// extractExistingSubmissionID inspects an error returned by AddReviewSubmissionItem
// to see if it indicates the version is already in another review submission.
// If so, it returns the existing submission's ID; otherwise it returns "".
func extractExistingSubmissionID(err error) string {
	var apiErr *asc.APIError
	if !errors.As(err, &apiErr) {
		return ""
	}
	for _, entries := range apiErr.AssociatedErrors {
		for _, entry := range entries {
			if m := alreadyAddedPattern.FindStringSubmatch(entry.Detail); len(m) == 2 {
				return m[1]
			}
		}
	}
	return ""
}

func addVersionToSubmissionOrRecover(
	ctx context.Context,
	client *asc.Client,
	submissionID, versionID string,
	recentlyCanceledSubmissionIDs map[string]struct{},
) (string, error) {
	for attempt := 0; ; attempt++ {
		_, err := client.AddReviewSubmissionItem(ctx, submissionID, versionID)
		if err == nil {
			return submissionID, nil
		}

		existingID := extractExistingSubmissionID(err)
		if existingID == "" {
			return "", err
		}
		if _, ok := recentlyCanceledSubmissionIDs[existingID]; !ok {
			fmt.Fprintf(os.Stderr, "Version already in review submission %s, reusing it.\n", existingID)
			return existingID, nil
		}
		if attempt >= len(submitCreateRecentlyCanceledRetryDelays) {
			return "", fmt.Errorf(
				"version is still attached to recently canceled review submission %s after %d retries: %w",
				existingID,
				len(submitCreateRecentlyCanceledRetryDelays),
				err,
			)
		}

		delay := submitCreateRecentlyCanceledRetryDelays[attempt]
		fmt.Fprintf(
			os.Stderr,
			"Version is still detaching from recently canceled review submission %s, retrying add in %s.\n",
			existingID,
			delay,
		)
		if err := sleepWithContext(ctx, delay); err != nil {
			return "", fmt.Errorf("waiting for recently canceled review submission %s to clear: %w", existingID, err)
		}
	}
}

func cleanupEmptyReviewSubmission(ctx context.Context, client *asc.Client, submissionID string) {
	if strings.TrimSpace(submissionID) == "" {
		return
	}
	if _, cancelErr := client.CancelReviewSubmission(ctx, submissionID); cancelErr != nil && !isExpectedNonCancellableReviewSubmissionError(cancelErr) {
		fmt.Fprintf(os.Stderr, "Warning: failed to cancel empty submission %s: %v\n", submissionID, cancelErr)
	}
}

// cancelStaleReviewSubmissions cancels any READY_FOR_REVIEW submissions for the
// given app and platform. These are orphans from prior failed submit attempts.
// Errors are logged to stderr but do not block the new submission.
func cancelStaleReviewSubmissions(ctx context.Context, client *asc.Client, appID, platform string) map[string]struct{} {
	existing, err := client.GetReviewSubmissions(ctx, appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)}),
		asc.WithReviewSubmissionsPlatforms([]string{platform}),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to query stale review submissions: %v\n", err)
		return nil
	}
	if len(existing.Data) == 0 {
		return nil
	}

	canceledSubmissionIDs := make(map[string]struct{}, len(existing.Data))
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(platform))
	for _, sub := range existing.Data {
		// Defensively re-check state/platform before canceling.
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}

		if _, cancelErr := client.CancelReviewSubmission(ctx, sub.ID); cancelErr != nil {
			if isExpectedNonCancellableReviewSubmissionError(cancelErr) {
				fmt.Fprintf(os.Stderr, "Skipped stale submission %s: already transitioned to a non-cancellable state\n", sub.ID)
			} else {
				fmt.Fprintf(os.Stderr, "Warning: failed to cancel stale submission %s: %v\n", sub.ID, cancelErr)
			}
			continue
		}
		canceledSubmissionIDs[sub.ID] = struct{}{}
		fmt.Fprintf(os.Stderr, "Canceled stale review submission %s\n", sub.ID)
	}

	if len(canceledSubmissionIDs) == 0 {
		return nil
	}
	return canceledSubmissionIDs
}

// printSubmissionErrorHints inspects an error returned by App Store Connect
// during submission and prints actionable fix suggestions to stderr.
func printSubmissionErrorHints(err error, appID string) {
	if err == nil {
		return
	}
	errMsg := err.Error()

	var hints []string
	if strings.Contains(errMsg, "ageRatingDeclaration") {
		hints = append(hints,
			fmt.Sprintf("Review current age rating: asc age-rating view --app %s", appID),
			"Review age-rating update flags: asc age-rating set --help",
		)
	}
	if strings.Contains(errMsg, "contentRightsDeclaration") {
		hints = append(hints,
			fmt.Sprintf("If your app does not use third-party content: asc apps update --id %s --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT", appID),
			fmt.Sprintf("If your app uses third-party content: asc apps update --id %s --content-rights USES_THIRD_PARTY_CONTENT", appID),
		)
	}
	if strings.Contains(errMsg, "usesNonExemptEncryption") {
		hints = append(hints,
			"Set Uses Non-Exempt Encryption for the attached build in App Store Connect, then retry submission.",
		)
	}
	if strings.Contains(errMsg, "appDataUsage") {
		hints = append(hints, fmt.Sprintf("Complete App Privacy at: https://appstoreconnect.apple.com/apps/%s/appPrivacy", appID))
	}
	if strings.Contains(errMsg, "primaryCategory") {
		hints = append(hints,
			"List available categories: asc categories list",
			"Review category update flags: asc app-setup categories set --help",
		)
	}

	if len(hints) > 0 {
		fmt.Fprintln(os.Stderr, "")
		for _, hint := range hints {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", hint)
		}
	}
}

func isExpectedNonCancellableReviewSubmissionError(err error) bool {
	return isResourceStateInvalid(err)
}

// isResourceStateInvalid returns true if the error message indicates the
// resource is not in a cancellable state — an expected condition when racing
// with App Store Connect state transitions.
func isResourceStateInvalid(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Resource is not in cancellable state") ||
		strings.Contains(msg, "Resource state is invalid")
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
