package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// ServerStatusCommand returns the server status subcommand.
func ServerStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	transactionID := fs.String("transaction-id", "", "Transaction ID")
	status := fs.String("status", "", "Filter by status: "+strings.Join(serverStatusList(), ", "))
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc server status --transaction-id \"TX_ID\" [flags]",
		ShortHelp:  "Get subscription status for a transaction.",
		LongHelp: `Get subscription status for a transaction.

Examples:
  asc server status --transaction-id "TX_ID"
  asc server status --transaction-id "TX_ID" --status ACTIVE,EXPIRED`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*transactionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --transaction-id is required")
				return flag.ErrHelp
			}

			statuses, err := normalizeServerStatuses(*status)
			if err != nil {
				return fmt.Errorf("server status: %w", err)
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server status: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAllSubscriptionStatuses(requestCtx, trimmedID, statuses)
			if err != nil {
				return fmt.Errorf("server status: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
