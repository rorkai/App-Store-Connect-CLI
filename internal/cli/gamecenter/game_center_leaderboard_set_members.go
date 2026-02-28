package gamecenter

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
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
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"`,
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

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
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
				return fmt.Errorf("game-center leaderboard-sets members list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets members list: %w", err)
			}

			id := strings.TrimSpace(*setID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
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

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	leaderboardIDs := fs.String("leaderboard-ids", "", "Comma-separated list of leaderboard IDs to set as members")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center leaderboard-sets members set --set-id \"SET_ID\" --leaderboard-ids \"id1,id2,id3\"",
		ShortHelp:  "Replace all leaderboard members in a leaderboard set.",
		LongHelp: `Replace all leaderboard members in a leaderboard set.

This command replaces ALL members of a leaderboard set with the specified leaderboard IDs.
To remove all members, pass an empty string for --leaderboard-ids.

Examples:
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids ""`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			// Parse leaderboard IDs from comma-separated string
			var ids []string
			if strings.TrimSpace(*leaderboardIDs) != "" {
				for leaderboardID := range strings.SplitSeq(*leaderboardIDs, ",") {
					trimmed := strings.TrimSpace(leaderboardID)
					if trimmed != "" {
						ids = append(ids, trimmed)
					}
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets members set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := updateLeaderboardSetMembers(requestCtx, client, id, ids); err != nil {
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

func updateLeaderboardSetMembers(ctx context.Context, client *asc.Client, setID string, ids []string) error {
	updateErr := client.UpdateGameCenterLeaderboardSetMembers(ctx, setID, ids)
	if updateErr == nil {
		return nil
	}

	// Apple may reject PATCH replace on currently empty sets. If we are adding
	// members and the set is empty, retry with POST add semantics.
	if len(ids) == 0 || !isConflict(updateErr) {
		return updateErr
	}

	currentMembers, err := client.GetGameCenterLeaderboardSetMembers(ctx, setID, asc.WithGCLeaderboardSetMembersLimit(1))
	if err != nil {
		return fmt.Errorf("replace request failed with conflict and empty-set check failed: %w", updateErr)
	}
	if len(currentMembers.Data) != 0 {
		return updateErr
	}

	if addErr := client.AddGameCenterLeaderboardSetMembers(ctx, setID, ids); addErr != nil {
		return fmt.Errorf("replace request failed because the set is empty, and add fallback failed: %w", addErr)
	}

	return nil
}

func isConflict(err error) bool {
	if errors.Is(err, asc.ErrConflict) {
		return true
	}
	var apiErr *asc.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}
