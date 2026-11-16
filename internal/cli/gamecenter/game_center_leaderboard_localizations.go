package gamecenter

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newGameCenterLeaderboardLocalizationsCommand returns the leaderboard localizations command group.
func newGameCenterLeaderboardLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardLocalizationsListCommand(d),
			newGameCenterLeaderboardLocalizationsGetCommand(d),
			newGameCenterLeaderboardLocalizationsCreateCommand(d),
			newGameCenterLeaderboardLocalizationsUpdateCommand(d),
			newGameCenterLeaderboardLocalizationsDeleteCommand(d),
			newGameCenterLeaderboardLocalizationImageCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardLocalizationsListCommand returns the leaderboard localizations list subcommand.
func newGameCenterLeaderboardLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
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
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsGetCommand returns the leaderboard localizations get subcommand.
func newGameCenterLeaderboardLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsCreateCommand returns the leaderboard localizations create subcommand.
func newGameCenterLeaderboardLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
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
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsUpdateCommand returns the leaderboard localizations update subcommand.
func newGameCenterLeaderboardLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
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
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsDeleteCommand returns the leaderboard localizations delete subcommand.
func newGameCenterLeaderboardLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
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
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationImageCommand returns the localization image command group.
func newGameCenterLeaderboardLocalizationImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center leaderboards localizations image view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View the image for a leaderboard localization.",
		LongHelp: `View the image for a leaderboard localization.

Examples:
  asc game-center leaderboards localizations image view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardLocalizationImageGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardLocalizationImageGetCommand returns the localization image get subcommand.
func newGameCenterLeaderboardLocalizationImageGetCommand(d *shared.Deps) *ffcli.Command {
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
		Deps: d,
	})
}
