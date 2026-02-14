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

// ReviewsSummarizationsCommand returns the review summarizations command.
func ReviewsSummarizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("summarizations", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	platforms := fs.String("platform", "", "Filter by platform(s), comma-separated: "+strings.Join(reviewSummarizationPlatformList(), ", "))
	territories := fs.String("territory", "", "Filter by 3-letter territory code(s), comma-separated (e.g., USA, GBR)")
	fields := fs.String("fields", "", "Fields to include: "+strings.Join(reviewSummarizationFieldsList(), ", "))
	territoryFields := fs.String("territory-fields", "", "Territory fields to include, comma-separated")
	include := fs.String("include", "", "Include related resources (e.g., territory), comma-separated")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "summarizations",
		ShortUsage: "asc reviews summarizations [flags]",
		ShortHelp:  "List App Store review summarizations.",
		LongHelp: `List App Store review summarizations for an app.

Examples:
  asc reviews summarizations --app "APP_ID"
  asc reviews summarizations --app "APP_ID" --platform IOS --territory USA
  asc reviews summarizations --app "APP_ID" --limit 50
  asc reviews summarizations --next "<links.next>"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("reviews summarizations: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("reviews summarizations: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			fieldsValue, err := normalizeReviewSummarizationFields(*fields)
			if err != nil {
				return fmt.Errorf("reviews summarizations: %w", err)
			}

			platformValues, err := shared.NormalizeAppStoreVersionPlatforms(shared.SplitCSVUpper(*platforms))
			if err != nil {
				return fmt.Errorf("reviews summarizations: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews summarizations: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.CustomerReviewSummarizationsOption{
				asc.WithCustomerReviewSummarizationsPlatforms(platformValues),
				asc.WithCustomerReviewSummarizationsTerritories(shared.SplitCSVUpper(*territories)),
				asc.WithCustomerReviewSummarizationsFields(fieldsValue),
				asc.WithCustomerReviewSummarizationsTerritoryFields(shared.SplitCSV(*territoryFields)),
				asc.WithCustomerReviewSummarizationsInclude(shared.SplitCSV(*include)),
				asc.WithCustomerReviewSummarizationsLimit(*limit),
				asc.WithCustomerReviewSummarizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCustomerReviewSummarizationsLimit(200))
				firstPage, err := client.GetCustomerReviewSummarizations(requestCtx, resolvedAppID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("reviews summarizations: failed to fetch: %w", err)
				}
				summaries, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCustomerReviewSummarizations(ctx, resolvedAppID, asc.WithCustomerReviewSummarizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("reviews summarizations: %w", err)
				}
				return shared.PrintOutput(summaries, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCustomerReviewSummarizations(requestCtx, resolvedAppID, opts...)
			if err != nil {
				return fmt.Errorf("reviews summarizations: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func reviewSummarizationFieldsList() []string {
	return []string{"createdDate", "locale", "platform", "text"}
}

func reviewSummarizationPlatformList() []string {
	return []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS"}
}

func normalizeReviewSummarizationFields(value string) ([]string, error) {
	values := shared.SplitCSV(value)
	if len(values) == 0 {
		return nil, nil
	}
	allowed := map[string]struct{}{
		"createdDate": {},
		"locale":      {},
		"platform":    {},
		"text":        {},
	}
	for _, field := range values {
		if _, ok := allowed[field]; !ok {
			return nil, fmt.Errorf("--fields must be one of: %s", strings.Join(reviewSummarizationFieldsList(), ", "))
		}
	}
	return values, nil
}
