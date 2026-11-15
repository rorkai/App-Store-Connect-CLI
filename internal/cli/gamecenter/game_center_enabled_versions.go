package gamecenter

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// GameCenterEnabledVersionsCommand returns the enabled versions command group.
func GameCenterEnabledVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("enabled-versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "enabled-versions",
		ShortUsage: "asc game-center enabled-versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center enabled versions.",
		LongHelp: `Manage Game Center enabled versions.

Examples:
  asc game-center enabled-versions list --app "APP_ID"
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterEnabledVersionsListCommand(),
			GameCenterEnabledVersionsCompatibleVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterEnabledVersionsListCommand returns the enabled versions list subcommand.
func GameCenterEnabledVersionsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center enabled-versions list [flags]",
		ShortHelp:  "List Game Center enabled versions for an app.",
		LongHelp: `List Game Center enabled versions for an app.

Examples:
  asc game-center enabled-versions list --app "APP_ID"
  asc game-center enabled-versions list --app "APP_ID" --limit 50
  asc game-center enabled-versions list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, appID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppGameCenterEnabledVersions(ctx, appID, asc.Limit[asc.GCEnabledVersionsOption](limit), asc.NextURL[asc.GCEnabledVersionsOption](next))
		},
	})
}

// GameCenterEnabledVersionsCompatibleVersionsCommand returns the compatible versions subcommand.
func GameCenterEnabledVersionsCompatibleVersionsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "compatible-versions",
		ShortUsage: "asc game-center enabled-versions compatible-versions --id \"ENABLED_VERSION_ID\"",
		ShortHelp:  "List compatible Game Center enabled versions.",
		LongHelp: `List compatible Game Center enabled versions.

Examples:
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID"
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID" --limit 50
  asc game-center enabled-versions compatible-versions --id "ENABLED_VERSION_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center enabled version ID",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, enabledVersionID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterEnabledVersionCompatibleVersions(ctx, enabledVersionID, asc.Limit[asc.GCEnabledVersionsOption](limit), asc.NextURL[asc.GCEnabledVersionsOption](next))
		},
	})
}
