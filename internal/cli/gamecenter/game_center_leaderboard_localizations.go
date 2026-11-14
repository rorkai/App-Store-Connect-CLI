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
  asc game-center leaderboards localizations view --id "LOCALIZATION_ID"
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
  asc game-center leaderboards localizations update --id "LOCALIZATION_ID" --name "Top Score"
  asc game-center leaderboards localizations delete --id "LOCALIZATION_ID" --confirm
  asc game-center leaderboards localizations image view --id "LOCALIZATION_ID"`,
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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center leaderboards localizations list --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:   "List localizations for a Game Center leaderboard.",
		LongHelp: `List localizations for a Game Center leaderboard.

Examples:
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID" --limit 50
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID" --paginate`,
		ParentFlag:           "leaderboard-id",
		ParentUsage:          "Game Center leaderboard ID",
		ParentType:           "gameCenterLeaderboards",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center leaderboards localizations list",
		FetchErrorPrefix:     "game-center leaderboards localizations list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, leaderboardId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardLocalizations(ctx, leaderboardId, asc.Limit[asc.GCLeaderboardLocalizationsOption](limit), asc.NextURL[asc.GCLeaderboardLocalizationsOption](next))
		},
	})
}

// GameCenterLeaderboardLocalizationsGetCommand returns the leaderboard localizations get subcommand.
func GameCenterLeaderboardLocalizationsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc game-center leaderboards localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:   "View a Game Center leaderboard localization by ID.",
		LongHelp: `View a Game Center leaderboard localization by ID.

Examples:
  asc game-center leaderboards localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center leaderboards localizations view",
		FetchErrorPrefix:   "game-center leaderboards localizations view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardLocalization(ctx, id)
		},
	})
}

// GameCenterLeaderboardLocalizationsCreateCommand returns the leaderboard localizations create subcommand.
func GameCenterLeaderboardLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	leaderboardID := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Game Center leaderboard ID")
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
				return shared.RequiredFlag("--leaderboard-id")
			}

			localeVal := strings.TrimSpace(*locale)
			if localeVal == "" {
				return shared.RequiredFlag("--locale")
			}

			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				return shared.RequiredFlag("--name")
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

	localizationID := shared.BindResourceIDFlag(fs, "id", "gameCenterLeaderboardLocalizations", "Game Center leaderboard localization ID")
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
				return shared.RequiredFlag("--id")
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
				return shared.MissingRequiredUsageError("")
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
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc game-center leaderboards localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:   "Delete a Game Center leaderboard localization.",
		LongHelp: `Delete a Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center leaderboards localizations delete",
		DeleteErrorPrefix:  "game-center leaderboards localizations delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.GameCenterLeaderboardLocalizationDeleteResult{ID: id, Deleted: true}
		},
	})
}

// GameCenterLeaderboardLocalizationImageCommand returns the localization image command group.
func GameCenterLeaderboardLocalizationImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center leaderboards localizations image view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View the image for a leaderboard localization.",
		LongHelp: `View the image for a leaderboard localization.

Examples:
  asc game-center leaderboards localizations image view --id "LOCALIZATION_ID"`,
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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc game-center leaderboards localizations image view --id \"LOCALIZATION_ID\"",
		ShortHelp:   "View a leaderboard localization image.",
		LongHelp: `View a leaderboard localization image.

Examples:
  asc game-center leaderboards localizations image view --id "LOCALIZATION_ID"`,
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center leaderboards localizations image view",
		FetchErrorPrefix:   "game-center leaderboards localizations image view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardLocalizationImage(ctx, id)
		},
	})
}
