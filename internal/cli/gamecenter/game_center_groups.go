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
  asc game-center groups get --id "GROUP_ID"
  asc game-center groups create --reference-name "Group 1"
  asc game-center groups update --id "GROUP_ID" --reference-name "New Name"
  asc game-center groups delete --id "GROUP_ID" --confirm
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2"
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2"
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID"
  asc game-center groups activities list --group-id "GROUP_ID"
  asc game-center groups challenges list --group-id "GROUP_ID"
  asc game-center groups challenges set --group-id "GROUP_ID" --ids "CH_1,CH_2"`,
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups list [flags]",
		ShortHelp:  "List Game Center groups.",
		LongHelp: `List Game Center groups.

Examples:
  asc game-center groups list --app "APP_ID"
  asc game-center groups list --app "APP_ID" --limit 50
  asc game-center groups list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center groups list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCGroupsOption{
				asc.WithGCGroupsLimit(*limit),
				asc.WithGCGroupsNextURL(*next),
			}
			if gcDetailID != "" {
				opts = append(opts, asc.WithGCGroupsGameCenterDetailIDs([]string{gcDetailID}))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCGroupsLimit(200))
				firstPage, err := client.GetGameCenterGroups(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center groups list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterGroups(ctx, asc.WithGCGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center groups list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterGroups(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("game-center groups list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupsGetCommand returns the groups get subcommand.
func GameCenterGroupsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	groupID := fs.String("id", "", "Game Center group ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center groups get --id \"GROUP_ID\"",
		ShortHelp:  "Get a Game Center group by ID.",
		LongHelp: `Get a Game Center group by ID.

Examples:
  asc game-center groups get --id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterGroup(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center groups get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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

	groupID := fs.String("id", "", "Game Center group ID")
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
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			if strings.TrimSpace(*referenceName) == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
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

	groupID := fs.String("id", "", "Game Center group ID")
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
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
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
		ShortUsage: "asc game-center groups achievements set --group-id \"GROUP_ID\" --ids \"ACH_1,ACH_2\"",
		ShortHelp:  "Manage group achievements relationships.",
		LongHelp: `Manage group achievements relationships.

Examples:
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2"`,
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	v2 := fs.Bool("v2", false, "Use v2 achievements endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups achievements list --group-id \"GROUP_ID\"",
		ShortHelp:  "List achievements for a Game Center group.",
		LongHelp: `List achievements for a Game Center group.

Examples:
  asc game-center groups achievements list --group-id "GROUP_ID"
  asc game-center groups achievements list --group-id "GROUP_ID" --limit 50
  asc game-center groups achievements list --group-id "GROUP_ID" --paginate
  asc game-center groups achievements list --group-id "GROUP_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups achievements list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups achievements list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups achievements list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementsOption{
				asc.WithGCAchievementsLimit(*limit),
				asc.WithGCAchievementsNextURL(*next),
			}

			fetch := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				if *v2 {
					return client.GetGameCenterGroupAchievementsV2(ctx, id, asc.WithGCAchievementsNextURL(nextURL))
				}
				return client.GetGameCenterGroupAchievements(ctx, id, asc.WithGCAchievementsNextURL(nextURL))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementsLimit(200))
				var firstPage *asc.GameCenterAchievementsResponse
				if *v2 {
					firstPage, err = client.GetGameCenterGroupAchievementsV2(requestCtx, id, paginateOpts...)
				} else {
					firstPage, err = client.GetGameCenterGroupAchievements(requestCtx, id, paginateOpts...)
				}
				if err != nil {
					return fmt.Errorf("game-center groups achievements list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, fetch)
				if err != nil {
					return fmt.Errorf("game-center groups achievements list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			var resp *asc.GameCenterAchievementsResponse
			if *v2 {
				resp, err = client.GetGameCenterGroupAchievementsV2(requestCtx, id, opts...)
			} else {
				resp, err = client.GetGameCenterGroupAchievements(requestCtx, id, opts...)
			}
			if err != nil {
				return fmt.Errorf("game-center groups achievements list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupAchievementsSetCommand returns the group achievements set subcommand.
func GameCenterGroupAchievementsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	ids := fs.String("ids", "", "Comma-separated achievement IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center groups achievements set --group-id \"GROUP_ID\" --ids \"ACH_1,ACH_2\"",
		ShortHelp:  "Replace group achievements relationships.",
		LongHelp: `Replace group achievements relationships.

Examples:
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2"
  asc game-center groups achievements set --group-id "GROUP_ID" --ids "ACH_1,ACH_2" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --ids is required")
				return flag.ErrHelp
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
		ShortUsage: "asc game-center groups leaderboards set --group-id \"GROUP_ID\" --ids \"LB_1,LB_2\"",
		ShortHelp:  "Manage group leaderboards relationships.",
		LongHelp: `Manage group leaderboards relationships.

Examples:
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2"`,
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups leaderboards list --group-id \"GROUP_ID\"",
		ShortHelp:  "List leaderboards for a Game Center group.",
		LongHelp: `List leaderboards for a Game Center group.

Examples:
  asc game-center groups leaderboards list --group-id "GROUP_ID"
  asc game-center groups leaderboards list --group-id "GROUP_ID" --limit 50
  asc game-center groups leaderboards list --group-id "GROUP_ID" --paginate
  asc game-center groups leaderboards list --group-id "GROUP_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups leaderboards list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups leaderboards list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups leaderboards list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardsOption{
				asc.WithGCLeaderboardsLimit(*limit),
				asc.WithGCLeaderboardsNextURL(*next),
			}

			fetch := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				if *v2 {
					return client.GetGameCenterGroupLeaderboardsV2(ctx, id, asc.WithGCLeaderboardsNextURL(nextURL))
				}
				return client.GetGameCenterGroupLeaderboards(ctx, id, asc.WithGCLeaderboardsNextURL(nextURL))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardsLimit(200))
				var firstPage *asc.GameCenterLeaderboardsResponse
				if *v2 {
					firstPage, err = client.GetGameCenterGroupLeaderboardsV2(requestCtx, id, paginateOpts...)
				} else {
					firstPage, err = client.GetGameCenterGroupLeaderboards(requestCtx, id, paginateOpts...)
				}
				if err != nil {
					return fmt.Errorf("game-center groups leaderboards list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, fetch)
				if err != nil {
					return fmt.Errorf("game-center groups leaderboards list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			var resp *asc.GameCenterLeaderboardsResponse
			if *v2 {
				resp, err = client.GetGameCenterGroupLeaderboardsV2(requestCtx, id, opts...)
			} else {
				resp, err = client.GetGameCenterGroupLeaderboards(requestCtx, id, opts...)
			}
			if err != nil {
				return fmt.Errorf("game-center groups leaderboards list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupLeaderboardsSetCommand returns the group leaderboards set subcommand.
func GameCenterGroupLeaderboardsSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	ids := fs.String("ids", "", "Comma-separated leaderboard IDs")
	v2 := fs.Bool("v2", false, "Use v2 relationships endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center groups leaderboards set --group-id \"GROUP_ID\" --ids \"LB_1,LB_2\"",
		ShortHelp:  "Replace group leaderboards relationships.",
		LongHelp: `Replace group leaderboards relationships.

Examples:
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2"
  asc game-center groups leaderboards set --group-id "GROUP_ID" --ids "LB_1,LB_2" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --ids is required")
				return flag.ErrHelp
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	v2 := fs.Bool("v2", false, "Use v2 leaderboard sets endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups leaderboard-sets list --group-id \"GROUP_ID\"",
		ShortHelp:  "List leaderboard sets for a Game Center group.",
		LongHelp: `List leaderboard sets for a Game Center group.

Examples:
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID"
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID" --limit 50
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID" --paginate
  asc game-center groups leaderboard-sets list --group-id "GROUP_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups leaderboard-sets list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups leaderboard-sets list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups leaderboard-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetsOption{
				asc.WithGCLeaderboardSetsLimit(*limit),
				asc.WithGCLeaderboardSetsNextURL(*next),
			}

			fetch := func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				if *v2 {
					return client.GetGameCenterGroupLeaderboardSetsV2(ctx, id, asc.WithGCLeaderboardSetsNextURL(nextURL))
				}
				return client.GetGameCenterGroupLeaderboardSets(ctx, id, asc.WithGCLeaderboardSetsNextURL(nextURL))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetsLimit(200))
				var firstPage *asc.GameCenterLeaderboardSetsResponse
				if *v2 {
					firstPage, err = client.GetGameCenterGroupLeaderboardSetsV2(requestCtx, id, paginateOpts...)
				} else {
					firstPage, err = client.GetGameCenterGroupLeaderboardSets(requestCtx, id, paginateOpts...)
				}
				if err != nil {
					return fmt.Errorf("game-center groups leaderboard-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, fetch)
				if err != nil {
					return fmt.Errorf("game-center groups leaderboard-sets list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			var resp *asc.GameCenterLeaderboardSetsResponse
			if *v2 {
				resp, err = client.GetGameCenterGroupLeaderboardSetsV2(requestCtx, id, opts...)
			} else {
				resp, err = client.GetGameCenterGroupLeaderboardSets(requestCtx, id, opts...)
			}
			if err != nil {
				return fmt.Errorf("game-center groups leaderboard-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups activities list --group-id \"GROUP_ID\"",
		ShortHelp:  "List activities for a Game Center group.",
		LongHelp: `List activities for a Game Center group.

Examples:
  asc game-center groups activities list --group-id "GROUP_ID"
  asc game-center groups activities list --group-id "GROUP_ID" --limit 50
  asc game-center groups activities list --group-id "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups activities list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups activities list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups activities list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCActivitiesOption{
				asc.WithGCActivitiesLimit(*limit),
				asc.WithGCActivitiesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCActivitiesLimit(200))
				firstPage, err := client.GetGameCenterGroupActivities(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center groups activities list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterGroupActivities(ctx, id, asc.WithGCActivitiesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center groups activities list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterGroupActivities(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center groups activities list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupChallengesCommand returns the group challenges command group.
func GameCenterGroupChallengesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("challenges", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "challenges",
		ShortUsage: "asc game-center groups challenges set --group-id \"GROUP_ID\" --ids \"CH_1,CH_2\"",
		ShortHelp:  "Manage group challenges relationships.",
		LongHelp: `Manage group challenges relationships.

Examples:
  asc game-center groups challenges list --group-id "GROUP_ID"
  asc game-center groups challenges set --group-id "GROUP_ID" --ids "CH_1,CH_2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterGroupChallengesListCommand(),
			GameCenterGroupChallengesSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterGroupChallengesListCommand returns the group challenges list subcommand.
func GameCenterGroupChallengesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups challenges list --group-id \"GROUP_ID\"",
		ShortHelp:  "List challenges for a Game Center group.",
		LongHelp: `List challenges for a Game Center group.

Examples:
  asc game-center groups challenges list --group-id "GROUP_ID"
  asc game-center groups challenges list --group-id "GROUP_ID" --limit 50
  asc game-center groups challenges list --group-id "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups challenges list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups challenges list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups challenges list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCChallengesOption{
				asc.WithGCChallengesLimit(*limit),
				asc.WithGCChallengesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCChallengesLimit(200))
				firstPage, err := client.GetGameCenterGroupChallenges(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center groups challenges list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterGroupChallenges(ctx, id, asc.WithGCChallengesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center groups challenges list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterGroupChallenges(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center groups challenges list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterGroupChallengesSetCommand returns the group challenges set subcommand.
func GameCenterGroupChallengesSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	ids := fs.String("ids", "", "Comma-separated challenge IDs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center groups challenges set --group-id \"GROUP_ID\" --ids \"CH_1,CH_2\"",
		ShortHelp:  "Replace group challenges relationships.",
		LongHelp: `Replace group challenges relationships.

Examples:
  asc game-center groups challenges set --group-id "GROUP_ID" --ids "CH_1,CH_2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*groupID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}
			idsValue := shared.SplitCSV(*ids)
			if len(idsValue) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --ids is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups challenges set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.UpdateGameCenterGroupChallenges(requestCtx, id, idsValue); err != nil {
				return fmt.Errorf("game-center groups challenges set: failed to update: %w", err)
			}

			result := &asc.LinkagesResponse{Data: resourceDataList(asc.ResourceTypeGameCenterChallenges, idsValue)}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
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
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center groups details list --group-id \"GROUP_ID\"",
		ShortHelp:  "List Game Center details for a group.",
		LongHelp: `List Game Center details for a group.

Examples:
  asc game-center groups details list --group-id "GROUP_ID"
  asc game-center groups details list --group-id "GROUP_ID" --limit 50
  asc game-center groups details list --group-id "GROUP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center groups details list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center groups details list: %w", err)
			}

			id := strings.TrimSpace(*groupID)
			nextURL := strings.TrimSpace(*next)
			if id == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center groups details list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCDetailsOption{
				asc.WithGCDetailsLimit(*limit),
				asc.WithGCDetailsNextURL(*next),
			}

			if *paginate {
				paginateOpts := []asc.GCDetailsOption{asc.WithGCDetailsNextURL(*next)}
				if nextURL == "" {
					paginateOpts = []asc.GCDetailsOption{asc.WithGCDetailsLimit(200)}
				}
				firstPage, err := client.GetGameCenterGroupGameCenterDetails(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center groups details list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterGroupGameCenterDetails(ctx, id, asc.WithGCDetailsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center groups details list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterGroupGameCenterDetails(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center groups details list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
