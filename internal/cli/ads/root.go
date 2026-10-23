package ads

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AdsCommand returns the Apple Ads root command.
func AdsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("ads", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "ads",
		ShortUsage: "asc ads <subcommand> [flags]",
		ShortHelp:  "Manage Apple Ads API resources.",
		LongHelp: `Manage Apple Ads API resources.

Apple Ads credentials are separate from App Store Connect API credentials.

Examples:
  asc ads auth login --name "Ads" --client-id "SEARCHADS..." --team-id "SEARCHADS..." --key-id "KEY_ID" --private-key ./private-key.pem --ad-account "123456"
  asc ads auth discover --output json
  asc ads apps search --ad-account "123456" --query "Example"
  asc ads api request --method GET --path v1/me`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: append([]*ffcli.Command{
			AuthCommand(),
			PlatformAPICommand(),
		}, platformEndpointCommands()...),
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 && args[0] == adsV5RemovedCommand {
				return removedAdsV5UsageError(args[1:])
			}
			return flag.ErrHelp
		},
	}
}
