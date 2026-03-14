package subscriptions

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SubscriptionsOffersCommand returns the canonical offers family.
func SubscriptionsOffersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("offers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "offers",
		ShortUsage: "asc subscriptions offers <subcommand> [flags]",
		ShortHelp:  "Manage subscription offers.",
		LongHelp: `Manage subscription offers.

Examples:
  asc subscriptions offers introductory list --subscription-id "SUB_ID"
  asc subscriptions offers promotional create --subscription-id "SUB_ID" --offer-code "SPRING" --name "Spring" --offer-duration ONE_MONTH --offer-mode FREE_TRIAL --number-of-periods 1 --prices "PRICE_ID"
  asc subscriptions offers offer-codes generate --offer-code-id "OFFER_CODE_ID" --quantity 10 --expiration-date "2026-02-01"
  asc subscriptions offers win-back list --subscription-id "SUB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			wrapSubscriptionsCommand(
				SubscriptionsIntroductoryOffersCommand(),
				"asc subscriptions introductory-offers",
				"asc subscriptions offers introductory",
				"introductory",
				"Manage subscription introductory offers.",
			),
			wrapSubscriptionsCommand(
				SubscriptionsPromotionalOffersCommand(),
				"asc subscriptions promotional-offers",
				"asc subscriptions offers promotional",
				"promotional",
				"Manage subscription promotional offers.",
			),
			wrapSubscriptionsCommand(
				SubscriptionsOfferCodesCommand(),
				"asc subscriptions offer-codes",
				"asc subscriptions offers offer-codes",
				"offer-codes",
				"Manage subscription offer codes.",
			),
			wrapSubscriptionsCommand(
				SubscriptionsWinBackOffersCommand(),
				"asc subscriptions win-back-offers",
				"asc subscriptions offers win-back",
				"win-back",
				"Manage subscription win-back offers.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
