package server

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

// ServerHistoryCommand returns the transaction history subcommand.
func ServerHistoryCommand() *ffcli.Command {
	fs := flag.NewFlagSet("history", flag.ExitOnError)

	transactionID := fs.String("transaction-id", "", "Transaction ID")
	revision := fs.String("next", "", "Fetch next page using a revision token")
	start := fs.String("start", "", "Start time (RFC3339 or unix ms)")
	end := fs.String("end", "", "End time (RFC3339 or unix ms)")
	productIDs := fs.String("product-id", "", "Filter by product IDs, comma-separated")
	productTypes := fs.String("product-type", "", "Filter by product types: "+strings.Join(serverProductTypeList(), ", "))
	subscriptionGroupIDs := fs.String("subscription-group-id", "", "Filter by subscription group IDs, comma-separated")
	ownershipType := fs.String("ownership-type", "", "Filter by ownership type: "+strings.Join(serverOwnershipTypeList(), ", "))
	sort := fs.String("sort", "", "Sort order: "+strings.Join(serverOrderList(), ", "))
	limit := fs.Int("limit", 0, "Maximum transactions to return")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	var revoked shared.OptionalBool
	fs.Var(&revoked, "revoked", "Filter revoked transactions (true/false)")

	return &ffcli.Command{
		Name:       "history",
		ShortUsage: "asc server history --transaction-id \"TX_ID\" [flags]",
		ShortHelp:  "Get transaction history for a transaction.",
		LongHelp: `Get transaction history for a transaction.

Examples:
  asc server history --transaction-id "TX_ID"
  asc server history --transaction-id "TX_ID" --product-type AUTO_RENEWABLE --start "2026-01-01T00:00:00Z" --end "2026-01-31T00:00:00Z"
  asc server history --transaction-id "TX_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*transactionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --transaction-id is required")
				return flag.ErrHelp
			}
			if *limit < 0 {
				return fmt.Errorf("server history: --limit must be positive")
			}

			startValue, err := parseServerTimestamp(*start, "--start")
			if err != nil {
				return fmt.Errorf("server history: %w", err)
			}
			endValue, err := parseServerTimestamp(*end, "--end")
			if err != nil {
				return fmt.Errorf("server history: %w", err)
			}
			if startValue != nil && endValue != nil && *startValue > *endValue {
				return fmt.Errorf("server history: --start must be before --end")
			}

			types, err := normalizeServerProductTypes(*productTypes)
			if err != nil {
				return fmt.Errorf("server history: %w", err)
			}
			ownership, err := normalizeServerOwnershipType(*ownershipType)
			if err != nil {
				return fmt.Errorf("server history: %w", err)
			}
			order, err := normalizeServerOrder(*sort)
			if err != nil {
				return fmt.Errorf("server history: %w", err)
			}

			var revokedValue *bool
			if revoked.IsSet() {
				value := revoked.Value()
				revokedValue = &value
			}

			request := asc.TransactionHistoryRequest{
				StartDate:                    startValue,
				EndDate:                      endValue,
				ProductIDs:                   splitCSV(*productIDs),
				ProductTypes:                 types,
				SubscriptionGroupIdentifiers: splitCSV(*subscriptionGroupIDs),
				InAppOwnershipType:           ownership,
				Revoked:                      revokedValue,
				Sort:                         order,
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server history: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetTransactionHistory(requestCtx, trimmedID, request, *revision)
			if err != nil {
				return fmt.Errorf("server history: failed to fetch: %w", err)
			}

			if *paginate {
				aggregated, err := paginateTransactionHistory(requestCtx, client, trimmedID, request, resp, *limit)
				if err != nil {
					return fmt.Errorf("server history: %w", err)
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

func paginateTransactionHistory(ctx context.Context, client *asc.ServerAPIClient, transactionID string, request asc.TransactionHistoryRequest, first *asc.HistoryResponse, limit int) (*asc.HistoryResponse, error) {
	aggregated := &asc.HistoryResponse{
		AppAppleID:         first.AppAppleID,
		BundleID:           first.BundleID,
		Environment:        first.Environment,
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
		next, err := client.GetTransactionHistory(ctx, transactionID, request, *current.Revision)
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

func hasMore(value *bool) bool {
	return value != nil && *value
}
