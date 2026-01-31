package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// SubscriptionsAppStoreReviewScreenshotCommand returns the app store review screenshot command group.
func SubscriptionsAppStoreReviewScreenshotCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-review-screenshot", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-store-review-screenshot",
		ShortUsage: "asc subscriptions app-store-review-screenshot <subcommand> [flags]",
		ShortHelp:  "Inspect the App Store review screenshot for a subscription.",
		LongHelp: `Inspect the App Store review screenshot for a subscription.

Examples:
  asc subscriptions app-store-review-screenshot get --id "SUBSCRIPTION_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsAppStoreReviewScreenshotGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsAppStoreReviewScreenshotGetCommand returns the get subcommand.
func SubscriptionsAppStoreReviewScreenshotGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-review-screenshot get", flag.ExitOnError)

	subscriptionID := fs.String("id", "", "Subscription ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions app-store-review-screenshot get --id \"SUBSCRIPTION_ID\"",
		ShortHelp:  "Get the App Store review screenshot for a subscription.",
		LongHelp: `Get the App Store review screenshot for a subscription.

Examples:
  asc subscriptions app-store-review-screenshot get --id "SUBSCRIPTION_ID"`,
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
				return fmt.Errorf("subscriptions app-store-review-screenshot get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions app-store-review-screenshot get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
