package betaapplocalizations

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const betaAppLocalizationsCanonicalRoot = "asc testflight app-localizations"

// DeprecatedBetaAppLocalizationsCommand returns the hidden compatibility root.
func DeprecatedBetaAppLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-app-localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-app-localizations",
		ShortUsage: betaAppLocalizationsCanonicalRoot + " <subcommand> [flags]",
		ShortHelp:  "DEPRECATED: use `asc testflight app-localizations ...`.",
		LongHelp: `Deprecated compatibility alias for TestFlight app localizations.

Canonical workflows now live under ` + "`asc testflight app-localizations ...`" + `.

Examples:
  asc testflight app-localizations list --app "APP_ID"
  asc testflight app-localizations create --app "APP_ID" --locale "en-US" --description "Welcome testers"`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaAppLocalizationsLeafCommand(
				BetaAppLocalizationsListCommand(),
				betaAppLocalizationsCanonicalRoot+" list [flags]",
				betaAppLocalizationsCanonicalRoot+" list",
				"Warning: `asc beta-app-localizations list` is deprecated. Use `asc testflight app-localizations list`.",
			),
			deprecatedBetaAppLocalizationsLeafCommand(
				BetaAppLocalizationsGetCommand(),
				betaAppLocalizationsCanonicalRoot+" view --id \"LOCALIZATION_ID\"",
				betaAppLocalizationsCanonicalRoot+" view",
				"Warning: `asc beta-app-localizations get` is deprecated. Use `asc testflight app-localizations view`.",
			),
			deprecatedBetaAppLocalizationsAppCommand(),
			deprecatedBetaAppLocalizationsLeafCommand(
				BetaAppLocalizationsCreateCommand(),
				betaAppLocalizationsCanonicalRoot+" create [flags]",
				betaAppLocalizationsCanonicalRoot+" create",
				"Warning: `asc beta-app-localizations create` is deprecated. Use `asc testflight app-localizations create`.",
			),
			deprecatedBetaAppLocalizationsLeafCommand(
				BetaAppLocalizationsUpdateCommand(),
				betaAppLocalizationsCanonicalRoot+" update [flags]",
				betaAppLocalizationsCanonicalRoot+" update",
				"Warning: `asc beta-app-localizations update` is deprecated. Use `asc testflight app-localizations update`.",
			),
			deprecatedBetaAppLocalizationsLeafCommand(
				BetaAppLocalizationsDeleteCommand(),
				betaAppLocalizationsCanonicalRoot+" delete --id \"LOCALIZATION_ID\" --confirm",
				betaAppLocalizationsCanonicalRoot+" delete",
				"Warning: `asc beta-app-localizations delete` is deprecated. Use `asc testflight app-localizations delete`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			fmt.Fprintln(os.Stderr, "Warning: `asc beta-app-localizations` is deprecated. Use `asc testflight app-localizations ...`.")
			return flag.ErrHelp
		},
	}
}

func deprecatedBetaAppLocalizationsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: betaAppLocalizationsCanonicalRoot + " app <subcommand> [flags]",
		ShortHelp:  "Compatibility alias: use `asc testflight app-localizations app view`.",
		LongHelp:   "Compatibility alias: use `asc testflight app-localizations app view --id LOCALIZATION_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaAppLocalizationsLeafCommand(
				BetaAppLocalizationsAppGetCommand(),
				betaAppLocalizationsCanonicalRoot+" app view --id \"LOCALIZATION_ID\"",
				betaAppLocalizationsCanonicalRoot+" app view",
				"Warning: `asc beta-app-localizations app get` is deprecated. Use `asc testflight app-localizations app view`.",
			),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

func deprecatedBetaAppLocalizationsLeafCommand(cmd *ffcli.Command, shortUsage, newCommand, warning string) *ffcli.Command {
	if cmd == nil {
		return nil
	}

	clone := *cmd
	clone.ShortUsage = shortUsage
	clone.ShortHelp = fmt.Sprintf("Compatibility alias: use `%s`.", newCommand)
	clone.LongHelp = fmt.Sprintf("Compatibility alias: use `%s`.", shortUsage)
	clone.UsageFunc = shared.DeprecatedUsageFunc

	origExec := cmd.Exec
	clone.Exec = func(ctx context.Context, args []string) error {
		fmt.Fprintln(os.Stderr, warning)
		return origExec(ctx, args)
	}

	return &clone
}
