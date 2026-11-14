package betaapplocalizations

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaAppLocalizationsAppCommand returns the app command group.
func BetaAppLocalizationsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc beta-app-localizations app <subcommand> [flags]",
		ShortHelp:  "View the app for a beta app localization.",
		LongHelp: `View the app for a beta app localization.

Examples:
  asc beta-app-localizations app view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaAppLocalizationsAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaAppLocalizationsAppGetCommand returns the app get subcommand.
func BetaAppLocalizationsAppGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
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
		ErrorPrefix:        "beta-app-localizations app view",
		FetchErrorPrefix:   "beta-app-localizations app view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetBetaAppLocalizationApp(ctx, id)
		},
	})
}
