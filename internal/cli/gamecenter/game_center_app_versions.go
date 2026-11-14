package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// GameCenterAppVersionsCommand returns the app versions command group.
func GameCenterAppVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-versions",
		ShortUsage: "asc game-center app-versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center app versions.",
		LongHelp: `Manage Game Center app versions.

Examples:
  asc game-center app-versions list --app "APP_ID"
  asc game-center app-versions view --id "GC_APP_VERSION_ID"
  asc game-center app-versions create --app-store-version-id "APP_STORE_VERSION_ID"
  asc game-center app-versions update --id "GC_APP_VERSION_ID" --enabled true
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID"
  asc game-center app-versions app-store-version view --id "GC_APP_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAppVersionsListCommand(),
			GameCenterAppVersionsGetCommand(),
			GameCenterAppVersionsCreateCommand(),
			GameCenterAppVersionsUpdateCommand(),
			GameCenterAppVersionCompatibilityCommand(),
			GameCenterAppVersionAppStoreVersionCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAppVersionsListCommand returns the app versions list subcommand.
func GameCenterAppVersionsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center app-versions list [flags]",
		ShortHelp:   "List Game Center app versions.",
		LongHelp: `List Game Center app versions.

Examples:
  asc game-center app-versions list --app "APP_ID"
  asc game-center app-versions list --app "APP_ID" --limit 50
  asc game-center app-versions list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center app-versions list",
		FetchErrorPrefix:     "game-center app-versions list: failed to fetch",
		LookupParent:         gameCenterDetailParent("game-center app-versions list"),
		FetchPage: func(ctx context.Context, client *asc.Client, detailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailGameCenterAppVersions(ctx, detailID, asc.WithGCAppVersionsLimit(limit), asc.WithGCAppVersionsNextURL(next))
		},
	})
}

// GameCenterAppVersionsGetCommand returns the app versions get subcommand.
func GameCenterAppVersionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc game-center app-versions view --id \"GC_APP_VERSION_ID\"",
		ShortHelp:   "View a Game Center app version by ID.",
		LongHelp: `View a Game Center app version by ID.

Examples:
  asc game-center app-versions view --id "GC_APP_VERSION_ID"`,
		IDUsage:            "Game Center app version ID",
		IDType:             "gameCenterAppVersions",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center app-versions view",
		FetchErrorPrefix:   "game-center app-versions view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterAppVersion(ctx, id)
		},
	})
}

// GameCenterAppVersionsCreateCommand returns the app versions create subcommand.
func GameCenterAppVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appStoreVersionID := shared.BindResourceIDFlag(fs, "app-store-version-id", "appStoreVersions", "App Store version ID to associate")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center app-versions create --app-store-version-id \"APP_STORE_VERSION_ID\"",
		ShortHelp:  "Create a Game Center app version.",
		LongHelp: `Create a Game Center app version.

Examples:
  asc game-center app-versions create --app-store-version-id "APP_STORE_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*appStoreVersionID)
			if id == "" {
				return shared.RequiredFlag("--app-store-version-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center app-versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterAppVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center app-versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAppVersionsUpdateCommand returns the app versions update subcommand.
func GameCenterAppVersionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	appVersionID := shared.BindResourceIDFlag(fs, "id", "gameCenterAppVersions", "Game Center app version ID")
	enabled := fs.String("enabled", "", "Enable or disable the app version (true/false)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center app-versions update --id \"GC_APP_VERSION_ID\" --enabled true",
		ShortHelp:  "Update a Game Center app version.",
		LongHelp: `Update a Game Center app version.

Examples:
  asc game-center app-versions update --id "GC_APP_VERSION_ID" --enabled true
  asc game-center app-versions update --id "GC_APP_VERSION_ID" --enabled false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*appVersionID)
			if id == "" {
				return shared.RequiredFlag("--id")
			}

			attrs := asc.GameCenterAppVersionUpdateAttributes{}
			hasUpdate := false

			enabledVal := strings.TrimSpace(*enabled)
			if enabledVal != "" {
				if enabledVal != "true" && enabledVal != "false" {
					fmt.Fprintln(os.Stderr, "Error: --enabled must be 'true' or 'false'")
					return flag.ErrHelp
				}
				b := enabledVal == "true"
				attrs.Enabled = &b
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required (--enabled)")
				return shared.MissingRequiredUsageError("--enabled")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center app-versions update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterAppVersion(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center app-versions update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAppVersionCompatibilityCommand returns the compatibility command group.
func GameCenterAppVersionCompatibilityCommand() *ffcli.Command {
	fs := flag.NewFlagSet("compatibility", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "compatibility",
		ShortUsage: "asc game-center app-versions compatibility list --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "List compatible Game Center app versions.",
		LongHelp: `List compatible Game Center app versions.

Examples:
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAppVersionCompatibilityListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAppVersionCompatibilityListCommand returns the compatibility list subcommand.
func GameCenterAppVersionCompatibilityListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center app-versions compatibility list --id \"GC_APP_VERSION_ID\"",
		ShortHelp:   "List compatible Game Center app versions.",
		LongHelp: `List compatible Game Center app versions.

Examples:
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID"
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID" --limit 50
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center app version ID",
		ParentType:           "gameCenterAppVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center app-versions compatibility list",
		FetchErrorPrefix:     "game-center app-versions compatibility list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterAppVersionCompatibilityVersions(ctx, id, asc.WithGCAppVersionsLimit(limit), asc.WithGCAppVersionsNextURL(next))
		},
	})
}

// GameCenterAppVersionAppStoreVersionCommand returns the app store version command group.
func GameCenterAppVersionAppStoreVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-version", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-store-version",
		ShortUsage: "asc game-center app-versions app-store-version view --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "View the App Store version for a Game Center app version.",
		LongHelp: `View the App Store version for a Game Center app version.

Examples:
  asc game-center app-versions app-store-version view --id "GC_APP_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAppVersionAppStoreVersionGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAppVersionAppStoreVersionGetCommand returns the app store version get subcommand.
func GameCenterAppVersionAppStoreVersionGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc game-center app-versions app-store-version view --id \"GC_APP_VERSION_ID\"",
		ShortHelp:   "View the App Store version for a Game Center app version.",
		LongHelp: `View the App Store version for a Game Center app version.

Examples:
  asc game-center app-versions app-store-version view --id "GC_APP_VERSION_ID"`,
		IDUsage:            "Game Center app version ID",
		IDType:             "gameCenterAppVersions",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center app-versions app-store-version view",
		FetchErrorPrefix:   "game-center app-versions app-store-version view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterAppVersionAppStoreVersion(ctx, id)
		},
	})
}
