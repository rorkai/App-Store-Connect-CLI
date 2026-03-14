package betabuildlocalizations

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaBuildLocalizationsBuildCommand returns the build command group.
func BetaBuildLocalizationsBuildCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "build",
		ShortUsage: "asc beta-build-localizations build <subcommand> [flags]",
		ShortHelp:  "DEPRECATED: legacy-only compatibility path.",
		LongHelp: `Deprecated compatibility path retained during migration.

No canonical ` + "`asc builds test-notes ...`" + ` replacement exists for the build relationship helper yet.

View the build for a beta build localization.

Examples:
  asc beta-build-localizations build get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaBuildLocalizationsLeafCommand(
				BetaBuildLocalizationsBuildGetCommand(),
				"",
				"Warning: `asc beta-build-localizations build get` is deprecated. No canonical replacement exists yet; this legacy helper remains available during transition.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: `asc beta-build-localizations build` is deprecated. No canonical replacement exists yet; this legacy helper remains available during transition.")
			return flag.ErrHelp
		},
	}
}

// BetaBuildLocalizationsBuildGetCommand returns the build get subcommand.
func BetaBuildLocalizationsBuildGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build get", flag.ExitOnError)

	id := fs.String("id", "", "Beta build localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc beta-build-localizations build get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get the build for a beta build localization.",
		LongHelp: `Get the build for a beta build localization.

Examples:
  asc beta-build-localizations build get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("beta-build-localizations build get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaBuildLocalizationBuild(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("beta-build-localizations build get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
