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

// GameCenterDetailsCommand returns the details command group.
func GameCenterDetailsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "details",
		ShortUsage: "asc game-center details <subcommand> [flags]",
		ShortHelp:  "Manage Game Center details and related resources.",
		LongHelp: `Manage Game Center details and related resources.

Examples:
  asc game-center details list --app "APP_ID"
  asc game-center details get --id "DETAIL_ID"
  asc game-center details app-versions list --id "DETAIL_ID"
  asc game-center details group get --id "DETAIL_ID"
  asc game-center details achievements-v2 list --id "DETAIL_ID"
  asc game-center details leaderboard-releases list --id "DETAIL_ID"
  asc game-center details metrics classic-matchmaking --id "DETAIL_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsListCommand(),
			GameCenterDetailsGetCommand(),
			GameCenterDetailsAppVersionsCommand(),
			GameCenterDetailsGroupCommand(),
			GameCenterDetailsAchievementsV2Command(),
			GameCenterDetailsLeaderboardsV2Command(),
			GameCenterDetailsLeaderboardSetsV2Command(),
			GameCenterDetailsAchievementReleasesCommand(),
			GameCenterDetailsLeaderboardReleasesCommand(),
			GameCenterDetailsLeaderboardSetReleasesCommand(),
			GameCenterDetailsMetricsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsListCommand returns the details list subcommand.
func GameCenterDetailsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details list [flags]",
		ShortHelp:  "List Game Center details.",
		LongHelp: `List Game Center details.

Examples:
  asc game-center details list --app "APP_ID"
  asc game-center details list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			nextURL := strings.TrimSpace(*next)
			if nextURL != "" {
				return fmt.Errorf("game-center details list: --next is not supported")
			}
			if *paginate {
				return fmt.Errorf("game-center details list: --paginate is not supported")
			}
			if *limit != 0 {
				return fmt.Errorf("game-center details list: --limit is not supported")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			detailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center details list: failed to get Game Center detail: %w", err)
			}

			detail, err := client.GetGameCenterDetail(requestCtx, detailID)
			if err != nil {
				return fmt.Errorf("game-center details list: failed to fetch: %w", err)
			}

			resp := &asc.GameCenterDetailsResponse{
				Data:     []asc.Resource[asc.GameCenterDetailAttributes]{detail.Data},
				Links:    detail.Links,
				Included: detail.Included,
				Meta:     detail.Meta,
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsGetCommand returns the details get subcommand.
func GameCenterDetailsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center details get --id \"DETAIL_ID\"",
		ShortHelp:  "Get a Game Center detail by ID.",
		LongHelp: `Get a Game Center detail by ID.

Examples:
  asc game-center details get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*detailID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterDetail(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center details get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsAppVersionsCommand returns the details app-versions command group.
func GameCenterDetailsAppVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app-versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-versions",
		ShortUsage: "asc game-center details app-versions list --id \"DETAIL_ID\"",
		ShortHelp:  "List Game Center app versions for a detail.",
		LongHelp: `List Game Center app versions for a detail.

Examples:
  asc game-center details app-versions list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsAppVersionsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsAppVersionsListCommand returns the details app-versions list subcommand.
func GameCenterDetailsAppVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details app-versions list --id \"DETAIL_ID\"",
		ShortHelp:  "List Game Center app versions for a detail.",
		LongHelp: `List Game Center app versions for a detail.

Examples:
  asc game-center details app-versions list --id "DETAIL_ID"
  asc game-center details app-versions list --id "DETAIL_ID" --limit 50
  asc game-center details app-versions list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details app-versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details app-versions list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			nextURL := strings.TrimSpace(*next)
			if id == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details app-versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAppVersionsOption{
				asc.WithGCAppVersionsLimit(*limit),
				asc.WithGCAppVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := []asc.GCAppVersionsOption{asc.WithGCAppVersionsNextURL(*next)}
				if nextURL == "" {
					paginateOpts = []asc.GCAppVersionsOption{asc.WithGCAppVersionsLimit(200)}
				}
				firstPage, err := client.GetGameCenterDetailGameCenterAppVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details app-versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailGameCenterAppVersions(ctx, id, asc.WithGCAppVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details app-versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailGameCenterAppVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details app-versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsGroupCommand returns the details group command group.
func GameCenterDetailsGroupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("group", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group",
		ShortUsage: "asc game-center details group get --id \"DETAIL_ID\"",
		ShortHelp:  "Get the Game Center group for a detail.",
		LongHelp: `Get the Game Center group for a detail.

Examples:
  asc game-center details group get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsGroupGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsGroupGetCommand returns the details group get subcommand.
func GameCenterDetailsGroupGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center details group get --id \"DETAIL_ID\"",
		ShortHelp:  "Get the Game Center group for a detail.",
		LongHelp: `Get the Game Center group for a detail.

Examples:
  asc game-center details group get --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*detailID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details group get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterDetailGameCenterGroup(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center details group get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsAchievementsV2Command returns the achievements v2 command group.
func GameCenterDetailsAchievementsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("achievements-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements-v2",
		ShortUsage: "asc game-center details achievements-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 achievements for a Game Center detail.",
		LongHelp: `List v2 achievements for a Game Center detail.

Examples:
  asc game-center details achievements-v2 list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsAchievementsV2ListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsAchievementsV2ListCommand returns the achievements v2 list subcommand.
func GameCenterDetailsAchievementsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details achievements-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 achievements for a Game Center detail.",
		LongHelp: `List v2 achievements for a Game Center detail.

Examples:
  asc game-center details achievements-v2 list --id "DETAIL_ID"
  asc game-center details achievements-v2 list --id "DETAIL_ID" --limit 50
  asc game-center details achievements-v2 list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details achievements-v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details achievements-v2 list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details achievements-v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementsOption{
				asc.WithGCAchievementsLimit(*limit),
				asc.WithGCAchievementsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementsLimit(200))
				firstPage, err := client.GetGameCenterDetailsAchievementsV2(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details achievements-v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailsAchievementsV2(ctx, id, asc.WithGCAchievementsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details achievements-v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailsAchievementsV2(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details achievements-v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsLeaderboardsV2Command returns the leaderboards v2 command group.
func GameCenterDetailsLeaderboardsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboards-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards-v2",
		ShortUsage: "asc game-center details leaderboards-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboards for a Game Center detail.",
		LongHelp: `List v2 leaderboards for a Game Center detail.

Examples:
  asc game-center details leaderboards-v2 list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsLeaderboardsV2ListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsLeaderboardsV2ListCommand returns the leaderboards v2 list subcommand.
func GameCenterDetailsLeaderboardsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboards-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboards for a Game Center detail.",
		LongHelp: `List v2 leaderboards for a Game Center detail.

Examples:
  asc game-center details leaderboards-v2 list --id "DETAIL_ID"
  asc game-center details leaderboards-v2 list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboards-v2 list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details leaderboards-v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details leaderboards-v2 list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details leaderboards-v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardsOption{
				asc.WithGCLeaderboardsLimit(*limit),
				asc.WithGCLeaderboardsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardsLimit(200))
				firstPage, err := client.GetGameCenterDetailsLeaderboardsV2(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details leaderboards-v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailsLeaderboardsV2(ctx, id, asc.WithGCLeaderboardsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details leaderboards-v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailsLeaderboardsV2(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details leaderboards-v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsLeaderboardSetsV2Command returns the leaderboard sets v2 command group.
func GameCenterDetailsLeaderboardSetsV2Command() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard-sets-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-sets-v2",
		ShortUsage: "asc game-center details leaderboard-sets-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboard sets for a Game Center detail.",
		LongHelp: `List v2 leaderboard sets for a Game Center detail.

Examples:
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsLeaderboardSetsV2ListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsLeaderboardSetsV2ListCommand returns the leaderboard sets v2 list subcommand.
func GameCenterDetailsLeaderboardSetsV2ListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboard-sets-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboard sets for a Game Center detail.",
		LongHelp: `List v2 leaderboard sets for a Game Center detail.

Examples:
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID"
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details leaderboard-sets-v2 list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details leaderboard-sets-v2 list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details leaderboard-sets-v2 list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetsOption{
				asc.WithGCLeaderboardSetsLimit(*limit),
				asc.WithGCLeaderboardSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetsLimit(200))
				firstPage, err := client.GetGameCenterDetailsLeaderboardSetsV2(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details leaderboard-sets-v2 list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailsLeaderboardSetsV2(ctx, id, asc.WithGCLeaderboardSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details leaderboard-sets-v2 list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailsLeaderboardSetsV2(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details leaderboard-sets-v2 list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsAchievementReleasesCommand returns the achievement releases command group.
func GameCenterDetailsAchievementReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("achievement-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievement-releases",
		ShortUsage: "asc game-center details achievement-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List achievement releases for a Game Center detail.",
		LongHelp: `List achievement releases for a Game Center detail.

Examples:
  asc game-center details achievement-releases list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsAchievementReleasesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsAchievementReleasesListCommand returns the achievement releases list subcommand.
func GameCenterDetailsAchievementReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details achievement-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List achievement releases for a Game Center detail.",
		LongHelp: `List achievement releases for a Game Center detail.

Examples:
  asc game-center details achievement-releases list --id "DETAIL_ID"
  asc game-center details achievement-releases list --id "DETAIL_ID" --limit 50
  asc game-center details achievement-releases list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details achievement-releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details achievement-releases list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details achievement-releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementReleasesOption{
				asc.WithGCAchievementReleasesLimit(*limit),
				asc.WithGCAchievementReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementReleasesLimit(200))
				firstPage, err := client.GetGameCenterDetailsAchievementReleases(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details achievement-releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailsAchievementReleases(ctx, id, asc.WithGCAchievementReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details achievement-releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailsAchievementReleases(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details achievement-releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsLeaderboardReleasesCommand returns the leaderboard releases command group.
func GameCenterDetailsLeaderboardReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-releases",
		ShortUsage: "asc game-center details leaderboard-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard releases for a Game Center detail.",
		LongHelp: `List leaderboard releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-releases list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsLeaderboardReleasesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsLeaderboardReleasesListCommand returns the leaderboard releases list subcommand.
func GameCenterDetailsLeaderboardReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboard-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard releases for a Game Center detail.",
		LongHelp: `List leaderboard releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-releases list --id "DETAIL_ID"
  asc game-center details leaderboard-releases list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboard-releases list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details leaderboard-releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details leaderboard-releases list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details leaderboard-releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardReleasesOption{
				asc.WithGCLeaderboardReleasesLimit(*limit),
				asc.WithGCLeaderboardReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardReleasesLimit(200))
				firstPage, err := client.GetGameCenterDetailsLeaderboardReleases(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details leaderboard-releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailsLeaderboardReleases(ctx, id, asc.WithGCLeaderboardReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details leaderboard-releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailsLeaderboardReleases(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details leaderboard-releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsLeaderboardSetReleasesCommand returns the leaderboard set releases command group.
func GameCenterDetailsLeaderboardSetReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard-set-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-set-releases",
		ShortUsage: "asc game-center details leaderboard-set-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard set releases for a Game Center detail.",
		LongHelp: `List leaderboard set releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsLeaderboardSetReleasesListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsLeaderboardSetReleasesListCommand returns the leaderboard set releases list subcommand.
func GameCenterDetailsLeaderboardSetReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboard-set-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard set releases for a Game Center detail.",
		LongHelp: `List leaderboard set releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID"
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center details leaderboard-set-releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center details leaderboard-set-releases list: %w", err)
			}

			id := strings.TrimSpace(*detailID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center details leaderboard-set-releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetReleasesOption{
				asc.WithGCLeaderboardSetReleasesLimit(*limit),
				asc.WithGCLeaderboardSetReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetReleasesLimit(200))
				firstPage, err := client.GetGameCenterDetailsLeaderboardSetReleases(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center details leaderboard-set-releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterDetailsLeaderboardSetReleases(ctx, id, asc.WithGCLeaderboardSetReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center details leaderboard-set-releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterDetailsLeaderboardSetReleases(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center details leaderboard-set-releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterDetailsMetricsCommand returns the details metrics command group.
func GameCenterDetailsMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc game-center details metrics <subcommand> [flags]",
		ShortHelp:  "Fetch Game Center details metrics.",
		LongHelp: `Fetch Game Center details metrics.

Examples:
  asc game-center details metrics classic-matchmaking --id "DETAIL_ID" --granularity P1D
  asc game-center details metrics rule-based-matchmaking --id "DETAIL_ID" --granularity P1D --group-by result`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterDetailsClassicMatchmakingCommand(),
			GameCenterDetailsRuleBasedMatchmakingCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterDetailsClassicMatchmakingCommand returns the classic matchmaking metrics subcommand.
func GameCenterDetailsClassicMatchmakingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("classic-matchmaking", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return detailsMetricsCommand("classic-matchmaking", fs, detailID, granularity, groupBy, filterResult, sort, limit, next, paginate, output, pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error) {
		return ascClient(ctx).GetGameCenterDetailsClassicMatchmakingRequests(ctx, id, opts...)
	})
}

// GameCenterDetailsRuleBasedMatchmakingCommand returns the rule-based matchmaking metrics subcommand.
func GameCenterDetailsRuleBasedMatchmakingCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rule-based-matchmaking", flag.ExitOnError)

	detailID := fs.String("id", "", "Game Center detail ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return detailsMetricsCommand("rule-based-matchmaking", fs, detailID, granularity, groupBy, filterResult, sort, limit, next, paginate, output, pretty, func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error) {
		return ascClient(ctx).GetGameCenterDetailsRuleBasedMatchmakingRequests(ctx, id, opts...)
	})
}

func detailsMetricsCommand(name string, fs *flag.FlagSet, detailID *string, granularity *string, groupBy *string, filterResult *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center details metrics " + name + " --id \"DETAIL_ID\" --granularity P1D",
		ShortHelp:  "Fetch Game Center details metrics.",
		LongHelp: `Fetch Game Center details metrics.

Examples:
  asc game-center details metrics ` + name + ` --id "DETAIL_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runDetailsMetrics(ctx, name, detailID, granularity, groupBy, filterResult, sort, limit, next, paginate, output, pretty, fetch)
		},
	}
}

func runDetailsMetrics(ctx context.Context, name string, detailID *string, granularity *string, groupBy *string, filterResult *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error)) error {
	if *limit != 0 && (*limit < 1 || *limit > 200) {
		return fmt.Errorf("game-center details metrics %s: --limit must be between 1 and 200", name)
	}
	if err := shared.ValidateNextURL(*next); err != nil {
		return fmt.Errorf("game-center details metrics %s: %w", name, err)
	}

	id := strings.TrimSpace(*detailID)
	if id == "" && strings.TrimSpace(*next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --id is required")
		return flag.ErrHelp
	}
	gran := strings.TrimSpace(*granularity)
	if gran == "" && strings.TrimSpace(*next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --granularity is required")
		return flag.ErrHelp
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	opts := []asc.GCMatchmakingMetricsOption{
		asc.WithGCMatchmakingMetricsGranularity(gran),
		asc.WithGCMatchmakingMetricsGroupBy(shared.SplitCSV(*groupBy)),
		asc.WithGCMatchmakingMetricsFilterResult(strings.TrimSpace(*filterResult)),
		asc.WithGCMatchmakingMetricsSort(shared.SplitCSV(*sort)),
		asc.WithGCMatchmakingMetricsLimit(*limit),
		asc.WithGCMatchmakingMetricsNextURL(*next),
	}

	if *paginate {
		paginateOpts := append(opts, asc.WithGCMatchmakingMetricsLimit(200))
		firstPage, err := fetch(requestCtx, id, paginateOpts...)
		if err != nil {
			return fmt.Errorf("game-center details metrics %s: failed to fetch: %w", name, err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetch(ctx, id, asc.WithGCMatchmakingMetricsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("game-center details metrics %s: %w", name, err)
		}

		return shared.PrintOutput(resp, *output, *pretty)
	}

	resp, err := fetch(requestCtx, id, opts...)
	if err != nil {
		return fmt.Errorf("game-center details metrics %s: failed to fetch: %w", name, err)
	}

	return shared.PrintOutput(resp, *output, *pretty)
}
