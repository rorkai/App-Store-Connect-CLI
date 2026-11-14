package reviews

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReviewsGetCommand gets a customer review by ID.
func ReviewsGetCommand() *ffcli.Command {
	var opts []asc.ReviewOption

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "reviews view",
		Name:        "view",
		ShortUsage:  "asc reviews view --id \"REVIEW_ID\" [flags]",
		ShortHelp:   "View a customer review by ID.",
		LongHelp: `View a customer review by ID.

Examples:
  asc reviews view --id "REVIEW_ID"
  asc reviews view --id "REVIEW_ID" --fields rating,title,reviewTerritory
  asc reviews view --id "REVIEW_ID" --include response,reviewTerritory --territory-fields currency`,
		IDUsage:            "Customer review ID",
		IDType:             "customerReviews",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			selection := BindReviewSelectionFlags(fs)
			return func() (err error) {
				opts, err = selection.ReviewOptions()
				return err
			}
		},
		ErrorPrefix:      "reviews view",
		FetchErrorPrefix: "reviews view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCustomerReview(ctx, id, opts...)
		},
	})
}
