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
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		FlagSetName: "list",
		Name:        "list",
		ShortUsage:  "asc game-center leaderboard-sets members list --set-id \"SET_ID\"",
		ShortHelp:   "List leaderboards in a leaderboard set.",
		LongHelp: `List leaderboards in a leaderboard set.

Examples:
  asc game-center leaderboard-sets members list --set-id "SET_ID"
  asc game-center leaderboard-sets members list --set-id "SET_ID" --limit 50
  asc game-center leaderboard-sets members list --set-id "SET_ID" --paginate`,
		ParentFlag:           "set-id",
		ParentUsage:          "Game Center leaderboard set ID",
		ParentType:           "gameCenterLeaderboardSets",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		ErrorPrefix:          "game-center leaderboard-sets members list",
		FetchErrorPrefix:     "game-center leaderboard-sets members list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, setId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardSetMembers(ctx, setId, asc.WithGCLeaderboardSetMembersLimit(limit), asc.WithGCLeaderboardSetMembersNextURL(next))
		},
	})
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
				return shared.RequiredFlag("--set-id")
			}

			if !flagPassed(fs, "leaderboard-ids") {
				return shared.RequiredFlag("--leaderboard-ids")
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
