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

// newGameCenterActivitiesCommand returns the activities command group.
func newGameCenterActivitiesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("activities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "activities",
		ShortUsage: "asc game-center activities <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activities.",
		LongHelp: `Manage Game Center activities.

Examples:
  asc game-center activities list --app "APP_ID"
  asc game-center activities view --id "ACTIVITY_ID"
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly"
  asc game-center activities update --id "ACTIVITY_ID" --archived true
  asc game-center activities delete --id "ACTIVITY_ID" --confirm
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1,ACH_2"
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1,LB_2"
  asc game-center activities versions list --activity-id "ACTIVITY_ID"
  asc game-center activities localizations list --version-id "VERSION_ID"
  asc game-center activities localizations image view --id "LOC_ID"
  asc game-center activities versions default-image view --id "VERSION_ID"
  asc game-center activities images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center activities releases list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivitiesListCommand(d),
			newGameCenterActivitiesGetCommand(d),
			newGameCenterActivitiesCreateCommand(d),
			newGameCenterActivitiesUpdateCommand(d),
			newGameCenterActivitiesDeleteCommand(d),
			newGameCenterActivityAchievementsCommand(d),
			newGameCenterActivityLeaderboardsCommand(d),
			newGameCenterActivityVersionsCommand(d),
			newGameCenterActivityLocalizationsCommand(d),
			newGameCenterActivityImagesCommand(d),
			newGameCenterActivityReleasesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivitiesListCommand returns the activities list subcommand.
func newGameCenterActivitiesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center activities list [flags]",
		ShortHelp:  "List Game Center activities for an app.",
		LongHelp: `List Game Center activities for an app.

Examples:
  asc game-center activities list --app "APP_ID"
  asc game-center activities list --app "APP_ID" --limit 50
  asc game-center activities list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		LookupParent:         gameCenterDetailParent("game-center activities list"),
		FetchPage: func(ctx context.Context, client *asc.Client, gcDetailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterActivities(ctx, gcDetailID, asc.Limit[asc.GCActivitiesOption](limit), asc.NextURL[asc.GCActivitiesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterActivitiesGetCommand returns the activities get subcommand.
func newGameCenterActivitiesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center activities view --id \"ACTIVITY_ID\"",
		ShortHelp:  "View a Game Center activity by ID.",
		LongHelp: `View a Game Center activity by ID.

Examples:
  asc game-center activities view --id "ACTIVITY_ID"`,
		IDUsage:            "Game Center activity ID",
		IDType:             "gameCenterActivities",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterActivity(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterActivitiesCreateCommand returns the activities create subcommand.
func newGameCenterActivitiesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the activity")
	vendorID := fs.String("vendor-id", "", "Vendor identifier for the activity")
	playStyle := fs.String("play-style", "", "Play style (ASYNCHRONOUS, SYNCHRONOUS)")
	minPlayers := fs.Int("min-players", 0, "Minimum players count")
	maxPlayers := fs.Int("max-players", 0, "Maximum players count")
	supportsPartyCode := fs.String("supports-party-code", "", "Supports party code (true/false)")
	fs.String("properties", "", "Activity properties as a JSON object of string values, inline or @file:PATH")
	createInitialVersion := fs.String("create-initial-version", "", "Create an initial activity version inline (true/false)")
	initialFallbackURL := fs.String("initial-fallback-url", "", "Fallback URL for the initial activity version")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center activities create --app \"APP_ID\" --reference-name \"Weekly\" --vendor-id \"com.example.weekly\"",
		ShortHelp:  "Create a Game Center activity.",
		LongHelp: `Create a Game Center activity.

Examples:
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly"
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --create-initial-version true
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --properties '{"mode":"ranked"}'
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --create-initial-version true --initial-fallback-url "https://example.com/fallback"
  asc game-center activities create --group-id "GROUP_ID" --reference-name "Weekly" --vendor-id "grp.com.example.weekly"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := d.ResolveAppID(*appID)
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
			if strings.TrimSpace(*initialFallbackURL) != "" && !createInitialVersionValue {
				fmt.Fprintln(d.IO.ErrOut, "Error: --initial-fallback-url requires --create-initial-version true")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterActivityCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
			}

			if strings.TrimSpace(*playStyle) != "" {
				value := strings.TrimSpace(*playStyle)
				attrs.PlayStyle = &value
			}
			if *minPlayers > 0 {
				value := *minPlayers
				attrs.MinimumPlayersCount = &value
			}
			if *maxPlayers > 0 {
				value := *maxPlayers
				attrs.MaximumPlayersCount = &value
			}
			if strings.TrimSpace(*supportsPartyCode) != "" {
				val, err := shared.ParseBoolFlag(*supportsPartyCode, "--supports-party-code")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.SupportsPartyCode = &val
			}
			properties, err := parseStringMapFlag(d, fs, "properties")
			if err != nil {
				return err
			}
			attrs.Properties = properties

			var initialVersion *asc.GameCenterActivityVersionCreateAttributes
			if createInitialVersionValue {
				initialVersion = &asc.GameCenterActivityVersionCreateAttributes{}
				if value := strings.TrimSpace(*initialFallbackURL); value != "" {
					initialVersion.FallbackURL = &value
				}
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center activities create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center activities create: failed to get Game Center detail: %w", err)
				}
			}

			resp, err := client.CreateGameCenterActivity(requestCtx, gcDetailID, attrs, group, initialVersion)
			if err != nil {
				return fmt.Errorf("game-center activities create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterActivitiesUpdateCommand returns the activities update subcommand.
func newGameCenterActivitiesUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	activityID := shared.BindResourceIDFlag(fs, "id", "gameCenterActivities", "Game Center activity ID")
	referenceName := fs.String("reference-name", "", "Reference name for the activity")
	playStyle := fs.String("play-style", "", "Play style (ASYNCHRONOUS, SYNCHRONOUS)")
	minPlayers := fs.Int("min-players", 0, "Minimum players count")
	maxPlayers := fs.Int("max-players", 0, "Maximum players count")
	supportsPartyCode := fs.String("supports-party-code", "", "Supports party code (true/false)")
	archived := fs.String("archived", "", "Archive the activity (true/false)")
	fs.String("properties", "", "Activity properties as a JSON object of string values, inline or @file:PATH")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center activities update --id \"ACTIVITY_ID\" [flags]",
		ShortHelp:  "Update a Game Center activity.",
		LongHelp: `Update a Game Center activity.

Examples:
  asc game-center activities update --id "ACTIVITY_ID" --reference-name "New Name"
  asc game-center activities update --id "ACTIVITY_ID" --archived true
  asc game-center activities update --id "ACTIVITY_ID" --properties @file:properties.json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterActivityUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				value := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*playStyle) != "" {
				value := strings.TrimSpace(*playStyle)
				attrs.PlayStyle = &value
				hasUpdate = true
			}
			if *minPlayers > 0 {
				value := *minPlayers
				attrs.MinimumPlayersCount = &value
				hasUpdate = true
			}
			if *maxPlayers > 0 {
				value := *maxPlayers
				attrs.MaximumPlayersCount = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*supportsPartyCode) != "" {
				val, err := shared.ParseBoolFlag(*supportsPartyCode, "--supports-party-code")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.SupportsPartyCode = &val
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
			properties, err := parseStringMapFlag(d, fs, "properties")
			if err != nil {
				return err
			}
			if properties != nil {
				attrs.Properties = properties
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, referenceName, playStyle, minPlayers, maxPlayers, supportsPartyCode, archived, fs.Lookup("properties").Value)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center activities update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterActivity(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center activities update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterActivitiesDeleteCommand returns the activities delete subcommand.
func newGameCenterActivitiesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center activities delete --id \"ACTIVITY_ID\" --confirm",
		ShortHelp:  "Delete a Game Center activity.",
		LongHelp: `Delete a Game Center activity.

Examples:
  asc game-center activities delete --id "ACTIVITY_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center activity ID",
		IDType:             "gameCenterActivities",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterActivity(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterActivityAchievementsCommand returns the activity achievements command group.
func newGameCenterActivityAchievementsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("achievements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements",
		ShortUsage: "asc game-center activities achievements set --activity-id \"ACTIVITY_ID\" --ids \"ACH_1,ACH_2\"",
		ShortHelp:  "Manage activity achievements relationships.",
		LongHelp: `Manage activity achievements relationships.

Use --remove to remove relationships instead of adding.

Examples:
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1,ACH_2"
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1" --remove --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityAchievementsSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityAchievementsSetCommand returns the activity achievements set subcommand.
func newGameCenterActivityAchievementsSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("set", flag.ExitOnError)

	activityID := shared.BindResourceIDFlag(fs, "activity-id", "gameCenterActivities", "Game Center activity ID")
	ids := shared.BindOnceCSVFlag(fs, "ids", "Comma-separated achievement IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	remove := fs.Bool("remove", false, "Remove relationships instead of adding")
	confirm := fs.Bool("confirm", false, "Confirm removal (required with --remove)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center activities achievements set --activity-id \"ACTIVITY_ID\" --ids \"ACH_1,ACH_2\"",
		ShortHelp:  "Update activity achievements relationships.",
		LongHelp: `Update activity achievements relationships. By default, this adds relationships.

Examples:
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1,ACH_2"
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1" --v2
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1" --remove --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				return d.RequiredFlag("--activity-id")
			}
			idsValue := shared.SplitCSV(ids.String())
			if len(idsValue) == 0 {
				return d.RequiredFlag("--ids")
			}
			if *confirm && !*remove {
				return d.UsageError("--confirm requires --remove")
			}
			if *remove && !*confirm {
				fmt.Fprintln(d.IO.ErrOut, "Error: --confirm is required with --remove")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center activities achievements set: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			removeRelationships, addRelationships := client.RemoveGameCenterActivityAchievements, client.AddGameCenterActivityAchievements
			if *v2 {
				removeRelationships, addRelationships = client.RemoveGameCenterActivityAchievementsV2, client.AddGameCenterActivityAchievementsV2
			}

			if *remove {
				if err := removeRelationships(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities achievements set: failed to remove: %w", err)
				}
			} else {
				if err := addRelationships(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities achievements set: failed to add: %w", err)
				}
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterAchievements, idsValue)}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterActivityLeaderboardsCommand returns the activity leaderboards command group.
func newGameCenterActivityLeaderboardsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboards", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards",
		ShortUsage: "asc game-center activities leaderboards set --activity-id \"ACTIVITY_ID\" --ids \"LB_1,LB_2\"",
		ShortHelp:  "Manage activity leaderboards relationships.",
		LongHelp: `Manage activity leaderboards relationships.

Use --remove to remove relationships instead of adding.

Examples:
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1,LB_2"
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1" --remove --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityLeaderboardsSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityLeaderboardsSetCommand returns the activity leaderboards set subcommand.
func newGameCenterActivityLeaderboardsSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("set", flag.ExitOnError)

	activityID := shared.BindResourceIDFlag(fs, "activity-id", "gameCenterActivities", "Game Center activity ID")
	ids := shared.BindOnceCSVFlag(fs, "ids", "Comma-separated leaderboard IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	remove := fs.Bool("remove", false, "Remove relationships instead of adding")
	confirm := fs.Bool("confirm", false, "Confirm removal (required with --remove)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center activities leaderboards set --activity-id \"ACTIVITY_ID\" --ids \"LB_1,LB_2\"",
		ShortHelp:  "Update activity leaderboards relationships.",
		LongHelp: `Update activity leaderboards relationships. By default, this adds relationships.

Examples:
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1,LB_2"
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1" --v2
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1" --remove --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				return d.RequiredFlag("--activity-id")
			}
			idsValue := shared.SplitCSV(ids.String())
			if len(idsValue) == 0 {
				return d.RequiredFlag("--ids")
			}
			if *confirm && !*remove {
				return d.UsageError("--confirm requires --remove")
			}
			if *remove && !*confirm {
				fmt.Fprintln(d.IO.ErrOut, "Error: --confirm is required with --remove")
				return shared.MissingRequiredUsageError("--confirm")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center activities leaderboards set: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			removeRelationships, addRelationships := client.RemoveGameCenterActivityLeaderboards, client.AddGameCenterActivityLeaderboards
			if *v2 {
				removeRelationships, addRelationships = client.RemoveGameCenterActivityLeaderboardsV2, client.AddGameCenterActivityLeaderboardsV2
			}

			if *remove {
				if err := removeRelationships(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities leaderboards set: failed to remove: %w", err)
				}
			} else {
				if err := addRelationships(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities leaderboards set: failed to add: %w", err)
				}
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterLeaderboards, idsValue)}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterActivityVersionsCommand returns the activity versions command group.
func newGameCenterActivityVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center activities versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activity versions.",
		LongHelp: `Manage Game Center activity versions.

Examples:
  asc game-center activities versions list --activity-id "ACTIVITY_ID"
  asc game-center activities versions view --id "VERSION_ID"
  asc game-center activities versions create --activity-id "ACTIVITY_ID" --fallback-url "https://example.com"
  asc game-center activities versions update --id "VERSION_ID" --fallback-url "https://example.com"
  asc game-center activities versions default-image view --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityVersionsListCommand(d),
			newGameCenterActivityVersionsGetCommand(d),
			newGameCenterActivityVersionsCreateCommand(d),
			newGameCenterActivityVersionsUpdateCommand(d),
			newGameCenterActivityVersionDefaultImageCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityVersionsListCommand returns the activity versions list subcommand.
func newGameCenterActivityVersionsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center activities versions list --activity-id \"ACTIVITY_ID\"",
		ShortHelp:  "List versions for a Game Center activity.",
		LongHelp: `List versions for a Game Center activity.

Examples:
  asc game-center activities versions list --activity-id "ACTIVITY_ID"
  asc game-center activities versions list --activity-id "ACTIVITY_ID" --limit 50
  asc game-center activities versions list --activity-id "ACTIVITY_ID" --paginate`,
		ParentFlag:           "activity-id",
		ParentUsage:          "Game Center activity ID",
		ParentType:           "gameCenterActivities",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, activityId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterActivityVersions(ctx, activityId, asc.Limit[asc.GCActivityVersionsOption](limit), asc.NextURL[asc.GCActivityVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterActivityVersionsGetCommand returns the activity versions get subcommand.
func newGameCenterActivityVersionsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center activities versions view --id \"VERSION_ID\"",
		ShortHelp:  "View a Game Center activity version by ID.",
		LongHelp: `View a Game Center activity version by ID.

Examples:
  asc game-center activities versions view --id "VERSION_ID"`,
		IDUsage:            "Game Center activity version ID",
		IDType:             "gameCenterActivityVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterActivityVersion(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterActivityVersionsCreateCommand returns the activity versions create subcommand.
func newGameCenterActivityVersionsCreateCommand(d *shared.Deps) *ffcli.Command {
	var fallbackURL string
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center activities versions create --activity-id \"ACTIVITY_ID\"",
		ShortHelp:  "Create a Game Center activity version.",
		LongHelp: `Create a Game Center activity version.

Examples:
  asc game-center activities versions create --activity-id "ACTIVITY_ID"
  asc game-center activities versions create --activity-id "ACTIVITY_ID" --fallback-url "https://example.com"`,
		IDFlag:  "activity-id",
		IDUsage: "Game Center activity ID",
		IDType:  "gameCenterActivities",
		Fields: []shared.AttributeFlag{
			shared.StringFlag(&fallbackURL, "fallback-url", "Fallback URL"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterActivityVersion(ctx, id, fallbackURL)
		},
		Deps: d,
	})
}

// newGameCenterActivityVersionsUpdateCommand returns the activity versions update subcommand.
func newGameCenterActivityVersionsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var fallbackURL string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center activities versions update --id \"VERSION_ID\" --fallback-url \"https://example.com\"",
		ShortHelp:  "Update a Game Center activity version.",
		LongHelp: `Update a Game Center activity version.

Examples:
  asc game-center activities versions update --id "VERSION_ID" --fallback-url "https://example.com"`,
		IDUsage: "Game Center activity version ID",
		IDType:  "gameCenterActivityVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&fallbackURL, "fallback-url", "Fallback URL"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterActivityVersion(ctx, id, &fallbackURL)
		},
		Deps: d,
	})
}

// newGameCenterActivityLocalizationsCommand returns the activity localizations command group.
func newGameCenterActivityLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center activities localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activity localizations.",
		LongHelp: `Manage Game Center activity localizations.

Examples:
  asc game-center activities localizations list --version-id "VERSION_ID"
  asc game-center activities localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"
  asc game-center activities localizations update --id "LOC_ID" --name "New Name"
  asc game-center activities localizations delete --id "LOC_ID" --confirm
  asc game-center activities localizations image view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityLocalizationsListCommand(d),
			newGameCenterActivityLocalizationsGetCommand(d),
			newGameCenterActivityLocalizationsCreateCommand(d),
			newGameCenterActivityLocalizationsUpdateCommand(d),
			newGameCenterActivityLocalizationsDeleteCommand(d),
			newGameCenterActivityLocalizationImageCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityLocalizationsListCommand returns the activity localizations list subcommand.
func newGameCenterActivityLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center activities localizations list --version-id \"VERSION_ID\"",
		ShortHelp:  "List localizations for an activity version.",
		LongHelp: `List localizations for an activity version.

Examples:
  asc game-center activities localizations list --version-id "VERSION_ID"
  asc game-center activities localizations list --version-id "VERSION_ID" --limit 50
  asc game-center activities localizations list --version-id "VERSION_ID" --paginate`,
		ParentFlag:           "version-id",
		ParentUsage:          "Game Center activity version ID",
		ParentType:           "gameCenterActivityVersions",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, versionId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterActivityLocalizations(ctx, versionId, asc.Limit[asc.GCActivityLocalizationsOption](limit), asc.NextURL[asc.GCActivityLocalizationsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterActivityLocalizationsGetCommand returns the activity localizations get subcommand.
func newGameCenterActivityLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center activities localizations view --id \"LOCALIZATION_ID\"",
		ShortHelp:  "View an activity localization by ID.",
		LongHelp: `View an activity localization by ID.

Examples:
  asc game-center activities localizations view --id "LOCALIZATION_ID"`,
		IDUsage:            "Game Center activity localization ID",
		IDType:             "gameCenterActivityLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterActivityLocalization(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterActivityLocalizationsCreateCommand returns the activity localizations create subcommand.
func newGameCenterActivityLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterActivityLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center activities localizations create --version-id \"VERSION_ID\" --locale en-US --name \"Weekly\" --description \"Win weekly\"",
		ShortHelp:  "Create an activity localization.",
		LongHelp: `Create an activity localization.

Examples:
  asc game-center activities localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center activity version ID",
		IDType:  "gameCenterActivityVersions",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Localization locale (e.g., en-US)"),
			shared.RequiredStringFlag(&attrs.Name, "name", "Localized name"),
			shared.RequiredStringFlag(&attrs.Description, "description", "Localized description"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterActivityLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterActivityLocalizationsUpdateCommand returns the activity localizations update subcommand.
func newGameCenterActivityLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.GameCenterActivityLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center activities localizations update --id \"LOCALIZATION_ID\" [flags]",
		ShortHelp:  "Update an activity localization.",
		LongHelp: `Update an activity localization.

Examples:
  asc game-center activities localizations update --id "LOCALIZATION_ID" --name "New Name"`,
		IDUsage: "Game Center activity localization ID",
		IDType:  "gameCenterActivityLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Localized name"),
			shared.OptionalStringFlag(&attrs.Description, "description", "Localized description"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterActivityLocalization(ctx, id, attrs)
		},
		Deps: d,
	})
}

// newGameCenterActivityLocalizationsDeleteCommand returns the activity localizations delete subcommand.
func newGameCenterActivityLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center activities localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete an activity localization.",
		LongHelp: `Delete an activity localization.

Examples:
  asc game-center activities localizations delete --id "LOCALIZATION_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center activity localization ID",
		IDType:             "gameCenterActivityLocalizations",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterActivityLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterActivityImagesCommand returns the activity images command group.
func newGameCenterActivityImagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center activities images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activity images.",
		LongHelp: `Manage Game Center activity images. Images are attached to activity localizations, or to activity versions as the default image.

Examples:
  asc game-center activities images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center activities images upload --version-id "VERSION_ID" --file path/to/image.png
  asc game-center activities images view --id "IMAGE_ID"
  asc game-center activities images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityImagesUploadCommand(d),
			newGameCenterActivityImagesGetCommand(d),
			newGameCenterActivityImagesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityImagesUploadCommand returns the activity images upload subcommand.
func newGameCenterActivityImagesUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterActivityLocalizations", "Activity localization ID (exactly one of --localization-id or --version-id)")
	versionID := shared.BindResourceIDFlag(fs, "version-id", "gameCenterActivityVersions", "Activity version ID, to set the version's default image (exactly one of --localization-id or --version-id)")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center activities images upload (--localization-id \"LOCALIZATION_ID\" | --version-id \"VERSION_ID\") --file path/to/image.png",
		ShortHelp:  "Upload an image for an activity localization or version.",
		LongHelp: `Upload an image for an activity localization, or a default image for an activity version.

The upload process reserves an upload slot, uploads the image file, and commits the upload.

Examples:
  asc game-center activities images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center activities images upload --version-id "VERSION_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
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
				return fmt.Errorf("game-center activities images upload: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterActivityImage(requestCtx, locID, verID, file)
			if err != nil {
				return fmt.Errorf("game-center activities images upload: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterActivityImagesGetCommand returns the activity images get subcommand.
func newGameCenterActivityImagesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center activities images view --id \"IMAGE_ID\"",
		ShortHelp:  "View an activity image by ID.",
		LongHelp: `View an activity image by ID.

Examples:
  asc game-center activities images view --id "IMAGE_ID"`,
		IDUsage:            "Activity image ID",
		IDType:             "gameCenterActivityImages",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterActivityImage(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterActivityImagesDeleteCommand returns the activity images delete subcommand.
func newGameCenterActivityImagesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center activities images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete an activity image.",
		LongHelp: `Delete an activity image.

Examples:
  asc game-center activities images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Activity image ID",
		IDType:             "gameCenterActivityImages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterActivityImage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterActivityReleasesCommand returns the activity releases command group.
func newGameCenterActivityReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center activities releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activity releases.",
		LongHelp: `Manage Game Center activity releases. Releases are used to publish activity versions to live.

Examples:
  asc game-center activities releases list --app "APP_ID"
  asc game-center activities releases create --version-id "VERSION_ID"
  asc game-center activities releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityReleasesListCommand(d),
			newGameCenterActivityReleasesCreateCommand(d),
			newGameCenterActivityReleasesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityReleasesListCommand returns the activity releases list subcommand.
func newGameCenterActivityReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center activities releases list --app \"APP_ID\"",
		ShortHelp:  "List releases for Game Center activities.",
		LongHelp: `List releases for Game Center activities.

Examples:
  asc game-center activities releases list --app "APP_ID"
  asc game-center activities releases list --app "APP_ID" --limit 50
  asc game-center activities releases list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		LookupParent:         gameCenterDetailParent("game-center activities releases list"),
		FetchPage: func(ctx context.Context, client *asc.Client, gcDetailID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterActivityVersionReleases(ctx, gcDetailID, asc.Limit[asc.GCActivityVersionReleasesOption](limit), asc.NextURL[asc.GCActivityVersionReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterActivityReleasesCreateCommand returns the activity releases create subcommand.
func newGameCenterActivityReleasesCreateCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center activities releases create --version-id \"VERSION_ID\"",
		ShortHelp:  "Create a Game Center activity release.",
		LongHelp: `Create a Game Center activity release. This publishes the version to live.

Examples:
  asc game-center activities releases create --version-id "VERSION_ID"`,
		IDFlag:  "version-id",
		IDUsage: "Game Center activity version ID",
		IDType:  "gameCenterActivityVersions",
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateGameCenterActivityVersionRelease(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterActivityReleasesDeleteCommand returns the activity releases delete subcommand.
func newGameCenterActivityReleasesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center activities releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center activity release.",
		LongHelp: `Delete a Game Center activity release.

Examples:
  asc game-center activities releases delete --id "RELEASE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center activity release ID",
		IDType:             "gameCenterActivityVersionReleases",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterActivityVersionRelease(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterActivityLocalizationImageCommand returns the activity localization image command group.
func newGameCenterActivityLocalizationImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center activities localizations image view --id \"LOC_ID\"",
		ShortHelp:  "Get the image for an activity localization.",
		LongHelp: `Get the image for an activity localization.

Examples:
  asc game-center activities localizations image view --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityLocalizationImageGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityLocalizationImageGetCommand returns the activity localization image get subcommand.
func newGameCenterActivityLocalizationImageGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center activities localizations image view --id \"LOC_ID\"",
		ShortHelp:  "View an activity localization image.",
		LongHelp: `View an activity localization image.

Examples:
  asc game-center activities localizations image view --id "LOC_ID"`,
		IDUsage:            "Game Center activity localization ID",
		IDType:             "gameCenterActivityLocalizations",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterActivityLocalizationImage(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterActivityVersionDefaultImageCommand returns the activity version default image command group.
func newGameCenterActivityVersionDefaultImageCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("default-image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "default-image",
		ShortUsage: "asc game-center activities versions default-image view --id \"VERSION_ID\"",
		ShortHelp:  "Get the default image for an activity version.",
		LongHelp: `Get the default image for an activity version.

Examples:
  asc game-center activities versions default-image view --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterActivityVersionDefaultImageGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterActivityVersionDefaultImageGetCommand returns the activity version default image get subcommand.
func newGameCenterActivityVersionDefaultImageGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center activities versions default-image view --id \"VERSION_ID\"",
		ShortHelp:  "View a default image for an activity version.",
		LongHelp: `View a default image for an activity version.

Examples:
  asc game-center activities versions default-image view --id "VERSION_ID"`,
		IDUsage:            "Game Center activity version ID",
		IDType:             "gameCenterActivityVersions",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterActivityVersionDefaultImage(ctx, id)
		},
		Deps: d,
	})
}

func resourceDataList(resourceType asc.ResourceType, ids []string) []asc.ResourceData {
	if len(ids) == 0 {
		return nil
	}
	data := make([]asc.ResourceData, 0, len(ids))
	for _, id := range ids {
		data = append(data, asc.ResourceData{
			Type: resourceType,
			ID:   strings.TrimSpace(id),
		})
	}
	return data
}
