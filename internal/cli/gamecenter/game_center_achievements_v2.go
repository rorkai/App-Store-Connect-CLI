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

// GameCenterAchievementsV2Command returns the achievements v2 command group.
func GameCenterAchievementsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "v2",
		ShortUsage: "asc game-center achievements v2 <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievements v2 resources.",
		LongHelp: `Manage Game Center achievements v2 resources.

Examples:
  asc game-center achievements v2 list --app "APP_ID"
  asc game-center achievements v2 versions list --achievement-id "ACH_ID"
  asc game-center achievements v2 localizations list --version-id "VER_ID"
  asc game-center achievements v2 images upload --localization-id "LOC_ID" --file "path/to/image.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementsV2ListCommand(),
			GameCenterAchievementVersionsV2Command(),
			GameCenterAchievementLocalizationsV2Command(),
			GameCenterAchievementImagesV2Command(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementsV2ListCommand returns the achievements v2 list subcommand.
func GameCenterAchievementsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center achievements v2 list [flags]",
		ShortHelp:  "List Game Center achievements (v2) for an app or group.",
		LongHelp: `List Game Center achievements (v2) for an app or group.

Examples:
  asc game-center achievements v2 list --app "APP_ID"
  asc game-center achievements v2 list --group-id "GROUP_ID"
  asc game-center achievements v2 list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center achievements v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center achievements v2 list: %w", err)
			}

			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if group == "" && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" && nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCAchievementsOption{
				asc.WithGCAchievementsLimit(*limit),
				asc.WithGCAchievementsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementsLimit(200))
				firstPage, err := client.GetGameCenterAchievementsV2(requestCtx, gcDetailID, group, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievementsV2(ctx, gcDetailID, group, asc.WithGCAchievementsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAchievementsV2(requestCtx, gcDetailID, group, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementVersionsV2Command returns the achievement versions v2 command group.
func GameCenterAchievementVersionsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center achievements v2 versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement versions (v2).",
		LongHelp: `Manage Game Center achievement versions (v2).

Examples:
  asc game-center achievements v2 versions list --achievement-id "ACH_ID"
  asc game-center achievements v2 versions get --id "VERSION_ID"
  asc game-center achievements v2 versions create --achievement-id "ACH_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementVersionsV2ListCommand(),
			GameCenterAchievementVersionsV2GetCommand(),
			GameCenterAchievementVersionsV2CreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementVersionsV2ListCommand returns the achievement versions v2 list subcommand.
func GameCenterAchievementVersionsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	achievementID := fs.String("achievement-id", "", "Game Center achievement ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center achievements v2 versions list --achievement-id \"ACH_ID\"",
		ShortHelp:  "List versions for a Game Center achievement (v2).",
		LongHelp: `List versions for a Game Center achievement (v2).

Examples:
  asc game-center achievements v2 versions list --achievement-id "ACH_ID"
  asc game-center achievements v2 versions list --achievement-id "ACH_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center achievements v2 versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center achievements v2 versions list: %w", err)
			}

			id := strings.TrimSpace(*achievementID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --achievement-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementVersionsOption{
				asc.WithGCAchievementVersionsLimit(*limit),
				asc.WithGCAchievementVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementVersionsLimit(200))
				firstPage, err := client.GetGameCenterAchievementVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievementVersions(ctx, id, asc.WithGCAchievementVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements v2 versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAchievementVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementVersionsV2GetCommand returns the achievement versions v2 get subcommand.
func GameCenterAchievementVersionsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center achievement version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements v2 versions get --id \"VERSION_ID\"",
		ShortHelp:  "Get a Game Center achievement version (v2) by ID.",
		LongHelp: `Get a Game Center achievement version (v2) by ID.

Examples:
  asc game-center achievements v2 versions get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementVersionsV2CreateCommand returns the achievement versions v2 create subcommand.
func GameCenterAchievementVersionsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	achievementID := fs.String("achievement-id", "", "Game Center achievement ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center achievements v2 versions create --achievement-id \"ACH_ID\"",
		ShortHelp:  "Create a new Game Center achievement version (v2).",
		LongHelp: `Create a new Game Center achievement version (v2).

Examples:
  asc game-center achievements v2 versions create --achievement-id "ACH_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*achievementID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --achievement-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterAchievementVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementLocalizationsV2Command returns the achievement localizations v2 command group.
func GameCenterAchievementLocalizationsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center achievements v2 localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement localizations (v2).",
		LongHelp: `Manage Game Center achievement localizations (v2).

Examples:
  asc game-center achievements v2 localizations list --version-id "VER_ID"
  asc game-center achievements v2 localizations create --version-id "VER_ID" --locale en-US --name "First Win" --before-earned-description "Before" --after-earned-description "After"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementLocalizationsV2ListCommand(),
			GameCenterAchievementLocalizationsV2GetCommand(),
			GameCenterAchievementLocalizationsV2CreateCommand(),
			GameCenterAchievementLocalizationsV2UpdateCommand(),
			GameCenterAchievementLocalizationsV2DeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementLocalizationsV2ListCommand returns the achievement localizations v2 list subcommand.
func GameCenterAchievementLocalizationsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center achievement version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center achievements v2 localizations list --version-id \"VER_ID\"",
		ShortHelp:  "List localizations for an achievement version (v2).",
		LongHelp: `List localizations for an achievement version (v2).

Examples:
  asc game-center achievements v2 localizations list --version-id "VER_ID"
  asc game-center achievements v2 localizations list --version-id "VER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center achievements v2 localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center achievements v2 localizations list: %w", err)
			}

			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementLocalizationsOption{
				asc.WithGCAchievementLocalizationsLimit(*limit),
				asc.WithGCAchievementLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterAchievementVersionLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievementVersionLocalizations(ctx, id, asc.WithGCAchievementLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements v2 localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAchievementVersionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementLocalizationsV2GetCommand returns the achievement localizations v2 get subcommand.
func GameCenterAchievementLocalizationsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements v2 localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center achievement localization (v2) by ID.",
		LongHelp: `Get a Game Center achievement localization (v2) by ID.

Examples:
  asc game-center achievements v2 localizations get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementLocalizationV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementLocalizationsV2CreateCommand returns the achievement localizations v2 create subcommand.
func GameCenterAchievementLocalizationsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center achievement version ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	name := fs.String("name", "", "Achievement name")
	beforeEarned := fs.String("before-earned-description", "", "Description shown before earning")
	afterEarned := fs.String("after-earned-description", "", "Description shown after earning")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center achievements v2 localizations create --version-id \"VER_ID\" --locale \"LOCALE\" --name \"NAME\" --before-earned-description \"TEXT\" --after-earned-description \"TEXT\"",
		ShortHelp:  "Create a new Game Center achievement localization (v2).",
		LongHelp: `Create a new Game Center achievement localization (v2).

Examples:
  asc game-center achievements v2 localizations create --version-id "VER_ID" --locale en-US --name "First Win" --before-earned-description "Before" --after-earned-description "After"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			loc := strings.TrimSpace(*locale)
			if loc == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			localizedName := strings.TrimSpace(*name)
			if localizedName == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			before := strings.TrimSpace(*beforeEarned)
			if before == "" {
				fmt.Fprintln(os.Stderr, "Error: --before-earned-description is required")
				return flag.ErrHelp
			}

			after := strings.TrimSpace(*afterEarned)
			if after == "" {
				fmt.Fprintln(os.Stderr, "Error: --after-earned-description is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterAchievementLocalizationCreateAttributes{
				Locale:                  loc,
				Name:                    localizedName,
				BeforeEarnedDescription: before,
				AfterEarnedDescription:  after,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterAchievementLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementLocalizationsV2UpdateCommand returns the achievement localizations v2 update subcommand.
func GameCenterAchievementLocalizationsV2UpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	name := fs.String("name", "", "Achievement name")
	beforeEarned := fs.String("before-earned-description", "", "Description shown before earning")
	afterEarned := fs.String("after-earned-description", "", "Description shown after earning")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center achievements v2 localizations update --id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a Game Center achievement localization (v2).",
		LongHelp: `Update a Game Center achievement localization (v2).

Examples:
  asc game-center achievements v2 localizations update --id "LOC_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterAchievementLocalizationUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				value := strings.TrimSpace(*name)
				attrs.Name = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*beforeEarned) != "" {
				value := strings.TrimSpace(*beforeEarned)
				attrs.BeforeEarnedDescription = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*afterEarned) != "" {
				value := strings.TrimSpace(*afterEarned)
				attrs.AfterEarnedDescription = &value
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterAchievementLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementLocalizationsV2DeleteCommand returns the achievement localizations v2 delete subcommand.
func GameCenterAchievementLocalizationsV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center achievements v2 localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement localization (v2).",
		LongHelp: `Delete a Game Center achievement localization (v2).

Examples:
  asc game-center achievements v2 localizations delete --id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterAchievementLocalizationV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center achievements v2 localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterAchievementLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementImagesV2Command returns the achievement images v2 command group.
func GameCenterAchievementImagesV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center achievements v2 images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement images (v2).",
		LongHelp: `Manage Game Center achievement images (v2). Images are attached to achievement localizations.

Examples:
  asc game-center achievements v2 images upload --localization-id "LOC_ID" --file "path/to/image.png"
  asc game-center achievements v2 images get --id "IMAGE_ID"
  asc game-center achievements v2 images get --localization-id "LOC_ID"
  asc game-center achievements v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementImagesV2UploadCommand(),
			GameCenterAchievementImagesV2GetCommand(),
			GameCenterAchievementImagesV2DeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementImagesV2UploadCommand returns the achievement images v2 upload subcommand.
func GameCenterAchievementImagesV2UploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Game Center achievement localization ID")
	filePath := fs.String("file", "", "Path to the image file to upload")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center achievements v2 images upload --localization-id \"LOC_ID\" --file \"path/to/image.png\"",
		ShortHelp:  "Upload an image for a Game Center achievement localization (v2).",
		LongHelp: `Upload an image for a Game Center achievement localization (v2).

The image file will be validated, reserved, uploaded in chunks, and committed.

Examples:
  asc game-center achievements v2 images upload --localization-id "LOC_ID" --file "path/to/image.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			path := strings.TrimSpace(*filePath)
			if path == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterAchievementImageV2(requestCtx, locID, path)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images upload: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementImagesV2GetCommand returns the achievement images v2 get subcommand.
func GameCenterAchievementImagesV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center achievement image ID")
	localizationID := fs.String("localization-id", "", "Game Center achievement localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements v2 images get --id \"IMAGE_ID\" | --localization-id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center achievement image (v2).",
		LongHelp: `Get a Game Center achievement image (v2).

Examples:
  asc game-center achievements v2 images get --id "IMAGE_ID"
  asc game-center achievements v2 images get --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			locID := strings.TrimSpace(*localizationID)
			if id == "" && locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --localization-id is required")
				return flag.ErrHelp
			}
			if id != "" && locID != "" {
				fmt.Fprintln(os.Stderr, "Error: --id cannot be used with --localization-id")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if locID != "" {
				resp, err := client.GetGameCenterAchievementLocalizationImageV2(requestCtx, locID)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 images get: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAchievementImageV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterAchievementImagesV2DeleteCommand returns the achievement images v2 delete subcommand.
func GameCenterAchievementImagesV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center achievement image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center achievements v2 images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement image (v2).",
		LongHelp: `Delete a Game Center achievement image (v2).

Examples:
  asc game-center achievements v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterAchievementImageV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center achievements v2 images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterAchievementImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
