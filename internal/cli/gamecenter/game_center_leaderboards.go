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

// GameCenterLeaderboardsCommand returns the leaderboards command group.
func GameCenterLeaderboardsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboards", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards",
		ShortUsage: "asc game-center leaderboards <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboards.",
		LongHelp: `Manage Game Center leaderboards.

Examples:
  asc game-center leaderboards list --app "APP_ID"
  asc game-center leaderboards get --id "LEADERBOARD_ID"
  asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
  asc game-center leaderboards update --id "LEADERBOARD_ID" --reference-name "New Name"
  asc game-center leaderboards delete --id "LEADERBOARD_ID" --confirm
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID"
  asc game-center leaderboards group-leaderboard get --id "LEADERBOARD_ID"
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases create --app "APP_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardsListCommand(),
			GameCenterLeaderboardsGetCommand(),
			GameCenterLeaderboardsCreateCommand(),
			GameCenterLeaderboardsUpdateCommand(),
			GameCenterLeaderboardsDeleteCommand(),
			GameCenterLeaderboardsSubmitCommand(),
			GameCenterLeaderboardGroupLeaderboardCommand(),
			GameCenterLeaderboardsV2Command(),
			GameCenterLeaderboardLocalizationsCommand(),
			GameCenterLeaderboardReleasesCommand(),
			GameCenterLeaderboardImagesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardImagesCommand returns the leaderboard images command group.
func GameCenterLeaderboardImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center leaderboards images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard images.",
		LongHelp: `Manage Game Center leaderboard images.

Images are attached to leaderboard localizations. Use the localization ID when uploading.

Examples:
  asc game-center leaderboards images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboards images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardImagesUploadCommand(),
			GameCenterLeaderboardImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardImagesUploadCommand returns the leaderboard images upload subcommand.
func GameCenterLeaderboardImagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Game Center leaderboard localization ID")
	filePath := fs.String("file", "", "Path to image file")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center leaderboards images upload --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a Game Center leaderboard localization.",
		LongHelp: `Upload an image for a Game Center leaderboard localization.

This command performs the full upload flow: reserves the upload, uploads the file, and commits.

Examples:
  asc game-center leaderboards images upload --localization-id "LOC_ID" --file leaderboard.png`,
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
				return fmt.Errorf("game-center leaderboards images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardImage(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboards images upload: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardImagesDeleteCommand returns the leaderboard images delete subcommand.
func GameCenterLeaderboardImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Game Center leaderboard image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard image.",
		LongHelp: `Delete a Game Center leaderboard image.

Examples:
  asc game-center leaderboards images delete --id "IMAGE_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboards images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardImage(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboards images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardsListCommand returns the leaderboards list subcommand.
func GameCenterLeaderboardsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards list [flags]",
		ShortHelp:  "List Game Center leaderboards for an app.",
		LongHelp: `List Game Center leaderboards for an app.

Examples:
  asc game-center leaderboards list --app "APP_ID"
  asc game-center leaderboards list --app "APP_ID" --limit 50
  asc game-center leaderboards list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboards list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboards list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCLeaderboardsOption{
				asc.WithGCLeaderboardsLimit(*limit),
				asc.WithGCLeaderboardsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboards(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboards(ctx, gcDetailID, asc.WithGCLeaderboardsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboards(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardsGetCommand returns the leaderboards get subcommand.
func GameCenterLeaderboardsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	leaderboardID := fs.String("id", "", "Game Center leaderboard ID")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards get --id \"LEADERBOARD_ID\" [--v2]",
		ShortHelp:  "Get a Game Center leaderboard by ID.",
		LongHelp: `Get a Game Center leaderboard by ID.

Examples:
  asc game-center leaderboards get --id "LEADERBOARD_ID"
  asc game-center leaderboards get --id "LEADERBOARD_ID" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*leaderboardID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.GameCenterLeaderboardResponse
			if *v2 {
				resp, err = client.GetGameCenterLeaderboardV2(requestCtx, id)
			} else {
				resp, err = client.GetGameCenterLeaderboard(requestCtx, id)
			}
			if err != nil {
				return fmt.Errorf("game-center leaderboards get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardsCreateCommand returns the leaderboards create subcommand.
func GameCenterLeaderboardsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.leaderboard)")
	formatter := fs.String("formatter", "", "Score formatter: INTEGER, DECIMAL_POINT_1_PLACE, DECIMAL_POINT_2_PLACE, DECIMAL_POINT_3_PLACE, ELAPSED_TIME_MILLISECOND, ELAPSED_TIME_SECOND, ELAPSED_TIME_MINUTE, MONEY_WHOLE, MONEY_POINT_2_PLACE")
	sortType := fs.String("sort", "", "Score sort type: ASC, DESC")
	submissionType := fs.String("submission-type", "", "Submission type: BEST_SCORE, MOST_RECENT_SCORE")
	scoreRangeStart := fs.String("score-range-start", "", "Score range start (optional)")
	scoreRangeEnd := fs.String("score-range-end", "", "Score range end (optional)")
	groupID := fs.String("group-id", "", "Game Center group ID (v2 only)")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard.",
		LongHelp: `Create a new Game Center leaderboard.

Examples:
  asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
  asc game-center leaderboards create --app "APP_ID" --reference-name "Time Trial" --vendor-id "com.example.timetrial" --formatter ELAPSED_TIME_MILLISECOND --sort ASC --submission-type BEST_SCORE
  asc game-center leaderboards create --group-id "GROUP_ID" --reference-name "Group Score" --vendor-id "grp.com.example.groupscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE --v2`,
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

			formatterVal := strings.TrimSpace(strings.ToUpper(*formatter))
			if formatterVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --formatter is required")
				return flag.ErrHelp
			}
			if !isValidLeaderboardFormatter(formatterVal) {
				fmt.Fprintf(os.Stderr, "Error: --formatter must be one of: %s\n", strings.Join(asc.ValidLeaderboardFormatters, ", "))
				return flag.ErrHelp
			}

			sortVal := strings.TrimSpace(strings.ToUpper(*sortType))
			if sortVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --sort is required")
				return flag.ErrHelp
			}
			if !isValidScoreSortType(sortVal) {
				fmt.Fprintf(os.Stderr, "Error: --sort must be one of: %s\n", strings.Join(asc.ValidScoreSortTypes, ", "))
				return flag.ErrHelp
			}

			submissionVal := strings.TrimSpace(strings.ToUpper(*submissionType))
			if submissionVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --submission-type is required")
				return flag.ErrHelp
			}
			if !isValidSubmissionType(submissionVal) {
				fmt.Fprintf(os.Stderr, "Error: --submission-type must be one of: %s\n", strings.Join(asc.ValidSubmissionTypes, ", "))
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards create: failed to get Game Center detail: %w", err)
				}
			}

			attrs := asc.GameCenterLeaderboardCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
				DefaultFormatter: formatterVal,
				ScoreSortType:    sortVal,
				SubmissionType:   submissionVal,
				ScoreRangeStart:  strings.TrimSpace(*scoreRangeStart),
				ScoreRangeEnd:    strings.TrimSpace(*scoreRangeEnd),
			}

			useV2 := *v2 || group != ""
			var resp *asc.GameCenterLeaderboardResponse
			if useV2 {
				resp, err = client.CreateGameCenterLeaderboardV2(requestCtx, gcDetailID, group, attrs)
			} else {
				resp, err = client.CreateGameCenterLeaderboard(requestCtx, gcDetailID, attrs)
			}
			if err != nil {
				return fmt.Errorf("game-center leaderboards create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardsUpdateCommand returns the leaderboards update subcommand.
func GameCenterLeaderboardsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	leaderboardID := fs.String("id", "", "Game Center leaderboard ID")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard")
	archived := fs.String("archived", "", "Archive the leaderboard (true/false)")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboards update [flags]",
		ShortHelp:  "Update a Game Center leaderboard.",
		LongHelp: `Update a Game Center leaderboard.

Examples:
  asc game-center leaderboards update --id "LEADERBOARD_ID" --reference-name "New Name"
  asc game-center leaderboards update --id "LEADERBOARD_ID" --archived true
  asc game-center leaderboards update --id "LEADERBOARD_ID" --archived true --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*leaderboardID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterLeaderboardUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				name := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &name
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
				return fmt.Errorf("game-center leaderboards update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			var resp *asc.GameCenterLeaderboardResponse
			if *v2 {
				resp, err = client.UpdateGameCenterLeaderboardV2(requestCtx, id, attrs)
			} else {
				resp, err = client.UpdateGameCenterLeaderboard(requestCtx, id, attrs)
			}
			if err != nil {
				return fmt.Errorf("game-center leaderboards update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardsDeleteCommand returns the leaderboards delete subcommand.
func GameCenterLeaderboardsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	leaderboardID := fs.String("id", "", "Game Center leaderboard ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards delete --id \"LEADERBOARD_ID\" --confirm [--v2]",
		ShortHelp:  "Delete a Game Center leaderboard.",
		LongHelp: `Delete a Game Center leaderboard.

Examples:
  asc game-center leaderboards delete --id "LEADERBOARD_ID" --confirm
  asc game-center leaderboards delete --id "LEADERBOARD_ID" --confirm --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*leaderboardID)
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
				return fmt.Errorf("game-center leaderboards delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if *v2 {
				if err := client.DeleteGameCenterLeaderboardV2(requestCtx, id); err != nil {
					return fmt.Errorf("game-center leaderboards delete: failed to delete: %w", err)
				}
			} else {
				if err := client.DeleteGameCenterLeaderboard(requestCtx, id); err != nil {
					return fmt.Errorf("game-center leaderboards delete: failed to delete: %w", err)
				}
			}

			result := &asc.GameCenterLeaderboardDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardsSubmitCommand submits a leaderboard entry.
func GameCenterLeaderboardsSubmitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)

	vendorID := fs.String("vendor-id", "", "Leaderboard vendor identifier")
	score := fs.String("score", "", "Score value")
	bundleID := fs.String("bundle-id", "", "App bundle ID")
	scopedPlayerID := fs.String("scoped-player-id", "", "Scoped player ID")
	scoreContext := fs.String("context", "", "Optional score context value")
	challengeIDs := fs.String("challenge-ids", "", "Challenge ID(s), comma-separated")
	submittedDate := fs.String("submitted-date", "", "Submission date (RFC3339)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc game-center leaderboards submit --vendor-id \"VENDOR_ID\" --score \"100\" --bundle-id \"BUNDLE_ID\" --scoped-player-id \"PLAYER_ID\"",
		ShortHelp:  "Submit a leaderboard entry.",
		LongHelp: `Submit a leaderboard entry.

Examples:
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID"
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --challenge-ids "CHALLENGE_ID"
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --submitted-date "2025-01-10T12:34:56Z"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vendorValue := strings.TrimSpace(*vendorID)
			if vendorValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --vendor-id is required")
				return flag.ErrHelp
			}
			scoreValue := strings.TrimSpace(*score)
			if scoreValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --score is required")
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
				return fmt.Errorf("game-center leaderboards submit: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.GameCenterLeaderboardEntrySubmissionAttributes{
				VendorIdentifier: vendorValue,
				Score:            scoreValue,
				BundleID:         bundleValue,
				ScopedPlayerID:   playerValue,
				ChallengeIDs:     shared.SplitCSV(*challengeIDs),
			}
			if value := strings.TrimSpace(*scoreContext); value != "" {
				attrs.Context = &value
			}
			if value := strings.TrimSpace(*submittedDate); value != "" {
				attrs.SubmittedDate = &value
			}

			resp, err := client.CreateGameCenterLeaderboardEntrySubmission(requestCtx, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboards submit: failed to submit: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardGroupLeaderboardCommand returns the group leaderboard command group.
func GameCenterLeaderboardGroupLeaderboardCommand() *ffcli.Command {
	fs := flag.NewFlagSet("group-leaderboard", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group-leaderboard",
		ShortUsage: "asc game-center leaderboards group-leaderboard get --id \"LEADERBOARD_ID\"",
		ShortHelp:  "Get the group leaderboard for a leaderboard.",
		LongHelp: `Get the group leaderboard for a Game Center leaderboard.

Examples:
  asc game-center leaderboards group-leaderboard get --id "LEADERBOARD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardGroupLeaderboardGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardGroupLeaderboardGetCommand returns the group leaderboard get subcommand.
func GameCenterLeaderboardGroupLeaderboardGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	leaderboardID := fs.String("id", "", "Game Center leaderboard ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboards group-leaderboard get --id \"LEADERBOARD_ID\"",
		ShortHelp:  "Get a group leaderboard by leaderboard ID.",
		LongHelp: `Get a group leaderboard by leaderboard ID.

Examples:
  asc game-center leaderboards group-leaderboard get --id "LEADERBOARD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*leaderboardID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards group-leaderboard get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardGroupLeaderboard(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboards group-leaderboard get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardReleasesCommand returns the releases command group.
func GameCenterLeaderboardReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center leaderboards releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard releases.",
		LongHelp: `Manage Game Center leaderboard releases.

Examples:
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases create --app "APP_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardReleasesListCommand(),
			GameCenterLeaderboardReleasesCreateCommand(),
			GameCenterLeaderboardReleasesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardReleasesListCommand returns the releases list subcommand.
func GameCenterLeaderboardReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards releases list --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "List releases for a Game Center leaderboard.",
		LongHelp: `List releases for a Game Center leaderboard.

Examples:
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID" --limit 50
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboards releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboards releases list: %w", err)
			}

			lbID := strings.TrimSpace(*leaderboardID)
			if lbID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardReleasesOption{
				asc.WithGCLeaderboardReleasesLimit(*limit),
				asc.WithGCLeaderboardReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardReleasesLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardReleases(requestCtx, lbID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardReleases(ctx, lbID, asc.WithGCLeaderboardReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboardReleases(requestCtx, lbID, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardReleasesCreateCommand returns the releases create subcommand.
func GameCenterLeaderboardReleasesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards releases create --app \"APP_ID\" --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "Create a release for a Game Center leaderboard.",
		LongHelp: `Create a release for a Game Center leaderboard.

A release associates a leaderboard with a Game Center detail, making it live.

Examples:
  asc game-center leaderboards releases create --app "APP_ID" --leaderboard-id "LEADERBOARD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			lbID := strings.TrimSpace(*leaderboardID)
			if lbID == "" {
				fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboards releases create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Get Game Center detail ID first
			gcDetailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center leaderboards releases create: failed to get Game Center detail: %w", err)
			}

			resp, err := client.CreateGameCenterLeaderboardRelease(requestCtx, gcDetailID, lbID)
			if err != nil {
				return fmt.Errorf("game-center leaderboards releases create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardReleasesDeleteCommand returns the releases delete subcommand.
func GameCenterLeaderboardReleasesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	releaseID := fs.String("id", "", "Game Center leaderboard release ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard release.",
		LongHelp: `Delete a Game Center leaderboard release.

Examples:
  asc game-center leaderboards releases delete --id "RELEASE_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboards releases delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardRelease(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboards releases delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardReleaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
