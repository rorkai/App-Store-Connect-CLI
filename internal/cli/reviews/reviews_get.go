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
	fs := flag.NewFlagSet("reviews get", flag.ExitOnError)

	reviewID := fs.String("id", "", "Customer review ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc reviews get --id \"REVIEW_ID\"",
		ShortHelp:  "Get a customer review by ID.",
		LongHelp: `Get a customer review by ID.

Examples:
  asc reviews get --id "REVIEW_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			reviewValue := strings.TrimSpace(*reviewID)
			if reviewValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("reviews get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetCustomerReview(requestCtx, reviewValue)
			if err != nil {
				return fmt.Errorf("reviews get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
