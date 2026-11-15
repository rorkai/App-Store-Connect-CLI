package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSubscriptionsAppStoreReviewScreenshotCommand returns the app store review screenshot command group.
func newSubscriptionsAppStoreReviewScreenshotCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-store-review-screenshot", flag.ExitOnError)

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
			newSubscriptionsAppStoreReviewScreenshotGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsAppStoreReviewScreenshotGetCommand returns the get subcommand.
func newSubscriptionsAppStoreReviewScreenshotGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-store-review-screenshot view", flag.ExitOnError)

	subscriptionID := shared.BindResourceIDFlag(fs, "subscription-id", "subscriptions", "Subscription ID, product ID, or exact current name")
	appID := addSubscriptionLookupAppFlag(fs)
	subscriptionFields := fs.String("subscription-fields", "", "Included subscription fields (comma-separated)")
	output := d.BindOutputFlags(fs)

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
			selectedSubscriptionFields, err := normalizeSparseFieldsFlag(fs, "", "subscription-fields", *subscriptionFields, subscriptionFieldsList())
			if err != nil {
				return err
			}
			id := strings.TrimSpace(*subscriptionID)
			if id == "" {
				return d.RequiredFlag("--subscription-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("subscriptions app-store-review-screenshot view: %w", err)
			}

			id, err = resolveSubscriptionLookupIDWithTimeoutWith(ctx, d, client, *appID, id)
			if err != nil {
				return err
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionAppStoreReviewScreenshotForSubscription(
				requestCtx, id,
				asc.WithSubscriptionAppStoreReviewScreenshotSubscriptionFields(selectedSubscriptionFields),
				asc.WithSubscriptionAppStoreReviewScreenshotInclude(includeRelationshipForFields(selectedSubscriptionFields, "subscription")),
			)
			if err != nil {
				return fmt.Errorf("subscriptions app-store-review-screenshot view: failed to fetch: %w", err)
			}
			if resp == nil || strings.TrimSpace(resp.Data.ID) == "" {
				return fmt.Errorf("subscriptions app-store-review-screenshot view: no App Store review screenshot found for subscription %q: %w", id, asc.ErrNotFound)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
