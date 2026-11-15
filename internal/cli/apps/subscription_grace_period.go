package apps

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppsSubscriptionGracePeriodCommand returns the subscription grace period command group.
func newAppsSubscriptionGracePeriodCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("subscription-grace-period", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "subscription-grace-period",
		ShortUsage: "asc apps subscription-grace-period <subcommand> [flags]",
		ShortHelp:  "Inspect an app's subscription grace period.",
		LongHelp: `Inspect an app's subscription grace period.

Examples:
  asc apps subscription-grace-period view --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppsSubscriptionGracePeriodGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppsSubscriptionGracePeriodGetCommand returns the subscription grace period get subcommand.
func newAppsSubscriptionGracePeriodGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "subscription-grace-period view",
		Name:        "view",
		ShortUsage:  "asc apps subscription-grace-period view --app \"APP_ID\"",
		ShortHelp:   "View an app's subscription grace period.",
		LongHelp: `View an app's subscription grace period.

Examples:
  asc apps subscription-grace-period view --app "APP_ID"`,
		IDFlag:             "app",
		IDUsage:            "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:         true,
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, appID string) (any, error) {
			return client.GetAppSubscriptionGracePeriod(ctx, appID)
		},
	})
}
