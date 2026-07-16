package betaapplocalizations

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
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
	fs := flag.NewFlagSet("app view", flag.ExitOnError)

	id := fs.String("id", "", "Beta app localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc beta-app-localizations app view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View the app for a beta app localization.",
		LongHelp: `View the app for a beta app localization.

Examples:
  asc beta-app-localizations app view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-app-localizations app view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaAppLocalizationApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-app-localizations app view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
