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

// GameCenterLeaderboardSetsV2Command returns the leaderboard-sets v2 command group.
func GameCenterLeaderboardSetsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "v2",
		ShortUsage: "asc game-center leaderboard-sets v2 <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard sets v2 resources.",
		LongHelp: `Manage Game Center leaderboard sets v2 resources.

Examples:
  asc game-center leaderboard-sets v2 list --app "APP_ID"
  asc game-center leaderboard-sets v2 get --id "SET_ID"
  asc game-center leaderboard-sets v2 create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetsV2ListCommand(),
			GameCenterLeaderboardSetsV2GetCommand(),
			GameCenterLeaderboardSetsV2CreateCommand(),
			GameCenterLeaderboardSetsV2UpdateCommand(),
			GameCenterLeaderboardSetsV2DeleteCommand(),
			GameCenterLeaderboardSetMembersV2Command(),
			GameCenterLeaderboardSetVersionsV2Command(),
			GameCenterLeaderboardSetLocalizationsV2Command(),
			GameCenterLeaderboardSetImagesV2Command(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetsV2ListCommand returns the leaderboard-sets v2 list subcommand.
func GameCenterLeaderboardSetsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Game Center group ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 list [flags]",
		ShortHelp:  "List Game Center leaderboard sets (v2) for an app or group.",
		LongHelp: `List Game Center leaderboard sets (v2) for an app or group.

Examples:
  asc game-center leaderboard-sets v2 list --app "APP_ID"
  asc game-center leaderboard-sets v2 list --group-id "GROUP_ID"
  asc game-center leaderboard-sets v2 list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 list: %w", err)
			}

			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if group == "" && resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" && nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCLeaderboardSetsOption{
				asc.WithGCLeaderboardSetsLimit(*limit),
				asc.WithGCLeaderboardSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetsV2(requestCtx, gcDetailID, group, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetsV2(ctx, gcDetailID, group, asc.WithGCLeaderboardSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetsV2(requestCtx, gcDetailID, group, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetsV2GetCommand returns the leaderboard-sets v2 get subcommand.
func GameCenterLeaderboardSetsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets v2 get --id \"SET_ID\"",
		ShortHelp:  "Get a Game Center leaderboard set (v2) by ID.",
		LongHelp: `Get a Game Center leaderboard set (v2) by ID.

Examples:
  asc game-center leaderboard-sets v2 get --id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetsV2CreateCommand returns the leaderboard-sets v2 create subcommand.
func GameCenterLeaderboardSetsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	groupID := fs.String("group-id", "", "Game Center group ID")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard set")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.set)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets v2 create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard set (v2).",
		LongHelp: `Create a new Game Center leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets v2 create --group-id "GROUP_ID" --reference-name "Group Season" --vendor-id "grp.com.example.groupseason"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(os.Stderr, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if group == "" && resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				fmt.Fprintln(os.Stderr, "Error: --reference-name is required")
				return flag.ErrHelp
			}

			vendor := strings.TrimSpace(*vendorID)
			if vendor == "" {
				fmt.Fprintln(os.Stderr, "Error: --vendor-id is required")
				return flag.ErrHelp
			}
			if group != "" && !strings.HasPrefix(vendor, "grp.") {
				fmt.Fprintln(os.Stderr, "Error: --vendor-id must start with \"grp.\" when using --group-id")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 create: failed to get Game Center detail: %w", err)
				}
			}

			attrs := asc.GameCenterLeaderboardSetCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
			}

			resp, err := client.CreateGameCenterLeaderboardSetV2(requestCtx, gcDetailID, group, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetsV2UpdateCommand returns the leaderboard-sets v2 update subcommand.
func GameCenterLeaderboardSetsV2UpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard set")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets v2 update [flags]",
		ShortHelp:  "Update a Game Center leaderboard set (v2).",
		LongHelp: `Update a Game Center leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 update --id "SET_ID" --reference-name "Season 1 - Updated"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterLeaderboardSetUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				name := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &name
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterLeaderboardSetV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetsV2DeleteCommand returns the leaderboard-sets v2 delete subcommand.
func GameCenterLeaderboardSetsV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets v2 delete --id \"SET_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set (v2).",
		LongHelp: `Delete a Game Center leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 delete --id "SET_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
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
				return fmt.Errorf("game-center leaderboard-sets v2 delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSetV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetMembersV2Command returns the leaderboard set members v2 command group.
func GameCenterLeaderboardSetMembersV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("members", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "members",
		ShortUsage: "asc game-center leaderboard-sets v2 members <subcommand> [flags]",
		ShortHelp:  "Manage leaderboard set members (v2).",
		LongHelp: `Manage leaderboard set members (v2). Members are the leaderboards that belong to a leaderboard set.

Examples:
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetMembersV2ListCommand(),
			GameCenterLeaderboardSetMembersV2SetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetMembersV2ListCommand returns the members v2 list subcommand.
func GameCenterLeaderboardSetMembersV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 members list --set-id \"SET_ID\"",
		ShortHelp:  "List leaderboards in a leaderboard set (v2).",
		LongHelp: `List leaderboards in a leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 members list --set-id "SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets v2 members list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members list: %w", err)
			}

			id := strings.TrimSpace(*setID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetMembersOption{
				asc.WithGCLeaderboardSetMembersLimit(*limit),
				asc.WithGCLeaderboardSetMembersNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetMembersLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetMembersV2(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 members list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetMembersV2(ctx, id, asc.WithGCLeaderboardSetMembersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 members list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetMembersV2(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetMembersV2SetCommand returns the members v2 set subcommand.
func GameCenterLeaderboardSetMembersV2SetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	leaderboardIDs := fs.String("leaderboard-ids", "", "Comma-separated leaderboard IDs to set as members")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set",
		ShortUsage: "asc game-center leaderboard-sets v2 members set --set-id \"SET_ID\" --leaderboard-ids \"id1,id2,id3\"",
		ShortHelp:  "Set leaderboard members for a leaderboard set (v2).",
		LongHelp: `Set leaderboard members for a leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"
  asc game-center leaderboard-sets v2 members set --set-id "SET_ID" --leaderboard-ids ""`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			ids := shared.SplitCSV(*leaderboardIDs)
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members set: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.UpdateGameCenterLeaderboardSetMembersV2(requestCtx, id, ids); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 members set: failed to update: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetMembersUpdateResult{
				SetID:       id,
				MemberCount: len(ids),
				Updated:     true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetVersionsV2Command returns the leaderboard set versions v2 command group.
func GameCenterLeaderboardSetVersionsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center leaderboard-sets v2 versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set versions (v2).",
		LongHelp: `Manage Game Center leaderboard set versions (v2).

Examples:
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 versions get --id "VERSION_ID"
  asc game-center leaderboard-sets v2 versions create --set-id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetVersionsV2ListCommand(),
			GameCenterLeaderboardSetVersionsV2GetCommand(),
			GameCenterLeaderboardSetVersionsV2CreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetVersionsV2ListCommand returns the leaderboard set versions v2 list subcommand.
func GameCenterLeaderboardSetVersionsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 versions list --set-id \"SET_ID\"",
		ShortHelp:  "List versions for a leaderboard set (v2).",
		LongHelp: `List versions for a leaderboard set (v2).

Examples:
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID"
  asc game-center leaderboard-sets v2 versions list --set-id "SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets v2 versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions list: %w", err)
			}

			id := strings.TrimSpace(*setID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetVersionsOption{
				asc.WithGCLeaderboardSetVersionsLimit(*limit),
				asc.WithGCLeaderboardSetVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetVersionsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetVersions(ctx, id, asc.WithGCLeaderboardSetVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetVersionsV2GetCommand returns the leaderboard set versions v2 get subcommand.
func GameCenterLeaderboardSetVersionsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center leaderboard set version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets v2 versions get --id \"VERSION_ID\"",
		ShortHelp:  "Get a Game Center leaderboard set version (v2) by ID.",
		LongHelp: `Get a Game Center leaderboard set version (v2) by ID.

Examples:
  asc game-center leaderboard-sets v2 versions get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetVersionsV2CreateCommand returns the leaderboard set versions v2 create subcommand.
func GameCenterLeaderboardSetVersionsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets v2 versions create --set-id \"SET_ID\"",
		ShortHelp:  "Create a new Game Center leaderboard set version (v2).",
		LongHelp: `Create a new Game Center leaderboard set version (v2).

Examples:
  asc game-center leaderboard-sets v2 versions create --set-id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterLeaderboardSetVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsV2Command returns the leaderboard set localizations v2 command group.
func GameCenterLeaderboardSetLocalizationsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set localizations (v2).",
		LongHelp: `Manage Game Center leaderboard set localizations (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID"
  asc game-center leaderboard-sets v2 localizations create --version-id "VER_ID" --locale en-US --name "Season 1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetLocalizationsV2ListCommand(),
			GameCenterLeaderboardSetLocalizationsV2GetCommand(),
			GameCenterLeaderboardSetLocalizationsV2CreateCommand(),
			GameCenterLeaderboardSetLocalizationsV2UpdateCommand(),
			GameCenterLeaderboardSetLocalizationsV2DeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetLocalizationsV2ListCommand returns the leaderboard set localizations v2 list subcommand.
func GameCenterLeaderboardSetLocalizationsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center leaderboard set version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations list --version-id \"VER_ID\"",
		ShortHelp:  "List localizations for a leaderboard set version (v2).",
		LongHelp: `List localizations for a leaderboard set version (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID"
  asc game-center leaderboard-sets v2 localizations list --version-id "VER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations list: %w", err)
			}

			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetLocalizationsOption{
				asc.WithGCLeaderboardSetLocalizationsLimit(*limit),
				asc.WithGCLeaderboardSetLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetVersionLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetVersionLocalizations(ctx, id, asc.WithGCLeaderboardSetLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetVersionLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsV2GetCommand returns the leaderboard set localizations v2 get subcommand.
func GameCenterLeaderboardSetLocalizationsV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center leaderboard set localization (v2) by ID.",
		LongHelp: `Get a Game Center leaderboard set localization (v2) by ID.

Examples:
  asc game-center leaderboard-sets v2 localizations get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetLocalizationV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsV2CreateCommand returns the leaderboard set localizations v2 create subcommand.
func GameCenterLeaderboardSetLocalizationsV2CreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center leaderboard set version ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US, de-DE)")
	name := fs.String("name", "", "Display name for the leaderboard set in this locale")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations create --version-id \"VER_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a new Game Center leaderboard set localization (v2).",
		LongHelp: `Create a new Game Center leaderboard set localization (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations create --version-id "VER_ID" --locale en-US --name "Season 1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			localeVal := strings.TrimSpace(*locale)
			if localeVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterLeaderboardSetLocalizationCreateAttributes{
				Locale: localeVal,
				Name:   nameVal,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterLeaderboardSetLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsV2UpdateCommand returns the leaderboard set localizations v2 update subcommand.
func GameCenterLeaderboardSetLocalizationsV2UpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	name := fs.String("name", "", "Display name for the leaderboard set in this locale")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations update --id \"LOC_ID\" --name \"NAME\"",
		ShortHelp:  "Update a Game Center leaderboard set localization (v2).",
		LongHelp: `Update a Game Center leaderboard set localization (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations update --id "LOC_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			nameVal := strings.TrimSpace(*name)
			if nameVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterLeaderboardSetLocalizationUpdateAttributes{
				Name: &nameVal,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterLeaderboardSetLocalizationV2(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsV2DeleteCommand returns the leaderboard set localizations v2 delete subcommand.
func GameCenterLeaderboardSetLocalizationsV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets v2 localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set localization (v2).",
		LongHelp: `Delete a Game Center leaderboard set localization (v2).

Examples:
  asc game-center leaderboard-sets v2 localizations delete --id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
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
				return fmt.Errorf("game-center leaderboard-sets v2 localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSetLocalizationV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetImagesV2Command returns the leaderboard set images v2 command group.
func GameCenterLeaderboardSetImagesV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center leaderboard-sets v2 images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set images (v2).",
		LongHelp: `Manage Game Center leaderboard set images (v2). Images are attached to leaderboard set localizations.

Examples:
  asc game-center leaderboard-sets v2 images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboard-sets v2 images get --id "IMAGE_ID"
  asc game-center leaderboard-sets v2 images get --localization-id "LOC_ID"
  asc game-center leaderboard-sets v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetImagesV2UploadCommand(),
			GameCenterLeaderboardSetImagesV2GetCommand(),
			GameCenterLeaderboardSetImagesV2DeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetImagesV2UploadCommand returns the leaderboard set images v2 upload subcommand.
func GameCenterLeaderboardSetImagesV2UploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Game Center leaderboard set localization ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center leaderboard-sets v2 images upload --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a Game Center leaderboard set localization (v2).",
		LongHelp: `Upload an image for a Game Center leaderboard set localization (v2).

This command performs the full upload flow: reserves the upload, uploads the file, and commits.

Examples:
  asc game-center leaderboard-sets v2 images upload --localization-id "LOC_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardSetImageV2(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images upload: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetImagesV2GetCommand returns the leaderboard set images v2 get subcommand.
func GameCenterLeaderboardSetImagesV2GetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center leaderboard set image ID")
	localizationID := fs.String("localization-id", "", "Game Center leaderboard set localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets v2 images get --id \"IMAGE_ID\" | --localization-id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center leaderboard set image (v2).",
		LongHelp: `Get a Game Center leaderboard set image (v2).

Examples:
  asc game-center leaderboard-sets v2 images get --id "IMAGE_ID"
  asc game-center leaderboard-sets v2 images get --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			locID := strings.TrimSpace(*localizationID)
			if id == "" && locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --localization-id is required")
				return flag.ErrHelp
			}
			if id != "" && locID != "" {
				fmt.Fprintln(os.Stderr, "Error: --id cannot be used with --localization-id")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if locID != "" {
				resp, err := client.GetGameCenterLeaderboardSetLocalizationImageV2(requestCtx, locID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets v2 images get: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetImageV2(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetImagesV2DeleteCommand returns the leaderboard set images v2 delete subcommand.
func GameCenterLeaderboardSetImagesV2DeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center leaderboard set image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets v2 images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set image (v2).",
		LongHelp: `Delete a Game Center leaderboard set image (v2).

Examples:
  asc game-center leaderboard-sets v2 images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
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
				return fmt.Errorf("game-center leaderboard-sets v2 images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSetImageV2(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets v2 images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
