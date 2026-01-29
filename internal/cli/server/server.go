package server

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// ServerCommand returns the server API command with subcommands.
func ServerCommand() *ffcli.Command {
	fs := flag.NewFlagSet("server", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "server",
		ShortUsage: "asc server <subcommand> [flags]",
		ShortHelp:  "Diagnostics for App Store Server API.",
		LongHelp: `Diagnostics for App Store Server API.

Examples:
  asc server status --transaction-id "TX_ID"
  asc server history --transaction-id "TX_ID" --paginate
  asc server notifications history --start "2026-01-01T00:00:00Z" --end "2026-01-02T00:00:00Z"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ServerStatusCommand(),
			ServerHistoryCommand(),
			ServerTransactionCommand(),
			ServerRefundsCommand(),
			ServerOrderCommand(),
			ServerNotificationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
