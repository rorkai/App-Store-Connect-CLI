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
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	stars := fs.Int("stars", 0, "Filter by star rating (1-5)")
	territory := fs.String("territory", "", "Filter by territory (e.g., US, GBR)")
	sort := fs.String("sort", "", "Sort by rating, -rating, createdDate, or -createdDate")
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
  asc reviews --app "123456789" --sort -createdDate --limit 5
  asc reviews --next "<links.next>"
  asc reviews --app "123456789" --paginate
  asc reviews get --id "REVIEW_ID"
  asc reviews ratings --app "123456789"
  asc reviews ratings --app "123456789" --all
  asc reviews summarizations --app "123456789" --platform IOS --territory US
  asc reviews respond --review-id "REVIEW_ID" --response "Thanks!"
  asc reviews response get --id "RESPONSE_ID"
  asc reviews response delete --id "RESPONSE_ID" --confirm
  asc reviews response for-review --review-id "REVIEW_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ReviewsListCommand(),
			ReviewsGetCommand(),
			ReviewsRatingsCommand(),
			ReviewsSummarizationsCommand(),
			ReviewsRespondCommand(),
			ReviewsResponseCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			// If no flags are set and no args, show help
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			// Execute the list functionality directly
			return executeReviewsList(ctx, resolvedAppID, *output, *pretty, *stars, *territory, *sort, *limit, *next, *paginate)
		},
	}
}

// ReviewsListCommand returns the reviews list subcommand.
func ReviewsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")
	stars := fs.Int("stars", 0, "Filter by star rating (1-5)")
	territory := fs.String("territory", "", "Filter by territory (e.g., US, GBR)")
	sort := fs.String("sort", "", "Sort by rating, -rating, createdDate, or -createdDate")
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
  asc reviews list --app "123456789" --territory US --sort -createdDate
  asc reviews list --next "<links.next>"
  asc reviews list --app "123456789" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintf(os.Stderr, "Error: --app is required (or set ASC_APP_ID)\n\n")
				return flag.ErrHelp
			}

			return executeReviewsList(ctx, resolvedAppID, *output, *pretty, *stars, *territory, *sort, *limit, *next, *paginate)
		},
	}
}

func executeReviewsList(ctx context.Context, appID, output string, pretty bool, stars int, territory, sort string, limit int, next string, paginate bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("reviews: --limit must be between 1 and 200")
	}
	if stars != 0 && (stars < 1 || stars > 5) {
		return fmt.Errorf("reviews: --stars must be between 1 and 5")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("reviews: %w", err)
	}
	if err := shared.ValidateSort(sort, "rating", "-rating", "createdDate", "-createdDate"); err != nil {
		return fmt.Errorf("reviews: %w", err)
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("reviews: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.ReviewOption{
		asc.WithRating(stars),
		asc.WithTerritory(territory),
		asc.WithLimit(limit),
		asc.WithNextURL(next),
	}
	if strings.TrimSpace(sort) != "" {
		opts = append(opts, asc.WithReviewSort(sort))
	}

	if paginate {
		// Fetch first page with limit set for consistent pagination
		paginateOpts := append(opts, asc.WithLimit(200))
		firstPage, err := client.GetReviews(requestCtx, appID, paginateOpts...)
		if err != nil {
			return fmt.Errorf("reviews: failed to fetch: %w", err)
		}

		// Fetch all remaining pages
		reviews, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetReviews(ctx, appID, asc.WithNextURL(nextURL))
		})
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
