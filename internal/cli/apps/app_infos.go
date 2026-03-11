package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const appInfosDeprecationWarning = "Warning: `asc app-infos list` is deprecated. Use `asc app-info list`."

// AppInfosCommand returns the app-infos command group.
func AppInfosCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-infos", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-infos",
		ShortUsage: "asc app-info list [flags]",
		ShortHelp:  "DEPRECATED: use `asc app-info list`.",
		LongHelp: `DEPRECATED: use ` + "`asc app-info list`" + `.

This compatibility alias preserves the legacy plural root while the canonical app
info discovery workflow lives under ` + "`asc app-info list`" + `.

Examples:
  asc app-info list --app "APP_ID"
  asc app-info list --app "APP_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			AppInfosListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppInfoListCommand returns the canonical list subcommand for app-info.
func AppInfoListCommand() *ffcli.Command {
	return newAppInfoListCommand(
		"app-info list",
		"list",
		"asc app-info list [flags]",
		"List all app info records for an app.",
		`List all app info records for an app.

An app can have multiple app info records (one per platform or state). Use this
command to find the specific app info ID when you encounter "multiple app infos
found" errors in other commands.

Examples:
  asc app-info list --app "APP_ID"
  asc app-info list --app "APP_ID" --output table
  asc app-info list --app "APP_ID" --output markdown`,
		"app-info list",
		"",
		shared.DefaultUsageFunc,
	)
}

// AppInfosListCommand returns the deprecated compatibility alias for app-infos list.
func AppInfosListCommand() *ffcli.Command {
	return newAppInfoListCommand(
		"app-infos list",
		"list",
		"asc app-info list [flags]",
		"DEPRECATED: use `asc app-info list`.",
		`DEPRECATED: use `+"`asc app-info list`"+`.

This compatibility alias preserves the legacy plural command while the canonical
discovery path lives under `+"`asc app-info list`"+`.

Examples:
  asc app-info list --app "APP_ID"
  asc app-info list --app "APP_ID" --output table
  asc app-info list --app "APP_ID" --output markdown`,
		"app-infos list",
		appInfosDeprecationWarning,
		shared.DeprecatedUsageFunc,
	)
}

func newAppInfoListCommand(
	flagSetName, commandName, shortUsage, shortHelp, longHelp, errorPrefix, deprecatedWarning string,
	usageFunc func(*ffcli.Command) string,
) *ffcli.Command {
	fs := flag.NewFlagSet(flagSetName, flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       commandName,
		ShortUsage: shortUsage,
		ShortHelp:  shortHelp,
		LongHelp:   longHelp,
		FlagSet:    fs,
		UsageFunc:  usageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(deprecatedWarning) != "" {
				fmt.Fprintln(os.Stderr, deprecatedWarning)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppInfos(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("%s: failed to fetch: %w", errorPrefix, err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
