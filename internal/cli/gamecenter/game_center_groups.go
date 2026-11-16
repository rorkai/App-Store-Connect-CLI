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

// newGameCenterGroupsCommand returns the groups command group.
func newGameCenterGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("groups", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupsListCommand(d),
			newGameCenterGroupsGetCommand(d),
			newGameCenterGroupsCreateCommand(d),
			newGameCenterGroupsUpdateCommand(d),
			newGameCenterGroupsDeleteCommand(d),
			newGameCenterGroupAchievementsCommand(d),
			newGameCenterGroupLeaderboardsCommand(d),
			newGameCenterGroupLeaderboardSetsCommand(d),
			newGameCenterGroupActivitiesCommand(d),
			newGameCenterGroupChallengesCommand(d),
			newGameCenterGroupDetailsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupsListCommand returns the groups list subcommand.
func newGameCenterGroupsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups list [flags]",
		ShortHelp:  "List Game Center groups.",
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
		Deps: d,
	})
}

// newGameCenterGroupsGetCommand returns the groups get subcommand.
func newGameCenterGroupsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center groups view --id \"GROUP_ID\"",
		ShortHelp:  "View a Game Center group by ID.",
		LongHelp: `View a Game Center group by ID.

Examples:
  asc game-center groups view --id "GROUP_ID"`,
		IDUsage:            "Game Center group ID",
		IDType:             "gameCenterGroups",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterGroup(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterGroupsCreateCommand returns the groups create subcommand.
func newGameCenterGroupsCreateCommand(d *shared.Deps) *ffcli.Command {
	var referenceName *string
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Name:       "create",
		ShortUsage: "asc game-center groups create [flags]",
		ShortHelp:  "Create a Game Center group.",
		LongHelp: `Create a Game Center group.

Examples:
  asc game-center groups create --reference-name "Group 1"`,
		TopLevel: true,
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&referenceName, "reference-name", "Reference name for the group"),
		},
		Send: func(ctx context.Context, client *asc.Client, _ string) (any, error) {
			return client.CreateGameCenterGroup(ctx, referenceName)
		},
		Deps: d,
	})
}

// newGameCenterGroupsUpdateCommand returns the groups update subcommand.
func newGameCenterGroupsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var referenceName string
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Name:       "update",
		ShortUsage: "asc game-center groups update --id \"GROUP_ID\" [flags]",
		ShortHelp:  "Update a Game Center group.",
		LongHelp: `Update a Game Center group.

Examples:
  asc game-center groups update --id "GROUP_ID" --reference-name "New Name"`,
		IDUsage: "Game Center group ID",
		IDType:  "gameCenterGroups",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&referenceName, "reference-name", "Reference name for the group"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateGameCenterGroup(ctx, id, &referenceName)
		},
		Deps: d,
	})
}

// newGameCenterGroupsDeleteCommand returns the groups delete subcommand.
func newGameCenterGroupsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center groups delete --id \"GROUP_ID\" --confirm",
		ShortHelp:  "Delete a Game Center group.",
		LongHelp: `Delete a Game Center group.

Examples:
  asc game-center groups delete --id "GROUP_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center group ID",
		IDType:             "gameCenterGroups",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterGroup(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterGroupAchievementsCommand returns the group achievements command group.
func newGameCenterGroupAchievementsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("achievements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements",
		ShortUsage: "asc game-center groups achievements set --group-id \"GROUP_ID\" --ids \"ACH_1,ACH_2\" --confirm",
		ShortHelp:  "Manage group achievements relationships.",
		LongHelp: `Manage group achievements relationships.

Examples:
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupAchievementsListCommand(d),
			newGameCenterGroupAchievementsSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupAchievementsListCommand returns the group achievements list subcommand.
func newGameCenterGroupAchievementsListCommand(d *shared.Deps) *ffcli.Command {
	var v2 *bool

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups achievements list --group-id \"GROUP_ID\"",
		ShortHelp:  "List achievements for a Game Center group.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, groupID string, limit int, next string) (asc.PaginatedResponse, error) {
			if *v2 {
				return client.GetGameCenterGroupAchievementsV2(ctx, groupID, asc.Limit[asc.GCAchievementsOption](limit), asc.NextURL[asc.GCAchievementsOption](next))
			}
			return client.GetGameCenterGroupAchievements(ctx, groupID, asc.Limit[asc.GCAchievementsOption](limit), asc.NextURL[asc.GCAchievementsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterGroupAchievementsSetCommand returns the group achievements set subcommand.
func newGameCenterGroupAchievementsSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("set", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	ids := shared.BindOnceCSVFlag(fs, "ids", "Comma-separated achievement IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	confirm := fs.Bool("confirm", false, "Confirm replacing all relationships (required)")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return d.RequiredFlag("--group-id")
			}
			idsValue := shared.SplitCSV(ids.String())
			if len(idsValue) == 0 {
				return d.RequiredFlag("--ids")
			}
			if err := validateGameCenterReplacementConfirm(d, fs, *confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center groups achievements set: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterGroupLeaderboardsCommand returns the group leaderboards command group.
func newGameCenterGroupLeaderboardsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboards", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards",
		ShortUsage: "asc game-center groups leaderboards set --group-id \"GROUP_ID\" --ids \"LB_1,LB_2\" --confirm",
		ShortHelp:  "Manage group leaderboards relationships.",
		LongHelp: `Manage group leaderboards relationships.

Examples:
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupLeaderboardsListCommand(d),
			newGameCenterGroupLeaderboardsSetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupLeaderboardsListCommand returns the group leaderboards list subcommand.
func newGameCenterGroupLeaderboardsListCommand(d *shared.Deps) *ffcli.Command {
	var v2 *bool

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups leaderboards list --group-id \"GROUP_ID\"",
		ShortHelp:  "List leaderboards for a Game Center group.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, groupID string, limit int, next string) (asc.PaginatedResponse, error) {
			if *v2 {
				return client.GetGameCenterGroupLeaderboardsV2(ctx, groupID, asc.Limit[asc.GCLeaderboardsOption](limit), asc.NextURL[asc.GCLeaderboardsOption](next))
			}
			return client.GetGameCenterGroupLeaderboards(ctx, groupID, asc.Limit[asc.GCLeaderboardsOption](limit), asc.NextURL[asc.GCLeaderboardsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterGroupLeaderboardsSetCommand returns the group leaderboards set subcommand.
func newGameCenterGroupLeaderboardsSetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("set", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID")
	ids := shared.BindOnceCSVFlag(fs, "ids", "Comma-separated leaderboard IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	confirm := fs.Bool("confirm", false, "Confirm replacing all relationships (required)")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				return d.RequiredFlag("--group-id")
			}
			idsValue := shared.SplitCSV(ids.String())
			if len(idsValue) == 0 {
				return d.RequiredFlag("--ids")
			}
			if err := validateGameCenterReplacementConfirm(d, fs, *confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center groups leaderboards set: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterGroupLeaderboardSetsCommand returns the group leaderboard sets command group.
func newGameCenterGroupLeaderboardSetsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-sets",
		ShortUsage: "asc game-center groups leaderboard-sets list --group-id \"GROUP_ID\"",
		ShortHelp:  "Manage group leaderboard sets relationships.",
		LongHelp: `Manage group leaderboard sets relationships.

Examples:
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupLeaderboardSetsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupLeaderboardSetsListCommand returns the group leaderboard sets list subcommand.
func newGameCenterGroupLeaderboardSetsListCommand(d *shared.Deps) *ffcli.Command {
	var v2 *bool

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups leaderboard-sets list --group-id \"GROUP_ID\"",
		ShortHelp:  "List leaderboard sets for a Game Center group.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, groupID string, limit int, next string) (asc.PaginatedResponse, error) {
			if *v2 {
				return client.GetGameCenterGroupLeaderboardSetsV2(ctx, groupID, asc.Limit[asc.GCLeaderboardSetsOption](limit), asc.NextURL[asc.GCLeaderboardSetsOption](next))
			}
			return client.GetGameCenterGroupLeaderboardSets(ctx, groupID, asc.Limit[asc.GCLeaderboardSetsOption](limit), asc.NextURL[asc.GCLeaderboardSetsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterGroupActivitiesCommand returns the group activities command group.
func newGameCenterGroupActivitiesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("activities", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "activities",
		ShortUsage: "asc game-center groups activities list --group-id \"GROUP_ID\"",
		ShortHelp:  "Manage group activities relationships.",
		LongHelp: `Manage group activities relationships.

Examples:
  asc game-center groups activities list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupActivitiesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupActivitiesListCommand returns the group activities list subcommand.
func newGameCenterGroupActivitiesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups activities list --group-id \"GROUP_ID\"",
		ShortHelp:  "List activities for a Game Center group.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, groupId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterGroupActivities(ctx, groupId, asc.Limit[asc.GCActivitiesOption](limit), asc.NextURL[asc.GCActivitiesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterGroupChallengesCommand returns the group challenges command group.
func newGameCenterGroupChallengesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("challenges", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupChallengesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupChallengesListCommand returns the group challenges list subcommand.
func newGameCenterGroupChallengesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups challenges list --group-id \"GROUP_ID\"",
		ShortHelp:  "List challenges for a Game Center group.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, groupId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterGroupChallenges(ctx, groupId, asc.Limit[asc.GCChallengesOption](limit), asc.NextURL[asc.GCChallengesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterGroupDetailsCommand returns the group details command group.
func newGameCenterGroupDetailsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "details",
		ShortUsage: "asc game-center groups details list --group-id \"GROUP_ID\"",
		ShortHelp:  "List Game Center details for a group.",
		LongHelp: `List Game Center details for a group.

Examples:
  asc game-center groups details list --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterGroupDetailsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterGroupDetailsListCommand returns the group details list subcommand.
func newGameCenterGroupDetailsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center groups details list --group-id \"GROUP_ID\"",
		ShortHelp:  "List Game Center details for a group.",
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
		FetchPage: func(ctx context.Context, client *asc.Client, groupId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterGroupGameCenterDetails(ctx, groupId, asc.Limit[asc.GCDetailsOption](limit), asc.NextURL[asc.GCDetailsOption](next))
		},
		Deps: d,
	})
}
