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
  asc game-center app-versions get --id "GC_APP_VERSION_ID"
  asc game-center app-versions create --app-store-version-id "APP_STORE_VERSION_ID"
  asc game-center app-versions update --id "GC_APP_VERSION_ID" --enabled true
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID"
  asc game-center app-versions app-store-version get --id "GC_APP_VERSION_ID"`,
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center app-versions list [flags]",
		ShortHelp:  "List Game Center app versions.",
		LongHelp: `List Game Center app versions.

Examples:
  asc game-center app-versions list --app "APP_ID"
  asc game-center app-versions list --app "APP_ID" --limit 50
  asc game-center app-versions list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center app-versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center app-versions list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center app-versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detailID := ""
			if nextURL == "" {
				var err error
				detailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center app-versions list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCAppVersionsOption{
				asc.WithGCAppVersionsLimit(*limit),
				asc.WithGCAppVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := []asc.GCAppVersionsOption{asc.WithGCAppVersionsNextURL(*next)}
				if nextURL == "" {
					paginateOpts = []asc.GCAppVersionsOption{asc.WithGCAppVersionsLimit(200)}
				}
				firstPage, err := client.GetGameCenterDetailGameCenterAppVersions(requestCtx, detailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center app-versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailGameCenterAppVersions(ctx, detailID, asc.WithGCAppVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center app-versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterDetailGameCenterAppVersions(requestCtx, detailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center app-versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAppVersionsGetCommand returns the app versions get subcommand.
func GameCenterAppVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	appVersionID := fs.String("id", "", "Game Center app version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center app-versions get --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "Get a Game Center app version by ID.",
		LongHelp: `Get a Game Center app version by ID.

Examples:
  asc game-center app-versions get --id "GC_APP_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*appVersionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center app-versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAppVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center app-versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAppVersionsCreateCommand returns the app versions create subcommand.
func GameCenterAppVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appStoreVersionID := fs.String("app-store-version-id", "", "App Store version ID to associate")
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
				fmt.Fprintln(os.Stderr, "Error: --app-store-version-id is required")
				return flag.ErrHelp
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

	appVersionID := fs.String("id", "", "Game Center app version ID")
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
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				return flag.ErrHelp
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appVersionID := fs.String("id", "", "Game Center app version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center app-versions compatibility list --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "List compatible Game Center app versions.",
		LongHelp: `List compatible Game Center app versions.

Examples:
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID"
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID" --limit 50
  asc game-center app-versions compatibility list --id "GC_APP_VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center app-versions compatibility list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center app-versions compatibility list: %w", err)
			}

			id := strings.TrimSpace(*appVersionID)
			nextURL := strings.TrimSpace(*next)
			if id == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center app-versions compatibility list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAppVersionsOption{
				asc.WithGCAppVersionsLimit(*limit),
				asc.WithGCAppVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := []asc.GCAppVersionsOption{asc.WithGCAppVersionsNextURL(*next)}
				if nextURL == "" {
					paginateOpts = []asc.GCAppVersionsOption{asc.WithGCAppVersionsLimit(200)}
				}
				firstPage, err := client.GetGameCenterAppVersionCompatibilityVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center app-versions compatibility list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAppVersionCompatibilityVersions(ctx, id, asc.WithGCAppVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center app-versions compatibility list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAppVersionCompatibilityVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center app-versions compatibility list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAppVersionAppStoreVersionCommand returns the app store version command group.
func GameCenterAppVersionAppStoreVersionCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-store-version", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-store-version",
		ShortUsage: "asc game-center app-versions app-store-version get --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "Get the App Store version for a Game Center app version.",
		LongHelp: `Get the App Store version for a Game Center app version.

Examples:
  asc game-center app-versions app-store-version get --id "GC_APP_VERSION_ID"`,
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	appVersionID := fs.String("id", "", "Game Center app version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center app-versions app-store-version get --id \"GC_APP_VERSION_ID\"",
		ShortHelp:  "Get the App Store version for a Game Center app version.",
		LongHelp: `Get the App Store version for a Game Center app version.

Examples:
  asc game-center app-versions app-store-version get --id "GC_APP_VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*appVersionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center app-versions app-store-version get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAppVersionAppStoreVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center app-versions app-store-version get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
