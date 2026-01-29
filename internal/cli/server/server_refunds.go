package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ServerRefundsCommand returns the refunds history subcommand.
func ServerRefundsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("refunds", flag.ExitOnError)

	transactionID := fs.String("transaction-id", "", "Transaction ID")
	revision := fs.String("next", "", "Fetch next page using a revision token")
	limit := fs.Int("limit", 0, "Maximum transactions to return")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "refunds",
		ShortUsage: "asc server refunds --transaction-id \"TX_ID\" [flags]",
		ShortHelp:  "Get refund history for a transaction.",
		LongHelp: `Get refund history for a transaction.

Examples:
  asc server refunds --transaction-id "TX_ID"
  asc server refunds --transaction-id "TX_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*transactionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --transaction-id is required")
				return flag.ErrHelp
			}
			if *limit < 0 {
				return fmt.Errorf("server refunds: --limit must be positive")
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server refunds: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetRefundHistory(requestCtx, trimmedID, *revision)
			if err != nil {
				return fmt.Errorf("server refunds: failed to fetch: %w", err)
			}

			if *paginate {
				aggregated, err := paginateRefundHistory(requestCtx, client, trimmedID, resp, *limit)
				if err != nil {
					return fmt.Errorf("server refunds: %w", err)
				}
				return printOutput(aggregated, *output, *pretty)
			}

			if *limit > 0 && len(resp.SignedTransactions) > *limit {
				resp.SignedTransactions = resp.SignedTransactions[:*limit]
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

func paginateRefundHistory(ctx context.Context, client *asc.ServerAPIClient, transactionID string, first *asc.RefundHistoryResponse, limit int) (*asc.RefundHistoryResponse, error) {
	aggregated := &asc.RefundHistoryResponse{
		HasMore:            first.HasMore,
		Revision:           first.Revision,
		SignedTransactions: append([]string{}, first.SignedTransactions...),
	}

	truncated := false
	current := first
	for hasMore(current.HasMore) && current.Revision != nil && strings.TrimSpace(*current.Revision) != "" {
		if limit > 0 && len(aggregated.SignedTransactions) >= limit {
			truncated = true
			break
		}
		next, err := client.GetRefundHistory(ctx, transactionID, *current.Revision)
		if err != nil {
			return nil, err
		}
		aggregated.SignedTransactions = append(aggregated.SignedTransactions, next.SignedTransactions...)
		aggregated.Revision = next.Revision
		aggregated.HasMore = next.HasMore
		current = next
	}

	if limit > 0 && len(aggregated.SignedTransactions) > limit {
		aggregated.SignedTransactions = aggregated.SignedTransactions[:limit]
		truncated = true
	}

	if truncated {
		value := true
		aggregated.HasMore = &value
	} else {
		value := false
		aggregated.HasMore = &value
	}

	return aggregated, nil
}
