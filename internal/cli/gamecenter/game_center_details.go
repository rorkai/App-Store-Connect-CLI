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

// newGameCenterDetailsCommand returns the details command group.
func newGameCenterDetailsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("details", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "details",
		ShortUsage: "asc game-center details <subcommand> [flags]",
		ShortHelp:  "Manage Game Center details and related resources.",
		LongHelp: `Manage Game Center details and related resources.

Examples:
  asc game-center details list --app "APP_ID"
  asc game-center details view --id "DETAIL_ID"
  asc game-center details create --app "APP_ID"
  asc game-center details update --id "DETAIL_ID" --game-center-group-id "GROUP_ID"
  asc game-center details app-versions list --id "DETAIL_ID"
  asc game-center details group view --id "DETAIL_ID"
  asc game-center details achievements-v2 list --id "DETAIL_ID"
  asc game-center details leaderboard-releases list --id "DETAIL_ID"
  asc game-center details metrics classic-matchmaking --id "DETAIL_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsListCommand(d),
			newGameCenterBlockedPlayersCommand(d),
			newGameCenterDetailsGetCommand(d),
			newGameCenterDetailsCreateCommand(d),
			newGameCenterDetailsUpdateCommand(d),
			newGameCenterDetailsAppVersionsCommand(d),
			newGameCenterDetailsGroupCommand(d),
			newGameCenterDetailsAchievementsV2Command(d),
			newGameCenterDetailsLeaderboardsV2Command(d),
			newGameCenterDetailsLeaderboardSetsV2Command(d),
			newGameCenterDetailsAchievementReleasesCommand(d),
			newGameCenterDetailsLeaderboardReleasesCommand(d),
			newGameCenterDetailsLeaderboardSetReleasesCommand(d),
			newGameCenterDetailsMetricsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsListCommand returns the details list subcommand.
func newGameCenterDetailsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center details list [flags]",
		ShortHelp:  "List Game Center details.",
		LongHelp: `List Game Center details.

Each app has at most one Game Center detail, so the lookup is not paginated.

Examples:
  asc game-center details list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center details list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			detailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center details list: failed to get Game Center detail: %w", err)
			}
			detailID = strings.TrimSpace(detailID)
			if detailID == "" {
				// App Store Connect returns 200 with an empty id when no detail exists yet.
				// Treat this as an empty list rather than attempting /v1/gameCenterDetails/.
				fmt.Fprintln(d.IO.ErrOut, noGameCenterDetailWarning)
				resp := &asc.GameCenterDetailsResponse{
					Data:  []asc.Resource[asc.GameCenterDetailAttributes]{},
					Links: asc.Links{},
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterDetailsGetCommand returns the details get subcommand.
func newGameCenterDetailsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center details view --id \"DETAIL_ID\"",
		ShortHelp:  "View a Game Center detail by ID.",
		LongHelp: `View a Game Center detail by ID.

Examples:
  asc game-center details view --id "DETAIL_ID"`,
		IDUsage:            "Game Center detail ID",
		IDType:             "gameCenterDetails",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterDetail(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterDetailsCreateCommand returns the details create subcommand.
func newGameCenterDetailsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center details create --app \"APP_ID\"",
		ShortHelp:  "Create a Game Center detail for an app.",
		LongHelp: `Create a Game Center detail for an app.

Examples:
  asc game-center details create --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := d.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center details create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterDetail(requestCtx, resolvedAppID, nil)
			if err != nil {
				return fmt.Errorf("game-center details create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterDetailsUpdateCommand returns the details update subcommand.
func newGameCenterDetailsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "id", "gameCenterDetails", "Game Center detail ID")
	gameCenterGroupID := shared.BindResourceIDFlag(fs, "game-center-group-id", "gameCenterGroups", "Game Center group ID to associate")
	defaultLeaderboardID := shared.BindResourceIDFlag(fs, "default-leaderboard-id", "gameCenterLeaderboards", "Default leaderboard ID (deprecated by Apple; sets the same default as --default-leaderboard-v2-id)")
	defaultLeaderboardV2ID := shared.BindResourceIDFlag(fs, "default-leaderboard-v2-id", "gameCenterLeaderboards", "Default V2 leaderboard ID")
	defaultGroupLeaderboardID := shared.BindResourceIDFlag(fs, "default-group-leaderboard-id", "gameCenterLeaderboards", "Default group leaderboard ID (deprecated by Apple; sets the same default as --default-group-leaderboard-v2-id)")
	defaultGroupLeaderboardV2ID := shared.BindResourceIDFlag(fs, "default-group-leaderboard-v2-id", "gameCenterLeaderboards", "Default V2 group leaderboard ID")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center details update --id \"DETAIL_ID\" [flags]",
		ShortHelp:  "Update a Game Center detail.",
		LongHelp: `Update a Game Center detail.

Examples:
  asc game-center details update --id "DETAIL_ID" --game-center-group-id "GROUP_ID"
  asc game-center details update --id "DETAIL_ID" --default-leaderboard-v2-id "LEADERBOARD_ID"
  asc game-center details update --id "DETAIL_ID" --default-group-leaderboard-v2-id "LEADERBOARD_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*detailID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			// Live: Apple accepts both relationships in one PATCH and keeps only the V2 value.
			if strings.TrimSpace(*defaultLeaderboardID) != "" && strings.TrimSpace(*defaultLeaderboardV2ID) != "" {
				return d.UsageError("--default-leaderboard-id and --default-leaderboard-v2-id set the same default; pass one")
			}
			if strings.TrimSpace(*defaultGroupLeaderboardID) != "" && strings.TrimSpace(*defaultGroupLeaderboardV2ID) != "" {
				return d.UsageError("--default-group-leaderboard-id and --default-group-leaderboard-v2-id set the same default; pass one")
			}

			rels := &asc.GameCenterDetailUpdateRelationships{}
			hasUpdate := false

			gcGroupID := strings.TrimSpace(*gameCenterGroupID)
			if gcGroupID != "" {
				rels.GameCenterGroup = &asc.Relationship{
					Data: asc.ResourceData{
						Type: asc.ResourceTypeGameCenterGroups,
						ID:   gcGroupID,
					},
				}
				hasUpdate = true
			}

			for _, leaderboard := range []struct {
				id     string
				target **asc.Relationship
			}{
				{*defaultLeaderboardID, &rels.DefaultLeaderboard},
				{*defaultLeaderboardV2ID, &rels.DefaultLeaderboardV2},
				{*defaultGroupLeaderboardID, &rels.DefaultGroupLeaderboard},
				{*defaultGroupLeaderboardV2ID, &rels.DefaultGroupLeaderboardV2},
			} {
				leaderboardID := strings.TrimSpace(leaderboard.id)
				if leaderboardID == "" {
					continue
				}
				*leaderboard.target = &asc.Relationship{
					Data: asc.ResourceData{
						Type: asc.ResourceTypeGameCenterLeaderboards,
						ID:   leaderboardID,
					},
				}
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, gameCenterGroupID, defaultLeaderboardID, defaultLeaderboardV2ID, defaultGroupLeaderboardID, defaultGroupLeaderboardV2ID)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center details update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterDetail(requestCtx, id, nil, rels)
			if err != nil {
				return fmt.Errorf("game-center details update: failed to update: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterDetailsAppVersionsCommand returns the details app-versions command group.
func newGameCenterDetailsAppVersionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app-versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app-versions",
		ShortUsage: "asc game-center details app-versions list --id \"DETAIL_ID\"",
		ShortHelp:  "List Game Center app versions for a detail.",
		LongHelp: `List Game Center app versions for a detail.

Examples:
  asc game-center details app-versions list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsAppVersionsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsAppVersionsListCommand returns the details app-versions list subcommand.
func newGameCenterDetailsAppVersionsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details app-versions list --id \"DETAIL_ID\"",
		ShortHelp:  "List Game Center app versions for a detail.",
		LongHelp: `List Game Center app versions for a detail.

Examples:
  asc game-center details app-versions list --id "DETAIL_ID"
  asc game-center details app-versions list --id "DETAIL_ID" --limit 50
  asc game-center details app-versions list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailGameCenterAppVersions(ctx, id, asc.Limit[asc.GCAppVersionsOption](limit), asc.NextURL[asc.GCAppVersionsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsGroupCommand returns the details group command group.
func newGameCenterDetailsGroupCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("group", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group",
		ShortUsage: "asc game-center details group view --id \"DETAIL_ID\"",
		ShortHelp:  "View the Game Center group for a detail.",
		LongHelp: `View the Game Center group for a detail.

Examples:
  asc game-center details group view --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsGroupGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsGroupGetCommand returns the details group get subcommand.
func newGameCenterDetailsGroupGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center details group view --id \"DETAIL_ID\"",
		ShortHelp:  "View the Game Center group for a detail.",
		LongHelp: `View the Game Center group for a detail.

Examples:
  asc game-center details group view --id "DETAIL_ID"`,
		IDUsage:            "Game Center detail ID",
		IDType:             "gameCenterDetails",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterDetailGameCenterGroup(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterDetailsAchievementsV2Command returns the achievements v2 command group.
func newGameCenterDetailsAchievementsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("achievements-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements-v2",
		ShortUsage: "asc game-center details achievements-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 achievements for a Game Center detail.",
		LongHelp: `List v2 achievements for a Game Center detail.

Examples:
  asc game-center details achievements-v2 list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsAchievementsV2ListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsAchievementsV2ListCommand returns the achievements v2 list subcommand.
func newGameCenterDetailsAchievementsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details achievements-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 achievements for a Game Center detail.",
		LongHelp: `List v2 achievements for a Game Center detail.

Examples:
  asc game-center details achievements-v2 list --id "DETAIL_ID"
  asc game-center details achievements-v2 list --id "DETAIL_ID" --limit 50
  asc game-center details achievements-v2 list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailsAchievementsV2(ctx, id, asc.Limit[asc.GCAchievementsOption](limit), asc.NextURL[asc.GCAchievementsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsLeaderboardsV2Command returns the leaderboards v2 command group.
func newGameCenterDetailsLeaderboardsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboards-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards-v2",
		ShortUsage: "asc game-center details leaderboards-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboards for a Game Center detail.",
		LongHelp: `List v2 leaderboards for a Game Center detail.

Examples:
  asc game-center details leaderboards-v2 list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsLeaderboardsV2ListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsLeaderboardsV2ListCommand returns the leaderboards v2 list subcommand.
func newGameCenterDetailsLeaderboardsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboards-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboards for a Game Center detail.",
		LongHelp: `List v2 leaderboards for a Game Center detail.

Examples:
  asc game-center details leaderboards-v2 list --id "DETAIL_ID"
  asc game-center details leaderboards-v2 list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboards-v2 list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailsLeaderboardsV2(ctx, id, asc.Limit[asc.GCLeaderboardsOption](limit), asc.NextURL[asc.GCLeaderboardsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsLeaderboardSetsV2Command returns the leaderboard sets v2 command group.
func newGameCenterDetailsLeaderboardSetsV2Command(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard-sets-v2", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-sets-v2",
		ShortUsage: "asc game-center details leaderboard-sets-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboard sets for a Game Center detail.",
		LongHelp: `List v2 leaderboard sets for a Game Center detail.

Examples:
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsLeaderboardSetsV2ListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsLeaderboardSetsV2ListCommand returns the leaderboard sets v2 list subcommand.
func newGameCenterDetailsLeaderboardSetsV2ListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboard-sets-v2 list --id \"DETAIL_ID\"",
		ShortHelp:  "List v2 leaderboard sets for a Game Center detail.",
		LongHelp: `List v2 leaderboard sets for a Game Center detail.

Examples:
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID"
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboard-sets-v2 list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailsLeaderboardSetsV2(ctx, id, asc.Limit[asc.GCLeaderboardSetsOption](limit), asc.NextURL[asc.GCLeaderboardSetsOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsAchievementReleasesCommand returns the achievement releases command group.
func newGameCenterDetailsAchievementReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("achievement-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievement-releases",
		ShortUsage: "asc game-center details achievement-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List achievement releases for a Game Center detail.",
		LongHelp: `List achievement releases for a Game Center detail.

Examples:
  asc game-center details achievement-releases list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsAchievementReleasesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsAchievementReleasesListCommand returns the achievement releases list subcommand.
func newGameCenterDetailsAchievementReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details achievement-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List achievement releases for a Game Center detail.",
		LongHelp: `List achievement releases for a Game Center detail.

Examples:
  asc game-center details achievement-releases list --id "DETAIL_ID"
  asc game-center details achievement-releases list --id "DETAIL_ID" --limit 50
  asc game-center details achievement-releases list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailsAchievementReleases(ctx, id, asc.Limit[asc.GCAchievementReleasesOption](limit), asc.NextURL[asc.GCAchievementReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsLeaderboardReleasesCommand returns the leaderboard releases command group.
func newGameCenterDetailsLeaderboardReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-releases",
		ShortUsage: "asc game-center details leaderboard-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard releases for a Game Center detail.",
		LongHelp: `List leaderboard releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-releases list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsLeaderboardReleasesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsLeaderboardReleasesListCommand returns the leaderboard releases list subcommand.
func newGameCenterDetailsLeaderboardReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboard-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard releases for a Game Center detail.",
		LongHelp: `List leaderboard releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-releases list --id "DETAIL_ID"
  asc game-center details leaderboard-releases list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboard-releases list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailsLeaderboardReleases(ctx, id, asc.Limit[asc.GCLeaderboardReleasesOption](limit), asc.NextURL[asc.GCLeaderboardReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsLeaderboardSetReleasesCommand returns the leaderboard set releases command group.
func newGameCenterDetailsLeaderboardSetReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboard-set-releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-set-releases",
		ShortUsage: "asc game-center details leaderboard-set-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard set releases for a Game Center detail.",
		LongHelp: `List leaderboard set releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsLeaderboardSetReleasesListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsLeaderboardSetReleasesListCommand returns the leaderboard set releases list subcommand.
func newGameCenterDetailsLeaderboardSetReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center details leaderboard-set-releases list --id \"DETAIL_ID\"",
		ShortHelp:  "List leaderboard set releases for a Game Center detail.",
		LongHelp: `List leaderboard set releases for a Game Center detail.

Examples:
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID"
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID" --limit 50
  asc game-center details leaderboard-set-releases list --id "DETAIL_ID" --paginate`,
		ParentFlag:           "id",
		ParentUsage:          "Game Center detail ID",
		ParentType:           "gameCenterDetails",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterDetailsLeaderboardSetReleases(ctx, id, asc.Limit[asc.GCLeaderboardSetReleasesOption](limit), asc.NextURL[asc.GCLeaderboardSetReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterDetailsMetricsCommand returns the details metrics command group.
func newGameCenterDetailsMetricsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc game-center details metrics <subcommand> [flags]",
		ShortHelp:  "Fetch Game Center details metrics.",
		LongHelp: `Fetch Game Center details metrics.

Examples:
  asc game-center details metrics classic-matchmaking --id "DETAIL_ID" --granularity P1D
  asc game-center details metrics rule-based-matchmaking --id "DETAIL_ID" --granularity P1D --group-by result`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterDetailsClassicMatchmakingCommand(d),
			newGameCenterDetailsRuleBasedMatchmakingCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterDetailsClassicMatchmakingCommand returns the classic matchmaking metrics subcommand.
func newGameCenterDetailsClassicMatchmakingCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("classic-matchmaking", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "id", "gameCenterDetails", "Game Center detail ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return detailsMetricsCommand(d, "classic-matchmaking", fs, detailID, granularity, groupBy, filterResult, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error) {
		return client.GetGameCenterDetailsClassicMatchmakingRequests(ctx, id, opts...)
	})
}

// newGameCenterDetailsRuleBasedMatchmakingCommand returns the rule-based matchmaking metrics subcommand.
func newGameCenterDetailsRuleBasedMatchmakingCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("rule-based-matchmaking", flag.ExitOnError)

	detailID := shared.BindResourceIDFlag(fs, "id", "gameCenterDetails", "Game Center detail ID")
	granularity := fs.String("granularity", "", "Granularity (P1D, PT1H, PT15M)")
	groupBy := fs.String("group-by", "", "Group by (comma-separated: result)")
	filterResult := fs.String("filter-result", "", "Filter result (MATCHED, CANCELED, EXPIRED)")
	sort := fs.String("sort", "", "Sort fields (comma-separated)")
	limit := fs.Int("limit", 0, "Maximum groups per page (1-200)")
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return detailsMetricsCommand(d, "rule-based-matchmaking", fs, detailID, granularity, groupBy, filterResult, sort, limit, next, paginate, output.Output, output.Pretty, func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error) {
		return client.GetGameCenterDetailsRuleBasedMatchmakingRequests(ctx, id, opts...)
	})
}

func detailsMetricsCommand(d *shared.Deps, name string, fs *flag.FlagSet, detailID *string, granularity *string, groupBy *string, filterResult *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error)) *ffcli.Command {
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc game-center details metrics " + name + " --id \"DETAIL_ID\" --granularity P1D",
		ShortHelp:  "Fetch Game Center details metrics.",
		LongHelp: `Fetch Game Center details metrics.

Examples:
  asc game-center details metrics ` + name + ` --id "DETAIL_ID" --granularity P1D`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runDetailsMetrics(ctx, d, name, fs, detailID, granularity, groupBy, filterResult, sort, limit, next, paginate, output, pretty, fetch)
		},
	}
}

func runDetailsMetrics(ctx context.Context, d *shared.Deps, name string, fs *flag.FlagSet, detailID *string, granularity *string, groupBy *string, filterResult *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool, fetch func(client *asc.Client, ctx context.Context, id string, opts ...asc.GCMatchmakingMetricsOption) (*asc.GameCenterMetricsResponse, error)) error {
	if err := d.ValidatePaginationFlags(ctx, fs, "game-center details metrics "+name, 200, "granularity", "group-by", "filter-result", "sort"); err != nil {
		return err
	}

	id := strings.TrimSpace(*detailID)
	if id == "" && strings.TrimSpace(*next) == "" {
		return d.RequiredFlag("--id")
	}
	gran := strings.TrimSpace(*granularity)
	if gran == "" && strings.TrimSpace(*next) == "" {
		return d.RequiredFlag("--granularity")
	}

	client, err := d.Client()
	if err != nil {
		return fmt.Errorf("game-center details metrics %s: %w", name, err)
	}

	requestCtx, cancel := d.Timeout(ctx)
	defer cancel()

	opts := []asc.GCMatchmakingMetricsOption{
		asc.WithGCMatchmakingMetricsGranularity(gran),
		asc.WithGCMatchmakingMetricsGroupBy(shared.SplitCSV(*groupBy)),
		asc.WithGCMatchmakingMetricsFilterResult(strings.TrimSpace(*filterResult)),
		asc.WithGCMatchmakingMetricsSort(shared.SplitCSV(*sort)),
		asc.Limit[asc.GCMatchmakingMetricsOption](*limit),
		asc.NextURL[asc.GCMatchmakingMetricsOption](*next),
	}

	if *paginate {
		paginateOpts := append(opts, asc.Limit[asc.GCMatchmakingMetricsOption](200))
		firstPage, err := fetch(client, requestCtx, id, paginateOpts...)
		if err != nil {
			return fmt.Errorf("game-center details metrics %s: failed to fetch: %w", name, err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return fetch(client, ctx, id, asc.NextURL[asc.GCMatchmakingMetricsOption](nextURL))
		})
		if err != nil {
			return fmt.Errorf("game-center details metrics %s: %w", name, err)
		}

		return d.PrintOutput(resp, *output, *pretty)
	}

	resp, err := fetch(client, requestCtx, id, opts...)
	if err != nil {
		return fmt.Errorf("game-center details metrics %s: failed to fetch: %w", name, err)
	}

	return d.PrintOutput(resp, *output, *pretty)
}
