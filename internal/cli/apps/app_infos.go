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

// AppInfosCommand returns the app-infos command group.
func AppInfosCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-infos", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-infos",
		ShortUsage: "asc app-infos <subcommand> [flags]",
		ShortHelp:  "List app info records for an app.",
		LongHelp: `List app info records for an app.

An app can have multiple app info records (one per platform or state). This command
helps you find the specific app info ID you need when commands report "multiple app
infos found" errors.

Examples:
  asc app-infos list --app "APP_ID"
  asc app-infos list --app "APP_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppInfosListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppInfosListCommand returns the list subcommand for app-infos.
func AppInfosListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-infos list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-infos list [flags]",
		ShortHelp:  "List all app info records for an app.",
		LongHelp: `List all app info records for an app.

An app can have multiple app info records (one per platform or state). Use this
command to find the specific app info ID when you encounter "multiple app infos
found" errors in other commands.

Examples:
  asc app-infos list --app "APP_ID"
  asc app-infos list --app "APP_ID" --output table
  asc app-infos list --app "APP_ID" --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if strings.TrimSpace(resolvedAppID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-infos list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppInfos(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("app-infos list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
