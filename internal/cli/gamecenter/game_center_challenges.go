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

// GameCenterChallengesCommand returns the challenges command group.
func GameCenterChallengesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("challenges", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "challenges",
		ShortUsage: "asc game-center challenges <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenges.",
		LongHelp: `Manage Game Center challenges.

Examples:
  asc game-center challenges list --app "APP_ID"
  asc game-center challenges get --id "CHALLENGE_ID"
  asc game-center challenges create --app "APP_ID" --reference-name "Weekly Challenge" --vendor-id "com.example.weekly" --leaderboard-id "LEADERBOARD_ID"
  asc game-center challenges update --id "CHALLENGE_ID" --archived true
  asc game-center challenges delete --id "CHALLENGE_ID" --confirm
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID"
  asc game-center challenges localizations list --version-id "VERSION_ID"
  asc game-center challenges localizations image get --id "LOC_ID"
  asc game-center challenges versions default-image get --id "VERSION_ID"
  asc game-center challenges images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center challenges releases list --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengesListCommand(),
			GameCenterChallengesGetCommand(),
			GameCenterChallengesCreateCommand(),
			GameCenterChallengesUpdateCommand(),
			GameCenterChallengesDeleteCommand(),
			GameCenterChallengeVersionsCommand(),
			GameCenterChallengeLocalizationsCommand(),
			GameCenterChallengeImagesCommand(),
			GameCenterChallengeReleasesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengesListCommand returns the challenges list subcommand.
func GameCenterChallengesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center challenges list [flags]",
		ShortHelp:  "List Game Center challenges for an app.",
		LongHelp: `List Game Center challenges for an app.

Examples:
  asc game-center challenges list --app "APP_ID"
  asc game-center challenges list --app "APP_ID" --limit 50
  asc game-center challenges list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center challenges list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center challenges list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center challenges list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCChallengesOption{
				asc.WithGCChallengesLimit(*limit),
				asc.WithGCChallengesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCChallengesLimit(200))
				firstPage, err := client.GetGameCenterChallenges(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center challenges list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterChallenges(ctx, gcDetailID, asc.WithGCChallengesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center challenges list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterChallenges(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center challenges list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengesGetCommand returns the challenges get subcommand.
func GameCenterChallengesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	challengeID := fs.String("id", "", "Game Center challenge ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center challenges get --id \"CHALLENGE_ID\"",
		ShortHelp:  "Get a Game Center challenge by ID.",
		LongHelp: `Get a Game Center challenge by ID.

Examples:
  asc game-center challenges get --id "CHALLENGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*challengeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterChallenge(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengesCreateCommand returns the challenges create subcommand.
func GameCenterChallengesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the challenge")
	vendorID := fs.String("vendor-id", "", "Vendor identifier for the challenge")
	repeatable := fs.String("repeatable", "", "Challenge can be earned multiple times (true/false)")
	leaderboardID := fs.String("leaderboard-id", "", "Leaderboard ID for the challenge")
	groupID := fs.String("group-id", "", "Game Center group ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center challenges create --app \"APP_ID\" --reference-name \"Weekly\" --vendor-id \"com.example.weekly\" --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "Create a Game Center challenge.",
		LongHelp: `Create a Game Center challenge.

Examples:
  asc game-center challenges create --app "APP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --leaderboard-id "LEADERBOARD_ID"
  asc game-center challenges create --group-id "GROUP_ID" --reference-name "Weekly" --vendor-id "com.example.weekly" --leaderboard-id "LEADERBOARD_ID"`,
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

			attrs := asc.GameCenterChallengeCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
				ChallengeType:    "LEADERBOARD",
			}

			if strings.TrimSpace(*repeatable) != "" {
				val, err := parseBool(*repeatable, "--repeatable")
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
				attrs.Repeatable = &val
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if group == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center challenges create: failed to get Game Center detail: %w", err)
				}
			}

			resp, err := client.CreateGameCenterChallenge(requestCtx, gcDetailID, attrs, strings.TrimSpace(*leaderboardID), group)
			if err != nil {
				return fmt.Errorf("game-center challenges create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengesUpdateCommand returns the challenges update subcommand.
func GameCenterChallengesUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	challengeID := fs.String("id", "", "Game Center challenge ID")
	referenceName := fs.String("reference-name", "", "Reference name for the challenge")
	repeatable := fs.String("repeatable", "", "Challenge can be earned multiple times (true/false)")
	archived := fs.String("archived", "", "Archive the challenge (true/false)")
	leaderboardID := fs.String("leaderboard-id", "", "Leaderboard ID for the challenge")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center challenges update --id \"CHALLENGE_ID\" [flags]",
		ShortHelp:  "Update a Game Center challenge.",
		LongHelp: `Update a Game Center challenge.

Examples:
  asc game-center challenges update --id "CHALLENGE_ID" --reference-name "New Name"
  asc game-center challenges update --id "CHALLENGE_ID" --archived true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*challengeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterChallengeUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*referenceName) != "" {
				value := strings.TrimSpace(*referenceName)
				attrs.ReferenceName = &value
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

			if !hasUpdate && strings.TrimSpace(*leaderboardID) == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterChallenge(requestCtx, id, attrs, strings.TrimSpace(*leaderboardID))
			if err != nil {
				return fmt.Errorf("game-center challenges update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengesDeleteCommand returns the challenges delete subcommand.
func GameCenterChallengesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	challengeID := fs.String("id", "", "Game Center challenge ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center challenges delete --id \"CHALLENGE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center challenge.",
		LongHelp: `Delete a Game Center challenge.

Examples:
  asc game-center challenges delete --id "CHALLENGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*challengeID)
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
				return fmt.Errorf("game-center challenges delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterChallenge(requestCtx, id); err != nil {
				return fmt.Errorf("game-center challenges delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterChallengeDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeVersionsCommand returns the challenge versions command group.
func GameCenterChallengeVersionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("versions", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "versions",
		ShortUsage: "asc game-center challenges versions <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge versions.",
		LongHelp: `Manage Game Center challenge versions.

Examples:
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID"
  asc game-center challenges versions get --id "VERSION_ID"
  asc game-center challenges versions create --challenge-id "CHALLENGE_ID"
  asc game-center challenges versions default-image get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengeVersionsListCommand(),
			GameCenterChallengeVersionsGetCommand(),
			GameCenterChallengeVersionsCreateCommand(),
			GameCenterChallengeVersionDefaultImageCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengeVersionsListCommand returns the challenge versions list subcommand.
func GameCenterChallengeVersionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	challengeID := fs.String("challenge-id", "", "Game Center challenge ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center challenges versions list --challenge-id \"CHALLENGE_ID\"",
		ShortHelp:  "List versions for a Game Center challenge.",
		LongHelp: `List versions for a Game Center challenge.

Examples:
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID"
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID" --limit 50
  asc game-center challenges versions list --challenge-id "CHALLENGE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center challenges versions list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center challenges versions list: %w", err)
			}

			id := strings.TrimSpace(*challengeID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --challenge-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges versions list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCChallengeVersionsOption{
				asc.WithGCChallengeVersionsLimit(*limit),
				asc.WithGCChallengeVersionsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCChallengeVersionsLimit(200))
				firstPage, err := client.GetGameCenterChallengeVersions(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center challenges versions list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterChallengeVersions(ctx, id, asc.WithGCChallengeVersionsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center challenges versions list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterChallengeVersions(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center challenges versions list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeVersionsGetCommand returns the challenge versions get subcommand.
func GameCenterChallengeVersionsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center challenge version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center challenges versions get --id \"VERSION_ID\"",
		ShortHelp:  "Get a Game Center challenge version by ID.",
		LongHelp: `Get a Game Center challenge version by ID.

Examples:
  asc game-center challenges versions get --id "VERSION_ID"`,
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
				return fmt.Errorf("game-center challenges versions get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterChallengeVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges versions get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeVersionsCreateCommand returns the challenge versions create subcommand.
func GameCenterChallengeVersionsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	challengeID := fs.String("challenge-id", "", "Game Center challenge ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center challenges versions create --challenge-id \"CHALLENGE_ID\"",
		ShortHelp:  "Create a Game Center challenge version.",
		LongHelp: `Create a Game Center challenge version.

Examples:
  asc game-center challenges versions create --challenge-id "CHALLENGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*challengeID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --challenge-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges versions create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterChallengeVersion(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges versions create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeLocalizationsCommand returns the challenge localizations command group.
func GameCenterChallengeLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center challenges localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge localizations.",
		LongHelp: `Manage Game Center challenge localizations.

Examples:
  asc game-center challenges localizations list --version-id "VERSION_ID"
  asc game-center challenges localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"
  asc game-center challenges localizations update --id "LOC_ID" --name "New Name"
  asc game-center challenges localizations delete --id "LOC_ID" --confirm
  asc game-center challenges localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengeLocalizationsListCommand(),
			GameCenterChallengeLocalizationsGetCommand(),
			GameCenterChallengeLocalizationsCreateCommand(),
			GameCenterChallengeLocalizationsUpdateCommand(),
			GameCenterChallengeLocalizationsDeleteCommand(),
			GameCenterChallengeLocalizationImageCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengeLocalizationsListCommand returns the challenge localizations list subcommand.
func GameCenterChallengeLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center challenge version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center challenges localizations list --version-id \"VERSION_ID\"",
		ShortHelp:  "List localizations for a challenge version.",
		LongHelp: `List localizations for a challenge version.

Examples:
  asc game-center challenges localizations list --version-id "VERSION_ID"
  asc game-center challenges localizations list --version-id "VERSION_ID" --limit 50
  asc game-center challenges localizations list --version-id "VERSION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center challenges localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center challenges localizations list: %w", err)
			}

			id := strings.TrimSpace(*versionID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCChallengeLocalizationsOption{
				asc.WithGCChallengeLocalizationsLimit(*limit),
				asc.WithGCChallengeLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCChallengeLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterChallengeLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center challenges localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterChallengeLocalizations(ctx, id, asc.WithGCChallengeLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center challenges localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterChallengeLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center challenges localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeLocalizationsGetCommand returns the challenge localizations get subcommand.
func GameCenterChallengeLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center challenge localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center challenges localizations get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a challenge localization by ID.",
		LongHelp: `Get a challenge localization by ID.

Examples:
  asc game-center challenges localizations get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center challenges localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterChallengeLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeLocalizationsCreateCommand returns the challenge localizations create subcommand.
func GameCenterChallengeLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center challenge version ID")
	locale := fs.String("locale", "", "Localization locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center challenges localizations create --version-id \"VERSION_ID\" --locale en-US --name \"Weekly\" --description \"Win weekly\"",
		ShortHelp:  "Create a challenge localization.",
		LongHelp: `Create a challenge localization.

Examples:
  asc game-center challenges localizations create --version-id "VERSION_ID" --locale en-US --name "Weekly" --description "Win weekly"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}
			loc := strings.TrimSpace(*locale)
			if loc == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}
			nameValue := strings.TrimSpace(*name)
			if nameValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --name is required")
				return flag.ErrHelp
			}
			descriptionValue := strings.TrimSpace(*description)
			if descriptionValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --description is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterChallengeLocalizationCreateAttributes{
				Locale:      loc,
				Name:        nameValue,
				Description: descriptionValue,
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterChallengeLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center challenges localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeLocalizationsUpdateCommand returns the challenge localizations update subcommand.
func GameCenterChallengeLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center challenge localization ID")
	name := fs.String("name", "", "Localized name")
	description := fs.String("description", "", "Localized description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center challenges localizations update --id \"LOCALIZATION_ID\" [flags]",
		ShortHelp:  "Update a challenge localization.",
		LongHelp: `Update a challenge localization.

Examples:
  asc game-center challenges localizations update --id "LOCALIZATION_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterChallengeLocalizationUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				value := strings.TrimSpace(*name)
				attrs.Name = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*description) != "" {
				value := strings.TrimSpace(*description)
				attrs.Description = &value
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterChallengeLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center challenges localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeLocalizationsDeleteCommand returns the challenge localizations delete subcommand.
func GameCenterChallengeLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center challenge localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center challenges localizations delete --id \"LOCALIZATION_ID\" --confirm",
		ShortHelp:  "Delete a challenge localization.",
		LongHelp: `Delete a challenge localization.

Examples:
  asc game-center challenges localizations delete --id "LOCALIZATION_ID" --confirm`,
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
				return fmt.Errorf("game-center challenges localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterChallengeLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("game-center challenges localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterChallengeLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeImagesCommand returns the challenge images command group.
func GameCenterChallengeImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center challenges images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge images.",
		LongHelp: `Manage Game Center challenge images. Images are attached to challenge localizations.

Examples:
  asc game-center challenges images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png
  asc game-center challenges images get --id "IMAGE_ID"
  asc game-center challenges images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengeImagesUploadCommand(),
			GameCenterChallengeImagesGetCommand(),
			GameCenterChallengeImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengeImagesUploadCommand returns the challenge images upload subcommand.
func GameCenterChallengeImagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Challenge localization ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center challenges images upload --localization-id \"LOCALIZATION_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a challenge localization.",
		LongHelp: `Upload an image for a challenge localization.

The upload process reserves an upload slot, uploads the image file, and commits the upload.

Examples:
  asc game-center challenges images upload --localization-id "LOCALIZATION_ID" --file path/to/image.png`,
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
				return fmt.Errorf("game-center challenges images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterChallengeImage(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center challenges images upload: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeImagesGetCommand returns the challenge images get subcommand.
func GameCenterChallengeImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Challenge image ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center challenges images get --id \"IMAGE_ID\"",
		ShortHelp:  "Get a challenge image by ID.",
		LongHelp: `Get a challenge image by ID.

Examples:
  asc game-center challenges images get --id "IMAGE_ID"`,
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
				return fmt.Errorf("game-center challenges images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterChallengeImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges images get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeImagesDeleteCommand returns the challenge images delete subcommand.
func GameCenterChallengeImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Challenge image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center challenges images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a challenge image.",
		LongHelp: `Delete a challenge image.

Examples:
  asc game-center challenges images delete --id "IMAGE_ID" --confirm`,
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
				return fmt.Errorf("game-center challenges images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterChallengeImage(requestCtx, id); err != nil {
				return fmt.Errorf("game-center challenges images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterChallengeImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeReleasesCommand returns the challenge releases command group.
func GameCenterChallengeReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center challenges releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center challenge releases.",
		LongHelp: `Manage Game Center challenge releases. Releases are used to publish challenge versions to live.

Examples:
  asc game-center challenges releases list --app "APP_ID"
  asc game-center challenges releases create --version-id "VERSION_ID"
  asc game-center challenges releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengeReleasesListCommand(),
			GameCenterChallengeReleasesCreateCommand(),
			GameCenterChallengeReleasesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengeReleasesListCommand returns the challenge releases list subcommand.
func GameCenterChallengeReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center challenges releases list --app \"APP_ID\"",
		ShortHelp:  "List releases for Game Center challenges.",
		LongHelp: `List releases for Game Center challenges.

Examples:
  asc game-center challenges releases list --app "APP_ID"
  asc game-center challenges releases list --app "APP_ID" --limit 50
  asc game-center challenges releases list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center challenges releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center challenges releases list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center challenges releases list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCChallengeVersionReleasesOption{
				asc.WithGCChallengeVersionReleasesLimit(*limit),
				asc.WithGCChallengeVersionReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCChallengeVersionReleasesLimit(200))
				firstPage, err := client.GetGameCenterChallengeVersionReleases(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center challenges releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterChallengeVersionReleases(ctx, gcDetailID, asc.WithGCChallengeVersionReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center challenges releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetGameCenterChallengeVersionReleases(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center challenges releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeReleasesCreateCommand returns the challenge releases create subcommand.
func GameCenterChallengeReleasesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	versionID := fs.String("version-id", "", "Game Center challenge version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center challenges releases create --version-id \"VERSION_ID\"",
		ShortHelp:  "Create a Game Center challenge release.",
		LongHelp: `Create a Game Center challenge release. This publishes the version to live.

Examples:
  asc game-center challenges releases create --version-id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*versionID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center challenges releases create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateGameCenterChallengeVersionRelease(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges releases create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeReleasesDeleteCommand returns the challenge releases delete subcommand.
func GameCenterChallengeReleasesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	releaseID := fs.String("id", "", "Game Center challenge release ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center challenges releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center challenge release.",
		LongHelp: `Delete a Game Center challenge release.

Examples:
  asc game-center challenges releases delete --id "RELEASE_ID" --confirm`,
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
				return fmt.Errorf("game-center challenges releases delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterChallengeVersionRelease(requestCtx, id); err != nil {
				return fmt.Errorf("game-center challenges releases delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterChallengeVersionReleaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeLocalizationImageCommand returns the challenge localization image command group.
func GameCenterChallengeLocalizationImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center challenges localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get the image for a challenge localization.",
		LongHelp: `Get the image for a challenge localization.

Examples:
  asc game-center challenges localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengeLocalizationImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengeLocalizationImageGetCommand returns the challenge localization image get subcommand.
func GameCenterChallengeLocalizationImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center challenge localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center challenges localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get a challenge localization image.",
		LongHelp: `Get a challenge localization image.

Examples:
  asc game-center challenges localizations image get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center challenges localizations image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterChallengeLocalizationImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges localizations image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterChallengeVersionDefaultImageCommand returns the challenge version default image command group.
func GameCenterChallengeVersionDefaultImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("default-image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "default-image",
		ShortUsage: "asc game-center challenges versions default-image get --id \"VERSION_ID\"",
		ShortHelp:  "Get the default image for a challenge version.",
		LongHelp: `Get the default image for a challenge version.

Examples:
  asc game-center challenges versions default-image get --id "VERSION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterChallengeVersionDefaultImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterChallengeVersionDefaultImageGetCommand returns the challenge version default image get subcommand.
func GameCenterChallengeVersionDefaultImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	versionID := fs.String("id", "", "Game Center challenge version ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center challenges versions default-image get --id \"VERSION_ID\"",
		ShortHelp:  "Get a default image for a challenge version.",
		LongHelp: `Get a default image for a challenge version.

Examples:
  asc game-center challenges versions default-image get --id "VERSION_ID"`,
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
				return fmt.Errorf("game-center challenges versions default-image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterChallengeVersionDefaultImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center challenges versions default-image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
