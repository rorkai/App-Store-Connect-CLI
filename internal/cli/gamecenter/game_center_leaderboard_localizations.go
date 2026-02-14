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

// GameCenterLeaderboardLocalizationsCommand returns the leaderboard localizations command group.
func GameCenterLeaderboardLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboards localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard localizations.",
		LongHelp: `Manage Game Center leaderboard localizations.

Examples:
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards localizations get --id "LOCALIZATION_ID"
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
  asc game-center leaderboards localizations update --id "LOCALIZATION_ID" --name "Top Score"
  asc game-center leaderboards localizations delete --id "LOCALIZATION_ID" --confirm
  asc game-center leaderboards localizations image get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardLocalizationsListCommand(),
			GameCenterLeaderboardLocalizationsGetCommand(),
			GameCenterLeaderboardLocalizationsCreateCommand(),
			GameCenterLeaderboardLocalizationsUpdateCommand(),
			GameCenterLeaderboardLocalizationsDeleteCommand(),
			GameCenterLeaderboardLocalizationImageCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardLocalizationsListCommand returns the leaderboard localizations list subcommand.
func GameCenterLeaderboardLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards localizations list --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "List localizations for a Game Center leaderboard.",
		LongHelp: `List localizations for a Game Center leaderboard.

Examples:
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID" --limit 50
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboards localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboards localizations list: %w", err)
			}

			lbID := strings.TrimSpace(*leaderboardID)
			if lbID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardLocalizationsOption{
				asc.WithGCLeaderboardLocalizationsLimit(*limit),
				asc.WithGCLeaderboardLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardLocalizations(requestCtx, lbID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardLocalizations(ctx, lbID, asc.WithGCLeaderboardLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardLocalizations(requestCtx, lbID, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsGetCommand returns the leaderboard localizations get subcommand.
func GameCenterLeaderboardLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards localizations get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a Game Center leaderboard localization by ID.",
		LongHelp: `Get a Game Center leaderboard localization by ID.

Examples:
  asc game-center leaderboards localizations get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center leaderboards localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsCreateCommand returns the leaderboard localizations create subcommand.
func GameCenterLeaderboardLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US, de-DE)")
	name := fs.String("name", "", "Display name for the leaderboard in this locale")
	formatterOverride := fs.String("formatter-override", "", "Override the default formatter (optional)")
	formatterSuffix := fs.String("formatter-suffix", "", "Suffix to append to formatted score (optional)")
	formatterSuffixSingular := fs.String("formatter-suffix-singular", "", "Singular suffix (optional)")
	description := fs.String("description", "", "Description for the leaderboard in this locale (optional)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards localizations create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard localization.",
		LongHelp: `Create a new Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale de-DE --name "Highscore" --formatter-suffix " Punkte"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			lbID := strings.TrimSpace(*leaderboardID)
			if lbID == "" {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

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

			resp, err := client.CreateGameCenterLeaderboardLocalization(requestCtx, lbID, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsUpdateCommand returns the leaderboard localizations update subcommand.
func GameCenterLeaderboardLocalizationsUpdateCommand() *ffcli.Command {
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
		ShortUsage: "asc game-center leaderboards localizations update [flags]",
		ShortHelp:  "Update a Game Center leaderboard localization.",
		LongHelp: `Update a Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations update --id "LOCALIZATION_ID" --name "Top Score"
  asc game-center leaderboards localizations update --id "LOCALIZATION_ID" --formatter-suffix " pts"`,
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
				nameVal := strings.TrimSpace(*name)
				attrs.Name = &nameVal
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
				return fmt.Errorf("game-center leaderboards localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterLeaderboardLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationsDeleteCommand returns the leaderboard localizations delete subcommand.
func GameCenterLeaderboardLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard localization.",
		LongHelp: `Delete a Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations delete --id "LOCALIZATION_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboards localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboards localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardLocalizationImageCommand returns the localization image command group.
func GameCenterLeaderboardLocalizationImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center leaderboards localizations image get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get the image for a leaderboard localization.",
		LongHelp: `Get the image for a leaderboard localization.

Examples:
  asc game-center leaderboards localizations image get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardLocalizationImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardLocalizationImageGetCommand returns the localization image get subcommand.
func GameCenterLeaderboardLocalizationImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards localizations image get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a leaderboard localization image.",
		LongHelp: `Get a leaderboard localization image.

Examples:
  asc game-center leaderboards localizations image get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center leaderboards localizations image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardLocalizationImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards localizations image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
