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

// GameCenterAchievementsCommand returns the achievements command group.
func GameCenterAchievementsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("achievements", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievements",
		ShortUsage: "asc game-center achievements <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievements.",
		LongHelp: `Manage Game Center achievements.

Examples:
  asc game-center achievements list --app "APP_ID"
  asc game-center achievements get --id "ACHIEVEMENT_ID"
  asc game-center achievements group-achievement get --id "ACHIEVEMENT_ID"
  asc game-center achievements create --app "APP_ID" --reference-name "First Win" --vendor-id "com.example.firstwin" --points 10
  asc game-center achievements update --id "ACHIEVEMENT_ID" --points 20
  asc game-center achievements delete --id "ACHIEVEMENT_ID" --confirm
  asc game-center achievements submit --vendor-id "com.example.achievement" --percentage 100 --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID"
  asc game-center achievements localizations list --achievement-id "ACHIEVEMENT_ID"
  asc game-center achievements localizations create --achievement-id "ACHIEVEMENT_ID" --locale en-US --name "First Win" --before-earned-description "Win your first game" --after-earned-description "You won!"
  asc game-center achievements localizations update --id "LOC_ID" --name "New Name"
  asc game-center achievements localizations delete --id "LOC_ID" --confirm
  asc game-center achievements localizations image get --id "LOC_ID"
  asc game-center achievements localizations achievement get --id "LOC_ID"
  asc game-center achievements images upload --localization-id "LOC_ID" --file "path/to/image.png"
  asc game-center achievements images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementsListCommand(),
			GameCenterAchievementsGetCommand(),
			GameCenterAchievementGroupAchievementCommand(),
			GameCenterAchievementsCreateCommand(),
			GameCenterAchievementsUpdateCommand(),
			GameCenterAchievementsDeleteCommand(),
			GameCenterAchievementsSubmitCommand(),
			GameCenterAchievementsV2Command(),
			GameCenterAchievementLocalizationsCommand(),
			GameCenterAchievementImagesCommand(),
			GameCenterAchievementReleasesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementsListCommand returns the achievements list subcommand.
func GameCenterAchievementsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center achievements list [flags]",
		ShortHelp:  "List Game Center achievements for an app.",
		LongHelp: `List Game Center achievements for an app.

Examples:
  asc game-center achievements list --app "APP_ID"
  asc game-center achievements list --app "APP_ID" --limit 50
  asc game-center achievements list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center achievements list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center achievements list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center achievements list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCAchievementsOption{
				asc.WithGCAchievementsLimit(*limit),
				asc.WithGCAchievementsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementsLimit(200))
				firstPage, err := client.GetGameCenterAchievements(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievements(ctx, gcDetailID, asc.WithGCAchievementsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterAchievements(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementsGetCommand returns the achievements get subcommand.
func GameCenterAchievementsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	achievementID := fs.String("id", "", "Game Center achievement ID")
	v2 := fs.Bool("v2", false, "Use v2 achievements endpoint")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements get --id \"ACHIEVEMENT_ID\" [--v2]",
		ShortHelp:  "Get a Game Center achievement by ID.",
		LongHelp: `Get a Game Center achievement by ID.

Examples:
  asc game-center achievements get --id "ACHIEVEMENT_ID"
  asc game-center achievements get --id "ACHIEVEMENT_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*achievementID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.GameCenterAchievementResponse
			if *v2 {
				resp, err = client.GetGameCenterAchievementV2(requestCtx, id)
			} else {
				resp, err = client.GetGameCenterAchievement(requestCtx, id)
			}
			if err != nil {
				return fmt.Errorf("game-center achievements get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementsCreateCommand returns the achievements create subcommand.
func GameCenterAchievementsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the achievement")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.achievement)")
	points := fs.Int("points", 0, "Points value (1-100)")
	showBeforeEarned := fs.Bool("show-before-earned", true, "Show achievement before it is earned")
	repeatable := fs.Bool("repeatable", false, "Achievement can be earned multiple times")
	groupID := fs.String("group-id", "", "Game Center group ID (v2 only)")
	v2 := fs.Bool("v2", false, "Use v2 achievements endpoint")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center achievements create [flags]",
		ShortHelp:  "Create a new Game Center achievement.",
		LongHelp: `Create a new Game Center achievement.

Examples:
  asc game-center achievements create --app "APP_ID" --reference-name "First Win" --vendor-id "com.example.firstwin" --points 10
  asc game-center achievements create --app "APP_ID" --reference-name "Master" --vendor-id "com.example.master" --points 100 --repeatable
  asc game-center achievements create --group-id "GROUP_ID" --reference-name "Group Win" --vendor-id "grp.com.example.groupwin" --points 10 --v2`,
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

			if *points < 1 || *points > 100 {
				fmt.Fprintln(os.Stderr, "Error: --points must be between 1 and 100")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center achievements create: failed to get Game Center detail: %w", err)
				}
			}

			attrs := asc.GameCenterAchievementCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
				Points:           *points,
				ShowBeforeEarned: *showBeforeEarned,
				Repeatable:       *repeatable,
			}

			useV2 := *v2 || group != ""
			var resp *asc.GameCenterAchievementResponse
			if useV2 {
				resp, err = client.CreateGameCenterAchievementV2(requestCtx, gcDetailID, group, attrs)
			} else {
				resp, err = client.CreateGameCenterAchievement(requestCtx, gcDetailID, attrs)
			}
			if err != nil {
				return fmt.Errorf("game-center achievements create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementsUpdateCommand returns the achievements update subcommand.
func GameCenterAchievementsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	achievementID := fs.String("id", "", "Game Center achievement ID")
	referenceName := fs.String("reference-name", "", "Reference name for the achievement")
	points := fs.Int("points", 0, "Points value (1-100)")
	showBeforeEarned := fs.String("show-before-earned", "", "Show achievement before it is earned (true/false)")
	repeatable := fs.String("repeatable", "", "Achievement can be earned multiple times (true/false)")
	archived := fs.String("archived", "", "Archive the achievement (true/false)")
	v2 := fs.Bool("v2", false, "Use v2 achievements endpoint")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center achievements update [flags]",
		ShortHelp:  "Update a Game Center achievement.",
		LongHelp: `Update a Game Center achievement.

Examples:
  asc game-center achievements update --id "ACHIEVEMENT_ID" --reference-name "New Name"
  asc game-center achievements update --id "ACHIEVEMENT_ID" --points 20
  asc game-center achievements update --id "ACHIEVEMENT_ID" --archived true
  asc game-center achievements update --id "ACHIEVEMENT_ID" --points 20 --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*achievementID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterAchievementUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				name := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &name
				hasUpdate = true
			}

			if *points != 0 {
				if *points < 1 || *points > 100 {
					fmt.Fprintln(os.Stderr, "Error: --points must be between 1 and 100")
					return flag.ErrHelp
				}
				attrs.Points = points
				hasUpdate = true
			}

			if strings.TrimSpace(*showBeforeEarned) != "" {
				val, err := parseBool(*showBeforeEarned, "--show-before-earned")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.ShowBeforeEarned = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*repeatable) != "" {
				val, err := parseBool(*repeatable, "--repeatable")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Repeatable = &val
				hasUpdate = true
			}

			if strings.TrimSpace(*archived) != "" {
				val, err := parseBool(*archived, "--archived")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Archived = &val
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.GameCenterAchievementResponse
			if *v2 {
				resp, err = client.UpdateGameCenterAchievementV2(requestCtx, id, attrs)
			} else {
				resp, err = client.UpdateGameCenterAchievement(requestCtx, id, attrs)
			}
			if err != nil {
				return fmt.Errorf("game-center achievements update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementsDeleteCommand returns the achievements delete subcommand.
func GameCenterAchievementsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	achievementID := fs.String("id", "", "Game Center achievement ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	v2 := fs.Bool("v2", false, "Use v2 achievements endpoint")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center achievements delete --id \"ACHIEVEMENT_ID\" --confirm [--v2]",
		ShortHelp:  "Delete a Game Center achievement.",
		LongHelp: `Delete a Game Center achievement.

Examples:
  asc game-center achievements delete --id "ACHIEVEMENT_ID" --confirm
  asc game-center achievements delete --id "ACHIEVEMENT_ID" --confirm --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*achievementID)
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
				return fmt.Errorf("game-center achievements delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *v2 {
				if err := client.DeleteGameCenterAchievementV2(requestCtx, id); err != nil {
					return fmt.Errorf("game-center achievements delete: failed to delete: %w", err)
				}
			} else {
				if err := client.DeleteGameCenterAchievement(requestCtx, id); err != nil {
					return fmt.Errorf("game-center achievements delete: failed to delete: %w", err)
				}
			}

			result := &asc.GameCenterAchievementDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterAchievementsSubmitCommand submits a player achievement.
func GameCenterAchievementsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	vendorID := fs.String("vendor-id", "", "Achievement vendor identifier")
	percentage := fs.Int("percentage", -1, "Percentage achieved (0-100)")
	bundleID := fs.String("bundle-id", "", "App bundle ID")
	scopedPlayerID := fs.String("scoped-player-id", "", "Scoped player ID")
	challengeIDs := fs.String("challenge-ids", "", "Challenge ID(s), comma-separated")
	submittedDate := fs.String("submitted-date", "", "Submission date (RFC3339)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc game-center achievements submit --vendor-id \"VENDOR_ID\" --percentage 100 --bundle-id \"BUNDLE_ID\" --scoped-player-id \"PLAYER_ID\"",
		ShortHelp:  "Submit a player achievement.",
		LongHelp: `Submit a player achievement.

Examples:
  asc game-center achievements submit --vendor-id "com.example.achievement" --percentage 100 --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID"
  asc game-center achievements submit --vendor-id "com.example.achievement" --percentage 50 --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --challenge-ids "CHALLENGE_ID"
  asc game-center achievements submit --vendor-id "com.example.achievement" --percentage 100 --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --submitted-date "2025-01-10T12:34:56Z"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vendorValue := strings.TrimSpace(*vendorID)
			if vendorValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --vendor-id is required")
				return flag.ErrHelp
			}
			if *percentage < 0 {
				fmt.Fprintln(os.Stderr, "Error: --percentage is required")
				return flag.ErrHelp
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --bundle-id is required")
				return flag.ErrHelp
			}
			playerValue := strings.TrimSpace(*scopedPlayerID)
			if playerValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --scoped-player-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.GameCenterPlayerAchievementSubmissionAttributes{
				VendorIdentifier:   vendorValue,
				PercentageAchieved: *percentage,
				BundleID:           bundleValue,
				ScopedPlayerID:     playerValue,
				ChallengeIDs:       shared.SplitCSV(*challengeIDs),
			}
			if value := strings.TrimSpace(*submittedDate); value != "" {
				attrs.SubmittedDate = &value
			}

			resp, err := client.CreateGameCenterPlayerAchievementSubmission(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("game-center achievements submit: failed to submit: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationsCommand returns the achievement localizations command group.
func GameCenterAchievementLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center achievements localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement localizations.",
		LongHelp: `Manage Game Center achievement localizations.

Examples:
  asc game-center achievements localizations list --achievement-id "ACHIEVEMENT_ID"
  asc game-center achievements localizations get --id "LOC_ID"
  asc game-center achievements localizations create --achievement-id "ACHIEVEMENT_ID" --locale en-US --name "First Win" --before-earned-description "Win your first game" --after-earned-description "You won!"
  asc game-center achievements localizations update --id "LOC_ID" --name "New Name"
  asc game-center achievements localizations delete --id "LOC_ID" --confirm
  asc game-center achievements localizations image get --id "LOC_ID"
  asc game-center achievements localizations achievement get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementLocalizationsListCommand(),
			GameCenterAchievementLocalizationsGetCommand(),
			GameCenterAchievementLocalizationsCreateCommand(),
			GameCenterAchievementLocalizationsUpdateCommand(),
			GameCenterAchievementLocalizationsDeleteCommand(),
			GameCenterAchievementLocalizationImageCommand(),
			GameCenterAchievementLocalizationAchievementCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementLocalizationsListCommand returns the localizations list subcommand.
func GameCenterAchievementLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	achievementID := fs.String("achievement-id", "", "Game Center achievement ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center achievements localizations list --achievement-id \"ACHIEVEMENT_ID\"",
		ShortHelp:  "List localizations for a Game Center achievement.",
		LongHelp: `List localizations for a Game Center achievement.

Examples:
  asc game-center achievements localizations list --achievement-id "ACHIEVEMENT_ID"
  asc game-center achievements localizations list --achievement-id "ACHIEVEMENT_ID" --limit 50
  asc game-center achievements localizations list --achievement-id "ACHIEVEMENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center achievements localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center achievements localizations list: %w", err)
			}

			achID := strings.TrimSpace(*achievementID)
			if achID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --achievement-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementLocalizationsOption{
				asc.WithGCAchievementLocalizationsLimit(*limit),
				asc.WithGCAchievementLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterAchievementLocalizations(requestCtx, achID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievementLocalizations(ctx, achID, asc.WithGCAchievementLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterAchievementLocalizations(requestCtx, achID, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationsGetCommand returns the localizations get subcommand.
func GameCenterAchievementLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center achievement localization by ID.",
		LongHelp: `Get a Game Center achievement localization by ID.

Examples:
  asc game-center achievements localizations get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center achievements localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationsCreateCommand returns the localizations create subcommand.
func GameCenterAchievementLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	achievementID := fs.String("achievement-id", "", "Game Center achievement ID")
	locale := fs.String("locale", "", "Locale code (e.g., en-US)")
	name := fs.String("name", "", "Display name")
	beforeEarnedDescription := fs.String("before-earned-description", "", "Description shown before achievement is earned")
	afterEarnedDescription := fs.String("after-earned-description", "", "Description shown after achievement is earned")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center achievements localizations create [flags]",
		ShortHelp:  "Create a new Game Center achievement localization.",
		LongHelp: `Create a new Game Center achievement localization.

Examples:
  asc game-center achievements localizations create --achievement-id "ACHIEVEMENT_ID" --locale en-US --name "First Win" --before-earned-description "Win your first game" --after-earned-description "You won!"
  asc game-center achievements localizations create --achievement-id "ACHIEVEMENT_ID" --locale de-DE --name "Erster Sieg" --before-earned-description "Gewinne dein erstes Spiel" --after-earned-description "Du hast gewonnen!"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			achID := strings.TrimSpace(*achievementID)
			if achID == "" {
				fmt.Fprintln(os.Stderr, "Error: --achievement-id is required")
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

			beforeVal := strings.TrimSpace(*beforeEarnedDescription)
			if beforeVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --before-earned-description is required")
				return flag.ErrHelp
			}

			afterVal := strings.TrimSpace(*afterEarnedDescription)
			if afterVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --after-earned-description is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.GameCenterAchievementLocalizationCreateAttributes{
				Locale:                  localeVal,
				Name:                    nameVal,
				BeforeEarnedDescription: beforeVal,
				AfterEarnedDescription:  afterVal,
			}

			resp, err := client.CreateGameCenterAchievementLocalization(requestCtx, achID, attrs)
			if err != nil {
				return fmt.Errorf("game-center achievements localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationsUpdateCommand returns the localizations update subcommand.
func GameCenterAchievementLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	name := fs.String("name", "", "Display name")
	beforeEarnedDescription := fs.String("before-earned-description", "", "Description shown before achievement is earned")
	afterEarnedDescription := fs.String("after-earned-description", "", "Description shown after achievement is earned")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center achievements localizations update [flags]",
		ShortHelp:  "Update a Game Center achievement localization.",
		LongHelp: `Update a Game Center achievement localization.

Examples:
  asc game-center achievements localizations update --id "LOC_ID" --name "New Name"
  asc game-center achievements localizations update --id "LOC_ID" --before-earned-description "Win a game" --after-earned-description "Winner!"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterAchievementLocalizationUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				nameVal := strings.TrimSpace(*name)
				attrs.Name = &nameVal
				hasUpdate = true
			}

			if strings.TrimSpace(*beforeEarnedDescription) != "" {
				beforeVal := strings.TrimSpace(*beforeEarnedDescription)
				attrs.BeforeEarnedDescription = &beforeVal
				hasUpdate = true
			}

			if strings.TrimSpace(*afterEarnedDescription) != "" {
				afterVal := strings.TrimSpace(*afterEarnedDescription)
				attrs.AfterEarnedDescription = &afterVal
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterAchievementLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center achievements localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationsDeleteCommand returns the localizations delete subcommand.
func GameCenterAchievementLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center achievements localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement localization.",
		LongHelp: `Delete a Game Center achievement localization.

Examples:
  asc game-center achievements localizations delete --id "LOC_ID" --confirm`,
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
				return fmt.Errorf("game-center achievements localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterAchievementLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("game-center achievements localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterAchievementLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterAchievementReleasesCommand returns the achievement releases command group.
func GameCenterAchievementReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center achievements releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement releases.",
		LongHelp: `Manage Game Center achievement releases. Releases are used to publish achievements to live.

Examples:
  asc game-center achievements releases list --achievement-id "ACHIEVEMENT_ID"
  asc game-center achievements releases create --app "APP_ID" --achievement-id "ACHIEVEMENT_ID"
  asc game-center achievements releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementReleasesListCommand(),
			GameCenterAchievementReleasesCreateCommand(),
			GameCenterAchievementReleasesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementReleasesListCommand returns the achievement releases list subcommand.
func GameCenterAchievementReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	achievementID := fs.String("achievement-id", "", "Game Center achievement ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center achievements releases list --achievement-id \"ACHIEVEMENT_ID\"",
		ShortHelp:  "List releases for a Game Center achievement.",
		LongHelp: `List releases for a Game Center achievement.

Examples:
  asc game-center achievements releases list --achievement-id "ACHIEVEMENT_ID"
  asc game-center achievements releases list --achievement-id "ACHIEVEMENT_ID" --limit 50
  asc game-center achievements releases list --achievement-id "ACHIEVEMENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center achievements releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center achievements releases list: %w", err)
			}

			id := strings.TrimSpace(*achievementID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --achievement-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCAchievementReleasesOption{
				asc.WithGCAchievementReleasesLimit(*limit),
				asc.WithGCAchievementReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCAchievementReleasesLimit(200))
				firstPage, err := client.GetGameCenterAchievementReleases(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center achievements releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterAchievementReleases(ctx, id, asc.WithGCAchievementReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center achievements releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterAchievementReleases(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center achievements releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementReleasesCreateCommand returns the achievement releases create subcommand.
func GameCenterAchievementReleasesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	achievementID := fs.String("achievement-id", "", "Game Center achievement ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center achievements releases create --app \"APP_ID\" --achievement-id \"ACHIEVEMENT_ID\"",
		ShortHelp:  "Create a new Game Center achievement release.",
		LongHelp: `Create a new Game Center achievement release. This publishes the achievement to live.

Examples:
  asc game-center achievements releases create --app "APP_ID" --achievement-id "ACHIEVEMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*achievementID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --achievement-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements releases create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Get Game Center detail ID first
			gcDetailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center achievements releases create: failed to get Game Center detail: %w", err)
			}

			resp, err := client.CreateGameCenterAchievementRelease(requestCtx, gcDetailID, id)
			if err != nil {
				return fmt.Errorf("game-center achievements releases create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementReleasesDeleteCommand returns the achievement releases delete subcommand.
func GameCenterAchievementReleasesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	releaseID := fs.String("id", "", "Game Center achievement release ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center achievements releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement release.",
		LongHelp: `Delete a Game Center achievement release.

Examples:
  asc game-center achievements releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*releaseID)
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
				return fmt.Errorf("game-center achievements releases delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterAchievementRelease(requestCtx, id); err != nil {
				return fmt.Errorf("game-center achievements releases delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterAchievementReleaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterAchievementImagesCommand returns the achievement images command group.
func GameCenterAchievementImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center achievements images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center achievement images.",
		LongHelp: `Manage Game Center achievement images. Images are attached to achievement localizations.

Examples:
  asc game-center achievements images upload --localization-id "LOC_ID" --file "path/to/image.png"
  asc game-center achievements images get --id "IMAGE_ID"
  asc game-center achievements images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementImagesUploadCommand(),
			GameCenterAchievementImagesGetCommand(),
			GameCenterAchievementImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementImagesUploadCommand returns the achievement images upload subcommand.
func GameCenterAchievementImagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Game Center achievement localization ID")
	filePath := fs.String("file", "", "Path to the image file to upload")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center achievements images upload --localization-id \"LOC_ID\" --file \"path/to/image.png\"",
		ShortHelp:  "Upload an image for a Game Center achievement localization.",
		LongHelp: `Upload an image for a Game Center achievement localization.

The image file will be validated, reserved, uploaded in chunks, and committed.

Examples:
  asc game-center achievements images upload --localization-id "LOC_ID" --file "path/to/image.png"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			path := strings.TrimSpace(*filePath)
			if path == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterAchievementImage(requestCtx, locID, path)
			if err != nil {
				return fmt.Errorf("game-center achievements images upload: %w", err)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterAchievementImagesGetCommand returns the achievement images get subcommand.
func GameCenterAchievementImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center achievement image ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements images get --id \"IMAGE_ID\"",
		ShortHelp:  "Get a Game Center achievement image by ID.",
		LongHelp: `Get a Game Center achievement image by ID.

Examples:
  asc game-center achievements images get --id "IMAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements images get: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementImagesDeleteCommand returns the achievement images delete subcommand.
func GameCenterAchievementImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center achievement image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center achievements images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center achievement image.",
		LongHelp: `Delete a Game Center achievement image.

Examples:
  asc game-center achievements images delete --id "IMAGE_ID" --confirm`,
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
				return fmt.Errorf("game-center achievements images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterAchievementImage(requestCtx, id); err != nil {
				return fmt.Errorf("game-center achievements images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterAchievementImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterAchievementGroupAchievementCommand returns the group achievement command group.
func GameCenterAchievementGroupAchievementCommand() *ffcli.Command {
	fs := flag.NewFlagSet("group-achievement", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group-achievement",
		ShortUsage: "asc game-center achievements group-achievement get --id \"ACHIEVEMENT_ID\"",
		ShortHelp:  "Get the group achievement for an achievement.",
		LongHelp: `Get the group achievement for a Game Center achievement.

Examples:
  asc game-center achievements group-achievement get --id "ACHIEVEMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementGroupAchievementGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementGroupAchievementGetCommand returns the group achievement get subcommand.
func GameCenterAchievementGroupAchievementGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	achievementID := fs.String("id", "", "Game Center achievement ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements group-achievement get --id \"ACHIEVEMENT_ID\"",
		ShortHelp:  "Get a group achievement by achievement ID.",
		LongHelp: `Get a group achievement by achievement ID.

Examples:
  asc game-center achievements group-achievement get --id "ACHIEVEMENT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*achievementID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center achievements group-achievement get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementGroupAchievement(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements group-achievement get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationImageCommand returns the localization image command group.
func GameCenterAchievementLocalizationImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center achievements localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get the image for an achievement localization.",
		LongHelp: `Get the image for an achievement localization.

Examples:
  asc game-center achievements localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementLocalizationImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementLocalizationImageGetCommand returns the localization image get subcommand.
func GameCenterAchievementLocalizationImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get an achievement localization image.",
		LongHelp: `Get an achievement localization image.

Examples:
  asc game-center achievements localizations image get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center achievements localizations image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementLocalizationImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements localizations image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterAchievementLocalizationAchievementCommand returns the localization achievement command group.
func GameCenterAchievementLocalizationAchievementCommand() *ffcli.Command {
	fs := flag.NewFlagSet("achievement", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "achievement",
		ShortUsage: "asc game-center achievements localizations achievement get --id \"LOC_ID\"",
		ShortHelp:  "Get the achievement for a localization.",
		LongHelp: `Get the achievement for a Game Center achievement localization.

Examples:
  asc game-center achievements localizations achievement get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterAchievementLocalizationAchievementGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterAchievementLocalizationAchievementGetCommand returns the localization achievement get subcommand.
func GameCenterAchievementLocalizationAchievementGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center achievement localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center achievements localizations achievement get --id \"LOC_ID\"",
		ShortHelp:  "Get an achievement for a localization.",
		LongHelp: `Get an achievement for a Game Center achievement localization.

Examples:
  asc game-center achievements localizations achievement get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center achievements localizations achievement get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterAchievementLocalizationAchievement(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center achievements localizations achievement get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
