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

// newGameCenterLeaderboardSetsV2Command returns the leaderboard-sets v2 command group.
func newGameCenterLeaderboardSetsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "v2",
		ShortUsage: "asc game-center leaderboard-sets v2 <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard sets v2 resources.",
		LongHelp: `Manage Game Center leaderboard sets v2 resources.

Examples:
  asc game-center leaderboard-sets v2 list --app "APP_ID"
  asc game-center leaderboard-sets v2 view --id "SET_ID"
  asc game-center leaderboard-sets v2 create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetsV2ListCommand(d),
			newGameCenterLeaderboardSetsV2GetCommand(d),
			newGameCenterLeaderboardSetsV2CreateCommand(d),
			newGameCenterLeaderboardSetsV2UpdateCommand(d),
			newGameCenterLeaderboardSetsV2DeleteCommand(d),
			newGameCenterLeaderboardSetMembersV2Command(d),
			newGameCenterLeaderboardSetVersionsV2Command(d),
			newGameCenterLeaderboardSetLocalizationsV2Command(d),
			newGameCenterLeaderboardSetImagesV2Command(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetsV2ListCommand returns the leaderboard-sets v2 list subcommand.
func newGameCenterLeaderboardSetsV2ListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 list [flags]",
		ShortHelp:  "List Game Center leaderboard sets (v2) for an app or group.",
		LongHelp: `List Game Center leaderboard sets (v2) for an app or group.

Examples:
  asc game-center leaderboard-sets v2 list --app "APP_ID"
  asc game-center leaderboard-sets v2 list --group-id "GROUP_ID"
  asc game-center leaderboard-sets v2 list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "game-center leaderboard-sets v2 list", 200); err != nil {
				return err
			}

			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if group == "" && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" && nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCLeaderboardSetsOption{
				asc.Limit[asc.GCLeaderboardSetsOption](*limit),
				asc.NextURL[asc.GCLeaderboardSetsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.GCLeaderboardSetsOption](200))
				firstPage, err := client.GetGameCenterLeaderboardSetsV2(requestCtx, gcDetailID, group, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetsV2(ctx, gcDetailID, group, asc.NextURL[asc.GCLeaderboardSetsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetsV2(requestCtx, gcDetailID, group, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetsV2GetCommand returns the leaderboard-sets v2 get subcommand.
func newGameCenterLeaderboardSetsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets v2 view --id \"SET_ID\"",
		ShortHelp:  "View a Game Center leaderboard set (v2) by ID.",
		LongHelp: `View a Game Center leaderboard set (v2) by ID.

Examples:
  asc game-center leaderboard-sets v2 view --id "SET_ID"`,
		IDUsage:            "Game Center leaderboard set ID",
		IDType:             "gameCenterLeaderboardSets",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetV2(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetsV2CreateCommand returns the leaderboard-sets v2 create subcommand.
func newGameCenterLeaderboardSetsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env); not used with --leaderboard-ids")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID; not used with --leaderboard-ids")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard set")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.set)")
	leaderboardIDs := shared.BindOnceCSVFlag(fs, "leaderboard-ids", "Comma-separated member leaderboard IDs; the set inherits its parent from them")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets v2 create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard set (v2).",
		LongHelp: `Create a new Game Center leaderboard set (v2).

Creates the required initial leaderboard set version inline.

With --leaderboard-ids, the set inherits its Game Center detail or group from the
member leaderboards, so --app and --group-id are not used.

Examples:
  asc game-center leaderboard-sets v2 create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets v2 create --group-id "GROUP_ID" --reference-name "Group Season" --vendor-id "grp.com.example.groupseason"
  asc game-center leaderboard-sets v2 create --reference-name "Season 2" --vendor-id "com.example.season2" --leaderboard-ids "id1,id2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			ids := shared.SplitUniqueCSV(leaderboardIDs.String())
			if leaderboardIDs.Provided() && len(ids) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --leaderboard-ids must include at least one ID")
				return flag.ErrHelp
			}
			// Apple returns 409 when gameCenterLeaderboards is sent with gameCenterDetail
			// or gameCenterGroup; the set inherits its parent from the leaderboards.
			if len(ids) > 0 {
				if strings.TrimSpace(*appID) != "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --leaderboard-ids; the set inherits its parent from the member leaderboards")
					return flag.ErrHelp
				}
				if group != "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --group-id cannot be used with --leaderboard-ids; the set inherits its parent from the member leaderboards")
					return flag.ErrHelp
				}
			}

			resolvedAppID := ""
			if len(ids) == 0 {
				resolvedAppID = shared.ResolveAppID(*appID)
				if group == "" && resolvedAppID == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
					return shared.MissingRequiredUsageError("")
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
			if group != "" && !strings.HasPrefix(vendor, "grp.") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --vendor-id must start with \"grp.\" when using --group-id")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" && len(ids) == 0 {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 create: failed to get Game Center detail: %w", err)
				}
			}

			attrs := asc.GameCenterLeaderboardSetCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
			}

			resp, err := client.CreateGameCenterLeaderboardSetV2(requestCtx, gcDetailID, group, attrs, ids)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetsV2UpdateCommand returns the leaderboard-sets v2 update subcommand.
func newGameCenterLeaderboardSetsV2UpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardSetUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets v2 update [flags]",
		ShortHelp:  "Update a Game Center leaderboard set (v2).",
		LongHelp: `Update a Game Center leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 update --id "SET_ID" --reference-name "Season 1 - Updated"`,
		IDUsage: "Game Center leaderboard set ID",
		IDType:  "gameCenterLeaderboardSets",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.ReferenceName, "reference-name", "Reference name for the leaderboard set"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterLeaderboardSetV2(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetsV2DeleteCommand returns the leaderboard-sets v2 delete subcommand.
func newGameCenterLeaderboardSetsV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets v2 delete --id \"SET_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set (v2).",
		LongHelp: `Delete a Game Center leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 delete --id "SET_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard set ID",
		IDType:             "gameCenterLeaderboardSets",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMembersV2Command returns the leaderboard set members v2 command group.
func newGameCenterLeaderboardSetMembersV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("members", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "members",
		ShortUsage: "asc game-center leaderboard-sets v2 members <subcommand> [flags]",
		ShortHelp:  "Manage leaderboard set members (v2).",
		LongHelp: `Manage leaderboard set members (v2). Members are the leaderboards that belong to a leaderboard set.

Examples:
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetMembersV2ListCommand(d),
			newGameCenterLeaderboardSetMembersV2SetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetMembersV2ListCommand returns the members v2 list subcommand.
func newGameCenterLeaderboardSetMembersV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 members list --set-id \"SET_ID\"",
		ShortHelp:  "List leaderboards in a leaderboard set (v2).",
		LongHelp: `List leaderboards in a leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID" --paginate`,
		ParentFlag:           "set-id",
		ParentUsage:          "Game Center leaderboard set ID",
		ParentType:           "gameCenterLeaderboardSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, setId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSetMembersV2(ctx, setId, asc.Limit[asc.GCLeaderboardSetMembersOption](limit), asc.NextURL[asc.GCLeaderboardSetMembersOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetMembersV2SetCommand returns the members v2 set subcommand.
func newGameCenterLeaderboardSetMembersV2SetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("set", flag.ExitOnError)

	setID := shared.BindResourceIDFlag(fs, "set-id", "gameCenterLeaderboardSets", "Game Center leaderboard set ID")
	leaderboardIDs := shared.BindOnceCSVFlag(fs, "leaderboard-ids", "Comma-separated leaderboard IDs to set as members")
	confirm := fs.Bool("confirm", false, "Confirm replacing all members (required)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center leaderboard-sets v2 members set --set-id \"SET_ID\" --leaderboard-ids \"id1,id2,id3\" --confirm",
		ShortHelp:  "Replace all leaderboard members in a leaderboard set (v2).",
		LongHelp: `Replace all leaderboard members in a leaderboard set (v2).

This command replaces ALL members of a leaderboard set with the specified leaderboard IDs.
Because replacement can remove existing members, --confirm is required.
To remove all members, pass an empty string for --leaderboard-ids with --confirm.

Examples:
  asc game-center leaderboard-sets v2 members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3" --confirm
  asc game-center leaderboard-sets v2 members set --set-id "SET_ID" --leaderboard-ids "" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				return d.RequiredFlag("--set-id")
			}

			if !flagPassed(fs, "leaderboard-ids") {
				return d.RequiredFlag("--leaderboard-ids")
			}

			if err := validateGameCenterReplacementConfirm(d, fs, *confirm); err != nil {
				return err
			}

			ids := shared.SplitUniqueCSV(leaderboardIDs.String())
			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members set: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if err := client.SetGameCenterLeaderboardSetMembersV2(requestCtx, id, ids); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members set: failed to update: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetMembersUpdateResult{
				SetID:       id,
				MemberCount: len(ids),
				MemberIDs:   ids,
				Updated:     true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetVersionsV2Command returns the leaderboard set versions v2 command group.
func newGameCenterLeaderboardSetVersionsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center leaderboard-sets v2 versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set versions (v2).",
		LongHelp: `Manage Game Center leaderboard set versions (v2).

Examples:
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 versions view --id "VERSION_ID"
  asc game-center leaderboard-sets v2 versions create --set-id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetVersionsV2ListCommand(d),
			newGameCenterLeaderboardSetVersionsV2GetCommand(d),
			newGameCenterLeaderboardSetVersionsV2CreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetVersionsV2ListCommand returns the leaderboard set versions v2 list subcommand.
func newGameCenterLeaderboardSetVersionsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 versions list --set-id \"SET_ID\"",
		ShortHelp:  "List versions for a leaderboard set (v2).",
		LongHelp: `List versions for a leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID" --paginate`,
		ParentFlag:           "set-id",
		ParentUsage:          "Game Center leaderboard set ID",
		ParentType:           "gameCenterLeaderboardSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, setId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSetVersions(ctx, setId, asc.Limit[asc.GCLeaderboardSetVersionsOption](limit), asc.NextURL[asc.GCLeaderboardSetVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetVersionsV2GetCommand returns the leaderboard set versions v2 get subcommand.
func newGameCenterLeaderboardSetVersionsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets v2 versions view --id \"VERSION_ID\"",
		ShortHelp:  "View a Game Center leaderboard set version (v2) by ID.",
		LongHelp: `View a Game Center leaderboard set version (v2) by ID.

Examples:
  asc game-center leaderboard-sets v2 versions view --id "VERSION_ID"`,
		IDUsage:            "Game Center leaderboard set version ID",
		IDType:             "gameCenterLeaderboardSetVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetVersionsV2CreateCommand returns the leaderboard set versions v2 create subcommand.
func newGameCenterLeaderboardSetVersionsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets v2 versions create --set-id \"SET_ID\"",
		ShortHelp:  "Create a new Game Center leaderboard set version (v2).",
		LongHelp: `Create a new Game Center leaderboard set version (v2).

Examples:
  asc game-center leaderboard-sets v2 versions create --set-id "SET_ID"`,
		IDFlag:  "set-id",
		IDUsage: "Game Center leaderboard set ID",
		IDType:  "gameCenterLeaderboardSets",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterLeaderboardSetVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsV2Command returns the leaderboard set localizations v2 command group.
func newGameCenterLeaderboardSetLocalizationsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set localizations (v2).",
		LongHelp: `Manage Game Center leaderboard set localizations (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID"
  asc game-center leaderboard-sets v2 localizations create --version-id "VER_ID" --locale en-US --name "Season 1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetLocalizationsV2ListCommand(d),
			newGameCenterLeaderboardSetLocalizationsV2GetCommand(d),
			newGameCenterLeaderboardSetLocalizationsV2CreateCommand(d),
			newGameCenterLeaderboardSetLocalizationsV2UpdateCommand(d),
			newGameCenterLeaderboardSetLocalizationsV2DeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetLocalizationsV2ListCommand returns the leaderboard set localizations v2 list subcommand.
func newGameCenterLeaderboardSetLocalizationsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations list --version-id \"VER_ID\"",
		ShortHelp:  "List localizations for a leaderboard set version (v2).",
		LongHelp: `List localizations for a leaderboard set version (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID"
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Game Center leaderboard set version ID",
		ParentType:           "gameCenterLeaderboardSetVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, versionId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSetVersionLocalizations(ctx, versionId, asc.Limit[asc.GCLeaderboardSetLocalizationsOption](limit), asc.NextURL[asc.GCLeaderboardSetLocalizationsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsV2GetCommand returns the leaderboard set localizations v2 get subcommand.
func newGameCenterLeaderboardSetLocalizationsV2GetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations view --id \"LOC_ID\"",
		ShortHelp:  "View a Game Center leaderboard set localization (v2) by ID.",
		LongHelp: `View a Game Center leaderboard set localization (v2) by ID.

Examples:
  asc game-center leaderboard-sets v2 localizations view --id "LOC_ID"`,
		IDUsage:            "Game Center leaderboard set localization ID",
		IDType:             "gameCenterLeaderboardSetLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardSetLocalizationV2(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsV2CreateCommand returns the leaderboard set localizations v2 create subcommand.
func newGameCenterLeaderboardSetLocalizationsV2CreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterLeaderboardSetLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations create --version-id \"VER_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a new Game Center leaderboard set localization (v2).",
		LongHelp: `Create a new Game Center leaderboard set localization (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations create --version-id "VER_ID" --locale en-US --name "Season 1"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center leaderboard set version ID",
		IDType:  "gameCenterLeaderboardSetVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US, de-DE)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Display name for the leaderboard set in this locale"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterLeaderboardSetLocalizationV2(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsV2UpdateCommand returns the leaderboard set localizations v2 update subcommand.
func newGameCenterLeaderboardSetLocalizationsV2UpdateCommand(d *shared.Deps) *ffcli.Command {
	var name string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations update --id \"LOC_ID\" --name \"NAME\"",
		ShortHelp:  "Update a Game Center leaderboard set localization (v2).",
		LongHelp: `Update a Game Center leaderboard set localization (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations update --id "LOC_ID" --name "New Name"`,
		IDUsage: "Game Center leaderboard set localization ID",
		IDType:  "gameCenterLeaderboardSetLocalizations",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&name, "name", "Display name for the leaderboard set in this locale"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterLeaderboardSetLocalizationV2(ctx, id, asc.GameCenterLeaderboardSetLocalizationUpdateAttributes{Name: &name})
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetLocalizationsV2DeleteCommand returns the leaderboard set localizations v2 delete subcommand.
func newGameCenterLeaderboardSetLocalizationsV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set localization (v2).",
		LongHelp: `Delete a Game Center leaderboard set localization (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations delete --id "LOC_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard set localization ID",
		IDType:             "gameCenterLeaderboardSetLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetLocalizationV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardSetImagesV2Command returns the leaderboard set images v2 command group.
func newGameCenterLeaderboardSetImagesV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center leaderboard-sets v2 images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set images (v2).",
		LongHelp: `Manage Game Center leaderboard set images (v2). Images are attached to leaderboard set localizations.

Examples:
  asc game-center leaderboard-sets v2 images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboard-sets v2 images view --id "IMAGE_ID"
  asc game-center leaderboard-sets v2 images view --localization-id "LOC_ID"
  asc game-center leaderboard-sets v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardSetImagesV2UploadCommand(d),
			newGameCenterLeaderboardSetImagesV2GetCommand(d),
			newGameCenterLeaderboardSetImagesV2DeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardSetImagesV2UploadCommand returns the leaderboard set images v2 upload subcommand.
func newGameCenterLeaderboardSetImagesV2UploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterLeaderboardSetLocalizations", "Game Center leaderboard set localization ID")
	filePath := fs.String("file", "", "Path to image file")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center leaderboard-sets v2 images upload --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a Game Center leaderboard set localization (v2).",
		LongHelp: `Upload an image for a Game Center leaderboard set localization (v2).

This command performs the full upload flow: reserves the upload, uploads the file, and commits.

Examples:
  asc game-center leaderboard-sets v2 images upload --localization-id "LOC_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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
				return fmt.Errorf("game-center leaderboard-sets v2 images upload: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardSetImageV2(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images upload: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetImagesV2GetCommand returns the leaderboard set images v2 get subcommand.
func newGameCenterLeaderboardSetImagesV2GetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("view", flag.ExitOnError)

	imageID := shared.BindResourceIDFlag(fs, "id", "gameCenterLeaderboardSetImages", "Game Center leaderboard set image ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterLeaderboardSetLocalizations", "Game Center leaderboard set localization ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc game-center leaderboard-sets v2 images view --id \"IMAGE_ID\" | --localization-id \"LOC_ID\"",
		ShortHelp:  "View a Game Center leaderboard set image (v2).",
		LongHelp: `View a Game Center leaderboard set image (v2).

Examples:
  asc game-center leaderboard-sets v2 images view --id "IMAGE_ID"
  asc game-center leaderboard-sets v2 images view --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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
				return fmt.Errorf("game-center leaderboard-sets v2 images view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			if locID != "" {
				resp, err := client.GetGameCenterLeaderboardSetLocalizationImageV2(requestCtx, locID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 images view: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetImageV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images view: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardSetImagesV2DeleteCommand returns the leaderboard set images v2 delete subcommand.
func newGameCenterLeaderboardSetImagesV2DeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets v2 images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set image (v2).",
		LongHelp: `Delete a Game Center leaderboard set image (v2).

Examples:
  asc game-center leaderboard-sets v2 images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard set image ID",
		IDType:             "gameCenterLeaderboardSetImages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetImageV2(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}
