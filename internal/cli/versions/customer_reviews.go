package versions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/reviews"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// VersionsCustomerReviewsCommand returns the customer reviews command group.
func VersionsCustomerReviewsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("customer-reviews", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "customer-reviews",
		ShortUsage: "asc versions customer-reviews <subcommand> [flags]",
		ShortHelp:  "Manage App Store version customer reviews.",
		LongHelp: `Manage App Store version customer reviews.

Examples:
  asc versions customer-reviews list --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			VersionsCustomerReviewsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// VersionsCustomerReviewsListCommand lists customer reviews for a version.
func VersionsCustomerReviewsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("customer-reviews list", flag.ExitOnError)

	versionID := shared.BindResourceIDFlag(fs, "version-id", "appStoreVersions", "App Store version ID")
	filters := reviews.BindReviewFilterFlags(fs)
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc versions customer-reviews list --version-id \"VERSION_ID\" [flags]",
		ShortHelp:  "List customer reviews for an app store version.",
		LongHelp: `List customer reviews for an app store version.

Apple exposes the same review filters here as on the app-level listing, so
these flags match ` + "`asc reviews list`" + `.

Examples:
  asc versions customer-reviews list --version-id "VERSION_ID"
  asc versions customer-reviews list --version-id "VERSION_ID" --stars 1,2
  asc versions customer-reviews list --version-id "VERSION_ID" --territory USA --sort -createdDate
  asc versions customer-reviews list --version-id "VERSION_ID" --only-unresponded --include-response
  asc versions customer-reviews list --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("versions customer-reviews list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("versions customer-reviews list: %v", err)
			}
			if err := reviews.ValidateReviewNextFlagConflicts(*next, fs, "version-id"); err != nil {
				return err
			}
			filterOpts, err := filters.ReviewOptions()
			if err != nil {
				return err
			}

			versionValue := strings.TrimSpace(*versionID)
			if versionValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return shared.MissingRequiredUsageError("--version-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("versions customer-reviews list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := make([]asc.ReviewOption, 0, len(filterOpts)+2)
			opts = append(opts, filterOpts...)
			opts = append(opts, asc.WithLimit(*limit), asc.WithNextURL(*next))

			if *paginate {
				paginateOpts := append(opts, asc.WithLimit(200))
				firstPage, err := client.GetAppStoreVersionCustomerReviews(requestCtx, versionValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("versions customer-reviews list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppStoreVersionCustomerReviews(ctx, versionValue, asc.WithNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("versions customer-reviews list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppStoreVersionCustomerReviews(requestCtx, versionValue, opts...)
			if err != nil {
				return fmt.Errorf("versions customer-reviews list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
