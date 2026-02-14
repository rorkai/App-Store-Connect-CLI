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

// GameCenterLeaderboardsV2Command returns the leaderboards v2 command group.
func GameCenterLeaderboardsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "v2",
		ShortUsage: "asc game-center leaderboards v2 <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboards v2 resources.",
		LongHelp: `Manage Game Center leaderboards v2 resources.

Examples:
  asc game-center leaderboards v2 list --app "APP_ID"
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID"
  asc game-center leaderboards v2 localizations list --version-id "VER_ID"
  asc game-center leaderboards v2 images upload --localization-id "LOC_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardsV2ListCommand(),
			GameCenterLeaderboardVersionsV2Command(),
			GameCenterLeaderboardLocalizationsV2Command(),
			GameCenterLeaderboardImagesV2Command(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardsV2ListCommand returns the leaderboards v2 list subcommand.
func GameCenterLeaderboardsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards v2 list [flags]",
		ShortHelp:  "List Game Center leaderboards (v2) for an app or group.",
		LongHelp: `List Game Center leaderboards (v2) for an app or group.

Examples:
  asc game-center leaderboards v2 list --app "APP_ID"
  asc game-center leaderboards v2 list --group-id "GROUP_ID"
  asc game-center leaderboards v2 list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboards v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboards v2 list: %w", err)
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
				return fmt.Errorf("game-center leaderboards v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" && nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCLeaderboardsOption{
				asc.WithGCLeaderboardsLimit(*limit),
				asc.WithGCLeaderboardsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardsV2(requestCtx, gcDetailID, group, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardsV2(ctx, gcDetailID, group, asc.WithGCLeaderboardsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardsV2(requestCtx, gcDetailID, group, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardVersionsV2Command returns the leaderboard versions v2 command group.
func GameCenterLeaderboardVersionsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center leaderboards v2 versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard versions (v2).",
		LongHelp: `Manage Game Center leaderboard versions (v2).

Examples:
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID"
  asc game-center leaderboards v2 versions get --id "VERSION_ID"
  asc game-center leaderboards v2 versions create --leaderboard-id "LB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardVersionsV2ListCommand(),
			GameCenterLeaderboardVersionsV2GetCommand(),
			GameCenterLeaderboardVersionsV2CreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardVersionsV2ListCommand returns the leaderboard versions v2 list subcommand.
func GameCenterLeaderboardVersionsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards v2 versions list --leaderboard-id \"LB_ID\"",
		ShortHelp:  "List versions for a Game Center leaderboard (v2).",
		LongHelp: `List versions for a Game Center leaderboard (v2).

Examples:
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID"
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboards v2 versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboards v2 versions list: %w", err)
			}

			id := strings.TrimSpace(*leaderboardID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardVersionsOption{
				asc.WithGCLeaderboardVersionsLimit(*limit),
				asc.WithGCLeaderboardVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardVersionsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardVersions(ctx, id, asc.WithGCLeaderboardVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardVersionsV2GetCommand returns the leaderboard versions v2 get subcommand.
func GameCenterLeaderboardVersionsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center leaderboard version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards v2 versions get --id \"VERSION_ID\"",
		ShortHelp:  "Get a Game Center leaderboard version (v2) by ID.",
		LongHelp: `Get a Game Center leaderboard version (v2) by ID.

Examples:
  asc game-center leaderboards v2 versions get --id "VERSION_ID"`,
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
				return fmt.Errorf("game-center leaderboards v2 versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardVersionsV2CreateCommand returns the leaderboard versions v2 create subcommand.
func GameCenterLeaderboardVersionsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards v2 versions create --leaderboard-id \"LB_ID\"",
		ShortHelp:  "Create a new Game Center leaderboard version (v2).",
		LongHelp: `Create a new Game Center leaderboard version (v2).

Examples:
  asc game-center leaderboards v2 versions create --leaderboard-id "LB_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*leaderboardID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterLeaderboardVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsV2Command returns the leaderboard localizations v2 command group.
func GameCenterLeaderboardLocalizationsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboards v2 localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard localizations (v2).",
		LongHelp: `Manage Game Center leaderboard localizations (v2).

Examples:
  asc game-center leaderboards v2 localizations list --version-id "VER_ID"
  asc game-center leaderboards v2 localizations create --version-id "VER_ID" --locale en-US --name "High Score"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardLocalizationsV2ListCommand(),
			GameCenterLeaderboardLocalizationsV2GetCommand(),
			GameCenterLeaderboardLocalizationsV2CreateCommand(),
			GameCenterLeaderboardLocalizationsV2UpdateCommand(),
			GameCenterLeaderboardLocalizationsV2DeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardLocalizationsV2ListCommand returns the leaderboard localizations v2 list subcommand.
func GameCenterLeaderboardLocalizationsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center leaderboard version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards v2 localizations list --version-id \"VER_ID\"",
		ShortHelp:  "List localizations for a leaderboard version (v2).",
		LongHelp: `List localizations for a leaderboard version (v2).

Examples:
  asc game-center leaderboards v2 localizations list --version-id "VER_ID"
  asc game-center leaderboards v2 localizations list --version-id "VER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboards v2 localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations list: %w", err)
			}

			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardLocalizationsOption{
				asc.WithGCLeaderboardLocalizationsLimit(*limit),
				asc.WithGCLeaderboardLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardVersionLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardVersionLocalizations(ctx, id, asc.WithGCLeaderboardLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardVersionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsV2GetCommand returns the leaderboard localizations v2 get subcommand.
func GameCenterLeaderboardLocalizationsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards v2 localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center leaderboard localization (v2) by ID.",
		LongHelp: `Get a Game Center leaderboard localization (v2) by ID.

Examples:
  asc game-center leaderboards v2 localizations get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center leaderboards v2 localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardLocalizationV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsV2CreateCommand returns the leaderboard localizations v2 create subcommand.
func GameCenterLeaderboardLocalizationsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center leaderboard version ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US, de-DE)")
	name := fs.String("name", "", "Display name for the leaderboard in this locale")
	formatterOverride := fs.String("formatter-override", "", "Override the default formatter (optional)")
	formatterSuffix := fs.String("formatter-suffix", "", "Suffix to append to formatted score (optional)")
	formatterSuffixSingular := fs.String("formatter-suffix-singular", "", "Singular suffix (optional)")
	description := fs.String("description", "", "Description for the leaderboard in this locale (optional)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards v2 localizations create --version-id \"VER_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a new Game Center leaderboard localization (v2).",
		LongHelp: `Create a new Game Center leaderboard localization (v2).

Examples:
  asc game-center leaderboards v2 localizations create --version-id "VER_ID" --locale en-US --name "High Score"
  asc game-center leaderboards v2 localizations create --version-id "VER_ID" --locale de-DE --name "Highscore" --formatter-suffix " Punkte"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			localeVal := strings.TrimSpace(*locale)
			if localeVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			var formatterOverrideVal *string
			if trimmed := strings.TrimSpace(*formatterOverride); trimmed != "" {
				formatterOverrideVal = &trimmed
			}

			var formatterSuffixVal *string
			if trimmed := strings.TrimSpace(*formatterSuffix); trimmed != "" {
				formatterSuffixVal = &trimmed
			}

			var formatterSuffixSingularVal *string
			if trimmed := strings.TrimSpace(*formatterSuffixSingular); trimmed != "" {
				formatterSuffixSingularVal = &trimmed
			}

			var descriptionVal *string
			if trimmed := strings.TrimSpace(*description); trimmed != "" {
				descriptionVal = &trimmed
			}

			attrs := asc.GameCenterLeaderboardLocalizationCreateAttributes{
				Locale:                  localeVal,
				Name:                    nameVal,
				FormatterOverride:       formatterOverrideVal,
				FormatterSuffix:         formatterSuffixVal,
				FormatterSuffixSingular: formatterSuffixSingularVal,
				Description:             descriptionVal,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterLeaderboardLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsV2UpdateCommand returns the leaderboard localizations v2 update subcommand.
func GameCenterLeaderboardLocalizationsV2UpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard localization ID")
	name := fs.String("name", "", "Display name for the leaderboard in this locale")
	formatterOverride := fs.String("formatter-override", "", "Override the default formatter")
	formatterSuffix := fs.String("formatter-suffix", "", "Suffix to append to formatted score")
	formatterSuffixSingular := fs.String("formatter-suffix-singular", "", "Singular suffix")
	description := fs.String("description", "", "Description for the leaderboard in this locale")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboards v2 localizations update --id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a Game Center leaderboard localization (v2).",
		LongHelp: `Update a Game Center leaderboard localization (v2).

Examples:
  asc game-center leaderboards v2 localizations update --id "LOC_ID" --name "Top Score"
  asc game-center leaderboards v2 localizations update --id "LOC_ID" --formatter-suffix " pts"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterLeaderboardLocalizationUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				val := strings.TrimSpace(*name)
				attrs.Name = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*formatterOverride) != "" {
				val := strings.TrimSpace(*formatterOverride)
				attrs.FormatterOverride = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*formatterSuffix) != "" {
				val := strings.TrimSpace(*formatterSuffix)
				attrs.FormatterSuffix = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*formatterSuffixSingular) != "" {
				val := strings.TrimSpace(*formatterSuffixSingular)
				attrs.FormatterSuffixSingular = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*description) != "" {
				val := strings.TrimSpace(*description)
				attrs.Description = &val
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterLeaderboardLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsV2DeleteCommand returns the leaderboard localizations v2 delete subcommand.
func GameCenterLeaderboardLocalizationsV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards v2 localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard localization (v2).",
		LongHelp: `Delete a Game Center leaderboard localization (v2).

Examples:
  asc game-center leaderboards v2 localizations delete --id "LOC_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboards v2 localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardLocalizationV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboards v2 localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardImagesV2Command returns the leaderboard images v2 command group.
func GameCenterLeaderboardImagesV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center leaderboards v2 images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard images (v2).",
		LongHelp: `Manage Game Center leaderboard images (v2). Images are attached to leaderboard localizations.

Examples:
  asc game-center leaderboards v2 images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboards v2 images get --id "IMAGE_ID"
  asc game-center leaderboards v2 images get --localization-id "LOC_ID"
  asc game-center leaderboards v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardImagesV2UploadCommand(),
			GameCenterLeaderboardImagesV2GetCommand(),
			GameCenterLeaderboardImagesV2DeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardImagesV2UploadCommand returns the leaderboard images v2 upload subcommand.
func GameCenterLeaderboardImagesV2UploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Game Center leaderboard localization ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center leaderboards v2 images upload --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a Game Center leaderboard localization (v2).",
		LongHelp: `Upload an image for a Game Center leaderboard localization (v2).

This command performs the full upload flow: reserves the upload, uploads the file, and commits.

Examples:
  asc game-center leaderboards v2 images upload --localization-id "LOC_ID" --file leaderboard.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardImageV2(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images upload: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardImagesV2GetCommand returns the leaderboard images v2 get subcommand.
func GameCenterLeaderboardImagesV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center leaderboard image ID")
	localizationID := fs.String("localization-id", "", "Game Center leaderboard localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards v2 images get --id \"IMAGE_ID\" | --localization-id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center leaderboard image (v2).",
		LongHelp: `Get a Game Center leaderboard image (v2).

Examples:
  asc game-center leaderboards v2 images get --id "IMAGE_ID"
  asc game-center leaderboards v2 images get --localization-id "LOC_ID"`,
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
				return fmt.Errorf("game-center leaderboards v2 images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if locID != "" {
				resp, err := client.GetGameCenterLeaderboardLocalizationImageV2(requestCtx, locID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 images get: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardImageV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardImagesV2DeleteCommand returns the leaderboard images v2 delete subcommand.
func GameCenterLeaderboardImagesV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center leaderboard image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards v2 images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard image (v2).",
		LongHelp: `Delete a Game Center leaderboard image (v2).

Examples:
  asc game-center leaderboards v2 images delete --id "IMAGE_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboards v2 images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardImageV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboards v2 images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
