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
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

var submitReadinessReportBuilder = validatecli.BuildReadinessReport

func SubmitCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc submit <subcommand> [flags]",
		ShortHelp:  "Submission lifecycle tools; use `publish appstore --submit` to ship.",
		LongHelp: `Submission lifecycle tools for App Store review.

Use:
  - asc publish appstore --submit for the canonical high-level App Store shipping path
  - asc validate for canonical readiness checks before submission
  - asc submit status/cancel for lower-level review submission lifecycle work

` + "`asc submit preflight`" + ` remains available as a deprecated compatibility
path for older scripts that still expect preflight-style text/json output.

` + "`asc submit create`" + ` remains available as a deprecated compatibility
path for older scripts that submit an already-prepared version directly.`,
		UsageFunc: shared.VisibleUsageFunc,
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
		ShortHelp:  "DEPRECATED: use `asc versions attach-build` + `asc review submissions-*`.",
		LongHelp: `Deprecated compatibility path for lower-level direct submission.

For already-uploaded builds, use:
  - ` + "`asc versions attach-build --version-id \"VERSION_ID\" --build \"BUILD_ID\"`" + `
  - ` + "`asc review submissions-create --app \"APP_ID\" --platform \"PLATFORM\"`" + `
  - ` + "`asc review items-add --submission \"SUBMISSION_ID\" --item-type appStoreVersions --item-id \"VERSION_ID\"`" + `
  - ` + "`asc review submissions-submit --id \"SUBMISSION_ID\" --confirm`" + `

Use ` + "`asc publish appstore --submit`" + ` only when you are starting from
an IPA upload or local build.

Keep ` + "`asc submit create`" + ` only for older automation that already prepared
the version and just needs the final review-submission step. For newly scripted
direct submission on an already-prepared version, prefer the
nondeprecated commands above instead of extending this alias.`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: `asc submit create` is deprecated. Use `asc versions attach-build` + `asc review submissions-*` for already-uploaded builds, or `asc publish appstore --submit` when starting from an IPA.")
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

			if err := runSubmitCreateLocalizationPreflight(ctx, client, resolvedAppID, resolvedVersionID, effectivePlatform, 0); err != nil {
				return err
			}

			resolvedBuildID := strings.TrimSpace(*buildID)
			if err := runSubmitCreateReadinessPreflight(ctx, client, resolvedAppID, resolvedVersionID, effectivePlatform, resolvedBuildID); err != nil {
				return err
			}

			// Attach the build only after blocking preflight checks pass so a
			// failed submit create does not mutate the version unnecessarily.
			attachCtx, attachCancel := shared.ContextWithTimeout(ctx)
			if err := client.AttachBuildToVersion(attachCtx, resolvedVersionID, resolvedBuildID); err != nil {
				attachCancel()
				return fmt.Errorf("submit create: failed to attach build: %w", err)
			}
			attachCancel()

			runSubmitCreateSubscriptionPreflight(ctx, client, resolvedAppID, 0)

			preparationCtx, preparationCancel := shared.ContextWithTimeout(ctx)
			preparedSubmission := prepareReviewSubmissionForCreate(preparationCtx, client, resolvedAppID, effectivePlatform, resolvedVersionID, nil)
			preparationCancel()

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer func() {
				if cancel != nil {
					cancel()
				}
			}()

			var createdSubmissionID string
			submissionIDToSubmit := strings.TrimSpace(preparedSubmission.reuseSubmissionID)

			if submissionIDToSubmit == "" {
				// Use the new reviewSubmissions API (the old appStoreVersionSubmissions is deprecated)
				// Step 1: Create review submission for the app
				reviewSubmission, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(effectivePlatform))
				if err != nil {
					return fmt.Errorf("submit create: failed to create review submission: %w", err)
				}
				createdSubmissionID = strings.TrimSpace(reviewSubmission.Data.ID)
				submissionIDToSubmit = createdSubmissionID
			}

			if !preparedSubmission.reuseSubmissionHasVersion {
				// Step 2: Add the app store version as a submission item.
				// If the version is already in another submission, recover by
				// submitting that existing submission instead. If the conflicting
				// submission is one we just canceled as stale, retry the add until
				// App Store Connect finishes detaching the version.
				submissionIDToSubmit, err = addVersionToSubmissionOrRecover(
					requestCtx,
					client,
					submissionIDToSubmit,
					resolvedVersionID,
					preparedSubmission.canceledSubmissionIDs,
					func(message string) {
						fmt.Fprintln(os.Stderr, message)
					},
				)
				if err != nil {
					if createdSubmissionID != "" {
						cleanupEmptyReviewSubmission(requestCtx, client, createdSubmissionID, nil)
					}
					printSubmissionErrorHints(err, submissionErrorHintContext{
						AppID:         resolvedAppID,
						Platform:      effectivePlatform,
						VersionID:     resolvedVersionID,
						VersionString: strings.TrimSpace(*version),
					})
					return fmt.Errorf("submit create: failed to add version to submission: %w", err)
				}
			}
			if createdSubmissionID != "" && submissionIDToSubmit != createdSubmissionID {
				cleanupEmptyReviewSubmission(requestCtx, client, createdSubmissionID, nil)
			}

			// Step 3: Submit for review
			submitResp, err := client.SubmitReviewSubmission(requestCtx, submissionIDToSubmit)
			if err != nil {
				printSubmissionErrorHints(err, submissionErrorHintContext{
					AppID:         resolvedAppID,
					Platform:      effectivePlatform,
					VersionID:     resolvedVersionID,
					VersionString: strings.TrimSpace(*version),
				})
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
				BuildID:      resolvedBuildID,
				CreatedDate:  createdDatePtr,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func runSubmitCreateLocalizationPreflight(ctx context.Context, client *asc.Client, appID, versionID, platform string, requestTimeout time.Duration) error {
	localizationsCtx, localizationsCancel := submitPreflightRequestContext(ctx, requestTimeout)
	localizations, err := client.GetAppStoreVersionLocalizations(localizationsCtx, versionID, asc.WithAppStoreVersionLocalizationsLimit(200))
	localizationsCancel()
	if err != nil {
		return fmt.Errorf("submit create: failed to fetch version localizations for preflight: %w", err)
	}
	if len(localizations.Data) == 0 {
		fmt.Fprintln(os.Stderr, "Submit preflight failed: no app store version localizations found for this version.")
		return fmt.Errorf("submit create: submit preflight failed")
	}

	updateCtx, updateCancel := submitPreflightRequestContext(ctx, requestTimeout)
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

func runSubmitCreateReadinessPreflight(ctx context.Context, client *asc.Client, appID, versionID, platform, buildID string) error {
	build, err := submitCreateReadinessBuild(ctx, client, buildID)
	if err != nil {
		return err
	}

	report, err := submitReadinessReportBuilder(ctx, validatecli.ReadinessOptions{
		AppID:     appID,
		VersionID: versionID,
		Platform:  platform,
		Build:     build,
	})
	if err != nil {
		return fmt.Errorf("submit create: failed to run readiness preflight: %w", err)
	}
	printSubmitCreateReadinessWarnings(report.Checks)
	if report.Summary.Blocking == 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "Submit preflight failed: %d blocking readiness issue(s) found:\n", report.Summary.Blocking)
	for _, check := range report.Checks {
		if check.Severity != validation.SeverityError {
			continue
		}
		fmt.Fprintf(os.Stderr, "  - %s: %s\n", submitCreateReadinessCheckLabel(check), check.Message)
	}
	fmt.Fprintf(
		os.Stderr,
		"Run `asc validate --app %s --version-id %s --platform %s` for the full report before retrying submit create.\n",
		appID,
		versionID,
		platform,
	)
	return fmt.Errorf("submit create: submit preflight failed")
}

func submitCreateReadinessBuild(ctx context.Context, client *asc.Client, buildID string) (*validation.Build, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" || client == nil {
		return nil, nil
	}

	buildCtx, buildCancel := shared.ContextWithTimeout(ctx)
	buildResp, err := client.GetBuild(buildCtx, buildID)
	buildCancel()
	if err != nil {
		return nil, fmt.Errorf("submit create: failed to fetch build %q for preflight: %w", buildID, err)
	}

	attrs := buildResp.Data.Attributes
	return &validation.Build{
		ID:                      strings.TrimSpace(buildResp.Data.ID),
		Version:                 attrs.Version,
		ProcessingState:         attrs.ProcessingState,
		Expired:                 attrs.Expired,
		UsesNonExemptEncryption: attrs.UsesNonExemptEncryption,
	}, nil
}

func printSubmitCreateReadinessWarnings(checks []validation.CheckResult) {
	for _, check := range checks {
		prefix, ok := submitCreateReadinessNoticePrefix(check)
		if !ok {
			continue
		}
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", prefix, submitCreateReadinessCheckLabel(check), check.Message)
		if remediation := strings.TrimSpace(check.Remediation); remediation != "" {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", remediation)
		}
	}
}

func submitCreateReadinessNoticePrefix(check validation.CheckResult) (string, bool) {
	switch check.Severity {
	case validation.SeverityWarning:
		switch check.ID {
		case "pricing.schedule.unverified", "availability.unverified":
			return "Warning", true
		}
	case validation.SeverityInfo:
		if check.ID == "privacy.publish_state.unverified" {
			return "Advisory", true
		}
	}

	return "", false
}

func submitCreateReadinessCheckLabel(check validation.CheckResult) string {
	label := "Readiness"

	switch {
	case strings.HasPrefix(check.ID, "review_details."):
		label = "App Store review details"
	case strings.HasPrefix(check.ID, "categories."):
		label = "Primary category"
	case strings.HasPrefix(check.ID, "build."):
		label = "Attached build"
	case strings.HasPrefix(check.ID, "pricing."):
		label = "Pricing"
	case strings.HasPrefix(check.ID, "availability."):
		label = "Availability"
	case strings.HasPrefix(check.ID, "privacy."):
		label = "App Privacy"
	case strings.HasPrefix(check.ID, "screenshots."):
		label = "Screenshots"
	case strings.HasPrefix(check.ID, "age_rating."):
		label = "Age rating"
	case strings.HasPrefix(check.ID, "legal."):
		label = "Legal metadata"
	case strings.HasPrefix(check.ID, "required_fields."):
		label = "Required metadata"
	default:
		switch strings.TrimSpace(check.ResourceType) {
		case "appStoreReviewDetail":
			label = "App Store review details"
		case "appInfo":
			label = "App information"
		case "build":
			label = "Attached build"
		case "appPrivacy":
			label = "App Privacy"
		case "appScreenshotSet", "appScreenshot":
			label = "Screenshots"
		}
	}

	var qualifiers []string
	if locale := strings.TrimSpace(check.Locale); locale != "" {
		qualifiers = append(qualifiers, locale)
	}
	if field := strings.TrimSpace(check.Field); field != "" {
		qualifiers = append(qualifiers, field)
	}
	if len(qualifiers) == 0 {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, strings.Join(qualifiers, ", "))
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
			defer func() {
				if cancel != nil {
					cancel()
				}
			}()

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

func isPotentiallyCancellableReviewSubmissionState(state asc.ReviewSubmissionState) bool {
	switch state {
	case asc.ReviewSubmissionStateInReview,
		asc.ReviewSubmissionStateWaitingForReview,
		asc.ReviewSubmissionStateUnresolvedIssues,
		asc.ReviewSubmissionStateReadyForReview:
		return true
	default:
		return false
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
	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID); used with --version-id for modern API lookup")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "cancel",
		ShortUsage: "asc submit cancel [flags]",
		ShortHelp:  "Cancel a submission.",
		LongHelp: `Cancel a submission.

Cancels an active review submission. Use --id for a known submission ID,
or --version-id to find and cancel the submission for a specific version.
When using --version-id, provide --app for reliable lookup via the modern
reviewSubmissions API; without --app, falls back to the legacy endpoint.

Examples:
  asc submit cancel --id "SUBMISSION_ID" --confirm
  asc submit cancel --version-id "VERSION_ID" --confirm
  asc submit cancel --version-id "VERSION_ID" --app "APP_ID" --confirm`,
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
			defer func() {
				if cancel != nil {
					cancel()
				}
			}()

			refreshRequestCtx := func() {
				if cancel != nil {
					cancel()
				}
				requestCtx, cancel = shared.ContextWithTimeout(ctx)
			}

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
				explicitAppID := strings.TrimSpace(*appID)

				// Prefer the app relationship on the version itself so a stale
				// ASC_APP_ID/config value does not misdirect the modern lookup.
				resolvedAppID := ""
				versionResp, vErr := client.GetAppStoreVersion(requestCtx, resolvedVersionID, asc.WithAppStoreVersionInclude([]string{"app"}))
				if vErr == nil {
					if aid, aidErr := resolveAppIDFromVersionResponse(versionResp); aidErr == nil {
						if explicitAppID != "" && explicitAppID != aid {
							return fmt.Errorf("submit cancel: version %q belongs to app %q, not %q", resolvedVersionID, aid, explicitAppID)
						}
						resolvedAppID = aid
					}
				}
				if resolvedAppID == "" {
					if explicitAppID != "" {
						resolvedAppID = explicitAppID
					} else {
						resolvedAppID = shared.ResolveAppID(*appID)
					}
				}

				if resolvedAppID != "" {
					submission, findErr := findReviewSubmissionForVersion(requestCtx, client, resolvedAppID, resolvedVersionID)
					if findErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: modern review submission lookup failed: %v (falling back to legacy)\n", findErr)
					} else if submission != nil {
						if submission.Attributes.SubmissionState == asc.ReviewSubmissionStateCanceling {
							resolvedSubmissionID = submission.ID
							result := &asc.AppStoreVersionSubmissionCancelResult{
								ID:        resolvedSubmissionID,
								Cancelled: true,
							}
							return shared.PrintOutput(result, *output.Output, *output.Pretty)
						}
						if !isPotentiallyCancellableReviewSubmissionState(submission.Attributes.SubmissionState) {
							submission = nil
						}
					}
					if submission != nil {
						_, cancelErr := client.CancelReviewSubmission(requestCtx, submission.ID)
						if cancelErr != nil {
							if isExpectedNonCancellableReviewSubmissionError(cancelErr) {
								if refreshedCanceling, refreshErr := reviewSubmissionIsState(requestCtx, client, submission.ID, asc.ReviewSubmissionStateCanceling); refreshErr == nil && refreshedCanceling {
									resolvedSubmissionID = submission.ID
									result := &asc.AppStoreVersionSubmissionCancelResult{
										ID:        resolvedSubmissionID,
										Cancelled: true,
									}
									return shared.PrintOutput(result, *output.Output, *output.Pretty)
								}
								return fmt.Errorf("submit cancel: submission %s is no longer cancellable: %w", submission.ID, cancelErr)
							} else {
								return fmt.Errorf("submit cancel: failed to cancel submission %s: %w", submission.ID, cancelErr)
							}
						}
						if submission != nil {
							resolvedSubmissionID = submission.ID
							result := &asc.AppStoreVersionSubmissionCancelResult{
								ID:        resolvedSubmissionID,
								Cancelled: true,
							}
							return shared.PrintOutput(result, *output.Output, *output.Pretty)
						}
					}
				}

				// Fall back to legacy version submission lookup.
				if requestCtx.Err() != nil {
					refreshRequestCtx()
				}
				submissionResp, err := client.GetAppStoreVersionSubmissionForVersion(requestCtx, resolvedVersionID)
				if err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no active submission found for version %q (tried modern and legacy APIs)", resolvedVersionID)
					}
					return fmt.Errorf("submit cancel: %w", err)
				}
				resolvedSubmissionID = strings.TrimSpace(submissionResp.Data.ID)
				if resolvedSubmissionID == "" {
					return fmt.Errorf("submit cancel: no submission found for version %q", resolvedVersionID)
				}

				// Try modern cancel, then legacy delete.
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

				if err := client.DeleteAppStoreVersionSubmission(requestCtx, resolvedSubmissionID); err != nil {
					if asc.IsNotFound(err) {
						return fmt.Errorf("submit cancel: no submission found for ID %q", resolvedSubmissionID)
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
func runSubmitCreateSubscriptionPreflight(ctx context.Context, client *asc.Client, appID string, requestTimeout time.Duration) {
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
		fmt.Fprintln(os.Stderr, "If this is their first review, run `asc web review subscriptions list --app \"APP_ID\"` to find the relevant IDs, then attach the group with `asc web review subscriptions attach-group --app \"APP_ID\" --group-id \"GROUP_ID\" --confirm` (or use `attach --subscription-id \"SUB_ID\"` for one subscription) before retrying `asc submit create`.")
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

var stillInProgressPattern = regexp.MustCompile(
	`(?i)reviewSubmission[s]?\s+with id\s+(\S+)\s+still in progress`,
)

type submissionConflictKind string

const (
	submissionConflictNone            submissionConflictKind = ""
	submissionConflictAlreadyAttached submissionConflictKind = "already_attached"
	submissionConflictStillInProgress submissionConflictKind = "still_in_progress"
)

type submissionConflict struct {
	Kind         submissionConflictKind
	SubmissionID string
}

type submissionErrorHintContext struct {
	AppID         string
	Platform      string
	VersionID     string
	VersionString string
}

type submissionErrorSignals struct {
	existingSubmissionID string
	activeSubmissionID   string
	versionNotReady      bool
	ageRating            bool
	contentRights        bool
	usesNonExempt        bool
	appDataUsage         bool
	primaryCategory      bool
}

func collectSubmissionErrorSignals(err error) submissionErrorSignals {
	var signals submissionErrorSignals
	if err == nil {
		return signals
	}

	type errorFrame struct {
		err      error
		expanded bool
	}

	stack := []errorFrame{{err: err}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		current := frame.err
		if current == nil {
			continue
		}

		if frame.expanded {
			signals.ingest("", "", current.Error())
			continue
		}

		if apiErr, ok := any(current).(*asc.APIError); ok {
			signals.ingest("", apiErr.Code, apiErr.Detail)
			for path, entries := range apiErr.AssociatedErrors {
				signals.ingest(path, "", "")
				for _, entry := range entries {
					signals.ingest(path, entry.Code, entry.Detail)
				}
			}
		}

		// Process descendants before the wrapper text so structured API detail wins.
		stack = append(stack, errorFrame{err: current, expanded: true})

		type unwrapMany interface {
			Unwrap() []error
		}
		if multi, ok := current.(unwrapMany); ok {
			children := multi.Unwrap()
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, errorFrame{err: children[i]})
			}
			continue
		}
		if next := errors.Unwrap(current); next != nil {
			stack = append(stack, errorFrame{err: next})
		}
	}

	return signals
}

func (signals *submissionErrorSignals) ingest(path, code, detail string) {
	combined := strings.ToLower(strings.Join([]string{path, code, detail}, " "))
	pathLower := strings.ToLower(path)

	if signals.existingSubmissionID == "" {
		if m := alreadyAddedPattern.FindStringSubmatch(detail); len(m) == 2 {
			signals.existingSubmissionID = m[1]
		}
	}
	if signals.activeSubmissionID == "" {
		if m := stillInProgressPattern.FindStringSubmatch(detail); len(m) == 2 {
			signals.activeSubmissionID = m[1]
		}
	}

	if strings.Contains(combined, "ageratingdeclaration") || strings.Contains(pathLower, "ageratingdeclaration") {
		signals.ageRating = true
	}
	if strings.Contains(combined, "contentrightsdeclaration") || strings.Contains(pathLower, "contentrightsdeclaration") {
		signals.contentRights = true
	}
	if strings.Contains(combined, "usesnonexemptencryption") {
		signals.usesNonExempt = true
	}
	if strings.Contains(combined, "appdatausage") || strings.Contains(pathLower, "appdatausages") || strings.Contains(pathLower, "appdatausage") {
		signals.appDataUsage = true
	}
	if strings.Contains(combined, "primarycategory") || strings.Contains(pathLower, "primarycategory") {
		signals.primaryCategory = true
	}
	if strings.Contains(combined, "not ready to be submitted") || strings.Contains(combined, "not in valid state") {
		signals.versionNotReady = true
	}
}

func extractSubmissionConflict(err error) submissionConflict {
	signals := collectSubmissionErrorSignals(err)
	switch {
	case signals.existingSubmissionID != "":
		return submissionConflict{
			Kind:         submissionConflictAlreadyAttached,
			SubmissionID: signals.existingSubmissionID,
		}
	case signals.activeSubmissionID != "":
		return submissionConflict{
			Kind:         submissionConflictStillInProgress,
			SubmissionID: signals.activeSubmissionID,
		}
	default:
		return submissionConflict{}
	}
}

// extractExistingSubmissionID inspects an error returned by AddReviewSubmissionItem
// to see if it indicates the version is already in another review submission.
// If so, it returns the existing submission's ID; otherwise it returns "".
func extractExistingSubmissionID(err error) string {
	conflict := extractSubmissionConflict(err)
	if conflict.Kind != submissionConflictAlreadyAttached {
		return ""
	}
	return conflict.SubmissionID
}

func addVersionToSubmissionOrRecover(
	ctx context.Context,
	client *asc.Client,
	submissionID, versionID string,
	recentlyCanceledSubmissionIDs map[string]struct{},
	emit func(string),
) (string, error) {
	for attempt := 0; ; attempt++ {
		_, err := client.AddReviewSubmissionItem(ctx, submissionID, versionID)
		if err == nil {
			return submissionID, nil
		}

		conflict := extractSubmissionConflict(err)
		conflictSubmissionID := strings.TrimSpace(conflict.SubmissionID)
		if conflictSubmissionID == "" {
			return "", err
		}

		switch conflict.Kind {
		case submissionConflictAlreadyAttached:
			if _, ok := recentlyCanceledSubmissionIDs[conflictSubmissionID]; !ok {
				message := fmt.Sprintf("Version already in review submission %s, reusing it.", conflictSubmissionID)
				if emit != nil {
					emit(message)
				} else {
					fmt.Fprintln(os.Stderr, message)
				}
				return conflictSubmissionID, nil
			}
		case submissionConflictStillInProgress:
			if _, ok := recentlyCanceledSubmissionIDs[conflictSubmissionID]; !ok {
				return "", err
			}
		default:
			return "", err
		}
		if attempt >= len(submitCreateRecentlyCanceledRetryDelays) {
			return "", fmt.Errorf(
				"version is still attached to recently canceled review submission %s after %d retries: %w",
				conflictSubmissionID,
				len(submitCreateRecentlyCanceledRetryDelays),
				err,
			)
		}

		delay := submitCreateRecentlyCanceledRetryDelays[attempt]
		message := fmt.Sprintf(
			"Version is still detaching from recently canceled review submission %s, retrying add in %s.",
			conflictSubmissionID,
			delay,
		)
		if emit != nil {
			emit(message)
		} else {
			fmt.Fprintln(os.Stderr, message)
		}
		if err := sleepWithContext(ctx, delay); err != nil {
			return "", fmt.Errorf("waiting for recently canceled review submission %s to clear: %w", conflictSubmissionID, err)
		}
	}
}

type submitCreateReviewSubmissionPreparation struct {
	reuseSubmissionID         string
	reuseSubmissionHasVersion bool
	canceledSubmissionIDs     map[string]struct{}
}

func prepareReviewSubmissionForCreate(
	ctx context.Context,
	client *asc.Client,
	appID, platform, versionID string,
	emit func(string),
) submitCreateReviewSubmissionPreparation {
	emitMessage := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		if emit != nil {
			emit(message)
			return
		}
		fmt.Fprintln(os.Stderr, message)
	}

	existing, err := client.GetReviewSubmissions(
		ctx,
		appID,
		asc.WithReviewSubmissionsStates([]string{string(asc.ReviewSubmissionStateReadyForReview)}),
		asc.WithReviewSubmissionsPlatforms([]string{platform}),
		asc.WithReviewSubmissionsInclude([]string{"appStoreVersionForReview"}),
		asc.WithReviewSubmissionsLimit(200),
	)
	if err != nil {
		emitMessage("Warning: failed to query stale review submissions: %v", err)
		return submitCreateReviewSubmissionPreparation{}
	}

	submissions := make([]asc.ReviewSubmissionResource, 0, len(existing.Data))
	for {
		submissions = append(submissions, existing.Data...)
		nextURL := strings.TrimSpace(existing.Links.Next)
		if nextURL == "" {
			break
		}
		existing, err = client.GetReviewSubmissions(ctx, appID, asc.WithReviewSubmissionsNextURL(nextURL))
		if err != nil {
			emitMessage("Warning: failed to query stale review submissions: %v", err)
			return submitCreateReviewSubmissionPreparation{}
		}
	}

	if len(submissions) == 0 {
		return submitCreateReviewSubmissionPreparation{}
	}

	result := submitCreateReviewSubmissionPreparation{
		canceledSubmissionIDs: make(map[string]struct{}, len(submissions)),
	}
	normalizedPlatform := strings.ToUpper(strings.TrimSpace(platform))
	targetVersionID := strings.TrimSpace(versionID)

	for i := range submissions {
		sub := submissions[i]
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}
		if currentVersionID := reviewSubmissionAppStoreVersionID(&sub); targetVersionID != "" && currentVersionID == targetVersionID {
			reusable, hasVersion, reuseErr := reviewSubmissionCanBeReusedForCreate(ctx, client, &sub, targetVersionID)
			if reuseErr != nil {
				emitMessage("Warning: failed to inspect review submission %s before reuse: %v", sub.ID, reuseErr)
				continue
			}
			if reusable {
				emitMessage("Reusing existing review submission %s because the target version is already attached.", sub.ID)
				result.reuseSubmissionID = strings.TrimSpace(sub.ID)
				result.reuseSubmissionHasVersion = hasVersion
				result.canceledSubmissionIDs = nil
				return result
			}
		}
	}

	for i := range submissions {
		sub := submissions[i]
		if sub.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
			continue
		}
		if normalizedPlatform != "" && !strings.EqualFold(string(sub.Attributes.Platform), normalizedPlatform) {
			continue
		}

		if _, cancelErr := client.CancelReviewSubmission(ctx, sub.ID); cancelErr != nil {
			if isExpectedNonCancellableReviewSubmissionError(cancelErr) {
				reuseSubmission, reuseHasVersion, reuseErr := reusableReviewSubmissionForCreate(ctx, client, &sub, targetVersionID)
				if reuseErr == nil && reuseSubmission != "" {
					if reuseHasVersion {
						emitMessage("Reusing existing review submission %s because the target version is already attached and App Store Connect would not cancel it.", reuseSubmission)
					} else {
						emitMessage("Reusing existing empty review submission %s because App Store Connect would not cancel it.", reuseSubmission)
					}
					result.reuseSubmissionID = reuseSubmission
					result.reuseSubmissionHasVersion = reuseHasVersion
					if len(result.canceledSubmissionIDs) == 0 {
						result.canceledSubmissionIDs = nil
					}
					return result
				}
				if reuseErr != nil {
					emitMessage("Warning: failed to inspect stale submission %s after cancel conflict: %v", sub.ID, reuseErr)
				}
				emitMessage("Skipped stale submission %s: already transitioned to a non-cancellable state", sub.ID)
			} else {
				emitMessage("Warning: failed to cancel stale submission %s: %v", sub.ID, cancelErr)
			}
			continue
		}
		result.canceledSubmissionIDs[sub.ID] = struct{}{}
		emitMessage("Canceled stale review submission %s", sub.ID)
	}

	if len(result.canceledSubmissionIDs) == 0 {
		result.canceledSubmissionIDs = nil
	}
	return result
}

func reviewSubmissionAppStoreVersionID(submission *asc.ReviewSubmissionResource) string {
	if submission == nil || submission.Relationships == nil || submission.Relationships.AppStoreVersionForReview == nil {
		return ""
	}
	return strings.TrimSpace(submission.Relationships.AppStoreVersionForReview.Data.ID)
}

type reviewSubmissionItemSummary struct {
	hasItems         bool
	hasTargetVersion bool
	hasOtherItems    bool
}

func reviewSubmissionCanBeReusedForCreate(
	ctx context.Context,
	client *asc.Client,
	submission *asc.ReviewSubmissionResource,
	targetVersionID string,
) (reusable bool, hasVersion bool, err error) {
	if submission == nil {
		return false, false, nil
	}

	submissionID := strings.TrimSpace(submission.ID)
	if submissionID == "" {
		return false, false, nil
	}

	itemSummary, err := summarizeReviewSubmissionItems(ctx, client, submissionID, targetVersionID)
	if err != nil {
		return false, false, err
	}
	if itemSummary.hasItems {
		if itemSummary.hasTargetVersion && !itemSummary.hasOtherItems {
			return true, true, nil
		}
		return false, false, nil
	}

	return true, false, nil
}

func reusableReviewSubmissionForCreate(
	ctx context.Context,
	client *asc.Client,
	submission *asc.ReviewSubmissionResource,
	targetVersionID string,
) (submissionID string, hasVersion bool, err error) {
	if submission == nil {
		return "", false, nil
	}

	submissionID = strings.TrimSpace(submission.ID)
	if submissionID == "" {
		return "", false, nil
	}
	refreshed, err := refreshReviewSubmission(ctx, client, submissionID)
	if err != nil {
		return "", false, err
	}
	if refreshed == nil || refreshed.Attributes.SubmissionState != asc.ReviewSubmissionStateReadyForReview {
		return "", false, nil
	}

	reusable, hasVersion, err := reviewSubmissionCanBeReusedForCreate(ctx, client, refreshed, targetVersionID)
	if err != nil {
		return "", false, err
	}
	if reusable {
		return submissionID, hasVersion, nil
	}
	return "", false, nil
}

func summarizeReviewSubmissionItems(
	ctx context.Context,
	client *asc.Client,
	submissionID, targetVersionID string,
) (reviewSubmissionItemSummary, error) {
	var summary reviewSubmissionItemSummary

	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return summary, nil
	}

	resp, err := client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsLimit(200))
	if err != nil {
		return summary, err
	}

	for {
		accumulateReviewSubmissionItemSummary(&summary, resp.Data, targetVersionID)

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return summary, nil
		}

		resp, err = client.GetReviewSubmissionItems(ctx, submissionID, asc.WithReviewSubmissionItemsNextURL(nextURL))
		if err != nil {
			return summary, err
		}
	}
}

func accumulateReviewSubmissionItemSummary(summary *reviewSubmissionItemSummary, items []asc.ReviewSubmissionItemResource, targetVersionID string) {
	if summary == nil {
		return
	}

	targetVersionID = strings.TrimSpace(targetVersionID)
	for _, item := range items {
		summary.hasItems = true

		versionID := reviewSubmissionItemVersionID(item)
		switch {
		case targetVersionID != "" && versionID == targetVersionID:
			summary.hasTargetVersion = true
		case versionID != "":
			summary.hasOtherItems = true
		default:
			// If the item is not the target version, treat it as unrelated and
			// avoid reusing the submission implicitly.
			summary.hasOtherItems = true
		}
	}
}

func reviewSubmissionItemVersionID(item asc.ReviewSubmissionItemResource) string {
	if item.Relationships == nil || item.Relationships.AppStoreVersion == nil {
		return ""
	}
	return strings.TrimSpace(item.Relationships.AppStoreVersion.Data.ID)
}

func refreshReviewSubmission(ctx context.Context, client *asc.Client, submissionID string) (*asc.ReviewSubmissionResource, error) {
	submissionID = strings.TrimSpace(submissionID)
	if submissionID == "" || client == nil {
		return nil, nil
	}
	resp, err := client.GetReviewSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func reviewSubmissionIsState(ctx context.Context, client *asc.Client, submissionID string, wantState asc.ReviewSubmissionState) (bool, error) {
	refreshed, err := refreshReviewSubmission(ctx, client, submissionID)
	if err != nil || refreshed == nil {
		return false, err
	}
	return refreshed.Attributes.SubmissionState == wantState, nil
}

func cleanupEmptyReviewSubmission(ctx context.Context, client *asc.Client, submissionID string, emit func(string)) {
	if strings.TrimSpace(submissionID) == "" {
		return
	}
	if _, cancelErr := client.CancelReviewSubmission(ctx, submissionID); cancelErr != nil && !isExpectedNonCancellableReviewSubmissionError(cancelErr) {
		message := fmt.Sprintf("Warning: failed to cancel empty submission %s: %v", submissionID, cancelErr)
		if emit != nil {
			emit(message)
		} else {
			fmt.Fprintln(os.Stderr, message)
		}
	}
}

// printSubmissionErrorHints inspects an error returned by App Store Connect
// during submission and prints actionable fix suggestions to stderr.
func printSubmissionErrorHints(err error, ctx submissionErrorHintContext) {
	if err == nil {
		return
	}

	signals := collectSubmissionErrorSignals(err)
	var hints []string
	if signals.ageRating && strings.TrimSpace(ctx.AppID) != "" {
		hints = append(hints,
			fmt.Sprintf("Review current age rating: asc age-rating view --app %s", ctx.AppID),
			"Review age-rating update flags: asc age-rating edit --help",
		)
	}
	if signals.contentRights && strings.TrimSpace(ctx.AppID) != "" {
		hints = append(hints,
			fmt.Sprintf("If your app does not use third-party content: asc apps update --id %s --content-rights DOES_NOT_USE_THIRD_PARTY_CONTENT", ctx.AppID),
			fmt.Sprintf("If your app uses third-party content: asc apps update --id %s --content-rights USES_THIRD_PARTY_CONTENT", ctx.AppID),
		)
	}
	if signals.usesNonExempt {
		hints = append(hints,
			"Set Uses Non-Exempt Encryption for the attached build in App Store Connect, then retry submission.",
		)
	}
	if signals.appDataUsage && strings.TrimSpace(ctx.AppID) != "" {
		hints = append(hints, fmt.Sprintf("Complete App Privacy at: https://appstoreconnect.apple.com/apps/%s/appPrivacy", ctx.AppID))
	}
	if signals.primaryCategory {
		hints = append(hints,
			"List available categories: asc categories list",
			"Review category update flags: asc app-setup categories set --help",
		)
	}
	if activeID := strings.TrimSpace(signals.activeSubmissionID); activeID != "" {
		hints = appendUniqueHints(hints,
			fmt.Sprintf("Check the active submission: asc submit status --id %s", activeID),
			fmt.Sprintf("Inspect the active submission payload: asc review submissions-get --id %s", activeID),
		)
	}
	if strings.TrimSpace(signals.existingSubmissionID) != "" && strings.TrimSpace(signals.activeSubmissionID) == "" {
		hints = appendUniqueHints(hints,
			fmt.Sprintf("Inspect the existing submission: asc submit status --id %s", signals.existingSubmissionID),
			fmt.Sprintf("Inspect the existing submission payload: asc review submissions-get --id %s", signals.existingSubmissionID),
		)
	}
	if signals.versionNotReady {
		if strings.TrimSpace(ctx.AppID) != "" && strings.TrimSpace(ctx.VersionID) != "" {
			hints = appendUniqueHints(hints, fmt.Sprintf("Re-run readiness validation: asc validate --app %s --version-id %s", ctx.AppID, ctx.VersionID))
		} else if strings.TrimSpace(ctx.AppID) != "" && strings.TrimSpace(ctx.VersionString) != "" {
			preflightHint := fmt.Sprintf("Re-run readiness validation: asc validate --app %s --version %s", ctx.AppID, ctx.VersionString)
			if strings.TrimSpace(ctx.Platform) != "" {
				preflightHint = fmt.Sprintf("%s --platform %s", preflightHint, strings.TrimSpace(ctx.Platform))
			}
			hints = appendUniqueHints(hints, preflightHint)
		}
		if strings.TrimSpace(ctx.AppID) != "" {
			hints = appendUniqueHints(hints, fmt.Sprintf("Review the release dashboard: asc status --app %s --include submission,appstore,review", ctx.AppID))
		}
	}

	if len(hints) > 0 {
		fmt.Fprintln(os.Stderr, "")
		for _, hint := range hints {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", hint)
		}
	}
}

func appendUniqueHints(hints []string, values ...string) []string {
	seen := make(map[string]struct{}, len(hints))
	for _, hint := range hints {
		seen[hint] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		hints = append(hints, value)
		seen[value] = struct{}{}
	}
	return hints
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
