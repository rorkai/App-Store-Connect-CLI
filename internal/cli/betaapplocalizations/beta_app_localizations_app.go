package betaapplocalizations

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBetaAppLocalizationsAppCommand returns the app command group built on d.
func newBetaAppLocalizationsAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc beta-app-localizations app <subcommand> [flags]",
		ShortHelp:  "View the app for a beta app localization.",
		LongHelp: `View the app for a beta app localization.

Examples:
  asc beta-app-localizations app view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaAppLocalizationsAppGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaAppLocalizationsAppGetCommand returns the app get subcommand built on d.
func newBetaAppLocalizationsAppGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
		FlagSetName: "app view",
		Name:        "view",
		ShortUsage:  "asc beta-app-localizations app view --id \"LOCALIZATION_ID\"",
		ShortHelp:   "View the app for a beta app localization.",
		LongHelp: `View the app for a beta app localization.

Examples:
  asc beta-app-localizations app view --id "LOCALIZATION_ID"`,
		IDUsage:            "Beta app localization ID",
		IDType:             "betaAppLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppLocalizationApp(ctx, id)
		},
	})
}
