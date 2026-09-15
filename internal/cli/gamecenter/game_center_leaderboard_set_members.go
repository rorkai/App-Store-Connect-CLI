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

// GameCenterLeaderboardSetMembersCommand returns the leaderboard set members command group.
func GameCenterLeaderboardSetMembersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("members", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "members",
		ShortUsage: "asc game-center leaderboard-sets members <subcommand> [flags]",
		ShortHelp:  "Manage leaderboard set members.",
		LongHelp: `Manage leaderboard set members. Members are the leaderboards that belong to a leaderboard set.

Examples:
  asc game-center leaderboard-sets members list --set-id "SET_ID"
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetMembersListCommand(),
			GameCenterLeaderboardSetMembersSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetMembersListCommand returns the members list subcommand.
func GameCenterLeaderboardSetMembersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	setID := shared.BindResourceIDFlag(fs, "set-id", "gameCenterLeaderboardSets", "Game Center leaderboard set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets members list --set-id \"SET_ID\"",
		ShortHelp:  "List leaderboards in a leaderboard set.",
		LongHelp: `List leaderboards in a leaderboard set.

Examples:
  asc game-center leaderboard-sets members list --set-id "SET_ID"
  asc game-center leaderboard-sets members list --set-id "SET_ID" --limit 50
  asc game-center leaderboard-sets members list --set-id "SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("game-center leaderboard-sets members list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("game-center leaderboard-sets members list: %v", err)
			}

			id := strings.TrimSpace(*setID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return shared.MissingRequiredUsageError("--set-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets members list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetMembersOption{
				asc.WithGCLeaderboardSetMembersLimit(*limit),
				asc.WithGCLeaderboardSetMembersNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetMembersLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetMembers(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets members list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetMembers(ctx, id, asc.WithGCLeaderboardSetMembersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets members list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetMembers(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets members list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetMembersSetCommand returns the members set subcommand.
func GameCenterLeaderboardSetMembersSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	setID := shared.BindResourceIDFlag(fs, "set-id", "gameCenterLeaderboardSets", "Game Center leaderboard set ID")
	leaderboardIDs := shared.BindOnceCSVFlag(fs, "leaderboard-ids", "Comma-separated list of leaderboard IDs to set as members")
	confirm := fs.Bool("confirm", false, "Confirm replacing all members (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center leaderboard-sets members set --set-id \"SET_ID\" --leaderboard-ids \"id1,id2,id3\" --confirm",
		ShortHelp:  "Replace all leaderboard members in a leaderboard set.",
		LongHelp: `Replace all leaderboard members in a leaderboard set.

This command replaces ALL members of a leaderboard set with the specified leaderboard IDs.
Because replacement can remove existing members, --confirm is required.
To remove all members, pass an empty string for --leaderboard-ids with --confirm.

Examples:
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3" --confirm
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return shared.MissingRequiredUsageError("--set-id")
			}

			leaderboardIDsProvided := false
			fs.Visit(func(flag *flag.Flag) {
				if flag.Name == "leaderboard-ids" {
					leaderboardIDsProvided = true
				}
			})
			if !leaderboardIDsProvided {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-ids is required")
				return shared.MissingRequiredUsageError("--leaderboard-ids")
			}

			if err := validateGameCenterReplacementConfirm(fs, *confirm); err != nil {
				return err
			}

			ids := shared.SplitUniqueCSV(leaderboardIDs.String())

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets members set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.SetGameCenterLeaderboardSetMembers(requestCtx, id, ids); err != nil {
				return fmt.Errorf("game-center leaderboard-sets members set: failed to update: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetMembersUpdateResult{
				SetID:       id,
				MemberCount: len(ids),
				MemberIDs:   ids,
				Updated:     true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
