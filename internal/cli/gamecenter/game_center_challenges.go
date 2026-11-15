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

// newGameCenterChallengesCommand returns the challenges command group.
func newGameCenterChallengesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("challenges", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "challenges",
		ShortUsage: "asc game-center challenges <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenges.",
		LongHelp: `Manage Game Center challenges.

Examples:
  asc game-center challenges list --app "APP_ID"
  asc game-center challenges view --id "CHALLENGE_ID"
  asc game-center challenges create --app "APP_ID" --reference-name "Weekly Challenge" --vendor-id "com.example.weekly" --leaderboard-id "LEADERBOARD_ID"
  asc game-center challenges update --id "CHALLENGE_ID" --archived true
  asc game-center challenges delete --id "CHALLENGE_ID" --confirm
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID"
  asc game-center challenges localizations list --version-id "VERSION_ID"
  asc game-center challenges localizations image view --id "LOC_ID"
  asc game-center challenges versions default-image view --id "VERSION_ID"
  asc game-center challenges images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center challenges releases list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengesListCommand(d),
			newGameCenterChallengesGetCommand(d),
			newGameCenterChallengesCreateCommand(d),
			newGameCenterChallengesUpdateCommand(d),
			newGameCenterChallengesDeleteCommand(d),
			newGameCenterChallengeVersionsCommand(d),
			newGameCenterChallengeLocalizationsCommand(d),
			newGameCenterChallengeImagesCommand(d),
			newGameCenterChallengeReleasesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengesListCommand returns the challenges list subcommand.
func newGameCenterChallengesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center challenges list [flags]",
		ShortHelp:  "List Game Center challenges for an app.",
		LongHelp: `List Game Center challenges for an app.

Examples:
  asc game-center challenges list --app "APP_ID"
  asc game-center challenges list --app "APP_ID" --limit 50
  asc game-center challenges list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		LookupParent:         gameCenterDetailParent("game-center challenges list"),
		FetchPage: func(ctx context.Context, client *asc.Client, gcDetailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterChallenges(ctx, gcDetailID, asc.Limit[asc.GCChallengesOption](limit), asc.NextURL[asc.GCChallengesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterChallengesGetCommand returns the challenges get subcommand.
func newGameCenterChallengesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center challenges view --id \"CHALLENGE_ID\"",
		ShortHelp:  "View a Game Center challenge by ID.",
		LongHelp: `View a Game Center challenge by ID.

Examples:
  asc game-center challenges view --id "CHALLENGE_ID"`,
		IDUsage:            "Game Center challenge ID",
		IDType:             "gameCenterChallenges",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterChallenge(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengesCreateCommand returns the challenges create subcommand.
func newGameCenterChallengesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the challenge")
	vendorID := fs.String("vendor-id", "", "Vendor identifier for the challenge")
	repeatable := fs.String("repeatable", "", "Challenge can be earned multiple times (true/false)")
	createInitialVersion := fs.String("create-initial-version", "", "Create an initial challenge version inline (true/false)")
	leaderboardID := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Leaderboard ID for the challenge")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center challenges create --app \"APP_ID\" --reference-name \"Weekly\" --vendor-id \"com.example.weekly\" --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "Create a Game Center challenge.",
		LongHelp: `Create a Game Center challenge.

Examples:
  asc game-center challenges create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --leaderboard-id "LEADERBOARD_ID"
  asc game-center challenges create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --leaderboard-id "LEADERBOARD_ID" --create-initial-version true
  asc game-center challenges create --group-id "GROUP_ID" --reference-name "Weekly" --vendor-id "grp.com.example.weekly" --leaderboard-id "LEADERBOARD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if group == "" && resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}

			vendor := strings.TrimSpace(*vendorID)
			if vendor == "" {
				return d.RequiredFlag("--vendor-id")
			}
			if group != "" && !strings.HasPrefix(vendor, "grp.") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --vendor-id must start with \"grp.\" when using --group-id")
				return flag.ErrHelp
			}

			createInitialVersionValue := false
			if strings.TrimSpace(*createInitialVersion) != "" {
				value, err := shared.ParseBoolFlag(*createInitialVersion, "--create-initial-version")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				createInitialVersionValue = value
			}

			attrs := asc.GameCenterChallengeCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
				ChallengeType:    "LEADERBOARD",
			}

			if strings.TrimSpace(*repeatable) != "" {
				val, err := shared.ParseBoolFlag(*repeatable, "--repeatable")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Repeatable = &val
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center challenges create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center challenges create: failed to get Game Center detail: %w", err)
				}
			}

			resp, err := client.CreateGameCenterChallenge(requestCtx, gcDetailID, attrs, strings.TrimSpace(*leaderboardID), group, createInitialVersionValue)
			if err != nil {
				return fmt.Errorf("game-center challenges create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterChallengesUpdateCommand returns the challenges update subcommand.
func newGameCenterChallengesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	challengeID := shared.BindResourceIDFlag(fs, "id", "gameCenterChallenges", "Game Center challenge ID")
	referenceName := fs.String("reference-name", "", "Reference name for the challenge")
	repeatable := fs.String("repeatable", "", "Challenge can be earned multiple times (true/false)")
	archived := fs.String("archived", "", "Archive the challenge (true/false)")
	leaderboardID := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Leaderboard ID for the challenge")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center challenges update --id \"CHALLENGE_ID\" [flags]",
		ShortHelp:  "Update a Game Center challenge.",
		LongHelp: `Update a Game Center challenge.

Examples:
  asc game-center challenges update --id "CHALLENGE_ID" --reference-name "New Name"
  asc game-center challenges update --id "CHALLENGE_ID" --archived true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*challengeID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterChallengeUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				value := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &value
				hasUpdate = true
			}

			if strings.TrimSpace(*repeatable) != "" {
				val, err := shared.ParseBoolFlag(*repeatable, "--repeatable")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Repeatable = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*archived) != "" {
				val, err := shared.ParseBoolFlag(*archived, "--archived")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Archived = &val
				hasUpdate = true
			}

			if !hasUpdate && strings.TrimSpace(*leaderboardID) == "" {
				return d.UpdateFlagRequired(fs, referenceName, repeatable, archived, leaderboardID)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center challenges update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterChallenge(requestCtx, id, attrs, strings.TrimSpace(*leaderboardID))
			if err != nil {
				return fmt.Errorf("game-center challenges update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterChallengesDeleteCommand returns the challenges delete subcommand.
func newGameCenterChallengesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center challenges delete --id \"CHALLENGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center challenge.",
		LongHelp: `Delete a Game Center challenge.

Examples:
  asc game-center challenges delete --id "CHALLENGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center challenge ID",
		IDType:             "gameCenterChallenges",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterChallenge(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterChallengeVersionsCommand returns the challenge versions command group.
func newGameCenterChallengeVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center challenges versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge versions.",
		LongHelp: `Manage Game Center challenge versions.

Examples:
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID"
  asc game-center challenges versions view --id "VERSION_ID"
  asc game-center challenges versions create --challenge-id "CHALLENGE_ID"
  asc game-center challenges versions default-image view --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengeVersionsListCommand(d),
			newGameCenterChallengeVersionsGetCommand(d),
			newGameCenterChallengeVersionsCreateCommand(d),
			newGameCenterChallengeVersionDefaultImageCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengeVersionsListCommand returns the challenge versions list subcommand.
func newGameCenterChallengeVersionsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center challenges versions list --challenge-id \"CHALLENGE_ID\"",
		ShortHelp:  "List versions for a Game Center challenge.",
		LongHelp: `List versions for a Game Center challenge.

Examples:
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID"
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID" --limit 50
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID" --paginate`,
		ParentFlag:           "challenge-id",
		ParentUsage:          "Game Center challenge ID",
		ParentType:           "gameCenterChallenges",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, challengeId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterChallengeVersions(ctx, challengeId, asc.Limit[asc.GCChallengeVersionsOption](limit), asc.NextURL[asc.GCChallengeVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterChallengeVersionsGetCommand returns the challenge versions get subcommand.
func newGameCenterChallengeVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center challenges versions view --id \"VERSION_ID\"",
		ShortHelp:  "View a Game Center challenge version by ID.",
		LongHelp: `View a Game Center challenge version by ID.

Examples:
  asc game-center challenges versions view --id "VERSION_ID"`,
		IDUsage:            "Game Center challenge version ID",
		IDType:             "gameCenterChallengeVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterChallengeVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengeVersionsCreateCommand returns the challenge versions create subcommand.
func newGameCenterChallengeVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center challenges versions create --challenge-id \"CHALLENGE_ID\"",
		ShortHelp:  "Create a Game Center challenge version.",
		LongHelp: `Create a Game Center challenge version.

Examples:
  asc game-center challenges versions create --challenge-id "CHALLENGE_ID"`,
		IDFlag:  "challenge-id",
		IDUsage: "Game Center challenge ID",
		IDType:  "gameCenterChallenges",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterChallengeVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengeLocalizationsCommand returns the challenge localizations command group.
func newGameCenterChallengeLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center challenges localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge localizations.",
		LongHelp: `Manage Game Center challenge localizations.

Examples:
  asc game-center challenges localizations list --version-id "VERSION_ID"
  asc game-center challenges localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"
  asc game-center challenges localizations update --id "LOC_ID" --name "New Name"
  asc game-center challenges localizations delete --id "LOC_ID" --confirm
  asc game-center challenges localizations image view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengeLocalizationsListCommand(d),
			newGameCenterChallengeLocalizationsGetCommand(d),
			newGameCenterChallengeLocalizationsCreateCommand(d),
			newGameCenterChallengeLocalizationsUpdateCommand(d),
			newGameCenterChallengeLocalizationsDeleteCommand(d),
			newGameCenterChallengeLocalizationImageCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengeLocalizationsListCommand returns the challenge localizations list subcommand.
func newGameCenterChallengeLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center challenges localizations list --version-id \"VERSION_ID\"",
		ShortHelp:  "List localizations for a challenge version.",
		LongHelp: `List localizations for a challenge version.

Examples:
  asc game-center challenges localizations list --version-id "VERSION_ID"
  asc game-center challenges localizations list --version-id "VERSION_ID" --limit 50
  asc game-center challenges localizations list --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Game Center challenge version ID",
		ParentType:           "gameCenterChallengeVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, versionId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterChallengeLocalizations(ctx, versionId, asc.Limit[asc.GCChallengeLocalizationsOption](limit), asc.NextURL[asc.GCChallengeLocalizationsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterChallengeLocalizationsGetCommand returns the challenge localizations get subcommand.
func newGameCenterChallengeLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center challenges localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a challenge localization by ID.",
		LongHelp: `View a challenge localization by ID.

Examples:
  asc game-center challenges localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Game Center challenge localization ID",
		IDType:             "gameCenterChallengeLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterChallengeLocalization(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengeLocalizationsCreateCommand returns the challenge localizations create subcommand.
func newGameCenterChallengeLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterChallengeLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center challenges localizations create --version-id \"VERSION_ID\" --locale en-US --name \"Weekly\" --description \"Win weekly\"",
		ShortHelp:  "Create a challenge localization.",
		LongHelp: `Create a challenge localization.

Examples:
  asc game-center challenges localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center challenge version ID",
		IDType:  "gameCenterChallengeVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Localization locale (e.g., en-US)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Localized name"),
			shared.RequiredStringFlag(&attrs.Description, "description", "Localized description"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterChallengeLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterChallengeLocalizationsUpdateCommand returns the challenge localizations update subcommand.
func newGameCenterChallengeLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterChallengeLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center challenges localizations update --id \"LOCALIZATION_ID\" [flags]",
		ShortHelp:  "Update a challenge localization.",
		LongHelp: `Update a challenge localization.

Examples:
  asc game-center challenges localizations update --id "LOCALIZATION_ID" --name "New Name"`,
		IDUsage: "Game Center challenge localization ID",
		IDType:  "gameCenterChallengeLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Localized name"),
			shared.OptionalStringFlag(&attrs.Description, "description", "Localized description"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterChallengeLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterChallengeLocalizationsDeleteCommand returns the challenge localizations delete subcommand.
func newGameCenterChallengeLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center challenges localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a challenge localization.",
		LongHelp: `Delete a challenge localization.

Examples:
  asc game-center challenges localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center challenge localization ID",
		IDType:             "gameCenterChallengeLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterChallengeLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterChallengeImagesCommand returns the challenge images command group.
func newGameCenterChallengeImagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center challenges images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge images.",
		LongHelp: `Manage Game Center challenge images. Images are attached to challenge localizations, or to challenge versions as the default image.

Examples:
  asc game-center challenges images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center challenges images upload --version-id "VERSION_ID" --file path/to/image.png
  asc game-center challenges images view --id "IMAGE_ID"
  asc game-center challenges images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengeImagesUploadCommand(d),
			newGameCenterChallengeImagesGetCommand(d),
			newGameCenterChallengeImagesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengeImagesUploadCommand returns the challenge images upload subcommand.
func newGameCenterChallengeImagesUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterChallengeLocalizations", "Challenge localization ID (exactly one of --localization-id or --version-id)")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "gameCenterChallengeVersions", "Challenge version ID, to set the version's default image (exactly one of --localization-id or --version-id)")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center challenges images upload (--localization-id \"LOCALIZATION_ID\" | --version-id \"VERSION_ID\") --file path/to/image.png",
		ShortHelp:  "Upload an image for a challenge localization or version.",
		LongHelp: `Upload an image for a challenge localization, or a default image for a challenge version.

The upload process reserves an upload slot, uploads the image file, and commits the upload.

Examples:
  asc game-center challenges images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center challenges images upload --version-id "VERSION_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			verID := strings.TrimSpace(*versionID)
			if (locID == "") == (verID == "") {
				return d.UsageError("exactly one of --localization-id or --version-id is required")
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				return d.RequiredFlag("--file")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center challenges images upload: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterChallengeImage(requestCtx, locID, verID, file)
			if err != nil {
				return fmt.Errorf("game-center challenges images upload: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterChallengeImagesGetCommand returns the challenge images get subcommand.
func newGameCenterChallengeImagesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center challenges images view --id \"IMAGE_ID\"",
		ShortHelp:  "View a challenge image by ID.",
		LongHelp: `View a challenge image by ID.

Examples:
  asc game-center challenges images view --id "IMAGE_ID"`,
		IDUsage:            "Challenge image ID",
		IDType:             "gameCenterChallengeImages",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterChallengeImage(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengeImagesDeleteCommand returns the challenge images delete subcommand.
func newGameCenterChallengeImagesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center challenges images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a challenge image.",
		LongHelp: `Delete a challenge image.

Examples:
  asc game-center challenges images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Challenge image ID",
		IDType:             "gameCenterChallengeImages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterChallengeImage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterChallengeReleasesCommand returns the challenge releases command group.
func newGameCenterChallengeReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center challenges releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge releases.",
		LongHelp: `Manage Game Center challenge releases. Releases are used to publish challenge versions to live.

Examples:
  asc game-center challenges releases list --app "APP_ID"
  asc game-center challenges releases create --version-id "VERSION_ID"
  asc game-center challenges releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengeReleasesListCommand(d),
			newGameCenterChallengeReleasesCreateCommand(d),
			newGameCenterChallengeReleasesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengeReleasesListCommand returns the challenge releases list subcommand.
func newGameCenterChallengeReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center challenges releases list --app \"APP_ID\"",
		ShortHelp:  "List releases for Game Center challenges.",
		LongHelp: `List releases for Game Center challenges.

Examples:
  asc game-center challenges releases list --app "APP_ID"
  asc game-center challenges releases list --app "APP_ID" --limit 50
  asc game-center challenges releases list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		LookupParent:         gameCenterDetailParent("game-center challenges releases list"),
		FetchPage: func(ctx context.Context, client *asc.Client, gcDetailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterChallengeVersionReleases(ctx, gcDetailID, asc.Limit[asc.GCChallengeVersionReleasesOption](limit), asc.NextURL[asc.GCChallengeVersionReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterChallengeReleasesCreateCommand returns the challenge releases create subcommand.
func newGameCenterChallengeReleasesCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center challenges releases create --version-id \"VERSION_ID\"",
		ShortHelp:  "Create a Game Center challenge release.",
		LongHelp: `Create a Game Center challenge release. This publishes the version to live.

Examples:
  asc game-center challenges releases create --version-id "VERSION_ID"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center challenge version ID",
		IDType:  "gameCenterChallengeVersions",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterChallengeVersionRelease(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengeReleasesDeleteCommand returns the challenge releases delete subcommand.
func newGameCenterChallengeReleasesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center challenges releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center challenge release.",
		LongHelp: `Delete a Game Center challenge release.

Examples:
  asc game-center challenges releases delete --id "RELEASE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center challenge release ID",
		IDType:             "gameCenterChallengeVersionReleases",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterChallengeVersionRelease(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterChallengeLocalizationImageCommand returns the challenge localization image command group.
func newGameCenterChallengeLocalizationImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center challenges localizations image view --id \"LOC_ID\"",
		ShortHelp:  "Get the image for a challenge localization.",
		LongHelp: `Get the image for a challenge localization.

Examples:
  asc game-center challenges localizations image view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengeLocalizationImageGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengeLocalizationImageGetCommand returns the challenge localization image get subcommand.
func newGameCenterChallengeLocalizationImageGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center challenges localizations image view --id \"LOC_ID\"",
		ShortHelp:  "View a challenge localization image.",
		LongHelp: `View a challenge localization image.

Examples:
  asc game-center challenges localizations image view --id "LOC_ID"`,
		IDUsage:            "Game Center challenge localization ID",
		IDType:             "gameCenterChallengeLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterChallengeLocalizationImage(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterChallengeVersionDefaultImageCommand returns the challenge version default image command group.
func newGameCenterChallengeVersionDefaultImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("default-image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "default-image",
		ShortUsage: "asc game-center challenges versions default-image view --id \"VERSION_ID\"",
		ShortHelp:  "Get the default image for a challenge version.",
		LongHelp: `Get the default image for a challenge version.

Examples:
  asc game-center challenges versions default-image view --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterChallengeVersionDefaultImageGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterChallengeVersionDefaultImageGetCommand returns the challenge version default image get subcommand.
func newGameCenterChallengeVersionDefaultImageGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center challenges versions default-image view --id \"VERSION_ID\"",
		ShortHelp:  "View a default image for a challenge version.",
		LongHelp: `View a default image for a challenge version.

Examples:
  asc game-center challenges versions default-image view --id "VERSION_ID"`,
		IDUsage:            "Game Center challenge version ID",
		IDType:             "gameCenterChallengeVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterChallengeVersionDefaultImage(ctx, id)
		},
		Deps: d,
	})
}
