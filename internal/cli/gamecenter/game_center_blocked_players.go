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

// GameCenterBlockedPlayersCommand returns the blocked player management group.
func GameCenterBlockedPlayersCommand() *ffcli.Command {
	return &ffcli.Command{Name: "blocked-players", ShortUsage: "asc game-center details blocked-players <subcommand> [flags]", ShortHelp: "List blocked players and manage player blocking.", FlagSet: flag.NewFlagSet("blocked-players", flag.ExitOnError), UsageFunc: shared.DefaultUsageFunc, Subcommands: []*ffcli.Command{gameCenterBlockedPlayersListCommand(), gameCenterBlockedPlayersUpdateCommand()}, Exec: func(context.Context, []string) error { return flag.ErrHelp }}
}

func gameCenterBlockedPlayersListCommand() *ffcli.Command {
	const command = "game-center details blocked-players list"
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "detail-id", "gameCenterDetails", "Game Center detail ID")
	fields := fs.String("fields", "", "Fields: nickname, blocked, bundleId")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "list", ShortUsage: "asc " + command + " --detail-id ID [flags]", ShortHelp: "List only blocked players for a Game Center detail.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		provided := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })
		if provided["limit"] && (*limit < 1 || *limit > 200) {
			return shared.UsageError(command + ": --limit must be between 1 and 200")
		}
		nextURL := strings.TrimSpace(*next)
		if err := shared.ValidateNextURL(nextURL); err != nil {
			return shared.UsageErrorf("%s: %v", command, err)
		}
		if err := shared.RejectNextFlagConflicts(fs, nextURL, command, "detail-id", "fields", "limit"); err != nil {
			return err
		}
		detailID := strings.TrimSpace(*id)
		if detailID == "" && nextURL == "" {
			return shared.UsageError(command + ": --detail-id is required")
		}
		selected, err := shared.NormalizeSelection(*fields, []string{"nickname", "blocked", "bundleId"}, "--fields")
		if err != nil {
			return shared.UsageError(err.Error())
		}
		if provided["fields"] && len(selected) == 0 {
			return shared.UsageError("--fields must not be empty")
		}
		client, err := shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		fetch := func(ctx context.Context, q asc.GCBlockedPlayersQuery) (*asc.GameCenterDetailPlayersResponse, error) {
			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			return client.GetGameCenterBlockedPlayers(requestCtx, detailID, q)
		}
		first, err := fetch(ctx, asc.GCBlockedPlayersQuery{Fields: selected, Limit: *limit, NextURL: nextURL})
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
			return shared.PrintOutput(all, *output.Output, *output.Pretty)
		}
		return shared.PrintOutput(first, *output.Output, *output.Pretty)
	}}
}

func gameCenterBlockedPlayersUpdateCommand() *ffcli.Command {
	const command = "game-center details blocked-players update"
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	id := shared.BindResourceIDFlag(fs, "id", "gameCenterDetailPlayers", "Game Center player ID")
	var blocked shared.OptionalBool
	fs.Var(&blocked, "blocked", "Block or unblock the player (true/false)")
	bundleID := fs.String("bundle-id", "", "Optional bundleId attribute to send with the update")
	confirm := fs.Bool("confirm", false, "Confirm player blocking change")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "update", ShortUsage: "asc " + command + " --id ID --blocked true|false --confirm", ShortHelp: "Block or unblock a Game Center player.", FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		playerID := strings.TrimSpace(*id)
		if playerID == "" {
			return shared.UsageError(command + ": --id is required")
		}
		if !blocked.IsSet() {
			return shared.UsageError(command + ": --blocked is required")
		}
		var bundle *string
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "bundle-id" {
				value := strings.TrimSpace(*bundleID)
				bundle = &value
			}
		})
		if bundle != nil && *bundle == "" {
			return shared.UsageError("--bundle-id must not be empty")
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
		response, err := client.UpdateGameCenterDetailPlayer(requestCtx, playerID, blocked.Value(), bundle)
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		return shared.PrintOutput(response, *output.Output, *output.Pretty)
	}}
}
