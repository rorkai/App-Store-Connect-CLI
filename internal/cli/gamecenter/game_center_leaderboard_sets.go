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

// newGameCenterLeaderboardSetsCommand returns the leaderboard-sets command group.
func newGameCenterLeaderboardSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-sets",
		ShortUsage: "asc game-center leaderboard-sets <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard sets.",
		LongHelp: `Manage Game Center leaderboard sets.

Examples:
  asc game-center leaderboard-sets list --app "APP_ID"
  asc game-center leaderboard-sets view --id "SET_ID"
  asc game-center leaderboard-sets group-leaderboard-set view --id "SET_ID"
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets update --id "SET_ID" --reference-name "Season 1 - Updated"
  asc game-center leaderboard-sets delete --id "SET_ID" --confirm
  asc game-center leaderboard-sets members list --set-id "SET_ID"
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3" --confirm
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboard-sets releases list --set-id "SET_ID"
  asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"
  asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm
  asc game-center leaderboard-sets localizations list --set-id "SET_ID"
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
  asc game-center leaderboard-sets localizations image view --id "LOC_ID"
  asc game-center leaderboard-sets images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboard-sets images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetsListCommand(d),
			newGameCenterLeaderboardSetsGetCommand(d),
			newGameCenterLeaderboardSetGroupLeaderboardSetCommand(d),
			newGameCenterLeaderboardSetsCreateCommand(d),
			newGameCenterLeaderboardSetsUpdateCommand(d),
			newGameCenterLeaderboardSetsDeleteCommand(d),
			newGameCenterLeaderboardSetsV2Command(d),
			newGameCenterLeaderboardSetMembersCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsCommand(d),
			newGameCenterLeaderboardSetReleasesCommand(d),
			newGameCenterLeaderboardSetImagesCommand(d),
			newGameCenterLeaderboardSetLocalizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetLocalizationsCommand returns the localizations command group.
func newGameCenterLeaderboardSetLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboard-sets localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set localizations.",
		LongHelp: `Manage Game Center leaderboard set localizations.

Examples:
  asc game-center leaderboard-sets localizations list --set-id "SET_ID"
  asc game-center leaderboard-sets localizations view --id "LOC_ID"
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
  asc game-center leaderboard-sets localizations update --id "LOC_ID" --name "New Name"
  asc game-center leaderboard-sets localizations delete --id "LOC_ID" --confirm
  asc game-center leaderboard-sets localizations image view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetLocalizationsListCommand(d),
			newGameCenterLeaderboardSetLocalizationsGetCommand(d),
			newGameCenterLeaderboardSetLocalizationsCreateCommand(d),
			newGameCenterLeaderboardSetLocalizationsUpdateCommand(d),
			newGameCenterLeaderboardSetLocalizationsDeleteCommand(d),
			newGameCenterLeaderboardSetLocalizationImageCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetLocalizationsListCommand returns the localizations list subcommand.
func newGameCenterLeaderboardSetLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets localizations list --set-id \"SET_ID\"",
		ShortHelp:  "List localizations for a Game Center leaderboard set.",
		LongHelp: `List localizations for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets localizations list --set-id "SET_ID"
  asc game-center leaderboard-sets localizations list --set-id "SET_ID" --limit 50
  asc game-center leaderboard-sets localizations list --set-id "SET_ID" --paginate`,
		ParentFlag:           "set-id",
		ParentUsage:          "Game Center leaderboard set ID",
		ParentType:           "gameCenterLeaderboardSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, setId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSetLocalizations(ctx, setId, asc.Limit[asc.GCLeaderboardSetLocalizationsOption](limit), asc.NextURL[asc.GCLeaderboardSetLocalizationsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsGetCommand returns the localizations get subcommand.
func newGameCenterLeaderboardSetLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets localizations view --id \"LOC_ID\"",
		ShortHelp:  "View a Game Center leaderboard set localization by ID.",
		LongHelp: `View a Game Center leaderboard set localization by ID.

Examples:
  asc game-center leaderboard-sets localizations view --id "LOC_ID"`,
		IDUsage:            "Game Center leaderboard set localization ID",
		IDType:             "gameCenterLeaderboardSetLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetLocalization(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsCreateCommand returns the localizations create subcommand.
func newGameCenterLeaderboardSetLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardSetLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets localizations create --set-id \"SET_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a localization for a Game Center leaderboard set.",
		LongHelp: `Create a localization for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale de-DE --name "Staffel 1"`,
		IDFlag:  "set-id",
		IDUsage: "Game Center leaderboard set ID",
		IDType:  "gameCenterLeaderboardSets",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale code (e.g., en-US, de-DE)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Display name for the leaderboard set"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterLeaderboardSetLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsUpdateCommand returns the localizations update subcommand.
func newGameCenterLeaderboardSetLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardSetLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets localizations update --id \"LOC_ID\" --name \"NAME\"",
		ShortHelp:  "Update a Game Center leaderboard set localization.",
		LongHelp: `Update a Game Center leaderboard set localization.

Examples:
  asc game-center leaderboard-sets localizations update --id "LOC_ID" --name "New Name"`,
		IDUsage: "Game Center leaderboard set localization ID",
		IDType:  "gameCenterLeaderboardSetLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Display name for the leaderboard set"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterLeaderboardSetLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsDeleteCommand returns the localizations delete subcommand.
func newGameCenterLeaderboardSetLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set localization.",
		LongHelp: `Delete a Game Center leaderboard set localization.

Examples:
  asc game-center leaderboard-sets localizations delete --id "LOC_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard set localization ID",
		IDType:             "gameCenterLeaderboardSetLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetsListCommand returns the leaderboard-sets list subcommand.
func newGameCenterLeaderboardSetsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets list [flags]",
		ShortHelp:  "List Game Center leaderboard sets for an app.",
		LongHelp: `List Game Center leaderboard sets for an app.

Examples:
  asc game-center leaderboard-sets list --app "APP_ID"
  asc game-center leaderboard-sets list --app "APP_ID" --limit 50
  asc game-center leaderboard-sets list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		LookupParent:         gameCenterDetailParent("game-center leaderboard-sets list"),
		FetchPage: func(ctx context.Context, client *asc.Client, gcDetailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSets(ctx, gcDetailID, asc.Limit[asc.GCLeaderboardSetsOption](limit), asc.NextURL[asc.GCLeaderboardSetsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetsGetCommand returns the leaderboard-sets get subcommand.
func newGameCenterLeaderboardSetsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets view --id \"SET_ID\"",
		ShortHelp:  "View a Game Center leaderboard set by ID.",
		LongHelp: `View a Game Center leaderboard set by ID.

Examples:
  asc game-center leaderboard-sets view --id "SET_ID"`,
		IDUsage:            "Game Center leaderboard set ID",
		IDType:             "gameCenterLeaderboardSets",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSet(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetsCreateCommand returns the leaderboard-sets create subcommand.
func newGameCenterLeaderboardSetsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env); not used with --leaderboard-ids")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard set")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.set)")
	leaderboardIDs := shared.BindOnceCSVFlag(fs, "leaderboard-ids", "Comma-separated member leaderboard IDs; the set inherits its parent from them")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard set.",
		LongHelp: `Create a new Game Center leaderboard set.

With --leaderboard-ids, the set inherits its Game Center detail or group from the
member leaderboards, so --app is not used.

Examples:
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Weekly Challenge" --vendor-id "com.example.weekly"
  asc game-center leaderboard-sets create --reference-name "Season 2" --vendor-id "com.example.season2" --leaderboard-ids "id1,id2"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			ids := shared.SplitUniqueCSV(leaderboardIDs.String())
			if leaderboardIDs.Provided() && len(ids) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --leaderboard-ids must include at least one ID")
				return flag.ErrHelp
			}
			// Apple returns 409 when gameCenterLeaderboards is sent with gameCenterDetail
			// or gameCenterGroup; the set inherits its parent from the leaderboards.
			if len(ids) > 0 && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --leaderboard-ids; the set inherits its parent from the member leaderboards")
				return flag.ErrHelp
			}

			resolvedAppID := ""
			if len(ids) == 0 {
				resolvedAppID = d.ResolveAppID(*appID)
				if resolvedAppID == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
					return shared.MissingRequiredUsageError("--app")
				}
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}

			vendor := strings.TrimSpace(*vendorID)
			if vendor == "" {
				return d.RequiredFlag("--vendor-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if len(ids) == 0 {
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets create: failed to get Game Center detail: %w", err)
				}
			}

			attrs := asc.GameCenterLeaderboardSetCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
			}

			resp, err := client.CreateGameCenterLeaderboardSet(requestCtx, gcDetailID, attrs, ids)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetsUpdateCommand returns the leaderboard-sets update subcommand.
func newGameCenterLeaderboardSetsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardSetUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets update [flags]",
		ShortHelp:  "Update a Game Center leaderboard set.",
		LongHelp: `Update a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets update --id "SET_ID" --reference-name "Season 1 - Updated"`,
		IDUsage: "Game Center leaderboard set ID",
		IDType:  "gameCenterLeaderboardSets",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.ReferenceName, "reference-name", "Reference name for the leaderboard set"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterLeaderboardSet(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetsDeleteCommand returns the leaderboard-sets delete subcommand.
func newGameCenterLeaderboardSetsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets delete --id \"SET_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set.",
		LongHelp: `Delete a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets delete --id "SET_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard set ID",
		IDType:             "gameCenterLeaderboardSets",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSet(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetReleasesCommand returns the leaderboard-sets releases command group.
func newGameCenterLeaderboardSetReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center leaderboard-sets releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set releases.",
		LongHelp: `Manage Game Center leaderboard set releases.

Releases control which Game Center details (apps) a leaderboard set is associated with.

Examples:
  asc game-center leaderboard-sets releases list --set-id "SET_ID"
  asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"
  asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetReleasesListCommand(d),
			newGameCenterLeaderboardSetReleasesCreateCommand(d),
			newGameCenterLeaderboardSetReleasesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetReleasesListCommand returns the leaderboard-sets releases list subcommand.
func newGameCenterLeaderboardSetReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets releases list --set-id \"SET_ID\"",
		ShortHelp:  "List releases for a Game Center leaderboard set.",
		LongHelp: `List releases for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets releases list --set-id "SET_ID"
  asc game-center leaderboard-sets releases list --set-id "SET_ID" --limit 50
  asc game-center leaderboard-sets releases list --set-id "SET_ID" --paginate`,
		ParentFlag:           "set-id",
		ParentUsage:          "Game Center leaderboard set ID",
		ParentType:           "gameCenterLeaderboardSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, setId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSetReleases(ctx, setId, asc.Limit[asc.GCLeaderboardSetReleasesOption](limit), asc.NextURL[asc.GCLeaderboardSetReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetReleasesCreateCommand returns the leaderboard-sets releases create subcommand.
func newGameCenterLeaderboardSetReleasesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	setID := shared.BindResourceIDFlag(fs, "set-id", "gameCenterLeaderboardSets", "Game Center leaderboard set ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets releases create --app \"APP_ID\" --set-id \"SET_ID\"",
		ShortHelp:  "Create a release for a Game Center leaderboard set.",
		LongHelp: `Create a release for a Game Center leaderboard set.

This associates the leaderboard set with the app's Game Center detail.

Examples:
  asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			id := strings.TrimSpace(*setID)
			if id == "" {
				return d.RequiredFlag("--set-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			// Get Game Center detail ID first
			gcDetailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases create: failed to get Game Center detail: %w", err)
			}

			resp, err := client.CreateGameCenterLeaderboardSetRelease(requestCtx, gcDetailID, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetReleasesDeleteCommand returns the leaderboard-sets releases delete subcommand.
func newGameCenterLeaderboardSetReleasesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set release.",
		LongHelp: `Delete a Game Center leaderboard set release.

Examples:
  asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard set release ID",
		IDType:             "gameCenterLeaderboardSetReleases",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetRelease(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetGroupLeaderboardSetCommand returns the group leaderboard set command group.
func newGameCenterLeaderboardSetGroupLeaderboardSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("group-leaderboard-set", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group-leaderboard-set",
		ShortUsage: "asc game-center leaderboard-sets group-leaderboard-set view --id \"SET_ID\"",
		ShortHelp:  "Get the group leaderboard set for a leaderboard set.",
		LongHelp: `Get the group leaderboard set for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets group-leaderboard-set view --id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetGroupLeaderboardSetGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetGroupLeaderboardSetGetCommand returns the group leaderboard set get subcommand.
func newGameCenterLeaderboardSetGroupLeaderboardSetGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets group-leaderboard-set view --id \"SET_ID\"",
		ShortHelp:  "View a group leaderboard set by leaderboard set ID.",
		LongHelp: `View a group leaderboard set by leaderboard set ID.

Examples:
  asc game-center leaderboard-sets group-leaderboard-set view --id "SET_ID"`,
		IDUsage:            "Game Center leaderboard set ID",
		IDType:             "gameCenterLeaderboardSets",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetGroupLeaderboardSet(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationImageCommand returns the localization image command group.
func newGameCenterLeaderboardSetLocalizationImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center leaderboard-sets localizations image view --id \"LOC_ID\"",
		ShortHelp:  "Get the image for a leaderboard set localization.",
		LongHelp: `Get the image for a leaderboard set localization.

Examples:
  asc game-center leaderboard-sets localizations image view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetLocalizationImageGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetLocalizationImageGetCommand returns the localization image get subcommand.
func newGameCenterLeaderboardSetLocalizationImageGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets localizations image view --id \"LOC_ID\"",
		ShortHelp:  "View a leaderboard set localization image.",
		LongHelp: `View a leaderboard set localization image.

Examples:
  asc game-center leaderboard-sets localizations image view --id "LOC_ID"`,
		IDUsage:            "Game Center leaderboard set localization ID",
		IDType:             "gameCenterLeaderboardSetLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetLocalizationImage(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMemberLocalizationsCommand returns the member localizations command group.
func newGameCenterLeaderboardSetMemberLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("member-localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "member-localizations",
		ShortUsage: "asc game-center leaderboard-sets member-localizations <subcommand> [flags]",
		ShortHelp:  "Manage leaderboard set member localizations.",
		LongHelp: `Manage leaderboard set member localizations.

Examples:
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboard-sets member-localizations view --id "LOCALIZATION_ID"
  asc game-center leaderboard-sets member-localizations create --leaderboard-set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --locale "en-US" --name "Top Score"
  asc game-center leaderboard-sets member-localizations update --id "LOCALIZATION_ID" --name "New Name"
  asc game-center leaderboard-sets member-localizations delete --id "LOCALIZATION_ID" --confirm
  asc game-center leaderboard-sets member-localizations leaderboard view --id "LOCALIZATION_ID"
  asc game-center leaderboard-sets member-localizations leaderboard-set view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetMemberLocalizationsListCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsGetCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsCreateCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsUpdateCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsDeleteCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsLeaderboardCommand(d),
			newGameCenterLeaderboardSetMemberLocalizationsLeaderboardSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetMemberLocalizationsListCommand returns the member localizations list subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	var leaderboardID *string

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets member-localizations list --set-id \"SET_ID\" --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "List leaderboard set member localizations.",
		LongHelp: `List leaderboard set member localizations.

Examples:
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --limit 50
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --paginate`,
		ParentFlag:           "set-id",
		ParentUsage:          "Game Center leaderboard set ID",
		ParentType:           "gameCenterLeaderboardSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Flags: func(fs *flag.FlagSet) func() error {
			leaderboardID = shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Game Center leaderboard ID")
			next := fs.Lookup("next").Value
			return func() error {
				if strings.TrimSpace(next.String()) == "" && strings.TrimSpace(*leaderboardID) == "" {
					return d.RequiredFlag("--leaderboard-id")
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, setID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.GCLeaderboardSetMemberLocalizationsOption{
				asc.Limit[asc.GCLeaderboardSetMemberLocalizationsOption](limit),
				asc.NextURL[asc.GCLeaderboardSetMemberLocalizationsOption](next),
			}
			if setID != "" {
				opts = append(opts, asc.WithGCLeaderboardSetMemberLocalizationsLeaderboardSetIDs([]string{setID}))
			}
			if leaderboard := strings.TrimSpace(*leaderboardID); leaderboard != "" {
				opts = append(opts, asc.WithGCLeaderboardSetMemberLocalizationsLeaderboardIDs([]string{leaderboard}))
			}
			return client.GetGameCenterLeaderboardSetMemberLocalizations(ctx, opts...)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMemberLocalizationsGetCommand returns the member localization view subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets member-localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a leaderboard set member localization by ID.",
		LongHelp: `View a leaderboard set member localization by ID.

App Store Connect does not expose a direct instance GET. This command resolves
the localization's leaderboard and leaderboard set, then searches their member
localizations for the exact ID.

Examples:
  asc game-center leaderboard-sets member-localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Leaderboard set member localization ID",
		IDType:             "gameCenterLeaderboardSetMemberLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetMemberLocalization(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMemberLocalizationsCreateCommand returns the member localizations create subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	leaderboardSetID := shared.BindResourceIDFlag(fs, "leaderboard-set-id", "gameCenterLeaderboardSets", "Game Center leaderboard set ID")
	leaderboardID := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Game Center leaderboard ID")
	locale := fs.String("locale", "", "Locale code (e.g., en-US, de-DE)")
	name := fs.String("name", "", "Display name for the member localization")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets member-localizations create --leaderboard-set-id \"SET_ID\" --leaderboard-id \"LEADERBOARD_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a leaderboard set member localization.",
		LongHelp: `Create a leaderboard set member localization.

Examples:
  asc game-center leaderboard-sets member-localizations create --leaderboard-set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "Top Score"
  asc game-center leaderboard-sets member-localizations create --leaderboard-set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --locale de-DE --name "Beste Punktzahl"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			setID := strings.TrimSpace(*leaderboardSetID)
			if setID == "" {
				return d.RequiredFlag("--leaderboard-set-id")
			}

			lbID := strings.TrimSpace(*leaderboardID)
			if lbID == "" {
				return d.RequiredFlag("--leaderboard-id")
			}

			localeVal := strings.TrimSpace(*locale)
			if localeVal == "" {
				return d.RequiredFlag("--locale")
			}

			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				return d.RequiredFlag("--name")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.GameCenterLeaderboardSetMemberLocalizationCreateAttributes{
				Name:   nameVal,
				Locale: localeVal,
			}

			resp, err := client.CreateGameCenterLeaderboardSetMemberLocalization(requestCtx, setID, lbID, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetMemberLocalizationsUpdateCommand returns the member localizations update subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardSetMemberLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets member-localizations update --id \"LOCALIZATION_ID\" --name \"NAME\"",
		ShortHelp:  "Update a leaderboard set member localization.",
		LongHelp: `Update a leaderboard set member localization.

Examples:
  asc game-center leaderboard-sets member-localizations update --id "LOCALIZATION_ID" --name "New Name"`,
		IDUsage: "Leaderboard set member localization ID",
		IDType:  "gameCenterLeaderboardSetMemberLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Display name for the member localization"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterLeaderboardSetMemberLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMemberLocalizationsDeleteCommand returns the member localizations delete subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets member-localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a leaderboard set member localization.",
		LongHelp: `Delete a leaderboard set member localization.

Examples:
  asc game-center leaderboard-sets member-localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Leaderboard set member localization ID",
		IDType:             "gameCenterLeaderboardSetMemberLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetMemberLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMemberLocalizationsLeaderboardCommand returns the member localization leaderboard command group.
func newGameCenterLeaderboardSetMemberLocalizationsLeaderboardCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get the leaderboard for a member localization.",
		LongHelp: `Get the leaderboard for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetMemberLocalizationsLeaderboardGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetMemberLocalizationsLeaderboardGetCommand returns the member localization leaderboard get subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsLeaderboardGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a leaderboard for a member localization.",
		LongHelp: `View a leaderboard for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard view --id "LOCALIZATION_ID"`,
		IDUsage:            "Leaderboard set member localization ID",
		IDType:             "gameCenterLeaderboardSetMemberLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetMemberLocalizationLeaderboard(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMemberLocalizationsLeaderboardSetCommand returns the member localization leaderboard set command group.
func newGameCenterLeaderboardSetMemberLocalizationsLeaderboardSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard-set", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-set",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard-set view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get the leaderboard set for a member localization.",
		LongHelp: `Get the leaderboard set for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard-set view --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetMemberLocalizationsLeaderboardSetGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetMemberLocalizationsLeaderboardSetGetCommand returns the member localization leaderboard set get subcommand.
func newGameCenterLeaderboardSetMemberLocalizationsLeaderboardSetGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard-set view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View a leaderboard set for a member localization.",
		LongHelp: `View a leaderboard set for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard-set view --id "LOCALIZATION_ID"`,
		IDUsage:            "Leaderboard set member localization ID",
		IDType:             "gameCenterLeaderboardSetMemberLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetMemberLocalizationLeaderboardSet(ctx, id)
		},
		Deps: d,
	})
}
