package iap

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// IAPPromotedPurchaseCommand returns the promoted purchase command group.
func IAPPromotedPurchaseCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promoted-purchase", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "promoted-purchase",
		ShortUsage: "asc iap promoted-purchase <subcommand> [flags]",
		ShortHelp:  "Inspect promoted purchase for an in-app purchase.",
		LongHelp: `Inspect promoted purchase for an in-app purchase.

Examples:
  asc iap promoted-purchase get --id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			IAPPromotedPurchaseGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// IAPPromotedPurchaseGetCommand returns the promoted purchase get subcommand.
func IAPPromotedPurchaseGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promoted-purchase get", flag.ExitOnError)

	iapID := fs.String("id", "", "In-app purchase ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc iap promoted-purchase get --id \"IAP_ID\"",
		ShortHelp:  "Get the promoted purchase for an in-app purchase.",
		LongHelp: `Get the promoted purchase for an in-app purchase.

Examples:
  asc iap promoted-purchase get --id "IAP_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*iapID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("iap promoted-purchase get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetInAppPurchasePromotedPurchase(requestCtx, id)
			if err != nil {
				return fmt.Errorf("iap promoted-purchase get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
