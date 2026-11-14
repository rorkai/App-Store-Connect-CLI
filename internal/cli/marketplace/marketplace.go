package marketplace

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewMarketplaceCommand returns the marketplace command with subcommands, built
// on d.
func NewMarketplaceCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("marketplace", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "marketplace",
		ShortUsage: "asc marketplace <subcommand> [flags]",
		ShortHelp:  "Manage marketplace resources.",
		LongHelp: `Manage marketplace resources.

Examples:
  asc marketplace search-details view --app "APP_ID"
  asc marketplace webhooks list`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newMarketplaceSearchDetailsCommand(d),
			newMarketplaceWebhooksCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
