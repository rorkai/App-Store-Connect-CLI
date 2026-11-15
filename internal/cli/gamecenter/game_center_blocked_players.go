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

// newGameCenterBlockedPlayersCommand returns the blocked player management group.
func newGameCenterBlockedPlayersCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{Name: "blocked-players", ShortUsage: "asc game-center details blocked-players <subcommand> [flags]", ShortHelp: "List blocked players and manage player blocking.", FlagSet: d.NewFlagSet("blocked-players", flag.ExitOnError), UsageFunc: d.DefaultUsageFunc, Subcommands: []*ffcli.Command{gameCenterBlockedPlayersListCommand(d), gameCenterBlockedPlayersUpdateCommand(d)}, Exec: func(context.Context, []string) error { return flag.ErrHelp }}
}

func gameCenterBlockedPlayersListCommand(d *shared.Deps) *ffcli.Command {
	const command = "game-center details blocked-players list"
	fs := d.NewFlagSet("list", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "detail-id", "gameCenterDetails", "Game Center detail ID")
	fields := fs.String("fields", "", "Fields: nickname, blocked, bundleId")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{Name: "list", ShortUsage: "asc " + command + " --detail-id ID [flags]", ShortHelp: "List only blocked players for a Game Center detail.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		provided := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
		if provided["limit"] && (*limit < 1 || *limit > 200) {
			return d.UsageError(command + ": --limit must be between 1 and 200")
		}
		if err := d.ValidatePaginationFlags(ctx, fs, command, 200, "fields", "limit"); err != nil {
			return err
		}
		nextURL := strings.TrimSpace(*next)
		detailID := strings.TrimSpace(*id)
		if detailID == "" && nextURL == "" {
			return d.UsageError(command + ": --detail-id is required")
		}
		selected, err := shared.NormalizeSelection(*fields, []string{"nickname", "blocked", "bundleId"}, "--fields")
		if err != nil {
			return d.UsageError(err.Error())
		}
		if provided["fields"] && len(selected) == 0 {
			return d.UsageError("--fields must not be empty")
		}
		client, err := d.Client()
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		fetch := func(ctx context.Context, q asc.GCBlockedPlayersQuery) (*asc.GameCenterDetailPlayersResponse, error) {
			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()
			return client.GetGameCenterBlockedPlayers(requestCtx, detailID, q)
		}
		firstPageLimit := *limit
		if *paginate && firstPageLimit == 0 {
			firstPageLimit = 200
		}
		first, err := fetch(ctx, asc.GCBlockedPlayersQuery{Fields: selected, Limit: firstPageLimit, NextURL: nextURL})
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		if *paginate {
			all, err := asc.PaginateAll(ctx, first, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return fetch(ctx, asc.GCBlockedPlayersQuery{NextURL: nextURL})
			})
			if err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
			return d.PrintOutput(all, *output.Output, *output.Pretty)
		}
		return d.PrintOutput(first, *output.Output, *output.Pretty)
	}}
}

func gameCenterBlockedPlayersUpdateCommand(d *shared.Deps) *ffcli.Command {
	const command = "game-center details blocked-players update"
	fs := d.NewFlagSet("update", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "gameCenterDetailPlayers", "Game Center player ID")
	var blocked shared.OptionalBool
	fs.Var(&blocked, "blocked", "Block or unblock the player (true/false)")
	bundleID := fs.String("bundle-id", "", "Optional bundleId attribute to send with the update")
	confirm := fs.Bool("confirm", false, "Confirm player blocking change")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{Name: "update", ShortUsage: "asc " + command + " --id ID --blocked true|false --confirm", ShortHelp: "Block or unblock a Game Center player.", FlagSet: fs, UsageFunc: d.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		playerID := strings.TrimSpace(*id)
		if playerID == "" {
			return d.UsageError(command + ": --id is required")
		}
		if !blocked.IsSet() {
			return d.UsageError(command + ": --blocked is required")
		}
		var bundle *string
		if flagPassed(fs, "bundle-id") {
			value := strings.TrimSpace(*bundleID)
			bundle = &value
		}
		if bundle != nil && *bundle == "" {
			return d.UsageError("--bundle-id must not be empty")
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
		response, err := client.UpdateGameCenterDetailPlayer(requestCtx, playerID, blocked.Value(), bundle)
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		return d.PrintOutput(response, *output.Output, *output.Pretty)
	}}
}
