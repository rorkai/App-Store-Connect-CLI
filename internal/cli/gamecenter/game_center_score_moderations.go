package gamecenter

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"slices"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newGameCenterScoreModerationsCommand returns the leaderboard score moderation group.
func newGameCenterScoreModerationsCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{Name: "score-moderations", ShortUsage: "asc game-center leaderboards v2 score-moderations <subcommand> [flags]", ShortHelp: "List submitted scores and manage their blocked status.", FlagSet: d.NewFlagSet("score-moderations", flag.ExitOnError), UsageFunc: d.DefaultUsageFunc, Subcommands: []*ffcli.Command{gameCenterScoreModerationsListCommand(d), gameCenterScoreModerationsUpdateCommand(d)}, Exec: func(context.Context, []string) error { return flag.ErrHelp }}
}

func gameCenterScoreModerationsListCommand(d *shared.Deps) *ffcli.Command {
	const command = "game-center leaderboards v2 score-moderations list"
	fs := d.NewFlagSet("list", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Game Center leaderboard ID")
	var blocked shared.OptionalBool
	fs.Var(&blocked, "exists-blocked", "Value for Apple's exists[blocked] filter (true/false)")
	fields := fs.String("fields", "", "Fields: rank, score, submittedDate, blocked, preReleased, context, challengeIds, player")
	playerFields := fs.String("player-fields", "", "Player fields: nickname, blocked, bundleId (requires --include player)")
	include := fs.String("include", "", "Relationships to include: player")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{Name: "list", ShortUsage: "asc " + command + " --leaderboard-id ID [flags]", ShortHelp: "List scores submitted to a v2 leaderboard.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		provided := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
		if provided["limit"] && (*limit < 1 || *limit > 200) {
			return d.UsageError(command + ": --limit must be between 1 and 200")
		}
		if err := d.ValidatePaginationFlags(ctx, fs, command, 200, "exists-blocked", "fields", "player-fields", "include", "limit"); err != nil {
			return err
		}
		nextURL := strings.TrimSpace(*next)
		leaderboardID := strings.TrimSpace(*id)
		if leaderboardID == "" && nextURL == "" {
			return d.UsageError(command + ": --leaderboard-id is required")
		}
		normalize := func(raw, name string, allowed []string) ([]string, error) {
			values, err := shared.NormalizeSelection(raw, allowed, "--"+name)
			if err != nil {
				return nil, d.UsageError(err.Error())
			}
			if provided[name] && len(values) == 0 {
				return nil, d.UsageErrorf("--%s must not be empty", name)
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
			return d.UsageError("--player-fields requires --include player")
		}
		if slices.Contains(includes, "player") && len(selected) > 0 && !slices.Contains(selected, "player") {
			selected = append(selected, "player")
		}
		firstPageLimit := *limit
		if *paginate && firstPageLimit == 0 {
			firstPageLimit = 200
		}
		query := asc.GCScoreModerationsQuery{Fields: selected, PlayerFields: players, Include: includes, Limit: firstPageLimit, NextURL: nextURL}
		if blocked.IsSet() {
			value := blocked.Value()
			query.ExistsBlocked = &value
		}
		client, err := d.Client()
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		fetch := func(ctx context.Context, q asc.GCScoreModerationsQuery) (*asc.GameCenterScoreModerationsResponse, error) {
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			return client.GetGameCenterScoreModerations(requestCtx, leaderboardID, q)
		}
		first, err := fetch(ctx, query)
		if err != nil {
			var apiErr *asc.APIError
			if nextURL == "" && errors.As(err, &apiErr) && slices.Contains(apiErr.AllCodes, "PATH_ERROR") {
				return fmt.Errorf("%s: %w (Apple documents this endpoint but does not serve it yet; to see a leaderboard's top scores and player nicknames, open App Store Connect > Apps > your app > Game Center > Manage Scores and Players)", command, err)
			}
			return fmt.Errorf("%s: %w", command, err)
		}
		if *paginate {
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return fetch(ctx, asc.GCScoreModerationsQuery{NextURL: nextURL})
			})
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			return d.PrintOutput(all, *output.Output, *output.Pretty)
		}
		return d.PrintOutput(first, *output.Output, *output.Pretty)
	}}
}

func gameCenterScoreModerationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	const command = "game-center leaderboards v2 score-moderations update"
	fs := d.NewFlagSet("update", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "gameCenterScoreModerations", "Score moderation ID")
	var blocked shared.OptionalBool
	fs.Var(&blocked, "blocked", "Block or unblock the submitted score (true/false)")
	confirm := fs.Bool("confirm", false, "Confirm score moderation")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{Name: "update", ShortUsage: "asc " + command + " --id ID --blocked true|false --confirm", ShortHelp: "Block or unblock a submitted score.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		scoreID := strings.TrimSpace(*id)
		if scoreID == "" {
			return d.UsageError(command + ": --id is required")
		}
		if !blocked.IsSet() {
			return d.UsageError(command + ": --blocked is required")
		}
		if err := validateGameCenterReplacementConfirm(d, fs, *confirm); err != nil {
			return err
		}
		client, err := d.Client()
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		requestCtx, cancel := d.Timeout(ctx)
		defer cancel()
		response, err := client.UpdateGameCenterScoreModeration(requestCtx, scoreID, blocked.Value())
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		return d.PrintOutput(response, *output.Output, *output.Pretty)
	}}
}
