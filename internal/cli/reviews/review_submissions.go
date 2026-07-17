package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var reviewSubmissionsClientFactory = shared.GetASCClient

// ReviewSubmissionsListCommand returns the review submissions list subcommand.
func ReviewSubmissionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	global := fs.Bool("global", false, "Use top-level /v1/reviewSubmissions endpoint")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by state (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	itemFields := fs.String("item-fields", "", "Review submission item fields: "+strings.Join(reviewSubmissionItemFields, ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewSubmissionIncludes, ", "))
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-list",
		ShortUsage: "asc review submissions-list [flags]",
		ShortHelp:  "List review submissions for an app or globally.",
		LongHelp: `List review submissions for an app or globally.

Examples:
  asc review submissions-list --app "123456789"
  asc review submissions-list --app "123456789" --platform IOS --state READY_FOR_REVIEW
  asc review submissions-list --app "123456789" --paginate
  asc review submissions-list --global --app "123456789"
  asc review submissions-list --global --app "123456789" --platform IOS --state READY_FOR_REVIEW`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}
			if err := rejectReviewNextFlagConflicts(
				fs, *next, "review submissions-list",
				"app", "global", "platform", "state", "limit", "item-fields", "include",
			); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("--limit must be between 1 and 200")
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platform))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			states, err := shared.NormalizeReviewSubmissionStates(shared.SplitCSVUpper(*state))
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedItemFields, err := shared.NormalizeSelection(*itemFields, reviewSubmissionItemFields, "--item-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedIncludes, err := shared.NormalizeSelection(*include, reviewSubmissionIncludes, "--include")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(normalizedItemFields) != 0 && !slices.Contains(normalizedIncludes, "items") {
				normalizedIncludes = append(normalizedIncludes, "items")
			}
			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)

			// Require one of --app or --global (unless --next is provided)
			if !*global && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app or --global is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}
			// Top-level /v1/reviewSubmissions requires filter[app].
			if *global && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required with --global (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionsOption{
				asc.WithReviewSubmissionsLimit(*limit),
				asc.WithReviewSubmissionsNextURL(*next),
				asc.WithReviewSubmissionsPlatforms(platforms),
				asc.WithReviewSubmissionsStates(states),
				asc.WithReviewSubmissionsItemFields(normalizedItemFields),
				asc.WithReviewSubmissionsInclude(normalizedIncludes),
			}
			if *global && resolvedAppID != "" {
				opts = append(opts, asc.WithReviewSubmissionsApps([]string{resolvedAppID}))
			}

			if *global {
				if *paginate {
					paginateOpts := append(opts, asc.WithReviewSubmissionsLimit(200))
					resp, err := shared.PaginateWithSpinner(
						requestCtx,
						func(ctx context.Context) (asc.PaginatedResponse, error) {
							return client.ListReviewSubmissions(ctx, paginateOpts...)
						},
						func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
							return client.ListReviewSubmissions(ctx, asc.WithReviewSubmissionsNextURL(nextURL))
						},
					)
					if err != nil {
						return fmt.Errorf("review submissions-list: %w", err)
					}

					return shared.PrintOutput(resp, *output.Output, *output.Pretty)
				}

				resp, err := client.ListReviewSubmissions(requestCtx, opts...)
				if err != nil {
					return fmt.Errorf("review submissions-list: failed to fetch: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionsLimit(200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissions(ctx, resolvedAppID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissions(ctx, resolvedAppID, asc.WithReviewSubmissionsNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("review submissions-list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetReviewSubmissions(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewSubmissionsGetCommand returns the review submissions get subcommand.
func ReviewSubmissionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-get", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	itemFields := fs.String("item-fields", "", "Review submission item fields: "+strings.Join(reviewSubmissionItemFields, ", "))
	include := fs.String("include", "", "Include relationships: "+strings.Join(reviewSubmissionIncludes, ", "))
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-get",
		ShortUsage: "asc review submissions-get [flags]",
		ShortHelp:  "Get a review submission by ID.",
		LongHelp: `Get a review submission by ID.

Examples:
  asc review submissions-get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			normalizedItemFields, err := shared.NormalizeSelection(*itemFields, reviewSubmissionItemFields, "--item-fields")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedIncludes, err := shared.NormalizeSelection(*include, reviewSubmissionIncludes, "--include")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if len(normalizedItemFields) != 0 && !slices.Contains(normalizedIncludes, "items") {
				normalizedIncludes = append(normalizedIncludes, "items")
			}
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetReviewSubmission(
				requestCtx,
				strings.TrimSpace(*submissionID),
				asc.WithReviewSubmissionItemFields(normalizedItemFields),
				asc.WithReviewSubmissionInclude(normalizedIncludes),
			)
			if err != nil {
				return fmt.Errorf("review submissions-get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

var reviewSubmissionIncludes = []string{
	"app",
	"items",
	"appStoreVersionForReview",
	"submittedByActor",
	"lastUpdatedByActor",
}

// ReviewSubmissionsCreateCommand returns the review submissions create subcommand.
func ReviewSubmissionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-create",
		ShortUsage: "asc review submissions-create [flags]",
		ShortHelp:  "Create a review submission.",
		LongHelp: `Create a review submission for an app.

Examples:
  asc review submissions-create --app "123456789" --platform IOS`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return fmt.Errorf("review submissions-create: %w", shared.UsageError(err.Error()))
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(normalizedPlatform))
			if err != nil {
				return fmt.Errorf("review submissions-create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewSubmissionsUpdateCommand returns the review submissions update subcommand.
func ReviewSubmissionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-update", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	platform := fs.String("platform", "", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	submitted := fs.Bool("submitted", false, "Whether the submission is submitted (true/false)")
	canceled := fs.Bool("canceled", false, "Whether the submission is canceled (true/false)")
	clearPlatform := fs.Bool("clear-platform", false, "Set platform to JSON null")
	clearSubmitted := fs.Bool("clear-submitted", false, "Set submitted to JSON null")
	clearCanceled := fs.Bool("clear-canceled", false, "Set canceled to JSON null")
	confirm := fs.Bool("confirm", false, "Confirm submission or cancellation when setting it true")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-update",
		ShortUsage: "asc review submissions-update --id \"SUBMISSION_ID\" [flags]",
		ShortHelp:  "Update a review submission.",
		LongHelp: `Update a review submission.

Use the matching --clear-* flag to send JSON null. Setting --submitted=true
or --canceled=true requires --confirm.

Examples:
  asc review submissions-update --id "SUBMISSION_ID" --platform IOS
  asc review submissions-update --id "SUBMISSION_ID" --submitted=false
  asc review submissions-update --id "SUBMISSION_ID" --clear-platform
  asc review submissions-update --id "SUBMISSION_ID" --canceled=true --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			trimmedID := strings.TrimSpace(*submissionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			platformProvided := reviewFlagWasProvided(fs, "platform")
			submittedProvided := reviewFlagWasProvided(fs, "submitted")
			canceledProvided := reviewFlagWasProvided(fs, "canceled")
			if platformProvided && *clearPlatform {
				return shared.UsageError("--platform cannot be combined with --clear-platform")
			}
			if submittedProvided && *clearSubmitted {
				return shared.UsageError("--submitted cannot be combined with --clear-submitted")
			}
			if canceledProvided && *clearCanceled {
				return shared.UsageError("--canceled cannot be combined with --clear-canceled")
			}
			if !platformProvided && !submittedProvided && !canceledProvided && !*clearPlatform && !*clearSubmitted && !*clearCanceled {
				return shared.UsageError("at least one update flag is required: --platform, --submitted, --canceled, or a matching --clear-* flag")
			}
			if canceledProvided && *canceled && !*confirm {
				return shared.UsageError("--confirm is required when --canceled=true")
			}
			if submittedProvided && *submitted && !*confirm {
				return shared.UsageError("--confirm is required when --submitted=true")
			}
			if submittedProvided && *submitted && canceledProvided && *canceled {
				return shared.UsageError("--submitted=true cannot be combined with --canceled=true")
			}

			attrs := asc.ReviewSubmissionUpdateAttributes{}
			if platformProvided {
				normalized, err := shared.NormalizeAppStoreVersionPlatform(*platform)
				if err != nil {
					return fmt.Errorf("review submissions-update: %w", shared.UsageError(err.Error()))
				}
				value := asc.Platform(normalized)
				attrs.Platform = &asc.NullablePlatform{Value: &value}
			} else if *clearPlatform {
				attrs.Platform = &asc.NullablePlatform{}
			}
			if submittedProvided {
				attrs.Submitted = &asc.NullableBool{Value: submitted}
			} else if *clearSubmitted {
				attrs.Submitted = &asc.NullableBool{}
			}
			if canceledProvided {
				attrs.Canceled = &asc.NullableBool{Value: canceled}
			} else if *clearCanceled {
				attrs.Canceled = &asc.NullableBool{}
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateReviewSubmission(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("review submissions-update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewSubmissionsSubmitCommand returns the review submissions submit subcommand.
func ReviewSubmissionsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-submit", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm submission (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-submit",
		ShortUsage: "asc review submissions-submit [flags]",
		ShortHelp:  "Submit a review submission.",
		LongHelp: `Submit a review submission for review.

Examples:
  asc review submissions-submit --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to submit")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.SubmitReviewSubmission(requestCtx, strings.TrimSpace(*submissionID))
			if err != nil {
				return fmt.Errorf("review submissions-submit: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewSubmissionsCancelCommand returns the review submissions cancel subcommand.
func ReviewSubmissionsCancelCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-cancel", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm cancellation (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-cancel",
		ShortUsage: "asc review submissions-cancel [flags]",
		ShortHelp:  "Cancel a review submission.",
		LongHelp: `Cancel a review submission.

Examples:
  asc review submissions-cancel --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to cancel")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-cancel: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CancelReviewSubmission(requestCtx, strings.TrimSpace(*submissionID))
			if err != nil {
				return fmt.Errorf("review submissions-cancel: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// ReviewSubmissionsItemsIDsCommand returns the review submission item IDs subcommand.
func ReviewSubmissionsItemsIDsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-items-ids", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submissions-items-ids",
		ShortUsage: "asc review submissions-items-ids --id \"SUBMISSION_ID\" [flags]",
		ShortHelp:  "List review submission item IDs for a submission.",
		LongHelp: `List review submission item IDs for a submission.

Examples:
  asc review submissions-items-ids --id "SUBMISSION_ID"
  asc review submissions-items-ids --id "SUBMISSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageError("unexpected positional arguments")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("review submissions-items-ids: %w", err)
			}
			if err := rejectReviewNextFlagConflicts(fs, *next, "review submissions-items-ids", "id", "limit"); err != nil {
				return err
			}
			trimmedID := strings.TrimSpace(*submissionID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review submissions-items-ids: %w", shared.UsageError("--limit must be between 1 and 200"))
			}

			client, err := reviewSubmissionsClientFactory()
			if err != nil {
				return fmt.Errorf("review submissions-items-ids: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissionItemsRelationships(ctx, trimmedID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetReviewSubmissionItemsRelationships(ctx, trimmedID, asc.WithLinkagesNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("review submissions-items-ids: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetReviewSubmissionItemsRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("review submissions-items-ids: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
