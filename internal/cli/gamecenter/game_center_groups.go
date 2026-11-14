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

// GameCenterGroupsCommand returns the groups command group.
func GameCenterGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "groups",
		ShortUsage: "asc game-center groups <subcommand> [flags]",
		ShortHelp:  "Manage Game Center groups.",
		LongHelp: `Manage Game Center groups.

Examples:
  asc game-center groups list --app "APP_ID"
  asc game-center groups view --id "GROUP_ID"
  asc game-center groups create --reference-name "Group 1"
  asc game-center groups update --id "GROUP_ID" --reference-name "New Name"
  asc game-center groups delete --id "GROUP_ID" --confirm
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2" --confirm
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2" --confirm
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID"
  asc game-center groups activities list --group-id "GROUP_ID"
  asc game-center groups challenges list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupsListCommand(),
			GameCenterGroupsGetCommand(),
			GameCenterGroupsCreateCommand(),
			GameCenterGroupsUpdateCommand(),
			GameCenterGroupsDeleteCommand(),
			GameCenterGroupAchievementsCommand(),
			GameCenterGroupLeaderboardsCommand(),
			GameCenterGroupLeaderboardSetsCommand(),
			GameCenterGroupActivitiesCommand(),
			GameCenterGroupChallengesCommand(),
			GameCenterGroupDetailsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupsListCommand returns the groups list subcommand.
func GameCenterGroupsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups list [flags]",
		ShortHelp:   "List Game Center groups.",
		LongHelp: `List Game Center groups.

Examples:
  asc game-center groups list --app "APP_ID"
  asc game-center groups list --app "APP_ID" --limit 50
  asc game-center groups list --app "APP_ID" --paginate`,
		ParentFlag:           "app",
		ParentUsage:          "App Store Connect app ID (or ASC_APP_ID env)",
		ResolveApp:           true,
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center groups list",
		FetchErrorPrefix:     "game-center groups list: failed to fetch",
		LookupParent:         gameCenterDetailParent("game-center groups list"),
		FetchPage: func(ctx context.Context, client *asc.Client, gcDetailID string, limit int, next string) (asc.PaginatedResponse, error) {
			opts := []asc.GCGroupsOption{
				asc.Limit[asc.GCGroupsOption](limit),
				asc.NextURL[asc.GCGroupsOption](next),
			}
			if gcDetailID != "" {
				opts = append(opts, asc.WithGCGroupsGameCenterDetailIDs([]string{gcDetailID}))
			}
			return client.GetGameCenterGroups(ctx, opts...)
		},
	})
}

// GameCenterGroupsGetCommand returns the groups get subcommand.
func GameCenterGroupsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc game-center groups view --id \"GROUP_ID\"",
		ShortHelp:   "View a Game Center group by ID.",
		LongHelp: `View a Game Center group by ID.

Examples:
  asc game-center groups view --id "GROUP_ID"`,
		IDUsage:            "Game Center group ID",
		IDType:             "gameCenterGroups",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center groups view",
		FetchErrorPrefix:   "game-center groups view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterGroup(ctx, id)
		},
	})
}

// GameCenterGroupsCreateCommand returns the groups create subcommand.
func GameCenterGroupsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	referenceName := fs.String("reference-name", "", "Reference name for the group")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center groups create [flags]",
		ShortHelp:  "Create a Game Center group.",
		LongHelp: `Create a Game Center group.

Examples:
  asc game-center groups create --reference-name "Group 1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			var ref *string
			if strings.TrimSpace(*referenceName) != "" {
				value := strings.TrimSpace(*referenceName)
				ref = &value
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterGroup(requestCtx, ref)
			if err != nil {
				return fmt.Errorf("game-center groups create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupsUpdateCommand returns the groups update subcommand.
func GameCenterGroupsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "id", "gameCenterGroups", "Game Center group ID")
	referenceName := fs.String("reference-name", "", "Reference name for the group")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center groups update --id \"GROUP_ID\" [flags]",
		ShortHelp:  "Update a Game Center group.",
		LongHelp: `Update a Game Center group.

Examples:
  asc game-center groups update --id "GROUP_ID" --reference-name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return shared.RequiredFlag("--id")
			}

			if strings.TrimSpace(*referenceName) == "" {
				return shared.RequiredFlag("--reference-name")
			}
			value := strings.TrimSpace(*referenceName)

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterGroup(requestCtx, id, &value)
			if err != nil {
				return fmt.Errorf("game-center groups update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupsDeleteCommand returns the groups delete subcommand.
func GameCenterGroupsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "id", "gameCenterGroups", "Game Center group ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:  "Delete a Game Center group.",
		LongHelp: `Delete a Game Center group.

Examples:
  asc game-center groups delete --id "GROUP_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return shared.RequiredFlag("--id")
			}
			if !*confirm {
				return shared.RequiredFlag("--confirm")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterGroup(requestCtx, id); err != nil {
				return fmt.Errorf("game-center groups delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterGroupDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupAchievementsCommand returns the group achievements command group.
func GameCenterGroupAchievementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("achievements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements",
		ShortUsage: "asc game-center groups achievements set --group-id \"GROUP_ID\" --ids \"ACH_1,ACH_2\" --confirm",
		ShortHelp:  "Manage group achievements relationships.",
		LongHelp: `Manage group achievements relationships.

Examples:
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupAchievementsListCommand(),
			GameCenterGroupAchievementsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupAchievementsListCommand returns the group achievements list subcommand.
func GameCenterGroupAchievementsListCommand() *ffcli.Command {
	var v2 *bool

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups achievements list --group-id \"GROUP_ID\"",
		ShortHelp:   "List achievements for a Game Center group.",
		LongHelp: `List achievements for a Game Center group.

Examples:
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements list --group-id "GROUP_ID" --limit 50
  asc game-center groups achievements list --group-id "GROUP_ID" --paginate
  asc game-center groups achievements list --group-id "GROUP_ID" --v2`,
		ParentFlag:           "group-id",
		ParentUsage:          "Game Center group ID",
		ParentType:           "gameCenterGroups",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Flags: func(fs *flag.FlagSet) func() error {
			v2 = fs.Bool("v2", false, "Use v2 achievements endpoint")
			return nil
		},
		ErrorPrefix:      "game-center groups achievements list",
		FetchErrorPrefix: "game-center groups achievements list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, groupID string, limit int, next string) (asc.PaginatedResponse, error) {
			if *v2 {
				return client.GetGameCenterGroupAchievementsV2(ctx, groupID, asc.Limit[asc.GCAchievementsOption](limit), asc.NextURL[asc.GCAchievementsOption](next))
			}
			return client.GetGameCenterGroupAchievements(ctx, groupID, asc.Limit[asc.GCAchievementsOption](limit), asc.NextURL[asc.GCAchievementsOption](next))
		},
	})
}

// GameCenterGroupAchievementsSetCommand returns the group achievements set subcommand.
func GameCenterGroupAchievementsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	ids := shared.BindOnceCSVFlag(fs, "ids", "Comma-separated achievement IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	confirm := fs.Bool("confirm", false, "Confirm replacing all relationships (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center groups achievements set --group-id \"GROUP_ID\" --ids \"ACH_1,ACH_2\" --confirm",
		ShortHelp:  "Replace group achievements relationships.",
		LongHelp: `Replace group achievements relationships.

Because replacement can remove existing relationships, --confirm is required.

Examples:
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2" --confirm
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2" --v2 --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return shared.RequiredFlag("--group-id")
			}
			idsValue := shared.SplitCSV(ids.String())
			if len(idsValue) == 0 {
				return shared.RequiredFlag("--ids")
			}
			if err := validateGameCenterReplacementConfirm(fs, *confirm); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups achievements set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *v2 {
				if err := client.UpdateGameCenterGroupAchievementsV2(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center groups achievements set: failed to update: %w", err)
				}
			} else {
				if err := client.UpdateGameCenterGroupAchievements(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center groups achievements set: failed to update: %w", err)
				}
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterAchievements, idsValue)}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupLeaderboardsCommand returns the group leaderboards command group.
func GameCenterGroupLeaderboardsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboards", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards",
		ShortUsage: "asc game-center groups leaderboards set --group-id \"GROUP_ID\" --ids \"LB_1,LB_2\" --confirm",
		ShortHelp:  "Manage group leaderboards relationships.",
		LongHelp: `Manage group leaderboards relationships.

Examples:
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupLeaderboardsListCommand(),
			GameCenterGroupLeaderboardsSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupLeaderboardsListCommand returns the group leaderboards list subcommand.
func GameCenterGroupLeaderboardsListCommand() *ffcli.Command {
	var v2 *bool

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups leaderboards list --group-id \"GROUP_ID\"",
		ShortHelp:   "List leaderboards for a Game Center group.",
		LongHelp: `List leaderboards for a Game Center group.

Examples:
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards list --group-id "GROUP_ID" --limit 50
  asc game-center groups leaderboards list --group-id "GROUP_ID" --paginate
  asc game-center groups leaderboards list --group-id "GROUP_ID" --v2`,
		ParentFlag:           "group-id",
		ParentUsage:          "Game Center group ID",
		ParentType:           "gameCenterGroups",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Flags: func(fs *flag.FlagSet) func() error {
			v2 = fs.Bool("v2", false, "Use v2 leaderboards endpoint")
			return nil
		},
		ErrorPrefix:      "game-center groups leaderboards list",
		FetchErrorPrefix: "game-center groups leaderboards list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, groupID string, limit int, next string) (asc.PaginatedResponse, error) {
			if *v2 {
				return client.GetGameCenterGroupLeaderboardsV2(ctx, groupID, asc.Limit[asc.GCLeaderboardsOption](limit), asc.NextURL[asc.GCLeaderboardsOption](next))
			}
			return client.GetGameCenterGroupLeaderboards(ctx, groupID, asc.Limit[asc.GCLeaderboardsOption](limit), asc.NextURL[asc.GCLeaderboardsOption](next))
		},
	})
}

// GameCenterGroupLeaderboardsSetCommand returns the group leaderboards set subcommand.
func GameCenterGroupLeaderboardsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	ids := shared.BindOnceCSVFlag(fs, "ids", "Comma-separated leaderboard IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	confirm := fs.Bool("confirm", false, "Confirm replacing all relationships (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center groups leaderboards set --group-id \"GROUP_ID\" --ids \"LB_1,LB_2\" --confirm",
		ShortHelp:  "Replace group leaderboards relationships.",
		LongHelp: `Replace group leaderboards relationships.

Because replacement can remove existing relationships, --confirm is required.

Examples:
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2" --confirm
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2" --v2 --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return shared.RequiredFlag("--group-id")
			}
			idsValue := shared.SplitCSV(ids.String())
			if len(idsValue) == 0 {
				return shared.RequiredFlag("--ids")
			}
			if err := validateGameCenterReplacementConfirm(fs, *confirm); err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups leaderboards set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *v2 {
				if err := client.UpdateGameCenterGroupLeaderboardsV2(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center groups leaderboards set: failed to update: %w", err)
				}
			} else {
				if err := client.UpdateGameCenterGroupLeaderboards(requestCtx, id, idsValue); err != nil {
					return fmt.Errorf("game-center groups leaderboards set: failed to update: %w", err)
				}
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterLeaderboards, idsValue)}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupLeaderboardSetsCommand returns the group leaderboard sets command group.
func GameCenterGroupLeaderboardSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-sets",
		ShortUsage: "asc game-center groups leaderboard-sets list --group-id \"GROUP_ID\"",
		ShortHelp:  "Manage group leaderboard sets relationships.",
		LongHelp: `Manage group leaderboard sets relationships.

Examples:
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupLeaderboardSetsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupLeaderboardSetsListCommand returns the group leaderboard sets list subcommand.
func GameCenterGroupLeaderboardSetsListCommand() *ffcli.Command {
	var v2 *bool

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups leaderboard-sets list --group-id \"GROUP_ID\"",
		ShortHelp:   "List leaderboard sets for a Game Center group.",
		LongHelp: `List leaderboard sets for a Game Center group.

Examples:
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID"
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID" --limit 50
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID" --paginate
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID" --v2`,
		ParentFlag:           "group-id",
		ParentUsage:          "Game Center group ID",
		ParentType:           "gameCenterGroups",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		Flags: func(fs *flag.FlagSet) func() error {
			v2 = fs.Bool("v2", false, "Use v2 leaderboard sets endpoint")
			return nil
		},
		ErrorPrefix:      "game-center groups leaderboard-sets list",
		FetchErrorPrefix: "game-center groups leaderboard-sets list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, groupID string, limit int, next string) (asc.PaginatedResponse, error) {
			if *v2 {
				return client.GetGameCenterGroupLeaderboardSetsV2(ctx, groupID, asc.Limit[asc.GCLeaderboardSetsOption](limit), asc.NextURL[asc.GCLeaderboardSetsOption](next))
			}
			return client.GetGameCenterGroupLeaderboardSets(ctx, groupID, asc.Limit[asc.GCLeaderboardSetsOption](limit), asc.NextURL[asc.GCLeaderboardSetsOption](next))
		},
	})
}

// GameCenterGroupActivitiesCommand returns the group activities command group.
func GameCenterGroupActivitiesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("activities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "activities",
		ShortUsage: "asc game-center groups activities list --group-id \"GROUP_ID\"",
		ShortHelp:  "Manage group activities relationships.",
		LongHelp: `Manage group activities relationships.

Examples:
  asc game-center groups activities list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupActivitiesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupActivitiesListCommand returns the group activities list subcommand.
func GameCenterGroupActivitiesListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups activities list --group-id \"GROUP_ID\"",
		ShortHelp:   "List activities for a Game Center group.",
		LongHelp: `List activities for a Game Center group.

Examples:
  asc game-center groups activities list --group-id "GROUP_ID"
  asc game-center groups activities list --group-id "GROUP_ID" --limit 50
  asc game-center groups activities list --group-id "GROUP_ID" --paginate`,
		ParentFlag:           "group-id",
		ParentUsage:          "Game Center group ID",
		ParentType:           "gameCenterGroups",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center groups activities list",
		FetchErrorPrefix:     "game-center groups activities list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, groupId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterGroupActivities(ctx, groupId, asc.Limit[asc.GCActivitiesOption](limit), asc.NextURL[asc.GCActivitiesOption](next))
		},
	})
}

// GameCenterGroupChallengesCommand returns the group challenges command group.
func GameCenterGroupChallengesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("challenges", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "challenges",
		ShortUsage: "asc game-center groups challenges list --group-id \"GROUP_ID\"",
		ShortHelp:  "List group challenges relationships.",
		LongHelp: `List group challenges relationships.

App Store Connect exposes a group's challenge relationships as read-only. To add
a challenge to a group, create it with:
  asc game-center challenges create --group-id "GROUP_ID" --reference-name "Weekly" --vendor-id "grp.com.example.weekly" --leaderboard-id "LEADERBOARD_ID"

Examples:
  asc game-center groups challenges list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupChallengesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupChallengesListCommand returns the group challenges list subcommand.
func GameCenterGroupChallengesListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups challenges list --group-id \"GROUP_ID\"",
		ShortHelp:   "List challenges for a Game Center group.",
		LongHelp: `List challenges for a Game Center group.

Examples:
  asc game-center groups challenges list --group-id "GROUP_ID"
  asc game-center groups challenges list --group-id "GROUP_ID" --limit 50
  asc game-center groups challenges list --group-id "GROUP_ID" --paginate`,
		ParentFlag:           "group-id",
		ParentUsage:          "Game Center group ID",
		ParentType:           "gameCenterGroups",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center groups challenges list",
		FetchErrorPrefix:     "game-center groups challenges list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, groupId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterGroupChallenges(ctx, groupId, asc.Limit[asc.GCChallengesOption](limit), asc.NextURL[asc.GCChallengesOption](next))
		},
	})
}

// GameCenterGroupDetailsCommand returns the group details command group.
func GameCenterGroupDetailsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "details",
		ShortUsage: "asc game-center groups details list --group-id \"GROUP_ID\"",
		ShortHelp:  "List Game Center details for a group.",
		LongHelp: `List Game Center details for a group.

Examples:
  asc game-center groups details list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupDetailsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupDetailsListCommand returns the group details list subcommand.
func GameCenterGroupDetailsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center groups details list --group-id \"GROUP_ID\"",
		ShortHelp:   "List Game Center details for a group.",
		LongHelp: `List Game Center details for a group.

Examples:
  asc game-center groups details list --group-id "GROUP_ID"
  asc game-center groups details list --group-id "GROUP_ID" --limit 50
  asc game-center groups details list --group-id "GROUP_ID" --paginate`,
		ParentFlag:           "group-id",
		ParentUsage:          "Game Center group ID",
		ParentType:           "gameCenterGroups",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center groups details list",
		FetchErrorPrefix:     "game-center groups details list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, groupId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterGroupGameCenterDetails(ctx, groupId, asc.Limit[asc.GCDetailsOption](limit), asc.NextURL[asc.GCDetailsOption](next))
		},
	})
}
