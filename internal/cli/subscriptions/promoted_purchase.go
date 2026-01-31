package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// SubscriptionsPromotedPurchaseCommand returns the promoted purchase command group.
func SubscriptionsPromotedPurchaseCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promoted-purchase", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "promoted-purchase",
		ShortUsage: "asc subscriptions promoted-purchase <subcommand> [flags]",
		ShortHelp:  "Inspect promoted purchase for a subscription.",
		LongHelp: `Inspect promoted purchase for a subscription.

Examples:
  asc subscriptions promoted-purchase get --id "SUBSCRIPTION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPromotedPurchaseGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPromotedPurchaseGetCommand returns the promoted purchase get subcommand.
func SubscriptionsPromotedPurchaseGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("promoted-purchase get", flag.ExitOnError)

	subscriptionID := fs.String("id", "", "Subscription ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions promoted-purchase get --id \"SUBSCRIPTION_ID\"",
		ShortHelp:  "Get the promoted purchase for a subscription.",
		LongHelp: `Get the promoted purchase for a subscription.

Examples:
  asc subscriptions promoted-purchase get --id "SUBSCRIPTION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions promoted-purchase get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionPromotedPurchase(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions promoted-purchase get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
