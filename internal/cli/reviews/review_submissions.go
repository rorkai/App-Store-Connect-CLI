package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReviewSubmissionsListCommand returns the review submissions list subcommand.
func ReviewSubmissionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "", "Filter by platform: IOS, MAC_OS, TV_OS, VISION_OS (comma-separated)")
	state := fs.String("state", "", "Filter by state (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Next page URL from a previous response")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submissions-list",
		ShortUsage: "asc review submissions-list [flags]",
		ShortHelp:  "List review submissions for an app.",
		LongHelp: `List review submissions for an app.

Examples:
  asc review submissions-list --app "123456789"
  asc review submissions-list --app "123456789" --platform IOS --state READY_FOR_REVIEW
  asc review submissions-list --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review submissions-list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}

			platforms, err := shared.NormalizeAppStoreVersionPlatforms(splitCSVUpper(*platform))
			if err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}
			states := splitCSVUpper(*state)

			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.ReviewSubmissionsOption{
				asc.WithReviewSubmissionsLimit(*limit),
				asc.WithReviewSubmissionsNextURL(*next),
				asc.WithReviewSubmissionsPlatforms(platforms),
				asc.WithReviewSubmissionsStates(states),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithReviewSubmissionsLimit(200))
				firstPage, err := client.GetReviewSubmissions(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("review submissions-list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetReviewSubmissions(ctx, resolvedAppID, asc.WithReviewSubmissionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("review submissions-list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetReviewSubmissions(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("review submissions-list: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewSubmissionsGetCommand returns the review submissions get subcommand.
func ReviewSubmissionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-get", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submissions-get",
		ShortUsage: "asc review submissions-get [flags]",
		ShortHelp:  "Get a review submission by ID.",
		LongHelp: `Get a review submission by ID.

Examples:
  asc review submissions-get --id "SUBMISSION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetReviewSubmission(requestCtx, strings.TrimSpace(*submissionID))
			if err != nil {
				return fmt.Errorf("review submissions-get: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewSubmissionsCreateCommand returns the review submissions create subcommand.
func ReviewSubmissionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID)")
	platform := fs.String("platform", "IOS", "Platform: IOS, MAC_OS, TV_OS, VISION_OS")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submissions-create",
		ShortUsage: "asc review submissions-create [flags]",
		ShortHelp:  "Create a review submission.",
		LongHelp: `Create a review submission for an app.

Examples:
  asc review submissions-create --app "123456789" --platform IOS`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := resolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			normalizedPlatform, err := normalizeSubmitPlatform(*platform)
			if err != nil {
				return fmt.Errorf("review submissions-create: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateReviewSubmission(requestCtx, resolvedAppID, asc.Platform(normalizedPlatform))
			if err != nil {
				return fmt.Errorf("review submissions-create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewSubmissionsUpdateCommand returns the review submissions update subcommand.
func ReviewSubmissionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-update", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	canceled := fs.Bool("canceled", false, "Cancel submission (true/false)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submissions-update",
		ShortUsage: "asc review submissions-update --id \"SUBMISSION_ID\" --canceled true [flags]",
		ShortHelp:  "Update a review submission.",
		LongHelp: `Update a review submission.

Examples:
  asc review submissions-update --id "SUBMISSION_ID" --canceled true`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*submissionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			visited := map[string]bool{}
			fs.Visit(func(f *flag.Flag) {
				visited[f.Name] = true
			})
			if !visited["canceled"] {
				fmt.Fprintln(os.Stderr, "Error: --canceled is required")
				return flag.ErrHelp
			}

			attrs := asc.ReviewSubmissionUpdateAttributes{}
			if visited["canceled"] {
				value := *canceled
				attrs.Canceled = &value
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateReviewSubmission(requestCtx, trimmedID, attrs)
			if err != nil {
				return fmt.Errorf("review submissions-update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// ReviewSubmissionsSubmitCommand returns the review submissions submit subcommand.
func ReviewSubmissionsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submissions-submit", flag.ExitOnError)

	submissionID := fs.String("id", "", "Review submission ID (required)")
	confirm := fs.Bool("confirm", false, "Confirm submission (required)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submissions-submit",
		ShortUsage: "asc review submissions-submit [flags]",
		ShortHelp:  "Submit a review submission.",
		LongHelp: `Submit a review submission for review.

Examples:
  asc review submissions-submit --id "SUBMISSION_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to submit")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*submissionID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-submit: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.SubmitReviewSubmission(requestCtx, strings.TrimSpace(*submissionID))
			if err != nil {
				return fmt.Errorf("review submissions-submit: %w", err)
			}

			return printOutput(resp, *output, *pretty)
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submissions-items-ids",
		ShortUsage: "asc review submissions-items-ids --id \"SUBMISSION_ID\" [flags]",
		ShortHelp:  "List review submission item IDs for a submission.",
		LongHelp: `List review submission item IDs for a submission.

Examples:
  asc review submissions-items-ids --id "SUBMISSION_ID"
  asc review submissions-items-ids --id "SUBMISSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*submissionID)
			if trimmedID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("review submissions-items-ids: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("review submissions-items-ids: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("review submissions-items-ids: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetReviewSubmissionItemsRelationships(requestCtx, trimmedID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("review submissions-items-ids: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetReviewSubmissionItemsRelationships(ctx, trimmedID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("review submissions-items-ids: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetReviewSubmissionItemsRelationships(requestCtx, trimmedID, opts...)
			if err != nil {
				return fmt.Errorf("review submissions-items-ids: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
