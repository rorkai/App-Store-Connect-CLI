package storekit

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// Command returns the StoreKit API command group.
func Command() *ffcli.Command {
	fs := flag.NewFlagSet("storekit", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "storekit",
		ShortUsage: "asc storekit <subcommand> [flags]",
		ShortHelp:  "Manage StoreKit server APIs with In-App Purchase API keys.",
		LongHelp: `Manage StoreKit server APIs with dedicated In-App Purchase API keys.

StoreKit credentials are separate from App Store Connect API credentials.

Examples:
  asc storekit auth login --name Production --key-id KEY_ID --issuer-id ISSUER_ID --private-key ./SubscriptionKey.p8 --bundle-id com.example.app
  asc storekit transactions history --transaction-id 2000000000000001 --environment production --paginate
  asc storekit subscriptions status --transaction-id 2000000000000001 --environment production
  asc storekit notifications history --start 2026-09-01 --end 2026-09-08 --environment production
  asc storekit retention-messaging messages list --environment sandbox
  asc storekit retention-messaging endpoint set --url https://example.com/retention --environment production`,
		FlagSet: fs,
		Subcommands: []*ffcli.Command{
			AuthCommand(),
			TransactionsCommand(),
			AppTransactionsCommand(),
			SubscriptionsCommand(),
			RefundsCommand(),
			OrdersCommand(),
			NotificationsCommand(),
			GroupsCommand(),
			RetentionMessagingCommand(),
		},
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
