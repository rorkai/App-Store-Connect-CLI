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

// ReviewsCommand returns the reviews command with subcommands.
func ReviewsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reviews", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)
	filters := BindReviewFilterFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "reviews",
		ShortUsage: "asc reviews [flags] | asc reviews <subcommand> [flags]",
		ShortHelp:  "List and manage App Store customer reviews.",
		LongHelp: `List and manage App Store customer reviews.

This command fetches customer reviews from the App Store,
helping you understand user feedback and sentiment.

When invoked with --app, lists reviews. Subcommands allow responding to reviews.

Examples:
  asc reviews --app "123456789"
  asc reviews --app "123456789" --stars 1 --territory US
  asc reviews --app "123456789" --stars 1,2
  asc reviews --app "123456789" --sort -createdDate --limit 5
  asc reviews --app "123456789" --response-state unreplied --include-response
  asc reviews --app "123456789" --only-unresponded
  asc reviews --next "<links.next>"
  asc reviews --app "123456789" --paginate
  asc reviews view --id "REVIEW_ID"
  asc reviews ratings --app "123456789"
  asc reviews ratings --app "123456789" --all
  asc reviews summarizations --app "123456789" --platform IOS --territory US
  asc reviews digest --app "123456789" --since 2026-10-01
  asc reviews respond --review-id "REVIEW_ID" --response "Thanks!"
  asc reviews respond-batch --app "123456789" --file replies.json --dry-run
  asc reviews response view --id "RESPONSE_ID"
  asc reviews response delete --id "RESPONSE_ID" --confirm
  asc reviews response for-review --review-id "REVIEW_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ReviewsListCommand(),
			ReviewsGetCommand(),
			ReviewsRatingsCommand(),
			ReviewsSummarizationsCommand(),
			ReviewsDigestCommand(),
			ReviewsRespondCommand(),
			ReviewsRespondBatchCommand(),
			ReviewsResponseCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.WithDiagnostic(shared.UsageErrorf("unexpected argument(s): %s", strings.Join(args, " ")), shared.DiagnosticInvalidInput, "")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.WithDiagnostic(shared.NewValidationError(shared.UsageErrorf("reviews: %v", err)), shared.DiagnosticInvalidInput, "--next")
			}
			if err := ValidateReviewNextFlagConflicts(*next, fs, "app"); err != nil {
				return err
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				return shared.WithDiagnostic(shared.UsageError("--app is required (or set ASC_APP_ID)"), shared.DiagnosticRequiredInputMissing, "--app")
			}

			// Execute the list functionality directly
			return executeReviewsList(ctx, resolvedAppID, *output.Output, *output.Pretty, filters, *limit, *next, *paginate)
		},
	}
}

// ReviewsListCommand returns the reviews list subcommand.
func ReviewsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)
	filters := BindReviewFilterFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc reviews list [flags]",
		ShortHelp:  "List App Store customer reviews.",
		LongHelp: `List App Store customer reviews.

Examples:
  asc reviews list --app "123456789"
  asc reviews list --app "123456789" --stars 5
  asc reviews list --app "123456789" --stars 1,2
  asc reviews list --app "123456789" --territory US --sort -createdDate
  asc reviews list --app "123456789" --response-state unreplied --include-response
  asc reviews list --app "123456789" --only-unresponded
  asc reviews list --next "<links.next>"
  asc reviews list --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.WithDiagnostic(shared.NewValidationError(shared.UsageErrorf("reviews: %v", err)), shared.DiagnosticInvalidInput, "--next")
			}
			if err := ValidateReviewNextFlagConflicts(*next, fs, "app"); err != nil {
				return err
			}
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return shared.MissingRequiredUsageError("--app")
			}

			return executeReviewsList(ctx, resolvedAppID, *output.Output, *output.Pretty, filters, *limit, *next, *paginate)
		},
	}
}

func executeReviewsList(ctx context.Context, appID, output string, pretty bool, filters *ReviewFilterFlags, limit int, next string, paginate bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return shared.WithDiagnostic(shared.NewValidationError(shared.UsageError("reviews: --limit must be between 1 and 200")), shared.DiagnosticInvalidInput, "--limit")
	}
	filterOpts, err := filters.ReviewOptions()
	if err != nil {
		return err
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return shared.WithDiagnostic(shared.NewValidationError(shared.UsageErrorf("reviews: %v", err)), shared.DiagnosticInvalidInput, "--next")
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("reviews: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := make([]asc.ReviewOption, 0, len(filterOpts)+2)
	opts = append(opts, filterOpts...)
	opts = append(opts, asc.WithLimit(limit), asc.WithNextURL(next))

	if paginate {
		paginateOpts := append(opts, asc.WithLimit(200))
		reviews, err := shared.PaginateWithSpinner(
			requestCtx,
			func(ctx context.Context) (asc.PaginatedResponse, error) {
				return client.GetReviews(ctx, appID, paginateOpts...)
			},
			func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetReviews(ctx, appID, asc.WithNextURL(nextURL))
			},
		)
		if err != nil {
			return fmt.Errorf("reviews: %w", err)
		}

		return shared.PrintOutput(reviews, output, pretty)
	}

	reviews, err := client.GetReviews(requestCtx, appID, opts...)
	if err != nil {
		return fmt.Errorf("reviews: failed to fetch: %w", err)
	}

	return shared.PrintOutput(reviews, output, pretty)
}
