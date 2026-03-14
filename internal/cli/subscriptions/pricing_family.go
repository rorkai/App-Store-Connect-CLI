package subscriptions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsPricingCommand returns the canonical pricing family.
func SubscriptionsPricingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc subscriptions pricing <subcommand> [flags]",
		ShortHelp:  "Manage subscription pricing.",
		LongHelp: `Manage subscription pricing.

Examples:
  asc subscriptions pricing summary --app "APP_ID"
  asc subscriptions pricing prices list --subscription-id "SUB_ID"
  asc subscriptions pricing prices set --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions pricing price-points list --subscription-id "SUB_ID" --territory "USA"
  asc subscriptions pricing availability get --subscription-id "SUB_ID"
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsPricingSummaryCommand(),
			SubscriptionsPricingPricesCommand(),
			SubscriptionsPricingPricePointsCommand(),
			SubscriptionsPricingAvailabilityCommand(),
			SubscriptionsPricingEqualizeCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricingPricesCommand returns the canonical prices subgroup.
func SubscriptionsPricingPricesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("pricing prices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions pricing prices <subcommand> [flags]",
		ShortHelp:  "Manage subscription price records.",
		LongHelp: `Manage subscription price records.

Examples:
  asc subscriptions pricing prices list --subscription-id "SUB_ID"
  asc subscriptions pricing prices set --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions pricing prices import --subscription-id "SUB_ID" --input "./prices.csv"
  asc subscriptions pricing prices delete --price-id "PRICE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			wrapSubscriptionsCommand(
				SubscriptionsPricesListCommand(),
				"asc subscriptions prices list",
				"asc subscriptions pricing prices list",
				"",
				"",
			),
			wrapSubscriptionsCommand(
				SubscriptionsPricesAddCommand(),
				"asc subscriptions prices add",
				"asc subscriptions pricing prices set",
				"set",
				"Set a subscription price.",
			),
			wrapSubscriptionsCommand(
				SubscriptionsPricesImportCommand(),
				"asc subscriptions prices import",
				"asc subscriptions pricing prices import",
				"",
				"",
			),
			wrapSubscriptionsCommand(
				SubscriptionsPricesDeleteCommand(),
				"asc subscriptions prices delete",
				"asc subscriptions pricing prices delete",
				"",
				"",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsPricingPricePointsCommand returns the canonical price points subgroup.
func SubscriptionsPricingPricePointsCommand() *ffcli.Command {
	return wrapSubscriptionsCommand(
		SubscriptionsPricePointsCommand(),
		"asc subscriptions price-points",
		"asc subscriptions pricing price-points",
		"price-points",
		"Manage subscription price points.",
	)
}

// SubscriptionsPricingAvailabilityCommand returns the canonical availability subgroup.
func SubscriptionsPricingAvailabilityCommand() *ffcli.Command {
	return wrapSubscriptionsCommand(
		SubscriptionsAvailabilityCommand(),
		"asc subscriptions availability",
		"asc subscriptions pricing availability",
		"availability",
		"Manage subscription availability.",
	)
}
