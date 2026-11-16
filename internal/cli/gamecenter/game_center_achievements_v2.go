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

// newGameCenterAchievementsV2Command returns the achievements v2 command group.
func newGameCenterAchievementsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("v2", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterAchievementsV2ListCommand(d),
			newGameCenterAchievementVersionsV2Command(d),
			newGameCenterAchievementLocalizationsV2Command(d),
			newGameCenterAchievementImagesV2Command(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAchievementsV2ListCommand returns the achievements v2 list subcommand.
func newGameCenterAchievementsV2ListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "game-center achievements v2 list", 200); err != nil {
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
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
				asc.Limit[asc.GCAchievementsOption](*limit),
				asc.NextURL[asc.GCAchievementsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.GCAchievementsOption](200))
				firstPage, err := client.GetGameCenterAchievementsV2(requestCtx, gcDetailID, group, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievementsV2(ctx, gcDetailID, group, asc.NextURL[asc.GCAchievementsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements v2 list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAchievementsV2(requestCtx, gcDetailID, group, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterAchievementVersionsV2Command returns the achievement versions v2 command group.
func newGameCenterAchievementVersionsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center achievements v2 versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement versions (v2).",
		LongHelp: `Manage Game Center achievement versions (v2).

Examples:
  asc game-center achievements v2 versions list --achievement-id "ACH_ID"
  asc game-center achievements v2 versions view --id "VERSION_ID"
  asc game-center achievements v2 versions create --achievement-id "ACH_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterAchievementVersionsV2ListCommand(d),
			newGameCenterAchievementVersionsV2GetCommand(d),
			newGameCenterAchievementVersionsV2CreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAchievementVersionsV2ListCommand returns the achievement versions v2 list subcommand.
func newGameCenterAchievementVersionsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center achievements v2 versions list --achievement-id \"ACH_ID\"",
		ShortHelp:  "List versions for a Game Center achievement (v2).",
		LongHelp: `List versions for a Game Center achievement (v2).

Examples:
  asc game-center achievements v2 versions list --achievement-id "ACH_ID"
  asc game-center achievements v2 versions list --achievement-id "ACH_ID" --paginate`,
		ParentFlag:           "achievement-id",
		ParentUsage:          "Game Center achievement ID",
		ParentType:           "gameCenterAchievements",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, achievementId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterAchievementVersions(ctx, achievementId, asc.Limit[asc.GCAchievementVersionsOption](limit), asc.NextURL[asc.GCAchievementVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterAchievementVersionsV2GetCommand returns the achievement versions v2 get subcommand.
func newGameCenterAchievementVersionsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center achievements v2 versions view --id \"VERSION_ID\"",
		ShortHelp:  "View a Game Center achievement version (v2) by ID.",
		LongHelp: `View a Game Center achievement version (v2) by ID.

Examples:
  asc game-center achievements v2 versions view --id "VERSION_ID"`,
		IDUsage:            "Game Center achievement version ID",
		IDType:             "gameCenterAchievementVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterAchievementVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterAchievementVersionsV2CreateCommand returns the achievement versions v2 create subcommand.
func newGameCenterAchievementVersionsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center achievements v2 versions create --achievement-id \"ACH_ID\"",
		ShortHelp:  "Create a new Game Center achievement version (v2).",
		LongHelp: `Create a new Game Center achievement version (v2).

Examples:
  asc game-center achievements v2 versions create --achievement-id "ACH_ID"`,
		IDFlag:  "achievement-id",
		IDUsage: "Game Center achievement ID",
		IDType:  "gameCenterAchievements",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterAchievementVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterAchievementLocalizationsV2Command returns the achievement localizations v2 command group.
func newGameCenterAchievementLocalizationsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center achievements v2 localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement localizations (v2).",
		LongHelp: `Manage Game Center achievement localizations (v2).

Examples:
  asc game-center achievements v2 localizations list --version-id "VER_ID"
  asc game-center achievements v2 localizations create --version-id "VER_ID" --locale en-US --name "First Win" --before-earned-description "Before" --after-earned-description "After"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterAchievementLocalizationsV2ListCommand(d),
			newGameCenterAchievementLocalizationsV2GetCommand(d),
			newGameCenterAchievementLocalizationsV2CreateCommand(d),
			newGameCenterAchievementLocalizationsV2UpdateCommand(d),
			newGameCenterAchievementLocalizationsV2DeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAchievementLocalizationsV2ListCommand returns the achievement localizations v2 list subcommand.
func newGameCenterAchievementLocalizationsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center achievements v2 localizations list --version-id \"VER_ID\"",
		ShortHelp:  "List localizations for an achievement version (v2).",
		LongHelp: `List localizations for an achievement version (v2).

Examples:
  asc game-center achievements v2 localizations list --version-id "VER_ID"
  asc game-center achievements v2 localizations list --version-id "VER_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Game Center achievement version ID",
		ParentType:           "gameCenterAchievementVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, versionId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterAchievementVersionLocalizations(ctx, versionId, asc.Limit[asc.GCAchievementLocalizationsOption](limit), asc.NextURL[asc.GCAchievementLocalizationsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterAchievementLocalizationsV2GetCommand returns the achievement localizations v2 get subcommand.
func newGameCenterAchievementLocalizationsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center achievements v2 localizations view --id \"LOC_ID\"",
		ShortHelp:  "View a Game Center achievement localization (v2) by ID.",
		LongHelp: `View a Game Center achievement localization (v2) by ID.

Examples:
  asc game-center achievements v2 localizations view --id "LOC_ID"`,
		IDUsage:            "Game Center achievement localization ID",
		IDType:             "gameCenterAchievementLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterAchievementLocalizationV2(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterAchievementLocalizationsV2CreateCommand returns the achievement localizations v2 create subcommand.
func newGameCenterAchievementLocalizationsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterAchievementLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center achievements v2 localizations create --version-id \"VER_ID\" --locale \"LOCALE\" --name \"NAME\" --before-earned-description \"TEXT\" --after-earned-description \"TEXT\"",
		ShortHelp:  "Create a new Game Center achievement localization (v2).",
		LongHelp: `Create a new Game Center achievement localization (v2).

Examples:
  asc game-center achievements v2 localizations create --version-id "VER_ID" --locale en-US --name "First Win" --before-earned-description "Before" --after-earned-description "After"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center achievement version ID",
		IDType:  "gameCenterAchievementVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Achievement name"),
			shared.RequiredStringFlag(&attrs.BeforeEarnedDescription, "before-earned-description", "Description shown before earning"),
			shared.RequiredStringFlag(&attrs.AfterEarnedDescription, "after-earned-description", "Description shown after earning"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterAchievementLocalizationV2(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterAchievementLocalizationsV2UpdateCommand returns the achievement localizations v2 update subcommand.
func newGameCenterAchievementLocalizationsV2UpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterAchievementLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center achievements v2 localizations update --id \"LOC_ID\" [flags]",
		ShortHelp:  "Update a Game Center achievement localization (v2).",
		LongHelp: `Update a Game Center achievement localization (v2).

Examples:
  asc game-center achievements v2 localizations update --id "LOC_ID" --name "New Name"`,
		IDUsage: "Game Center achievement localization ID",
		IDType:  "gameCenterAchievementLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Achievement name"),
			shared.OptionalStringFlag(&attrs.BeforeEarnedDescription, "before-earned-description", "Description shown before earning"),
			shared.OptionalStringFlag(&attrs.AfterEarnedDescription, "after-earned-description", "Description shown after earning"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterAchievementLocalizationV2(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterAchievementLocalizationsV2DeleteCommand returns the achievement localizations v2 delete subcommand.
func newGameCenterAchievementLocalizationsV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center achievements v2 localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement localization (v2).",
		LongHelp: `Delete a Game Center achievement localization (v2).

Examples:
  asc game-center achievements v2 localizations delete --id "LOC_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center achievement localization ID",
		IDType:             "gameCenterAchievementLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterAchievementLocalizationV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterAchievementImagesV2Command returns the achievement images v2 command group.
func newGameCenterAchievementImagesV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center achievements v2 images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement images (v2).",
		LongHelp: `Manage Game Center achievement images (v2). Images are attached to achievement localizations.

Examples:
  asc game-center achievements v2 images upload --localization-id "LOC_ID" --file "path/to/image.png"
  asc game-center achievements v2 images view --id "IMAGE_ID"
  asc game-center achievements v2 images view --localization-id "LOC_ID"
  asc game-center achievements v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterAchievementImagesV2UploadCommand(d),
			newGameCenterAchievementImagesV2GetCommand(d),
			newGameCenterAchievementImagesV2DeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterAchievementImagesV2UploadCommand returns the achievement images v2 upload subcommand.
func newGameCenterAchievementImagesV2UploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterAchievementLocalizations", "Game Center achievement localization ID")
	filePath := fs.String("file", "", "Path to the image file to upload")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center achievements v2 images upload --localization-id \"LOC_ID\" --file \"path/to/image.png\"",
		ShortHelp:  "Upload an image for a Game Center achievement localization (v2).",
		LongHelp: `Upload an image for a Game Center achievement localization (v2).

The image file will be validated, reserved, uploaded in chunks, and committed.

Examples:
  asc game-center achievements v2 images upload --localization-id "LOC_ID" --file "path/to/image.png"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				return d.RequiredFlag("--localization-id")
			}

			path := strings.TrimSpace(*filePath)
			if path == "" {
				return d.RequiredFlag("--file")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images upload: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterAchievementImageV2(requestCtx, locID, path)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images upload: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterAchievementImagesV2GetCommand returns the achievement images v2 get subcommand.
func newGameCenterAchievementImagesV2GetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	imageID := shared.BindResourceIDFlag(fs, "id", "gameCenterAchievementImages", "Game Center achievement image ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterAchievementLocalizations", "Game Center achievement localization ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc game-center achievements v2 images view --id \"IMAGE_ID\" | --localization-id \"LOC_ID\"",
		ShortHelp:  "View a Game Center achievement image (v2).",
		LongHelp: `View a Game Center achievement image (v2).

Examples:
  asc game-center achievements v2 images view --id "IMAGE_ID"
  asc game-center achievements v2 images view --localization-id "LOC_ID"`,
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
				return fmt.Errorf("game-center achievements v2 images view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if locID != "" {
				resp, err := client.GetGameCenterAchievementLocalizationImageV2(requestCtx, locID)
				if err != nil {
					return fmt.Errorf("game-center achievements v2 images view: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterAchievementImageV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements v2 images view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterAchievementImagesV2DeleteCommand returns the achievement images v2 delete subcommand.
func newGameCenterAchievementImagesV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center achievements v2 images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement image (v2).",
		LongHelp: `Delete a Game Center achievement image (v2).

Examples:
  asc game-center achievements v2 images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center achievement image ID",
		IDType:             "gameCenterAchievementImages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterAchievementImageV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}
