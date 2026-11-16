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

// newGameCenterLeaderboardsV2Command returns the leaderboards v2 command group.
func newGameCenterLeaderboardsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("v2", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardsV2ListCommand(d),
			newGameCenterScoreModerationsCommand(d),
			newGameCenterLeaderboardVersionsV2Command(d),
			newGameCenterLeaderboardLocalizationsV2Command(d),
			newGameCenterLeaderboardImagesV2Command(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardsV2ListCommand returns the leaderboards v2 list subcommand.
func newGameCenterLeaderboardsV2ListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "game-center leaderboards v2 list", 200); err != nil {
				return err
			}

			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := d.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if group == "" && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
				asc.Limit[asc.GCLeaderboardsOption](*limit),
				asc.NextURL[asc.GCLeaderboardsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.GCLeaderboardsOption](200))
				firstPage, err := client.GetGameCenterLeaderboardsV2(requestCtx, gcDetailID, group, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardsV2(ctx, gcDetailID, group, asc.NextURL[asc.GCLeaderboardsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardsV2(requestCtx, gcDetailID, group, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardVersionsV2Command returns the leaderboard versions v2 command group.
func newGameCenterLeaderboardVersionsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center leaderboards v2 versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard versions (v2).",
		LongHelp: `Manage Game Center leaderboard versions (v2).

Examples:
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID"
  asc game-center leaderboards v2 versions view --id "VERSION_ID"
  asc game-center leaderboards v2 versions create --leaderboard-id "LB_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardVersionsV2ListCommand(d),
			newGameCenterLeaderboardVersionsV2GetCommand(d),
			newGameCenterLeaderboardVersionsV2CreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardVersionsV2ListCommand returns the leaderboard versions v2 list subcommand.
func newGameCenterLeaderboardVersionsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards v2 versions list --leaderboard-id \"LB_ID\"",
		ShortHelp:  "List versions for a Game Center leaderboard (v2).",
		LongHelp: `List versions for a Game Center leaderboard (v2).

Examples:
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID"
  asc game-center leaderboards v2 versions list --leaderboard-id "LB_ID" --paginate`,
		ParentFlag:           "leaderboard-id",
		ParentUsage:          "Game Center leaderboard ID",
		ParentType:           "gameCenterLeaderboards",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, leaderboardId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardVersions(ctx, leaderboardId, asc.Limit[asc.GCLeaderboardVersionsOption](limit), asc.NextURL[asc.GCLeaderboardVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardVersionsV2GetCommand returns the leaderboard versions v2 get subcommand.
func newGameCenterLeaderboardVersionsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboards v2 versions view --id \"VERSION_ID\"",
		ShortHelp:  "View a Game Center leaderboard version (v2) by ID.",
		LongHelp: `View a Game Center leaderboard version (v2) by ID.

Examples:
  asc game-center leaderboards v2 versions view --id "VERSION_ID"`,
		IDUsage:            "Game Center leaderboard version ID",
		IDType:             "gameCenterLeaderboardVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardVersionsV2CreateCommand returns the leaderboard versions v2 create subcommand.
func newGameCenterLeaderboardVersionsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards v2 versions create --leaderboard-id \"LB_ID\"",
		ShortHelp:  "Create a new Game Center leaderboard version (v2).",
		LongHelp: `Create a new Game Center leaderboard version (v2).

Examples:
  asc game-center leaderboards v2 versions create --leaderboard-id "LB_ID"`,
		IDFlag:  "leaderboard-id",
		IDUsage: "Game Center leaderboard ID",
		IDType:  "gameCenterLeaderboards",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterLeaderboardVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsV2Command returns the leaderboard localizations v2 command group.
func newGameCenterLeaderboardLocalizationsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboards v2 localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard localizations (v2).",
		LongHelp: `Manage Game Center leaderboard localizations (v2).

Examples:
  asc game-center leaderboards v2 localizations list --version-id "VER_ID"
  asc game-center leaderboards v2 localizations create --version-id "VER_ID" --locale en-US --name "High Score"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardLocalizationsV2ListCommand(d),
			newGameCenterLeaderboardLocalizationsV2GetCommand(d),
			newGameCenterLeaderboardLocalizationsV2CreateCommand(d),
			newGameCenterLeaderboardLocalizationsV2UpdateCommand(d),
			newGameCenterLeaderboardLocalizationsV2DeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardLocalizationsV2ListCommand returns the leaderboard localizations v2 list subcommand.
func newGameCenterLeaderboardLocalizationsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards v2 localizations list --version-id \"VER_ID\"",
		ShortHelp:  "List localizations for a leaderboard version (v2).",
		LongHelp: `List localizations for a leaderboard version (v2).

Examples:
  asc game-center leaderboards v2 localizations list --version-id "VER_ID"
  asc game-center leaderboards v2 localizations list --version-id "VER_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Game Center leaderboard version ID",
		ParentType:           "gameCenterLeaderboardVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, versionId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardVersionLocalizations(ctx, versionId, asc.Limit[asc.GCLeaderboardLocalizationsOption](limit), asc.NextURL[asc.GCLeaderboardLocalizationsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsV2GetCommand returns the leaderboard localizations v2 get subcommand.
func newGameCenterLeaderboardLocalizationsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboards v2 localizations view --id \"LOC_ID\"",
		ShortHelp:  "View a Game Center leaderboard localization (v2) by ID.",
		LongHelp: `View a Game Center leaderboard localization (v2) by ID.

Examples:
  asc game-center leaderboards v2 localizations view --id "LOC_ID"`,
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardLocalizationV2(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsV2CreateCommand returns the leaderboard localizations v2 create subcommand.
func newGameCenterLeaderboardLocalizationsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards v2 localizations create --version-id \"VER_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a new Game Center leaderboard localization (v2).",
		LongHelp: `Create a new Game Center leaderboard localization (v2).

Examples:
  asc game-center leaderboards v2 localizations create --version-id "VER_ID" --locale en-US --name "High Score"
  asc game-center leaderboards v2 localizations create --version-id "VER_ID" --locale de-DE --name "Highscore" --formatter-suffix " Punkte"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center leaderboard version ID",
		IDType:  "gameCenterLeaderboardVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US, de-DE)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Display name for the leaderboard in this locale"),
			shared.OptionalStringFlag(&attrs.FormatterOverride, "formatter-override", "Override the default formatter (optional)"),
			shared.OptionalStringFlag(&attrs.FormatterSuffix, "formatter-suffix", "Suffix to append to formatted score (optional)"),
			shared.OptionalStringFlag(&attrs.FormatterSuffixSingular, "formatter-suffix-singular", "Singular suffix (optional)"),
			shared.OptionalStringFlag(&attrs.Description, "description", "Description for the leaderboard in this locale (optional)"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterLeaderboardLocalizationV2(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsV2UpdateCommand returns the leaderboard localizations v2 update subcommand.
func newGameCenterLeaderboardLocalizationsV2UpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboards v2 localizations update --id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a Game Center leaderboard localization (v2).",
		LongHelp: `Update a Game Center leaderboard localization (v2).

Examples:
  asc game-center leaderboards v2 localizations update --id "LOC_ID" --name "Top Score"
  asc game-center leaderboards v2 localizations update --id "LOC_ID" --formatter-suffix " pts"`,
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
			return client.UpdateGameCenterLeaderboardLocalizationV2(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardLocalizationsV2DeleteCommand returns the leaderboard localizations v2 delete subcommand.
func newGameCenterLeaderboardLocalizationsV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards v2 localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard localization (v2).",
		LongHelp: `Delete a Game Center leaderboard localization (v2).

Examples:
  asc game-center leaderboards v2 localizations delete --id "LOC_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard localization ID",
		IDType:             "gameCenterLeaderboardLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardLocalizationV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardImagesV2Command returns the leaderboard images v2 command group.
func newGameCenterLeaderboardImagesV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center leaderboards v2 images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard images (v2).",
		LongHelp: `Manage Game Center leaderboard images (v2). Images are attached to leaderboard localizations.

Examples:
  asc game-center leaderboards v2 images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboards v2 images view --id "IMAGE_ID"
  asc game-center leaderboards v2 images view --localization-id "LOC_ID"
  asc game-center leaderboards v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardImagesV2UploadCommand(d),
			newGameCenterLeaderboardImagesV2GetCommand(d),
			newGameCenterLeaderboardImagesV2DeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardImagesV2UploadCommand returns the leaderboard images v2 upload subcommand.
func newGameCenterLeaderboardImagesV2UploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterLeaderboardLocalizations", "Game Center leaderboard localization ID")
	filePath := fs.String("file", "", "Path to image file")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center leaderboards v2 images upload --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a Game Center leaderboard localization (v2).",
		LongHelp: `Upload an image for a Game Center leaderboard localization (v2).

This command performs the full upload flow: reserves the upload, uploads the file, and commits.

Examples:
  asc game-center leaderboards v2 images upload --localization-id "LOC_ID" --file leaderboard.png`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				return d.RequiredFlag("--localization-id")
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				return d.RequiredFlag("--file")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images upload: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardImageV2(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images upload: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardImagesV2GetCommand returns the leaderboard images v2 get subcommand.
func newGameCenterLeaderboardImagesV2GetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	imageID := shared.BindResourceIDFlag(fs, "id", "gameCenterLeaderboardImages", "Game Center leaderboard image ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterLeaderboardLocalizations", "Game Center leaderboard localization ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc game-center leaderboards v2 images view --id \"IMAGE_ID\" | --localization-id \"LOC_ID\"",
		ShortHelp:  "View a Game Center leaderboard image (v2).",
		LongHelp: `View a Game Center leaderboard image (v2).

Examples:
  asc game-center leaderboards v2 images view --id "IMAGE_ID"
  asc game-center leaderboards v2 images view --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			locID := strings.TrimSpace(*localizationID)
			if id == "" && locID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
			}
			if id != "" && locID != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --id cannot be used with --localization-id")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if locID != "" {
				resp, err := client.GetGameCenterLeaderboardLocalizationImageV2(requestCtx, locID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards v2 images view: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardImageV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards v2 images view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardImagesV2DeleteCommand returns the leaderboard images v2 delete subcommand.
func newGameCenterLeaderboardImagesV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards v2 images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard image (v2).",
		LongHelp: `Delete a Game Center leaderboard image (v2).

Examples:
  asc game-center leaderboards v2 images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard image ID",
		IDType:             "gameCenterLeaderboardImages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardImageV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}
