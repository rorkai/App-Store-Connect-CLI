package server

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// ServerOrderCommand returns the order command group.
func ServerOrderCommand() *ffcli.Command {
	fs := flag.NewFlagSet("order", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "order",
		ShortUsage: "asc server order <subcommand> [flags]",
		ShortHelp:  "Lookup server order details.",
		LongHelp: `Lookup server order details.

Examples:
  asc server order lookup --order-id "ORDER_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ServerOrderLookupCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// ServerOrderLookupCommand returns the order lookup subcommand.
func ServerOrderLookupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("lookup", flag.ExitOnError)

	orderID := fs.String("order-id", "", "Order ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "lookup",
		ShortUsage: "asc server order lookup --order-id \"ORDER_ID\" [flags]",
		ShortHelp:  "Look up transactions by order ID.",
		LongHelp: `Look up transactions by order ID.

Examples:
  asc server order lookup --order-id "ORDER_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedID := strings.TrimSpace(*orderID)
			if trimmedID == "" {
				fmt.Fprintln(os.Stderr, "Error: --order-id is required")
				return flag.ErrHelp
			}

			client, err := getServerClient()
			if err != nil {
				return fmt.Errorf("server order lookup: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.LookUpOrderID(requestCtx, trimmedID)
			if err != nil {
				return fmt.Errorf("server order lookup: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
