package apps

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AppsSubscriptionGracePeriodCommand returns the subscription grace period command group.
func AppsSubscriptionGracePeriodCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscription-grace-period", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscription-grace-period",
		ShortUsage: "asc apps subscription-grace-period <subcommand> [flags]",
		ShortHelp:  "Inspect an app's subscription grace period.",
		LongHelp: `Inspect an app's subscription grace period.

Examples:
  asc apps subscription-grace-period get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppsSubscriptionGracePeriodGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppsSubscriptionGracePeriodGetCommand returns the subscription grace period get subcommand.
func AppsSubscriptionGracePeriodGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("subscription-grace-period get", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc apps subscription-grace-period get --app \"APP_ID\"",
		ShortHelp:  "Get an app's subscription grace period.",
		LongHelp: `Get an app's subscription grace period.

Examples:
  asc apps subscription-grace-period get --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("apps subscription-grace-period get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppSubscriptionGracePeriod(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("apps subscription-grace-period get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
