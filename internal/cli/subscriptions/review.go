package subscriptions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsReviewCommand returns the canonical review family.
func SubscriptionsReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc subscriptions review <subcommand> [flags]",
		ShortHelp:  "Manage subscription review workflows.",
		LongHelp: `Manage subscription review workflows.

Examples:
  asc subscriptions review screenshots create --subscription-id "SUB_ID" --file "./screenshot.png"
  asc subscriptions review app-store-screenshot view --subscription-id "SUB_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"
  asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			wrapSubscriptionsCommand(
				SubscriptionsReviewScreenshotsCommand(),
				"asc subscriptions review-screenshots",
				"asc subscriptions review screenshots",
				"screenshots",
				"Manage subscription App Store review screenshots.",
			),
			wrapSubscriptionsCommand(
				SubscriptionsAppStoreReviewScreenshotCommand(),
				"asc subscriptions app-store-review-screenshot",
				"asc subscriptions review app-store-screenshot",
				"app-store-screenshot",
				"Inspect the App Store review screenshot for a subscription.",
			),
			shared.DeprecatedCommand(wrapSubscriptionsCommand(
				SubscriptionsSubmitCommand(),
				"asc subscriptions submit",
				"asc subscriptions review submit",
				"submit",
				"Submit a subscription for review.",
			), "asc subscriptions review submit", `asc review items add --submission "SUBMISSION_ID" --item-type subscriptionVersions --item-id "SUBSCRIPTION_VERSION_ID"`),
			shared.DeprecatedCommand(wrapSubscriptionsCommand(
				SubscriptionsGroupsSubmitCommand(),
				"asc subscriptions groups submit",
				"asc subscriptions review submit-group",
				"submit-group",
				"Submit a subscription group for review.",
			), "asc subscriptions review submit-group", `asc review items add --submission "SUBMISSION_ID" --item-type subscriptionGroupVersions --item-id "GROUP_VERSION_ID"`),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
