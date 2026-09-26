package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// GameCenterScoreModerationsCommand returns the leaderboard score moderation group.
func GameCenterScoreModerationsCommand() *ffcli.Command {
	return &ffcli.Command{Name: "score-moderations", ShortUsage: "asc game-center leaderboards v2 score-moderations <subcommand> [flags]", ShortHelp: "List submitted scores and manage their blocked status.", FlagSet: flag.NewFlagSet("score-moderations", flag.ExitOnError), UsageFunc: shared.DefaultUsageFunc, Subcommands: []*ffcli.Command{gameCenterScoreModerationsListCommand(), gameCenterScoreModerationsUpdateCommand()}, Exec: func(context.Context, []string) error { return flag.ErrHelp }}
}

func gameCenterScoreModerationsListCommand() *ffcli.Command {
	const command = "game-center leaderboards v2 score-moderations list"
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Game Center leaderboard ID")
	var blocked shared.OptionalBool
	fs.Var(&blocked, "exists-blocked", "Value for Apple's exists[blocked] filter (true/false)")
	fields := fs.String("fields", "", "Fields: rank, score, submittedDate, blocked, preReleased, context, challengeIds, player")
	playerFields := fs.String("player-fields", "", "Player fields: nickname, blocked, bundleId (requires --include player)")
	include := fs.String("include", "", "Relationships to include: player")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "list", ShortUsage: "asc " + command + " --leaderboard-id ID [flags]", ShortHelp: "List scores submitted to a v2 leaderboard.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		provided := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
		if provided["limit"] && (*limit < 1 || *limit > 200) {
			return shared.UsageError(command + ": --limit must be between 1 and 200")
		}
		nextURL := strings.TrimSpace(*next)
		if err := shared.ValidateNextURL(nextURL); err != nil {
			return shared.UsageErrorf("%s: %v", command, err)
		}
		if err := shared.RejectNextFlagConflicts(fs, nextURL, command, "leaderboard-id", "exists-blocked", "fields", "player-fields", "include", "limit"); err != nil {
			return err
		}
		leaderboardID := strings.TrimSpace(*id)
		if leaderboardID == "" && nextURL == "" {
			return shared.UsageError(command + ": --leaderboard-id is required")
		}
		normalize := func(raw, name string, allowed []string) ([]string, error) {
			values, err := shared.NormalizeSelection(raw, allowed, "--"+name)
			if err != nil {
				return nil, shared.UsageError(err.Error())
			}
			if provided[name] && len(values) == 0 {
				return nil, shared.UsageErrorf("--%s must not be empty", name)
			}
			return values, nil
		}
		selected, err := normalize(*fields, "fields", []string{"rank", "score", "submittedDate", "blocked", "preReleased", "context", "challengeIds", "player"})
		if err != nil {
			return err
		}
		players, err := normalize(*playerFields, "player-fields", []string{"nickname", "blocked", "bundleId"})
		if err != nil {
			return err
		}
		includes, err := normalize(*include, "include", []string{"player"})
		if err != nil {
			return err
		}
		if len(players) > 0 && !slices.Contains(includes, "player") {
			return shared.UsageError("--player-fields requires --include player")
		}
		if slices.Contains(includes, "player") && len(selected) > 0 && !slices.Contains(selected, "player") {
			selected = append(selected, "player")
		}
		query := asc.GCScoreModerationsQuery{Fields: selected, PlayerFields: players, Include: includes, Limit: *limit, NextURL: nextURL}
		if blocked.IsSet() {
			value := blocked.Value()
			query.ExistsBlocked = &value
		}
		client, err := shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		fetch := func(ctx context.Context, q asc.GCScoreModerationsQuery) (*asc.GameCenterScoreModerationsResponse, error) {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			return client.GetGameCenterScoreModerations(requestCtx, leaderboardID, q)
		}
		first, err := fetch(ctx, query)
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		if *paginate {
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return fetch(ctx, asc.GCScoreModerationsQuery{NextURL: nextURL})
			})
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			return shared.PrintOutput(all, *output.Output, *output.Pretty)
		}
		return shared.PrintOutput(first, *output.Output, *output.Pretty)
	}}
}

func gameCenterScoreModerationsUpdateCommand() *ffcli.Command {
	const command = "game-center leaderboards v2 score-moderations update"
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "gameCenterScoreModerations", "Score moderation ID")
	var blocked shared.OptionalBool
	fs.Var(&blocked, "blocked", "Block or unblock the submitted score (true/false)")
	confirm := fs.Bool("confirm", false, "Confirm score moderation")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "update", ShortUsage: "asc " + command + " --id ID --blocked true|false --confirm", ShortHelp: "Block or unblock a submitted score.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		scoreID := strings.TrimSpace(*id)
		if scoreID == "" {
			return shared.UsageError(command + ": --id is required")
		}
		if !blocked.IsSet() {
			return shared.UsageError(command + ": --blocked is required")
		}
		if err := validateGameCenterReplacementConfirm(fs, *confirm); err != nil {
			return err
		}
		client, err := shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		response, err := client.UpdateGameCenterScoreModeration(requestCtx, scoreID, blocked.Value())
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		return shared.PrintOutput(response, *output.Output, *output.Pretty)
	}}
}
