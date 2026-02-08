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

// GameCenterActivitiesCommand returns the activities command group.
func GameCenterActivitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("activities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "activities",
		ShortUsage: "asc game-center activities <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activities.",
		LongHelp: `Manage Game Center activities.

Examples:
  asc game-center activities list --app "APP_ID"
  asc game-center activities get --id "ACTIVITY_ID"
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly"
  asc game-center activities update --id "ACTIVITY_ID" --archived true
  asc game-center activities delete --id "ACTIVITY_ID" --confirm
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1,ACH_2"
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1,LB_2"
  asc game-center activities versions list --activity-id "ACTIVITY_ID"
  asc game-center activities localizations list --version-id "VERSION_ID"
  asc game-center activities localizations image get --id "LOC_ID"
  asc game-center activities versions default-image get --id "VERSION_ID"
  asc game-center activities images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center activities releases list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivitiesListCommand(),
			GameCenterActivitiesGetCommand(),
			GameCenterActivitiesCreateCommand(),
			GameCenterActivitiesUpdateCommand(),
			GameCenterActivitiesDeleteCommand(),
			GameCenterActivityAchievementsCommand(),
			GameCenterActivityLeaderboardsCommand(),
			GameCenterActivityVersionsCommand(),
			GameCenterActivityLocalizationsCommand(),
			GameCenterActivityImagesCommand(),
			GameCenterActivityReleasesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivitiesListCommand returns the activities list subcommand.
func GameCenterActivitiesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center activities list [flags]",
		ShortHelp:  "List Game Center activities for an app.",
		LongHelp: `List Game Center activities for an app.

Examples:
  asc game-center activities list --app "APP_ID"
  asc game-center activities list --app "APP_ID" --limit 50
  asc game-center activities list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center activities list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center activities list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center activities list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCActivitiesOption{
				asc.WithGCActivitiesLimit(*limit),
				asc.WithGCActivitiesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCActivitiesLimit(200))
				firstPage, err := client.GetGameCenterActivities(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center activities list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterActivities(ctx, gcDetailID, asc.WithGCActivitiesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center activities list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterActivities(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center activities list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivitiesGetCommand returns the activities get subcommand.
func GameCenterActivitiesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	activityID := fs.String("id", "", "Game Center activity ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center activities get --id \"ACTIVITY_ID\"",
		ShortHelp:  "Get a Game Center activity by ID.",
		LongHelp: `Get a Game Center activity by ID.

Examples:
  asc game-center activities get --id "ACTIVITY_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterActivity(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivitiesCreateCommand returns the activities create subcommand.
func GameCenterActivitiesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the activity")
	vendorID := fs.String("vendor-id", "", "Vendor identifier for the activity")
	playStyle := fs.String("play-style", "", "Play style (ASYNCHRONOUS, SYNCHRONOUS)")
	minPlayers := fs.Int("min-players", 0, "Minimum players count")
	maxPlayers := fs.Int("max-players", 0, "Maximum players count")
	supportsPartyCode := fs.String("supports-party-code", "", "Supports party code (true/false)")
	groupID := fs.String("group-id", "", "Game Center group ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center activities create --app \"APP_ID\" --reference-name \"Weekly\" --vendor-id \"com.example.weekly\"",
		ShortHelp:  "Create a Game Center activity.",
		LongHelp: `Create a Game Center activity.

Examples:
  asc game-center activities create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly"
  asc game-center activities create --group-id "GROUP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if group == "" && resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}

			vendor := strings.TrimSpace(*vendorID)
			if vendor == "" {
				fmt.Fprintln(os.Stderr, "Error: --vendor-id is required")
				return flag.ErrHelp
			}
			if group != "" && !strings.HasPrefix(vendor, "grp.") {
				fmt.Fprintln(os.Stderr, "Error: --vendor-id must start with \"grp.\" when using --group-id")
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
				val, err := parseBool(*supportsPartyCode, "--supports-party-code")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.SupportsPartyCode = &val
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center activities create: failed to get Game Center detail: %w", err)
				}
			}

			resp, err := client.CreateGameCenterActivity(requestCtx, gcDetailID, attrs, group)
			if err != nil {
				return fmt.Errorf("game-center activities create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivitiesUpdateCommand returns the activities update subcommand.
func GameCenterActivitiesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	activityID := fs.String("id", "", "Game Center activity ID")
	referenceName := fs.String("reference-name", "", "Reference name for the activity")
	playStyle := fs.String("play-style", "", "Play style (ASYNCHRONOUS, SYNCHRONOUS)")
	minPlayers := fs.Int("min-players", 0, "Minimum players count")
	maxPlayers := fs.Int("max-players", 0, "Maximum players count")
	supportsPartyCode := fs.String("supports-party-code", "", "Supports party code (true/false)")
	archived := fs.String("archived", "", "Archive the activity (true/false)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center activities update --id \"ACTIVITY_ID\" [flags]",
		ShortHelp:  "Update a Game Center activity.",
		LongHelp: `Update a Game Center activity.

Examples:
  asc game-center activities update --id "ACTIVITY_ID" --reference-name "New Name"
  asc game-center activities update --id "ACTIVITY_ID" --archived true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
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
				val, err := parseBool(*supportsPartyCode, "--supports-party-code")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.SupportsPartyCode = &val
				hasUpdate = true
			}
			if strings.TrimSpace(*archived) != "" {
				val, err := parseBool(*archived, "--archived")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Archived = &val
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterActivity(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center activities update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivitiesDeleteCommand returns the activities delete subcommand.
func GameCenterActivitiesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	activityID := fs.String("id", "", "Game Center activity ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center activities delete --id \"ACTIVITY_ID\" --confirm",
		ShortHelp:  "Delete a Game Center activity.",
		LongHelp: `Delete a Game Center activity.

Examples:
  asc game-center activities delete --id "ACTIVITY_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
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
				return fmt.Errorf("game-center activities delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterActivity(requestCtx, id); err != nil {
				return fmt.Errorf("game-center activities delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterActivityDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityAchievementsCommand returns the activity achievements command group.
func GameCenterActivityAchievementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("achievements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements",
		ShortUsage: "asc game-center activities achievements set --activity-id \"ACTIVITY_ID\" --ids \"ACH_1,ACH_2\"",
		ShortHelp:  "Manage activity achievements relationships.",
		LongHelp: `Manage activity achievements relationships.

Use --remove to remove relationships instead of adding.

Examples:
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1,ACH_2"
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1" --remove`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityAchievementsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityAchievementsSetCommand returns the activity achievements set subcommand.
func GameCenterActivityAchievementsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	activityID := fs.String("activity-id", "", "Game Center activity ID")
	ids := fs.String("ids", "", "Comma-separated achievement IDs")
	remove := fs.Bool("remove", false, "Remove relationships instead of adding")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center activities achievements set --activity-id \"ACTIVITY_ID\" --ids \"ACH_1,ACH_2\"",
		ShortHelp:  "Update activity achievements relationships.",
		LongHelp: `Update activity achievements relationships. By default, this adds relationships.

Examples:
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1,ACH_2"
  asc game-center activities achievements set --activity-id "ACTIVITY_ID" --ids "ACH_1" --remove`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --activity-id is required")
				return flag.ErrHelp
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --ids is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities achievements set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *remove {
				if err := client.RemoveGameCenterActivityAchievements(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities achievements set: failed to remove: %w", err)
				}
			} else {
				if err := client.AddGameCenterActivityAchievements(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities achievements set: failed to add: %w", err)
				}
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterAchievements, idsValue)}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityLeaderboardsCommand returns the activity leaderboards command group.
func GameCenterActivityLeaderboardsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboards", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards",
		ShortUsage: "asc game-center activities leaderboards set --activity-id \"ACTIVITY_ID\" --ids \"LB_1,LB_2\"",
		ShortHelp:  "Manage activity leaderboards relationships.",
		LongHelp: `Manage activity leaderboards relationships.

Use --remove to remove relationships instead of adding.

Examples:
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1,LB_2"
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1" --remove`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityLeaderboardsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityLeaderboardsSetCommand returns the activity leaderboards set subcommand.
func GameCenterActivityLeaderboardsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	activityID := fs.String("activity-id", "", "Game Center activity ID")
	ids := fs.String("ids", "", "Comma-separated leaderboard IDs")
	remove := fs.Bool("remove", false, "Remove relationships instead of adding")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center activities leaderboards set --activity-id \"ACTIVITY_ID\" --ids \"LB_1,LB_2\"",
		ShortHelp:  "Update activity leaderboards relationships.",
		LongHelp: `Update activity leaderboards relationships. By default, this adds relationships.

Examples:
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1,LB_2"
  asc game-center activities leaderboards set --activity-id "ACTIVITY_ID" --ids "LB_1" --remove`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --activity-id is required")
				return flag.ErrHelp
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --ids is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities leaderboards set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *remove {
				if err := client.RemoveGameCenterActivityLeaderboards(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities leaderboards set: failed to remove: %w", err)
				}
			} else {
				if err := client.AddGameCenterActivityLeaderboards(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center activities leaderboards set: failed to add: %w", err)
				}
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterLeaderboards, idsValue)}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityVersionsCommand returns the activity versions command group.
func GameCenterActivityVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center activities versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activity versions.",
		LongHelp: `Manage Game Center activity versions.

Examples:
  asc game-center activities versions list --activity-id "ACTIVITY_ID"
  asc game-center activities versions get --id "VERSION_ID"
  asc game-center activities versions create --activity-id "ACTIVITY_ID" --fallback-url "https://example.com"
  asc game-center activities versions update --id "VERSION_ID" --fallback-url "https://example.com"
  asc game-center activities versions default-image get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityVersionsListCommand(),
			GameCenterActivityVersionsGetCommand(),
			GameCenterActivityVersionsCreateCommand(),
			GameCenterActivityVersionsUpdateCommand(),
			GameCenterActivityVersionDefaultImageCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityVersionsListCommand returns the activity versions list subcommand.
func GameCenterActivityVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	activityID := fs.String("activity-id", "", "Game Center activity ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center activities versions list --activity-id \"ACTIVITY_ID\"",
		ShortHelp:  "List versions for a Game Center activity.",
		LongHelp: `List versions for a Game Center activity.

Examples:
  asc game-center activities versions list --activity-id "ACTIVITY_ID"
  asc game-center activities versions list --activity-id "ACTIVITY_ID" --limit 50
  asc game-center activities versions list --activity-id "ACTIVITY_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center activities versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center activities versions list: %w", err)
			}

			id := strings.TrimSpace(*activityID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --activity-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCActivityVersionsOption{
				asc.WithGCActivityVersionsLimit(*limit),
				asc.WithGCActivityVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCActivityVersionsLimit(200))
				firstPage, err := client.GetGameCenterActivityVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center activities versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterActivityVersions(ctx, id, asc.WithGCActivityVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center activities versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterActivityVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center activities versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityVersionsGetCommand returns the activity versions get subcommand.
func GameCenterActivityVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center activity version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center activities versions get --id \"VERSION_ID\"",
		ShortHelp:  "Get a Game Center activity version by ID.",
		LongHelp: `Get a Game Center activity version by ID.

Examples:
  asc game-center activities versions get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterActivityVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityVersionsCreateCommand returns the activity versions create subcommand.
func GameCenterActivityVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	activityID := fs.String("activity-id", "", "Game Center activity ID")
	fallbackURL := fs.String("fallback-url", "", "Fallback URL")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center activities versions create --activity-id \"ACTIVITY_ID\"",
		ShortHelp:  "Create a Game Center activity version.",
		LongHelp: `Create a Game Center activity version.

Examples:
  asc game-center activities versions create --activity-id "ACTIVITY_ID"
  asc game-center activities versions create --activity-id "ACTIVITY_ID" --fallback-url "https://example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*activityID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --activity-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterActivityVersion(requestCtx, id, strings.TrimSpace(*fallbackURL))
			if err != nil {
				return fmt.Errorf("game-center activities versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityVersionsUpdateCommand returns the activity versions update subcommand.
func GameCenterActivityVersionsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center activity version ID")
	fallbackURL := fs.String("fallback-url", "", "Fallback URL")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center activities versions update --id \"VERSION_ID\" --fallback-url \"https://example.com\"",
		ShortHelp:  "Update a Game Center activity version.",
		LongHelp: `Update a Game Center activity version.

Examples:
  asc game-center activities versions update --id "VERSION_ID" --fallback-url "https://example.com"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*fallbackURL) == "" {
				fmt.Fprintln(os.Stderr, "Error: --fallback-url is required")
				return flag.ErrHelp
			}

			value := strings.TrimSpace(*fallbackURL)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities versions update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterActivityVersion(requestCtx, id, &value)
			if err != nil {
				return fmt.Errorf("game-center activities versions update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityLocalizationsCommand returns the activity localizations command group.
func GameCenterActivityLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

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
  asc game-center activities localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityLocalizationsListCommand(),
			GameCenterActivityLocalizationsGetCommand(),
			GameCenterActivityLocalizationsCreateCommand(),
			GameCenterActivityLocalizationsUpdateCommand(),
			GameCenterActivityLocalizationsDeleteCommand(),
			GameCenterActivityLocalizationImageCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityLocalizationsListCommand returns the activity localizations list subcommand.
func GameCenterActivityLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center activity version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center activities localizations list --version-id \"VERSION_ID\"",
		ShortHelp:  "List localizations for an activity version.",
		LongHelp: `List localizations for an activity version.

Examples:
  asc game-center activities localizations list --version-id "VERSION_ID"
  asc game-center activities localizations list --version-id "VERSION_ID" --limit 50
  asc game-center activities localizations list --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center activities localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center activities localizations list: %w", err)
			}

			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCActivityLocalizationsOption{
				asc.WithGCActivityLocalizationsLimit(*limit),
				asc.WithGCActivityLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCActivityLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterActivityLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center activities localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterActivityLocalizations(ctx, id, asc.WithGCActivityLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center activities localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterActivityLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center activities localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityLocalizationsGetCommand returns the activity localizations get subcommand.
func GameCenterActivityLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center activity localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center activities localizations get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get an activity localization by ID.",
		LongHelp: `Get an activity localization by ID.

Examples:
  asc game-center activities localizations get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center activities localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterActivityLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityLocalizationsCreateCommand returns the activity localizations create subcommand.
func GameCenterActivityLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center activity version ID")
	locale := fs.String("locale", "", "Localization locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center activities localizations create --version-id \"VERSION_ID\" --locale en-US --name \"Weekly\" --description \"Win weekly\"",
		ShortHelp:  "Create an activity localization.",
		LongHelp: `Create an activity localization.

Examples:
  asc game-center activities localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			loc := strings.TrimSpace(*locale)
			if loc == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			descriptionValue := strings.TrimSpace(*description)
			if descriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --description is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterActivityLocalizationCreateAttributes{
				Locale:      loc,
				Name:        nameValue,
				Description: descriptionValue,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterActivityLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center activities localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityLocalizationsUpdateCommand returns the activity localizations update subcommand.
func GameCenterActivityLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center activity localization ID")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center activities localizations update --id \"LOCALIZATION_ID\" [flags]",
		ShortHelp:  "Update an activity localization.",
		LongHelp: `Update an activity localization.

Examples:
  asc game-center activities localizations update --id "LOCALIZATION_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterActivityLocalizationUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				value := strings.TrimSpace(*name)
				attrs.Name = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*description) != "" {
				value := strings.TrimSpace(*description)
				attrs.Description = &value
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterActivityLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center activities localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityLocalizationsDeleteCommand returns the activity localizations delete subcommand.
func GameCenterActivityLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center activity localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center activities localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete an activity localization.",
		LongHelp: `Delete an activity localization.

Examples:
  asc game-center activities localizations delete --id "LOCALIZATION_ID" --confirm`,
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
				return fmt.Errorf("game-center activities localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterActivityLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("game-center activities localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterActivityLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityImagesCommand returns the activity images command group.
func GameCenterActivityImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center activities images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center activity images.",
		LongHelp: `Manage Game Center activity images. Images are attached to activity localizations.

Examples:
  asc game-center activities images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center activities images get --id "IMAGE_ID"
  asc game-center activities images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityImagesUploadCommand(),
			GameCenterActivityImagesGetCommand(),
			GameCenterActivityImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityImagesUploadCommand returns the activity images upload subcommand.
func GameCenterActivityImagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Activity localization ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center activities images upload --localization-id \"LOCALIZATION_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for an activity localization.",
		LongHelp: `Upload an image for an activity localization.

The upload process reserves an upload slot, uploads the image file, and commits the upload.

Examples:
  asc game-center activities images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterActivityImage(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center activities images upload: %w", err)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityImagesGetCommand returns the activity images get subcommand.
func GameCenterActivityImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Activity image ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center activities images get --id \"IMAGE_ID\"",
		ShortHelp:  "Get an activity image by ID.",
		LongHelp: `Get an activity image by ID.

Examples:
  asc game-center activities images get --id "IMAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterActivityImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities images get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityImagesDeleteCommand returns the activity images delete subcommand.
func GameCenterActivityImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Activity image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center activities images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete an activity image.",
		LongHelp: `Delete an activity image.

Examples:
  asc game-center activities images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
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
				return fmt.Errorf("game-center activities images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterActivityImage(requestCtx, id); err != nil {
				return fmt.Errorf("game-center activities images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterActivityImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityReleasesCommand returns the activity releases command group.
func GameCenterActivityReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("releases", flag.ExitOnError)

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
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityReleasesListCommand(),
			GameCenterActivityReleasesCreateCommand(),
			GameCenterActivityReleasesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityReleasesListCommand returns the activity releases list subcommand.
func GameCenterActivityReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center activities releases list --app \"APP_ID\"",
		ShortHelp:  "List releases for Game Center activities.",
		LongHelp: `List releases for Game Center activities.

Examples:
  asc game-center activities releases list --app "APP_ID"
  asc game-center activities releases list --app "APP_ID" --limit 50
  asc game-center activities releases list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center activities releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center activities releases list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center activities releases list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCActivityVersionReleasesOption{
				asc.WithGCActivityVersionReleasesLimit(*limit),
				asc.WithGCActivityVersionReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCActivityVersionReleasesLimit(200))
				firstPage, err := client.GetGameCenterActivityVersionReleases(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center activities releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterActivityVersionReleases(ctx, gcDetailID, asc.WithGCActivityVersionReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center activities releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterActivityVersionReleases(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center activities releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityReleasesCreateCommand returns the activity releases create subcommand.
func GameCenterActivityReleasesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center activity version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center activities releases create --version-id \"VERSION_ID\"",
		ShortHelp:  "Create a Game Center activity release.",
		LongHelp: `Create a Game Center activity release. This publishes the version to live.

Examples:
  asc game-center activities releases create --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities releases create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterActivityVersionRelease(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities releases create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityReleasesDeleteCommand returns the activity releases delete subcommand.
func GameCenterActivityReleasesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	releaseID := fs.String("id", "", "Game Center activity release ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center activities releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center activity release.",
		LongHelp: `Delete a Game Center activity release.

Examples:
  asc game-center activities releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*releaseID)
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
				return fmt.Errorf("game-center activities releases delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterActivityVersionRelease(requestCtx, id); err != nil {
				return fmt.Errorf("game-center activities releases delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterActivityVersionReleaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterActivityLocalizationImageCommand returns the activity localization image command group.
func GameCenterActivityLocalizationImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center activities localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get the image for an activity localization.",
		LongHelp: `Get the image for an activity localization.

Examples:
  asc game-center activities localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityLocalizationImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityLocalizationImageGetCommand returns the activity localization image get subcommand.
func GameCenterActivityLocalizationImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center activity localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center activities localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get an activity localization image.",
		LongHelp: `Get an activity localization image.

Examples:
  asc game-center activities localizations image get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center activities localizations image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterActivityLocalizationImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities localizations image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterActivityVersionDefaultImageCommand returns the activity version default image command group.
func GameCenterActivityVersionDefaultImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("default-image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "default-image",
		ShortUsage: "asc game-center activities versions default-image get --id \"VERSION_ID\"",
		ShortHelp:  "Get the default image for an activity version.",
		LongHelp: `Get the default image for an activity version.

Examples:
  asc game-center activities versions default-image get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterActivityVersionDefaultImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterActivityVersionDefaultImageGetCommand returns the activity version default image get subcommand.
func GameCenterActivityVersionDefaultImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center activity version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center activities versions default-image get --id \"VERSION_ID\"",
		ShortHelp:  "Get a default image for an activity version.",
		LongHelp: `Get a default image for an activity version.

Examples:
  asc game-center activities versions default-image get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center activities versions default-image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterActivityVersionDefaultImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center activities versions default-image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
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
