package subscriptions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSubscriptionsReviewCommand returns the canonical review family.
func newSubscriptionsReviewCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc subscriptions review <subcommand> [flags]",
		ShortHelp:  "Manage subscription review workflows.",
		LongHelp: `Manage subscription review workflows.

These subcommands manage the review assets on a subscription. Adding the
subscription or group version to a review submission runs through the review
command group:
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"

Examples:
  asc subscriptions review screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"
  asc subscriptions review app-store-screenshot view --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			wrapSubscriptionsCommand(
				newSubscriptionsReviewScreenshotsCommand(d),
				"asc subscriptions review-screenshots",
				"asc subscriptions review screenshots",
				"screenshots",
				"Manage subscription App Store review screenshots.",
			),
			wrapSubscriptionsCommand(
				newSubscriptionsAppStoreReviewScreenshotCommand(d),
				"asc subscriptions app-store-review-screenshot",
				"asc subscriptions review app-store-screenshot",
				"app-store-screenshot",
				"Inspect the App Store review screenshot for a subscription.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
