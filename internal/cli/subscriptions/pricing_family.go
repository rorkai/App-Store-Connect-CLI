package subscriptions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSubscriptionsPricingCommand returns the canonical pricing family.
func newSubscriptionsPricingCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "pricing",
		ShortUsage: "asc subscriptions pricing <subcommand> [flags]",
		ShortHelp:  "Manage subscription pricing.",
		LongHelp: `Manage subscription pricing.

Examples:
  asc subscriptions pricing summary --app "APP_ID"
  asc subscriptions pricing prices list --subscription-id "SUB_ID"
  asc subscriptions pricing prices additional-price-tiers --price-id "PRICE_ID"
  asc subscriptions pricing prices additional-price-tiers-links --price-id "PRICE_ID"
  asc subscriptions pricing prices set --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions pricing price-points list --subscription-id "SUB_ID" --territory "United States"
  asc subscriptions pricing price-tiers list --subscription-id "SUB_ID" --territory "USA"
  asc subscriptions pricing price-tiers equalize --subscription-id "SUB_ID" --base-price-point "POINT_ID" --additional-price-tiers "./tiers.json"
  asc subscriptions pricing price-change-preview --subscription-id "SUB_ID" --proposed-changes "./changes.json"
  asc subscriptions pricing availability view --subscription-id "SUB_ID"
  asc subscriptions pricing plan-availability show --subscription-id "SUB_ID"
  asc subscriptions pricing monthly-commitment list --subscription-id "SUB_ID"
  asc subscriptions pricing equalize --subscription-id "SUB_ID" --base-price "3.49" --dry-run
  asc subscriptions pricing derive --source-subscription-id "MONTHLY_ID" --target-subscription-id "YEARLY_ID" --multiplier "10" --dry-run
  asc subscriptions pricing ppp --subscription-id "SUB_ID" --base-price "9.99" --dry-run`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsPricingSummaryCommand(d),
			newSubscriptionsPricingPricesCommand(d),
			newSubscriptionsPricingPricePointsCommand(d),
			newSubscriptionsPricingPriceTiersCommand(d),
			newSubscriptionsPriceChangePreviewCommand(d),
			newSubscriptionsPricingAvailabilityCommand(d),
			newSubscriptionsPricingPlanAvailabilityCommand(d),
			newSubscriptionsPricingMonthlyCommitmentCommand(d),
			newSubscriptionsPricingEqualizeCommand(d),
			newSubscriptionsPricingDeriveCommand(d),
			newSubscriptionsPricingPPPCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newSubscriptionsPricingPricesCommand returns the canonical prices subgroup.
func newSubscriptionsPricingPricesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("pricing prices", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "prices",
		ShortUsage: "asc subscriptions pricing prices <subcommand> [flags]",
		ShortHelp:  "Manage subscription price records.",
		LongHelp: `Manage subscription price records.

Examples:
  asc subscriptions pricing prices list --subscription-id "SUB_ID"
  asc subscriptions pricing prices additional-price-tiers --price-id "PRICE_ID"
  asc subscriptions pricing prices additional-price-tiers-links --price-id "PRICE_ID"
  asc subscriptions pricing prices set --subscription-id "SUB_ID" --price-point "PRICE_POINT_ID"
  asc subscriptions pricing prices import --subscription-id "SUB_ID" --input "./prices.csv" --confirm
  asc subscriptions pricing prices delete --price-id "PRICE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubscriptionsPricesAdditionalPriceTiersCommand(d),
			newSubscriptionsPricesAdditionalPriceTiersLinksCommand(d),
			wrapSubscriptionsCommand(
				newSubscriptionsPricesListCommand(d),
				"asc subscriptions prices list",
				"asc subscriptions pricing prices list",
				"",
				"",
			),
			wrapSubscriptionsCommand(
				newSubscriptionsPricesAddCommand(d),
				"asc subscriptions prices add",
				"asc subscriptions pricing prices set",
				"set",
				"Set a subscription price.",
			),
			wrapSubscriptionsCommand(
				newSubscriptionsPricesImportCommand(d),
				"asc subscriptions prices import",
				"asc subscriptions pricing prices import",
				"",
				"",
			),
			wrapSubscriptionsCommand(
				newSubscriptionsPricesDeleteCommand(d),
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

// newSubscriptionsPricingPricePointsCommand returns the canonical price points subgroup.
func newSubscriptionsPricingPricePointsCommand(d *shared.Deps) *ffcli.Command {
	return wrapSubscriptionsCommand(
		newSubscriptionsPricePointsCommand(d),
		"asc subscriptions price-points",
		"asc subscriptions pricing price-points",
		"price-points",
		"Manage subscription price points.",
	)
}

// newSubscriptionsPricingAvailabilityCommand returns the canonical availability subgroup.
func newSubscriptionsPricingAvailabilityCommand(d *shared.Deps) *ffcli.Command {
	cmd := NewSubscriptionsAvailabilityCommand(d)
	cmd.ShortHelp = "Manage subscription availability."
	return cmd
}
