package versions

import (
	"context"
	"flag"

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
	var filterOpts []asc.ReviewOption

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "customer-reviews list",
		Name:        "list",
		ShortUsage:  "asc versions customer-reviews list --version-id \"VERSION_ID\" [flags]",
		ShortHelp:   "List customer reviews for an app store version.",
		LongHelp: `List customer reviews for an app store version.

Apple exposes the same review filters here as on the app-level listing, so
these flags match ` + "`asc reviews list`" + `.

Examples:
  asc versions customer-reviews list --version-id "VERSION_ID"
  asc versions customer-reviews list --version-id "VERSION_ID" --stars 1,2
  asc versions customer-reviews list --version-id "VERSION_ID" --territory USA --sort -createdDate
  asc versions customer-reviews list --version-id "VERSION_ID" --review-territory USA,GBR --include reviewTerritory
  asc versions customer-reviews list --version-id "VERSION_ID" --only-unresponded --include-response
  asc versions customer-reviews list --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "App Store version ID",
		ParentType:           "appStoreVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        reviews.ReviewNextConflicts,
		Flags: func(fs *flag.FlagSet) func() error {
			filters := reviews.BindReviewFilterFlags(fs)
			return func() error {
				var err error
				filterOpts, err = filters.ReviewOptions()
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, versionID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := make([]asc.ReviewOption, 0, len(filterOpts)+2)
			opts = append(opts, filterOpts...)
			opts = append(opts, asc.Limit[asc.ReviewOption](limit), asc.NextURL[asc.ReviewOption](next))
			return client.GetAppStoreVersionCustomerReviews(ctx, versionID, opts...)
		},
	})
}
