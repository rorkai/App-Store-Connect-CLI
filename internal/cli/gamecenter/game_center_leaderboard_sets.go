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

// GameCenterLeaderboardSetsCommand returns the leaderboard-sets command group.
func GameCenterLeaderboardSetsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard-sets", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-sets",
		ShortUsage: "asc game-center leaderboard-sets <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard sets.",
		LongHelp: `Manage Game Center leaderboard sets.

Examples:
  asc game-center leaderboard-sets list --app "APP_ID"
  asc game-center leaderboard-sets get --id "SET_ID"
  asc game-center leaderboard-sets group-leaderboard-set get --id "SET_ID"
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets update --id "SET_ID" --reference-name "Season 1 - Updated"
  asc game-center leaderboard-sets delete --id "SET_ID" --confirm
  asc game-center leaderboard-sets members list --set-id "SET_ID"
  asc game-center leaderboard-sets members set --set-id "SET_ID" --leaderboard-ids "id1,id2,id3"
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboard-sets releases list --set-id "SET_ID"
  asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"
  asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm
  asc game-center leaderboard-sets localizations list --set-id "SET_ID"
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
  asc game-center leaderboard-sets localizations image get --id "LOC_ID"
  asc game-center leaderboard-sets images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboard-sets images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetsListCommand(),
			GameCenterLeaderboardSetsGetCommand(),
			GameCenterLeaderboardSetGroupLeaderboardSetCommand(),
			GameCenterLeaderboardSetsCreateCommand(),
			GameCenterLeaderboardSetsUpdateCommand(),
			GameCenterLeaderboardSetsDeleteCommand(),
			GameCenterLeaderboardSetsV2Command(),
			GameCenterLeaderboardSetMembersCommand(),
			GameCenterLeaderboardSetMemberLocalizationsCommand(),
			GameCenterLeaderboardSetReleasesCommand(),
			GameCenterLeaderboardSetImagesCommand(),
			GameCenterLeaderboardSetLocalizationsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetLocalizationsCommand returns the localizations command group.
func GameCenterLeaderboardSetLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc game-center leaderboard-sets localizations <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set localizations.",
		LongHelp: `Manage Game Center leaderboard set localizations.

Examples:
  asc game-center leaderboard-sets localizations list --set-id "SET_ID"
  asc game-center leaderboard-sets localizations get --id "LOC_ID"
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
  asc game-center leaderboard-sets localizations update --id "LOC_ID" --name "New Name"
  asc game-center leaderboard-sets localizations delete --id "LOC_ID" --confirm
  asc game-center leaderboard-sets localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetLocalizationsListCommand(),
			GameCenterLeaderboardSetLocalizationsGetCommand(),
			GameCenterLeaderboardSetLocalizationsCreateCommand(),
			GameCenterLeaderboardSetLocalizationsUpdateCommand(),
			GameCenterLeaderboardSetLocalizationsDeleteCommand(),
			GameCenterLeaderboardSetLocalizationImageCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetLocalizationsListCommand returns the localizations list subcommand.
func GameCenterLeaderboardSetLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets localizations list --set-id \"SET_ID\"",
		ShortHelp:  "List localizations for a Game Center leaderboard set.",
		LongHelp: `List localizations for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets localizations list --set-id "SET_ID"
  asc game-center leaderboard-sets localizations list --set-id "SET_ID" --limit 50
  asc game-center leaderboard-sets localizations list --set-id "SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations list: %w", err)
			}

			id := strings.TrimSpace(*setID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetLocalizationsOption{
				asc.WithGCLeaderboardSetLocalizationsLimit(*limit),
				asc.WithGCLeaderboardSetLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetLocalizations(ctx, id, asc.WithGCLeaderboardSetLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsGetCommand returns the localizations get subcommand.
func GameCenterLeaderboardSetLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets localizations get --id \"LOC_ID\"",
		ShortHelp:  "Get a Game Center leaderboard set localization by ID.",
		LongHelp: `Get a Game Center leaderboard set localization by ID.

Examples:
  asc game-center leaderboard-sets localizations get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsCreateCommand returns the localizations create subcommand.
func GameCenterLeaderboardSetLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	locale := fs.String("locale", "", "Locale code (e.g., en-US, de-DE)")
	name := fs.String("name", "", "Display name for the leaderboard set")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets localizations create --set-id \"SET_ID\" --locale \"LOCALE\" --name \"NAME\"",
		ShortHelp:  "Create a localization for a Game Center leaderboard set.",
		LongHelp: `Create a localization for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale en-US --name "Season 1"
  asc game-center leaderboard-sets localizations create --set-id "SET_ID" --locale de-DE --name "Staffel 1"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			attrs := asc.GameCenterLeaderboardSetLocalizationCreateAttributes{
				Locale: localeVal,
				Name:   nameVal,
			}

			resp, err := client.CreateGameCenterLeaderboardSetLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsUpdateCommand returns the localizations update subcommand.
func GameCenterLeaderboardSetLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	name := fs.String("name", "", "Display name for the leaderboard set")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets localizations update --id \"LOC_ID\" --name \"NAME\"",
		ShortHelp:  "Update a Game Center leaderboard set localization.",
		LongHelp: `Update a Game Center leaderboard set localization.

Examples:
  asc game-center leaderboard-sets localizations update --id "LOC_ID" --name "New Name"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			attrs := asc.GameCenterLeaderboardSetLocalizationUpdateAttributes{}
			hasUpdate := false

			if strings.TrimSpace(*name) != "" {
				nameVal := strings.TrimSpace(*name)
				attrs.Name = &nameVal
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required (--name)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterLeaderboardSetLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationsDeleteCommand returns the localizations delete subcommand.
func GameCenterLeaderboardSetLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets localizations delete --id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set localization.",
		LongHelp: `Delete a Game Center leaderboard set localization.

Examples:
  asc game-center leaderboard-sets localizations delete --id "LOC_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboard-sets localizations delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSetLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetsListCommand returns the leaderboard-sets list subcommand.
func GameCenterLeaderboardSetsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets list [flags]",
		ShortHelp:  "List Game Center leaderboard sets for an app.",
		LongHelp: `List Game Center leaderboard sets for an app.

Examples:
  asc game-center leaderboard-sets list --app "APP_ID"
  asc game-center leaderboard-sets list --app "APP_ID" --limit 50
  asc game-center leaderboard-sets list --app "APP_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets list: %w", err)
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			nextURL := strings.TrimSpace(*next)
			if resolvedAppID == "" && nextURL == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			gcDetailID := ""
			if nextURL == "" {
				// Get Game Center detail ID first
				var err error
				gcDetailID, err = client.GetGameCenterDetailID(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets list: failed to get Game Center detail: %w", err)
				}
			}

			opts := []asc.GCLeaderboardSetsOption{
				asc.WithGCLeaderboardSetsLimit(*limit),
				asc.WithGCLeaderboardSetsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSets(requestCtx, gcDetailID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSets(ctx, gcDetailID, asc.WithGCLeaderboardSetsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSets(requestCtx, gcDetailID, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetsGetCommand returns the leaderboard-sets get subcommand.
func GameCenterLeaderboardSetsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets get --id \"SET_ID\"",
		ShortHelp:  "Get a Game Center leaderboard set by ID.",
		LongHelp: `Get a Game Center leaderboard set by ID.

Examples:
  asc game-center leaderboard-sets get --id "SET_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSet(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetsCreateCommand returns the leaderboard-sets create subcommand.
func GameCenterLeaderboardSetsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard set")
	vendorID := fs.String("vendor-id", "", "Vendor identifier (e.g., com.example.set)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets create [flags]",
		ShortHelp:  "Create a new Game Center leaderboard set.",
		LongHelp: `Create a new Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Season 1" --vendor-id "com.example.season1"
  asc game-center leaderboard-sets create --app "APP_ID" --reference-name "Weekly Challenge" --vendor-id "com.example.weekly"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
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

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Get Game Center detail ID first
			gcDetailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets create: failed to get Game Center detail: %w", err)
			}

			attrs := asc.GameCenterLeaderboardSetCreateAttributes{
				ReferenceName:    name,
				VendorIdentifier: vendor,
			}

			resp, err := client.CreateGameCenterLeaderboardSet(requestCtx, gcDetailID, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetsUpdateCommand returns the leaderboard-sets update subcommand.
func GameCenterLeaderboardSetsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("update", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	referenceName := fs.String("reference-name", "", "Reference name for the leaderboard set")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc game-center leaderboard-sets update [flags]",
		ShortHelp:  "Update a Game Center leaderboard set.",
		LongHelp: `Update a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets update --id "SET_ID" --reference-name "Season 1 - Updated"`,
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
				return fmt.Errorf("game-center leaderboard-sets update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateGameCenterLeaderboardSet(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetsDeleteCommand returns the leaderboard-sets delete subcommand.
func GameCenterLeaderboardSetsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets delete --id \"SET_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set.",
		LongHelp: `Delete a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets delete --id "SET_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboard-sets delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSet(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetReleasesCommand returns the leaderboard-sets releases command group.
func GameCenterLeaderboardSetReleasesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("releases", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "releases",
		ShortUsage: "asc game-center leaderboard-sets releases <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set releases.",
		LongHelp: `Manage Game Center leaderboard set releases.

Releases control which Game Center details (apps) a leaderboard set is associated with.

Examples:
  asc game-center leaderboard-sets releases list --set-id "SET_ID"
  asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"
  asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetReleasesListCommand(),
			GameCenterLeaderboardSetReleasesCreateCommand(),
			GameCenterLeaderboardSetReleasesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetReleasesListCommand returns the leaderboard-sets releases list subcommand.
func GameCenterLeaderboardSetReleasesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets releases list --set-id \"SET_ID\"",
		ShortHelp:  "List releases for a Game Center leaderboard set.",
		LongHelp: `List releases for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets releases list --set-id "SET_ID"
  asc game-center leaderboard-sets releases list --set-id "SET_ID" --limit 50
  asc game-center leaderboard-sets releases list --set-id "SET_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets releases list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases list: %w", err)
			}

			id := strings.TrimSpace(*setID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetReleasesOption{
				asc.WithGCLeaderboardSetReleasesLimit(*limit),
				asc.WithGCLeaderboardSetReleasesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetReleasesLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetReleases(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets releases list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetReleases(ctx, id, asc.WithGCLeaderboardSetReleasesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets releases list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetReleases(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetReleasesCreateCommand returns the leaderboard-sets releases create subcommand.
func GameCenterLeaderboardSetReleasesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc game-center leaderboard-sets releases create --app \"APP_ID\" --set-id \"SET_ID\"",
		ShortHelp:  "Create a release for a Game Center leaderboard set.",
		LongHelp: `Create a release for a Game Center leaderboard set.

This associates the leaderboard set with the app's Game Center detail.

Examples:
  asc game-center leaderboard-sets releases create --app "APP_ID" --set-id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID := shared.ResolveAppID(*appID)
			if resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			id := strings.TrimSpace(*setID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --set-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			// Get Game Center detail ID first
			gcDetailID, err := client.GetGameCenterDetailID(requestCtx, resolvedAppID)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases create: failed to get Game Center detail: %w", err)
			}

			resp, err := client.CreateGameCenterLeaderboardSetRelease(requestCtx, gcDetailID, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetReleasesDeleteCommand returns the leaderboard-sets releases delete subcommand.
func GameCenterLeaderboardSetReleasesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	releaseID := fs.String("id", "", "Game Center leaderboard set release ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets releases delete --id \"RELEASE_ID\" --confirm",
		ShortHelp:  "Delete a Game Center leaderboard set release.",
		LongHelp: `Delete a Game Center leaderboard set release.

Examples:
  asc game-center leaderboard-sets releases delete --id "RELEASE_ID" --confirm`,
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
				return fmt.Errorf("game-center leaderboard-sets releases delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSetRelease(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets releases delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetReleaseDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetGroupLeaderboardSetCommand returns the group leaderboard set command group.
func GameCenterLeaderboardSetGroupLeaderboardSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("group-leaderboard-set", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "group-leaderboard-set",
		ShortUsage: "asc game-center leaderboard-sets group-leaderboard-set get --id \"SET_ID\"",
		ShortHelp:  "Get the group leaderboard set for a leaderboard set.",
		LongHelp: `Get the group leaderboard set for a Game Center leaderboard set.

Examples:
  asc game-center leaderboard-sets group-leaderboard-set get --id "SET_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetGroupLeaderboardSetGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetGroupLeaderboardSetGetCommand returns the group leaderboard set get subcommand.
func GameCenterLeaderboardSetGroupLeaderboardSetGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	setID := fs.String("id", "", "Game Center leaderboard set ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets group-leaderboard-set get --id \"SET_ID\"",
		ShortHelp:  "Get a group leaderboard set by leaderboard set ID.",
		LongHelp: `Get a group leaderboard set by leaderboard set ID.

Examples:
  asc game-center leaderboard-sets group-leaderboard-set get --id "SET_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets group-leaderboard-set get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetGroupLeaderboardSet(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets group-leaderboard-set get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetLocalizationImageCommand returns the localization image command group.
func GameCenterLeaderboardSetLocalizationImageCommand() *ffcli.Command {
	fs := flag.NewFlagSet("image", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "image",
		ShortUsage: "asc game-center leaderboard-sets localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get the image for a leaderboard set localization.",
		LongHelp: `Get the image for a leaderboard set localization.

Examples:
  asc game-center leaderboard-sets localizations image get --id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetLocalizationImageGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetLocalizationImageGetCommand returns the localization image get subcommand.
func GameCenterLeaderboardSetLocalizationImageGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Game Center leaderboard set localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets localizations image get --id \"LOC_ID\"",
		ShortHelp:  "Get a leaderboard set localization image.",
		LongHelp: `Get a leaderboard set localization image.

Examples:
  asc game-center leaderboard-sets localizations image get --id "LOC_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets localizations image get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetLocalizationImage(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets localizations image get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsCommand returns the member localizations command group.
func GameCenterLeaderboardSetMemberLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("member-localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "member-localizations",
		ShortUsage: "asc game-center leaderboard-sets member-localizations <subcommand> [flags]",
		ShortHelp:  "Manage leaderboard set member localizations.",
		LongHelp: `Manage leaderboard set member localizations.

Examples:
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboard-sets member-localizations get --id "LOCALIZATION_ID"
  asc game-center leaderboard-sets member-localizations leaderboard get --id "LOCALIZATION_ID"
  asc game-center leaderboard-sets member-localizations leaderboard-set get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetMemberLocalizationsListCommand(),
			GameCenterLeaderboardSetMemberLocalizationsGetCommand(),
			GameCenterLeaderboardSetMemberLocalizationsLeaderboardCommand(),
			GameCenterLeaderboardSetMemberLocalizationsLeaderboardSetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsListCommand returns the member localizations list subcommand.
func GameCenterLeaderboardSetMemberLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	setID := fs.String("set-id", "", "Game Center leaderboard set ID")
	leaderboardID := fs.String("leaderboard-id", "", "Game Center leaderboard ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc game-center leaderboard-sets member-localizations list --set-id \"SET_ID\" --leaderboard-id \"LEADERBOARD_ID\"",
		ShortHelp:  "List leaderboard set member localizations.",
		LongHelp: `List leaderboard set member localizations.

Examples:
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID"
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --limit 50
  asc game-center leaderboard-sets member-localizations list --set-id "SET_ID" --leaderboard-id "LEADERBOARD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("game-center leaderboard-sets member-localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations list: %w", err)
			}

			set := strings.TrimSpace(*setID)
			leaderboard := strings.TrimSpace(*leaderboardID)
			if strings.TrimSpace(*next) == "" {
				if set == "" {
					fmt.Fprintln(os.Stderr, "Error: --set-id is required")
					return flag.ErrHelp
				}
				if leaderboard == "" {
					fmt.Fprintln(os.Stderr, "Error: --leaderboard-id is required")
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.GCLeaderboardSetMemberLocalizationsOption{
				asc.WithGCLeaderboardSetMemberLocalizationsLimit(*limit),
				asc.WithGCLeaderboardSetMemberLocalizationsNextURL(*next),
			}
			if set != "" {
				opts = append(opts, asc.WithGCLeaderboardSetMemberLocalizationsLeaderboardSetIDs([]string{set}))
			}
			if leaderboard != "" {
				opts = append(opts, asc.WithGCLeaderboardSetMemberLocalizationsLeaderboardIDs([]string{leaderboard}))
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithGCLeaderboardSetMemberLocalizationsLimit(200))
				firstPage, err := client.GetGameCenterLeaderboardSetMemberLocalizations(requestCtx, paginateOpts...)
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets member-localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetGameCenterLeaderboardSetMemberLocalizations(ctx, asc.WithGCLeaderboardSetMemberLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("game-center leaderboard-sets member-localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetGameCenterLeaderboardSetMemberLocalizations(requestCtx, opts...)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsGetCommand returns the member localization get subcommand.
func GameCenterLeaderboardSetMemberLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Leaderboard set member localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets member-localizations get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a leaderboard set member localization by ID.",
		LongHelp: `Get a leaderboard set member localization by ID.

Examples:
  asc game-center leaderboard-sets member-localizations get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets member-localizations get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetMemberLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsLeaderboardCommand returns the member localization leaderboard command group.
func GameCenterLeaderboardSetMemberLocalizationsLeaderboardCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get the leaderboard for a member localization.",
		LongHelp: `Get the leaderboard for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetMemberLocalizationsLeaderboardGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsLeaderboardGetCommand returns the member localization leaderboard get subcommand.
func GameCenterLeaderboardSetMemberLocalizationsLeaderboardGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Leaderboard set member localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a leaderboard for a member localization.",
		LongHelp: `Get a leaderboard for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets member-localizations leaderboard get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetMemberLocalizationLeaderboard(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations leaderboard get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsLeaderboardSetCommand returns the member localization leaderboard set command group.
func GameCenterLeaderboardSetMemberLocalizationsLeaderboardSetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leaderboard-set", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "leaderboard-set",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard-set get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get the leaderboard set for a member localization.",
		LongHelp: `Get the leaderboard set for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard-set get --id "LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetMemberLocalizationsLeaderboardSetGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetMemberLocalizationsLeaderboardSetGetCommand returns the member localization leaderboard set get subcommand.
func GameCenterLeaderboardSetMemberLocalizationsLeaderboardSetGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	localizationID := fs.String("id", "", "Leaderboard set member localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc game-center leaderboard-sets member-localizations leaderboard-set get --id \"LOCALIZATION_ID\"",
		ShortHelp:  "Get a leaderboard set for a member localization.",
		LongHelp: `Get a leaderboard set for a member localization.

Examples:
  asc game-center leaderboard-sets member-localizations leaderboard-set get --id "LOCALIZATION_ID"`,
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
				return fmt.Errorf("game-center leaderboard-sets member-localizations leaderboard-set get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetGameCenterLeaderboardSetMemberLocalizationLeaderboardSet(requestCtx, id)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets member-localizations leaderboard-set get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
