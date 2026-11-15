package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newGameCenterAppVersionsCommand returns the app versions command group.
func newGameCenterAppVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-versions", flag.ExitOnError)

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
			newGameCenterAppVersionsListCommand(d),
			newGameCenterAppVersionsGetCommand(d),
			newGameCenterAppVersionsCreateCommand(d),
			newGameCenterAppVersionsUpdateCommand(d),
			newGameCenterAppVersionCompatibilityCommand(d),
			newGameCenterAppVersionAppStoreVersionCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAppVersionsListCommand returns the app versions list subcommand.
func newGameCenterAppVersionsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center app-versions list [flags]",
		ShortHelp:  "List Game Center app versions.",
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
		LookupParent:         gameCenterDetailParent("game-center app-versions list"),
		FetchPage: func(ctx context.Context, client *asc.Client, detailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailGameCenterAppVersions(ctx, detailID, asc.Limit[asc.GCAppVersionsOption](limit), asc.NextURL[asc.GCAppVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterAppVersionsGetCommand returns the app versions get subcommand.
func newGameCenterAppVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center app-versions view --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "View a Game Center app version by ID.",
		LongHelp: `View a Game Center app version by ID.

Examples:
  asc game-center app-versions view --id "GC_APP_VERSION_ID"`,
		IDUsage:            "Game Center app version ID",
		IDType:             "gameCenterAppVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterAppVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterAppVersionsCreateCommand returns the app versions create subcommand.
func newGameCenterAppVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center app-versions create --app-store-version-id \"APP_STORE_VERSION_ID\"",
		ShortHelp:  "Create a Game Center app version.",
		LongHelp: `Create a Game Center app version.

Examples:
  asc game-center app-versions create --app-store-version-id "APP_STORE_VERSION_ID"`,
		IDFlag:  "app-store-version-id",
		IDUsage: "App Store version ID to associate",
		IDType:  "appStoreVersions",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterAppVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterAppVersionsUpdateCommand returns the app versions update subcommand.
func newGameCenterAppVersionsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	appVersionID := shared.BindResourceIDFlag(fs, "id", "gameCenterAppVersions", "Game Center app version ID")
	enabled := fs.String("enabled", "", "Enable or disable the app version (true/false)")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterAppVersionUpdateAttributes{}
			hasUpdate := false

			enabledVal := strings.TrimSpace(*enabled)
			if enabledVal != "" {
				if enabledVal != "true" && enabledVal != "false" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --enabled must be 'true' or 'false'")
					return flag.ErrHelp
				}
				b := enabledVal == "true"
				attrs.Enabled = &b
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, enabled)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center app-versions update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterAppVersion(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center app-versions update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterAppVersionCompatibilityCommand returns the compatibility command group.
func newGameCenterAppVersionCompatibilityCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("compatibility", flag.ExitOnError)

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
			newGameCenterAppVersionCompatibilityListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAppVersionCompatibilityListCommand returns the compatibility list subcommand.
func newGameCenterAppVersionCompatibilityListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center app-versions compatibility list --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "List compatible Game Center app versions.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterAppVersionCompatibilityVersions(ctx, id, asc.Limit[asc.GCAppVersionsOption](limit), asc.NextURL[asc.GCAppVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterAppVersionAppStoreVersionCommand returns the app store version command group.
func newGameCenterAppVersionAppStoreVersionCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-store-version", flag.ExitOnError)

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
			newGameCenterAppVersionAppStoreVersionGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAppVersionAppStoreVersionGetCommand returns the app store version get subcommand.
func newGameCenterAppVersionAppStoreVersionGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center app-versions app-store-version view --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "View the App Store version for a Game Center app version.",
		LongHelp: `View the App Store version for a Game Center app version.

Examples:
  asc game-center app-versions app-store-version view --id "GC_APP_VERSION_ID"`,
		IDUsage:            "Game Center app version ID",
		IDType:             "gameCenterAppVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterAppVersionAppStoreVersion(ctx, id)
		},
		Deps: d,
	})
}
