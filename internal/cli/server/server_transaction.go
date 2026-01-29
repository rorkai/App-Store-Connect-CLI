package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// ServerTransactionCommand returns the transaction command group.
func ServerTransactionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("transaction", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "transaction",
		ShortUsage: "asc server transaction <subcommand> [flags]",
		ShortHelp:  "Inspect server transaction details.",
		LongHelp: `Inspect server transaction details.

Examples:
  asc server transaction get --transaction-id "TX_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ServerTransactionGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ServerTransactionGetCommand returns the transaction get subcommand.
func ServerTransactionGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	transactionID := fs.String("transaction-id", "", "Transaction ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc server transaction get --transaction-id \"TX_ID\" [flags]",
		ShortHelp:  "Get transaction info.",
		LongHelp: `Get transaction info.

Examples:
  asc server transaction get --transaction-id "TX_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*transactionID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --transaction-id is required")
				return flag.ErrHelp
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server transaction get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetTransactionInfo(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("server transaction get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
