package gamecenter

import (
	"context"
	"flag"

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
		Name:       "list",
		ShortUsage: "asc game-center leaderboards localizations list --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "List localizations for a Game Center leaderboard.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, leaderboardId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardLocalizations(ctx, leaderboardId, asc.Limit[asc.GCLeaderboardLocalizationsOption](limit), asc.NextURL[asc.GCLeaderboardLocalizationsOption](next))
		},
	})
}

// GameCenterLeaderboardLocalizationsGetCommand returns the leaderboard localizations get subcommand.
func GameCenterLeaderboardLocalizationsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboards localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a Game Center leaderboard localization by ID.",
		LongHelp: `View a Game Center leaderboard localization by ID.

Examples:
  asc game-center leaderboards localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardLocalization(ctx, id)
		},
	})
}

// GameCenterLeaderboardLocalizationsCreateCommand returns the leaderboard localizations create subcommand.
func GameCenterLeaderboardLocalizationsCreateCommand() *ffcli.Command {
	var attrs asc.GameCenterLeaderboardLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards localizations create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard localization.",
		LongHelp: `Create a new Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale de-DE --name "Highscore" --formatter-suffix " Punkte"`,
		IDFlag:  "leaderboard-id",
		IDUsage: "Game Center leaderboard ID",
		IDType:  "gameCenterLeaderboards",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US, de-DE)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Display name for the leaderboard in this locale"),
			shared.OptionalStringFlag(&attrs.FormatterOverride, "formatter-override", "Override the default formatter (optional)"),
			shared.OptionalStringFlag(&attrs.FormatterSuffix, "formatter-suffix", "Suffix to append to formatted score (optional)"),
			shared.OptionalStringFlag(&attrs.FormatterSuffixSingular, "formatter-suffix-singular", "Singular suffix (optional)"),
			shared.OptionalStringFlag(&attrs.Description, "description", "Description for the leaderboard in this locale (optional)"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterLeaderboardLocalization(ctx, id, attrs)
		},
	})
}

// GameCenterLeaderboardLocalizationsUpdateCommand returns the leaderboard localizations update subcommand.
func GameCenterLeaderboardLocalizationsUpdateCommand() *ffcli.Command {
	var attrs asc.GameCenterLeaderboardLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboards localizations update [flags]",
		ShortHelp:  "Update a Game Center leaderboard localization.",
		LongHelp: `Update a Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations update --id "LOCALIZATION_ID" --name "Top Score"
  asc game-center leaderboards localizations update --id "LOCALIZATION_ID" --formatter-suffix " pts"`,
		IDUsage: "Game Center leaderboard localization ID",
		IDType:  "gameCenterLeaderboardLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Display name for the leaderboard in this locale"),
			shared.OptionalStringFlag(&attrs.FormatterOverride, "formatter-override", "Override the default formatter"),
			shared.OptionalStringFlag(&attrs.FormatterSuffix, "formatter-suffix", "Suffix to append to formatted score"),
			shared.OptionalStringFlag(&attrs.FormatterSuffixSingular, "formatter-suffix-singular", "Singular suffix"),
			shared.OptionalStringFlag(&attrs.Description, "description", "Description for the leaderboard in this locale"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterLeaderboardLocalization(ctx, id, attrs)
		},
	})
}

// GameCenterLeaderboardLocalizationsDeleteCommand returns the leaderboard localizations delete subcommand.
func GameCenterLeaderboardLocalizationsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard localization.",
		LongHelp: `Delete a Game Center leaderboard localization.

Examples:
  asc game-center leaderboards localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
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
		Name:       "view",
		ShortUsage: "asc game-center leaderboards localizations image view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a leaderboard localization image.",
		LongHelp: `View a leaderboard localization image.

Examples:
  asc game-center leaderboards localizations image view --id "LOCALIZATION_ID"`,
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardLocalizationImage(ctx, id)
		},
	})
}
