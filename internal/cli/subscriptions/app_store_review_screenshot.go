package subscriptions

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

// SubscriptionsAppStoreReviewScreenshotCommand returns the app store review screenshot command group.
func SubscriptionsAppStoreReviewScreenshotCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-review-screenshot", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-store-review-screenshot",
		ShortUsage: "asc subscriptions app-store-review-screenshot <subcommand> [flags]",
		ShortHelp:  "Inspect the App Store review screenshot for a subscription.",
		LongHelp: `Inspect the App Store review screenshot for a subscription.

Examples:
  asc subscriptions app-store-review-screenshot view --subscription-id "SUBSCRIPTION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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
	fs := flag.NewFlagSet("app-store-review-screenshot view", flag.ExitOnError)

	subscriptionID := fs.String("subscription-id", "", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc subscriptions app-store-review-screenshot view --subscription-id \"SUBSCRIPTION_ID\"",
		ShortHelp:  "View the App Store review screenshot for a subscription.",
		LongHelp: `View the App Store review screenshot for a subscription.

Examples:
  asc subscriptions app-store-review-screenshot view --subscription-id "SUBSCRIPTION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --subscription-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions app-store-review-screenshot view: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeout(ctx, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions app-store-review-screenshot view: failed to fetch: %w", err)
			}
			if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
				return fmt.Errorf("subscriptions app-store-review-screenshot view: no App Store review screenshot found for subscription %q: %w", id, asc.ErrNotFound)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
