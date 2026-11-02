package reviews

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReviewsGetCommand gets a customer review by ID.
func ReviewsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reviews view", flag.ExitOnError)

	reviewID := shared.BindResourceIDFlag(fs, "id", "customerReviews", "Customer review ID")
	selection := BindReviewSelectionFlags(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc reviews view --id \"REVIEW_ID\" [flags]",
		ShortHelp:  "View a customer review by ID.",
		LongHelp: `View a customer review by ID.

Examples:
  asc reviews view --id "REVIEW_ID"
  asc reviews view --id "REVIEW_ID" --fields rating,title,reviewTerritory
  asc reviews view --id "REVIEW_ID" --include response,reviewTerritory --territory-fields currency`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			reviewValue := strings.TrimSpace(*reviewID)
			if reviewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			opts, err := selection.ReviewOptions()
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetCustomerReview(requestCtx, reviewValue, opts...)
			if err != nil {
				return fmt.Errorf("reviews view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
