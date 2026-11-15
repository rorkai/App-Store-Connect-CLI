package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newGameCenterLeaderboardsCommand returns the leaderboards command group.
func newGameCenterLeaderboardsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("leaderboards", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboards",
		ShortUsage: "asc game-center leaderboards <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboards.",
		LongHelp: `Manage Game Center leaderboards.

Examples:
  asc game-center leaderboards list --app "APP_ID"
  asc game-center leaderboards view --id "LEADERBOARD_ID"
  asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
  asc game-center leaderboards update --id "LEADERBOARD_ID" --reference-name "New Name"
  asc game-center leaderboards delete --id "LEADERBOARD_ID" --confirm
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID"
  asc game-center leaderboards group-leaderboard view --id "LEADERBOARD_ID"
  asc game-center leaderboards localizations list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards localizations create --leaderboard-id "LEADERBOARD_ID" --locale en-US --name "High Score"
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases create --app "APP_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardsListCommand(d),
			newGameCenterLeaderboardsGetCommand(d),
			newGameCenterLeaderboardsCreateCommand(d),
			newGameCenterLeaderboardsUpdateCommand(d),
			newGameCenterLeaderboardsDeleteCommand(d),
			newGameCenterLeaderboardsSubmitCommand(d),
			newGameCenterLeaderboardGroupLeaderboardCommand(d),
			newGameCenterLeaderboardsV2Command(d),
			newGameCenterLeaderboardLocalizationsCommand(d),
			newGameCenterLeaderboardReleasesCommand(d),
			newGameCenterLeaderboardImagesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardImagesCommand returns the leaderboard images command group.
func newGameCenterLeaderboardImagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

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
			newGameCenterLeaderboardImagesUploadCommand(d),
			newGameCenterLeaderboardImagesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardImagesUploadCommand returns the leaderboard images upload subcommand.
func newGameCenterLeaderboardImagesUploadCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterLeaderboardLocalizations", "Game Center leaderboard localization ID")
	filePath := fs.String("file", "", "Path to image file")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--localization-id")
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				return d.RequiredFlag("--file")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards images upload: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardImage(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboards images upload: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardImagesDeleteCommand returns the leaderboard images delete subcommand.
func newGameCenterLeaderboardImagesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard image.",
		LongHelp: `Delete a Game Center leaderboard image.

Examples:
  asc game-center leaderboards images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard image ID",
		IDType:             "gameCenterLeaderboardImages",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardImage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardsListCommand returns the leaderboards list subcommand.
func newGameCenterLeaderboardsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

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
			if err := d.ValidatePaginationFlags(ctx, fs, "game-center leaderboards list", 200); err != nil {
				return err
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards list: failed to get Game Center detail: %w", err)
				}
				gcDetailID = strings.TrimSpace(gcDetailID)
				if gcDetailID == "" {
					fmt.Fprintln(d.IO.ErrOut, noGameCenterDetailWarning)
					resp := &asc.GameCenterLeaderboardsResponse{
						Data:  []asc.Resource[asc.GameCenterLeaderboardAttributes]{},
						Links: asc.Links{},
					}
					return d.PrintOutput(resp, *output.Output, *output.Pretty)
				}
			}

			opts := []asc.GCLeaderboardsOption{
				asc.Limit[asc.GCLeaderboardsOption](*limit),
				asc.NextURL[asc.GCLeaderboardsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.GCLeaderboardsOption](200))
				firstPage, err := client.GetGameCenterLeaderboards(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboards list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboards(ctx, gcDetailID, asc.NextURL[asc.GCLeaderboardsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboards list: %w", err)
				}

				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterLeaderboards(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboards list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardsGetCommand returns the leaderboards get subcommand.
func newGameCenterLeaderboardsGetCommand(d *shared.Deps) *ffcli.Command {
	var v2 *bool

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboards view --id \"LEADERBOARD_ID\" [--v2]",
		ShortHelp:  "View a Game Center leaderboard by ID.",
		LongHelp: `View a Game Center leaderboard by ID.

Examples:
  asc game-center leaderboards view --id "LEADERBOARD_ID"
  asc game-center leaderboards view --id "LEADERBOARD_ID" --v2`,
		IDUsage:            "Game Center leaderboard ID",
		IDType:             "gameCenterLeaderboards",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			v2 = fs.Bool("v2", false, "Use v2 leaderboards endpoint")
			return nil
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			if *v2 {
				return client.GetGameCenterLeaderboardV2(ctx, id)
			}
			return client.GetGameCenterLeaderboard(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardsCreateCommand returns the leaderboards create subcommand.
func newGameCenterLeaderboardsCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env); not used with --set-ids")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.leaderboard)")
	formatter := fs.String("formatter", "", "Score formatter: "+strings.Join(asc.ValidLeaderboardFormatters, ", "))
	sortType := fs.String("sort", "", "Score sort type: ASC, DESC")
	submissionType := fs.String("submission-type", "", "Submission type: BEST_SCORE, MOST_RECENT_SCORE")
	scoreRangeStart := fs.String("score-range-start", "", "Score range start (optional)")
	scoreRangeEnd := fs.String("score-range-end", "", "Score range end (optional)")
	bindLeaderboardRecurrenceFlags(fs)
	fs.String("visibility", "", "Leaderboard visibility: "+strings.Join(asc.ValidLeaderboardVisibilities, ", "))
	fs.String("activity-properties", "", "Activity properties as a JSON object of string values, inline or @file:PATH")
	groupID := shared.BindResourceIDFlag(fs, "group-id", "gameCenterGroups", "Game Center group ID (v2 only); not used with --set-ids")
	setIDs := shared.BindOnceCSVFlag(fs, "set-ids", "Comma-separated leaderboard set IDs to add the leaderboard to; it inherits its parent from them")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboards create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard.",
		LongHelp: `Create a new Game Center leaderboard.

V2 creates the required initial leaderboard version inline.

With --set-ids, the leaderboard inherits its Game Center detail or group from the
leaderboard sets, so --app and --group-id are not used.

Recurring leaderboards take all three recurrence flags. --recurrence-duration
uses time components only (PT168H for a week; Apple rejects P7D), and
--recurrence-rule must be FREQ=MINUTELY, HOURLY, or DAILY with ;INTERVAL=N.

Examples:
  asc game-center leaderboards create --app "APP_ID" --reference-name "High Score" --vendor-id "com.example.highscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE
  asc game-center leaderboards create --app "APP_ID" --reference-name "Time Trial" --vendor-id "com.example.timetrial" --formatter ELAPSED_TIME_CENTISECOND --sort ASC --submission-type BEST_SCORE
  asc game-center leaderboards create --app "APP_ID" --reference-name "Weekly Score" --vendor-id "com.example.weeklyscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE --recurrence-start-date "2026-12-01T00:00:00Z" --recurrence-duration "PT168H" --recurrence-rule "FREQ=DAILY;INTERVAL=7"
  asc game-center leaderboards create --app "APP_ID" --reference-name "Ranked" --vendor-id "com.example.ranked" --formatter INTEGER --sort DESC --submission-type BEST_SCORE --visibility HIDE_FOR_ALL --activity-properties '{"mode":"ranked"}'
  asc game-center leaderboards create --group-id "GROUP_ID" --reference-name "Group Score" --vendor-id "grp.com.example.groupscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE --v2
  asc game-center leaderboards create --reference-name "Weekly Score" --vendor-id "com.example.weeklyscore" --formatter INTEGER --sort DESC --submission-type BEST_SCORE --v2 --set-ids "id1,id2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			group := strings.TrimSpace(*groupID)
			if group != "" && strings.TrimSpace(*appID) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --group-id")
				return flag.ErrHelp
			}

			ids := shared.SplitUniqueCSV(setIDs.String())
			if setIDs.Provided() && len(ids) == 0 {
				fmt.Fprintln(d.IO.ErrOut, "Error: --set-ids must include at least one ID")
				return flag.ErrHelp
			}
			// Apple returns 409 when gameCenterLeaderboardSets is sent with gameCenterDetail
			// or gameCenterGroup; the leaderboard inherits its parent from the sets.
			if len(ids) > 0 {
				if strings.TrimSpace(*appID) != "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app cannot be used with --set-ids; the leaderboard inherits its parent from the leaderboard sets")
					return flag.ErrHelp
				}
				if group != "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --group-id cannot be used with --set-ids; the leaderboard inherits its parent from the leaderboard sets")
					return flag.ErrHelp
				}
			}

			resolvedAppID := ""
			if len(ids) == 0 {
				resolvedAppID = shared.ResolveAppID(*appID)
				if group == "" && resolvedAppID == "" {
					fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
					return shared.MissingRequiredUsageError("--app")
				}
			}

			name := strings.TrimSpace(*referenceName)
			if name == "" {
				return d.RequiredFlag("--reference-name")
			}

			vendor := strings.TrimSpace(*vendorID)
			if vendor == "" {
				return d.RequiredFlag("--vendor-id")
			}
			if group != "" && !strings.HasPrefix(vendor, "grp.") {
				fmt.Fprintln(d.IO.ErrOut, "Error: --vendor-id must start with \"grp.\" when using --group-id")
				return flag.ErrHelp
			}

			formatterVal := strings.TrimSpace(strings.ToUpper(*formatter))
			if formatterVal == "" {
				return d.RequiredFlag("--formatter")
			}
			if !isValidLeaderboardFormatter(formatterVal) {
				fmt.Fprintf(d.IO.ErrOut, "Error: --formatter must be one of: %s\n", strings.Join(asc.ValidLeaderboardFormatters, ", "))
				return flag.ErrHelp
			}

			sortVal := strings.TrimSpace(strings.ToUpper(*sortType))
			if sortVal == "" {
				return d.RequiredFlag("--sort")
			}
			if !isValidScoreSortType(sortVal) {
				fmt.Fprintf(d.IO.ErrOut, "Error: --sort must be one of: %s\n", strings.Join(asc.ValidScoreSortTypes, ", "))
				return flag.ErrHelp
			}

			submissionVal := strings.TrimSpace(strings.ToUpper(*submissionType))
			if submissionVal == "" {
				return d.RequiredFlag("--submission-type")
			}
			if !isValidSubmissionType(submissionVal) {
				fmt.Fprintf(d.IO.ErrOut, "Error: --submission-type must be one of: %s\n", strings.Join(asc.ValidSubmissionTypes, ", "))
				return flag.ErrHelp
			}

			recurrence, err := parseLeaderboardRecurrence(d, fs)
			if err != nil {
				return err
			}
			visibility, err := parseLeaderboardVisibilityFlag(d, fs)
			if err != nil {
				return err
			}
			activityProperties, err := parseStringMapFlag(d, fs, "activity-properties")
			if err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" && len(ids) == 0 {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboards create: failed to get Game Center detail: %w", err)
				}
			}

			attrs := asc.GameCenterLeaderboardCreateAttributes{
				ReferenceName:       name,
				VendorIdentifier:    vendor,
				DefaultFormatter:    formatterVal,
				ScoreSortType:       sortVal,
				SubmissionType:      submissionVal,
				ScoreRangeStart:     strings.TrimSpace(*scoreRangeStart),
				ScoreRangeEnd:       strings.TrimSpace(*scoreRangeEnd),
				RecurrenceStartDate: recurrence.startDate,
				RecurrenceDuration:  recurrence.duration,
				RecurrenceRule:      recurrence.rule,
				ActivityProperties:  activityProperties,
				Visibility:          visibility,
			}

			useV2 := *v2 || group != ""
			var resp *asc.GameCenterLeaderboardResponse
			if useV2 {
				resp, err = client.CreateGameCenterLeaderboardV2(requestCtx, gcDetailID, group, attrs, ids)
			} else {
				resp, err = client.CreateGameCenterLeaderboard(requestCtx, gcDetailID, attrs, ids)
			}
			if err != nil {
				return fmt.Errorf("game-center leaderboards create: failed to create: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardsUpdateCommand returns the leaderboards update subcommand.
func newGameCenterLeaderboardsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("update", flag.ExitOnError)

	leaderboardID := shared.BindResourceIDFlag(fs, "id", "gameCenterLeaderboards", "Game Center leaderboard ID")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard")
	archived := fs.String("archived", "", "Archive the leaderboard (true/false)")
	formatter := fs.String("formatter", "", "Score formatter: "+strings.Join(asc.ValidLeaderboardFormatters, ", "))
	sortType := fs.String("sort", "", "Score sort type: ASC, DESC")
	submissionType := fs.String("submission-type", "", "Submission type: BEST_SCORE, MOST_RECENT_SCORE")
	scoreRangeStart := fs.String("score-range-start", "", "Score range start")
	scoreRangeEnd := fs.String("score-range-end", "", "Score range end")
	bindLeaderboardRecurrenceFlags(fs)
	fs.String("visibility", "", "Leaderboard visibility: "+strings.Join(asc.ValidLeaderboardVisibilities, ", "))
	fs.String("activity-properties", "", "Activity properties as a JSON object of string values, inline or @file:PATH")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboards update [flags]",
		ShortHelp:  "Update a Game Center leaderboard.",
		LongHelp: `Update a Game Center leaderboard.

Recurrence flags work only on a leaderboard created as recurring; Apple does
not convert a classic leaderboard. Each flag can be set alone.

Examples:
  asc game-center leaderboards update --id "LEADERBOARD_ID" --reference-name "New Name"
  asc game-center leaderboards update --id "LEADERBOARD_ID" --archived true
  asc game-center leaderboards update --id "LEADERBOARD_ID" --visibility HIDE_FOR_ALL
  asc game-center leaderboards update --id "LEADERBOARD_ID" --activity-properties @file:properties.json
  asc game-center leaderboards update --id "LEADERBOARD_ID" --archived true --v2
  asc game-center leaderboards update --id "LEADERBOARD_ID" --formatter ELAPSED_TIME_SECOND --sort ASC --submission-type MOST_RECENT_SCORE
  asc game-center leaderboards update --id "LEADERBOARD_ID" --score-range-start 0 --score-range-end 5000 --v2
  asc game-center leaderboards update --id "LEADERBOARD_ID" --recurrence-duration "PT168H" --v2`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*leaderboardID)
			if id == "" {
				return d.RequiredFlag("--id")
			}

			attrs := asc.GameCenterLeaderboardUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				name := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &name
				hasUpdate = true
			}

			if strings.TrimSpace(*archived) != "" {
				val, err := shared.ParseBoolFlag(*archived, "--archived")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Archived = &val
				hasUpdate = true
			}

			provided := map[string]bool{}
			fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })

			if provided["formatter"] {
				val := strings.TrimSpace(strings.ToUpper(*formatter))
				if !isValidLeaderboardFormatter(val) {
					return d.UsageErrorf("--formatter must be one of: %s", strings.Join(asc.ValidLeaderboardFormatters, ", "))
				}
				attrs.DefaultFormatter = &val
				hasUpdate = true
			}

			if provided["sort"] {
				val := strings.TrimSpace(strings.ToUpper(*sortType))
				if !isValidScoreSortType(val) {
					return d.UsageErrorf("--sort must be one of: %s", strings.Join(asc.ValidScoreSortTypes, ", "))
				}
				attrs.ScoreSortType = &val
				hasUpdate = true
			}

			if provided["submission-type"] {
				val := strings.TrimSpace(strings.ToUpper(*submissionType))
				if !isValidSubmissionType(val) {
					return d.UsageErrorf("--submission-type must be one of: %s", strings.Join(asc.ValidSubmissionTypes, ", "))
				}
				attrs.SubmissionType = &val
				hasUpdate = true
			}

			if provided["score-range-start"] {
				val := strings.TrimSpace(*scoreRangeStart)
				if val == "" {
					return d.UsageError("--score-range-start must not be empty")
				}
				attrs.ScoreRangeStart = &val
				hasUpdate = true
			}

			if provided["score-range-end"] {
				val := strings.TrimSpace(*scoreRangeEnd)
				if val == "" {
					return d.UsageError("--score-range-end must not be empty")
				}
				attrs.ScoreRangeEnd = &val
				hasUpdate = true
			}

			recurrence, err := parseLeaderboardRecurrence(d, fs)
			if err != nil {
				return err
			}
			if recurrence.startDate != "" {
				attrs.RecurrenceStartDate = &recurrence.startDate
				hasUpdate = true
			}
			if recurrence.duration != "" {
				attrs.RecurrenceDuration = &recurrence.duration
				hasUpdate = true
			}
			if recurrence.rule != "" {
				attrs.RecurrenceRule = &recurrence.rule
				hasUpdate = true
			}

			visibility, err := parseLeaderboardVisibilityFlag(d, fs)
			if err != nil {
				return err
			}
			if visibility != "" {
				attrs.Visibility = &visibility
				hasUpdate = true
			}

			activityProperties, err := parseStringMapFlag(d, fs, "activity-properties")
			if err != nil {
				return err
			}
			if activityProperties != nil {
				attrs.ActivityProperties = activityProperties
				hasUpdate = true
			}

			if !hasUpdate {
				return d.UpdateFlagRequired(fs, referenceName, archived, formatter, sortType, submissionType, scoreRangeStart, scoreRangeEnd, fs.Lookup("recurrence-start-date").Value, fs.Lookup("recurrence-duration").Value, fs.Lookup("recurrence-rule").Value, fs.Lookup("visibility").Value, fs.Lookup("activity-properties").Value)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardsDeleteCommand returns the leaderboards delete subcommand.
func newGameCenterLeaderboardsDeleteCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("delete", flag.ExitOnError)

	leaderboardID := shared.BindResourceIDFlag(fs, "id", "gameCenterLeaderboards", "Game Center leaderboard ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	v2 := fs.Bool("v2", false, "Use v2 leaderboards endpoint")
	output := d.BindOutputFlags(fs)

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
				return d.RequiredFlag("--id")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards delete: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			result := &asc.DeleteResult{
				ID:      id,
				Deleted: true,
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardsSubmitCommand submits a leaderboard entry.
func newGameCenterLeaderboardsSubmitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submit", flag.ExitOnError)

	vendorID := fs.String("vendor-id", "", "Leaderboard vendor identifier")
	score := fs.String("score", "", "Score value")
	bundleID := fs.String("bundle-id", "", "App bundle ID")
	scopedPlayerID := fs.String("scoped-player-id", "", "Scoped player ID")
	scoreContext := fs.String("context", "", "Optional score context value")
	challengeIDs := fs.String("challenge-ids", "", "Challenge ID(s), comma-separated")
	preReleased := fs.String("pre-released", "", "Apply the submission to the prerelease version (true/false)")
	submittedDate := fs.String("submitted-date", "", "Submission date (RFC3339)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc game-center leaderboards submit --vendor-id \"VENDOR_ID\" --score \"100\" --bundle-id \"BUNDLE_ID\" --scoped-player-id \"PLAYER_ID\"",
		ShortHelp:  "Submit a leaderboard entry.",
		LongHelp: `Submit a leaderboard entry.

Examples:
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID"
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --challenge-ids "CHALLENGE_ID"
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --pre-released true
  asc game-center leaderboards submit --vendor-id "com.example.leaderboard" --score "100" --bundle-id "com.example.app" --scoped-player-id "PLAYER_ID" --submitted-date "2025-01-10T12:34:56Z"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			vendorValue := strings.TrimSpace(*vendorID)
			if vendorValue == "" {
				return d.RequiredFlag("--vendor-id")
			}
			scoreValue := strings.TrimSpace(*score)
			if scoreValue == "" {
				return d.RequiredFlag("--score")
			}
			bundleValue := strings.TrimSpace(*bundleID)
			if bundleValue == "" {
				return d.RequiredFlag("--bundle-id")
			}
			playerValue := strings.TrimSpace(*scopedPlayerID)
			if playerValue == "" {
				return d.RequiredFlag("--scoped-player-id")
			}

			var preReleasedValue *bool
			if strings.TrimSpace(*preReleased) != "" {
				value, err := shared.ParseBoolFlag(*preReleased, "--pre-released")
				if err != nil {
					fmt.Fprintln(d.IO.ErrOut, "Error:", err.Error())
					return flag.ErrHelp
				}
				preReleasedValue = &value
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards submit: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			attrs := asc.GameCenterLeaderboardEntrySubmissionAttributes{
				VendorIdentifier: vendorValue,
				Score:            scoreValue,
				BundleID:         bundleValue,
				ScopedPlayerID:   playerValue,
				ChallengeIDs:     shared.SplitCSV(*challengeIDs),
				PreReleased:      preReleasedValue,
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardGroupLeaderboardCommand returns the group leaderboard command group.
func newGameCenterLeaderboardGroupLeaderboardCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("group-leaderboard", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group-leaderboard",
		ShortUsage: "asc game-center leaderboards group-leaderboard view --id \"LEADERBOARD_ID\"",
		ShortHelp:  "View the group leaderboard for a leaderboard.",
		LongHelp: `View the group leaderboard for a Game Center leaderboard.

Examples:
  asc game-center leaderboards group-leaderboard view --id "LEADERBOARD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newGameCenterLeaderboardGroupLeaderboardGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardGroupLeaderboardGetCommand returns the group leaderboard get subcommand.
func newGameCenterLeaderboardGroupLeaderboardGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc game-center leaderboards group-leaderboard view --id \"LEADERBOARD_ID\"",
		ShortHelp:  "View a group leaderboard by leaderboard ID.",
		LongHelp: `View a group leaderboard by leaderboard ID.

Examples:
  asc game-center leaderboards group-leaderboard view --id "LEADERBOARD_ID"`,
		IDUsage:            "Game Center leaderboard ID",
		IDType:             "gameCenterLeaderboards",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetGameCenterLeaderboardGroupLeaderboard(ctx, id)
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardReleasesCommand returns the releases command group.
func newGameCenterLeaderboardReleasesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("releases", flag.ExitOnError)

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
			newGameCenterLeaderboardReleasesListCommand(d),
			newGameCenterLeaderboardReleasesCreateCommand(d),
			newGameCenterLeaderboardReleasesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newGameCenterLeaderboardReleasesListCommand returns the releases list subcommand.
func newGameCenterLeaderboardReleasesListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "list",
		ShortUsage: "asc game-center leaderboards releases list --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "List releases for a Game Center leaderboard.",
		LongHelp: `List releases for a Game Center leaderboard.

Examples:
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID" --limit 50
  asc game-center leaderboards releases list --leaderboard-id "LEADERBOARD_ID" --paginate`,
		ParentFlag:           "leaderboard-id",
		ParentUsage:          "Game Center leaderboard ID",
		ParentType:           "gameCenterLeaderboards",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, leaderboardId string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetGameCenterLeaderboardReleases(ctx, leaderboardId, asc.Limit[asc.GCLeaderboardReleasesOption](limit), asc.NextURL[asc.GCLeaderboardReleasesOption](next))
		},
		Deps: d,
	})
}

// newGameCenterLeaderboardReleasesCreateCommand returns the releases create subcommand.
func newGameCenterLeaderboardReleasesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	leaderboardID := shared.BindResourceIDFlag(fs, "leaderboard-id", "gameCenterLeaderboards", "Game Center leaderboard ID")
	output := d.BindOutputFlags(fs)

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
				fmt.Fprintln(d.IO.ErrOut, "Error: --app is required (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError("--app")
			}

			lbID := strings.TrimSpace(*leaderboardID)
			if lbID == "" {
				return d.RequiredFlag("--leaderboard-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("game-center leaderboards releases create: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newGameCenterLeaderboardReleasesDeleteCommand returns the releases delete subcommand.
func newGameCenterLeaderboardReleasesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboards releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard release.",
		LongHelp: `Delete a Game Center leaderboard release.

Examples:
  asc game-center leaderboards releases delete --id "RELEASE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Game Center leaderboard release ID",
		IDType:             "gameCenterLeaderboardReleases",
		RequiredDiagnostic: true,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardRelease(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
		Deps: d,
	})
}

// leaderboardRecurrenceDuration allows time components only. OpenAPI says
// "duration", but Apple rejects P7D and P7DT0H with "Expected an ISO 8601
// duration with time components" and accepts PT168H. It also matches "PT",
// which the caller rejects.
var leaderboardRecurrenceDuration = regexp.MustCompile(`^PT(\d+H)?(\d+M)?(\d+S)?$`)

// leaderboardRecurrenceRule is the only rule shape Apple accepts:
// "Format must be 'FREQ=[MINUTELY,HOURLY,DAILY];INTERVAL=$INT'".
var leaderboardRecurrenceRule = regexp.MustCompile(`^FREQ=(MINUTELY|HOURLY|DAILY);INTERVAL=[1-9][0-9]*$`)

// leaderboardRecurrence holds validated recurrence flags. An empty field means
// the flag was omitted.
type leaderboardRecurrence struct {
	startDate string
	duration  string
	rule      string
}

func bindLeaderboardRecurrenceFlags(fs *flag.FlagSet) {
	fs.String("recurrence-start-date", "", "Recurrence start date (RFC 3339)")
	fs.String("recurrence-duration", "", "Recurrence duration, time components only (e.g., PT168H for a week)")
	fs.String("recurrence-rule", "", "Recurrence rule: FREQ=MINUTELY|HOURLY|DAILY;INTERVAL=N (e.g., FREQ=DAILY;INTERVAL=7)")
}

func parseLeaderboardRecurrence(d *shared.Deps, fs *flag.FlagSet) (leaderboardRecurrence, error) {
	var recurrence leaderboardRecurrence
	var err error
	if recurrence.startDate, err = leaderboardRecurrenceFlagValue(d, fs, "recurrence-start-date"); err != nil {
		return recurrence, err
	}
	if recurrence.duration, err = leaderboardRecurrenceFlagValue(d, fs, "recurrence-duration"); err != nil {
		return recurrence, err
	}
	if recurrence.rule, err = leaderboardRecurrenceFlagValue(d, fs, "recurrence-rule"); err != nil {
		return recurrence, err
	}

	if recurrence.startDate != "" {
		if _, err := time.Parse(time.RFC3339, recurrence.startDate); err != nil {
			return recurrence, d.UsageError("--recurrence-start-date must be an RFC 3339 date-time, such as 2026-12-01T00:00:00Z")
		}
	}
	if duration := recurrence.duration; duration != "" &&
		(duration == "PT" || !leaderboardRecurrenceDuration.MatchString(duration)) {
		return recurrence, d.UsageError("--recurrence-duration must be an ISO 8601 duration with time components only, such as PT168H for a week")
	}
	if rule := recurrence.rule; rule != "" && !leaderboardRecurrenceRule.MatchString(rule) {
		return recurrence, d.UsageError("--recurrence-rule must be FREQ=MINUTELY, HOURLY, or DAILY with ;INTERVAL=N, such as FREQ=DAILY;INTERVAL=7")
	}

	return recurrence, nil
}

func leaderboardRecurrenceFlagValue(d *shared.Deps, fs *flag.FlagSet, name string) (string, error) {
	value := strings.TrimSpace(fs.Lookup(name).Value.String())
	if flagPassed(fs, name) && value == "" {
		return "", d.UsageErrorf("--%s must not be blank", name)
	}
	return value, nil
}
