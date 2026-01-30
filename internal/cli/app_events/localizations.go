package app_events

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// AppEventLocalizationsCommand returns the app event localizations command group.
func AppEventLocalizationsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "localizations",
		ShortUsage: "asc app-events localizations <subcommand> [flags]",
		ShortHelp:  "Manage in-app event localizations.",
		LongHelp: `Manage in-app event localizations.

Examples:
  asc app-events localizations list --event-id "EVENT_ID"
  asc app-events localizations get --localization-id "LOC_ID"
  asc app-events localizations screenshots list --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppEventLocalizationsListCommand(),
			AppEventLocalizationsGetCommand(),
			AppEventLocalizationScreenshotsCommand(),
			AppEventLocalizationVideoClipsCommand(),
			AppEventLocalizationScreenshotsRelationshipsCommand(),
			AppEventLocalizationVideoClipsRelationshipsCommand(),
			AppEventLocalizationsCreateCommand(),
			AppEventLocalizationsUpdateCommand(),
			AppEventLocalizationsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEventLocalizationsListCommand returns the app event localizations list subcommand.
func AppEventLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations list", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events localizations list [flags]",
		ShortHelp:  "List localizations for an in-app event.",
		LongHelp: `List localizations for an in-app event.

Examples:
  asc app-events localizations list --event-id "EVENT_ID"
  asc app-events localizations list --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events localizations list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events localizations list: %w", err)
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEventLocalizationsOption{
				asc.WithAppEventLocalizationsLimit(*limit),
				asc.WithAppEventLocalizationsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEventLocalizationsLimit(200))
				firstPage, err := client.GetAppEventLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventLocalizations(ctx, id, asc.WithAppEventLocalizationsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventLocalizationsGetCommand returns the app event localizations get subcommand.
func AppEventLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations get", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-events localizations get --localization-id \"LOC_ID\"",
		ShortHelp:  "Get an in-app event localization by ID.",
		LongHelp: `Get an in-app event localization by ID.

Examples:
  asc app-events localizations get --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEventLocalization(requestCtx, id)
			if err != nil {
				return fmt.Errorf("app-events localizations get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventLocalizationsCreateCommand returns the app event localizations create subcommand.
func AppEventLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations create", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	shortDescription := fs.String("short-description", "", "Short description")
	longDescription := fs.String("long-description", "", "Long description")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events localizations create [flags]",
		ShortHelp:  "Create an in-app event localization.",
		LongHelp: `Create an in-app event localization.

Examples:
  asc app-events localizations create --event-id "EVENT_ID" --locale "en-US" --name "Summer Challenge"
  asc app-events localizations create --event-id "EVENT_ID" --locale "ja-JP" --short-description "Short text"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id is required")
				return flag.ErrHelp
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --locale is required")
				return flag.ErrHelp
			}

			attrs := asc.AppEventLocalizationCreateAttributes{
				Locale:           localeValue,
				Name:             strings.TrimSpace(*name),
				ShortDescription: strings.TrimSpace(*shortDescription),
				LongDescription:  strings.TrimSpace(*longDescription),
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations create: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppEventLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events localizations create: failed to create: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventLocalizationsUpdateCommand returns the app event localizations update subcommand.
func AppEventLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations update", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	name := fs.String("name", "", "Localized name")
	shortDescription := fs.String("short-description", "", "Short description")
	longDescription := fs.String("long-description", "", "Long description")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-events localizations update [flags]",
		ShortHelp:  "Update an in-app event localization.",
		LongHelp: `Update an in-app event localization.

Examples:
  asc app-events localizations update --localization-id "LOC_ID" --name "New Name"
  asc app-events localizations update --localization-id "LOC_ID" --short-description "Updated text"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			var (
				attrs     asc.AppEventLocalizationUpdateAttributes
				hasUpdate bool
			)

			if strings.TrimSpace(*name) != "" {
				value := strings.TrimSpace(*name)
				attrs.Name = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*shortDescription) != "" {
				value := strings.TrimSpace(*shortDescription)
				attrs.ShortDescription = &value
				hasUpdate = true
			}
			if strings.TrimSpace(*longDescription) != "" {
				value := strings.TrimSpace(*longDescription)
				attrs.LongDescription = &value
				hasUpdate = true
			}

			if !hasUpdate {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations update: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppEventLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events localizations update: failed to update: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventLocalizationsDeleteCommand returns the app event localizations delete subcommand.
func AppEventLocalizationsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations delete", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-events localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:  "Delete an in-app event localization.",
		LongHelp: `Delete an in-app event localization.

Examples:
  asc app-events localizations delete --localization-id "LOC_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations delete: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppEventLocalization(requestCtx, id); err != nil {
				return fmt.Errorf("app-events localizations delete: failed to delete: %w", err)
			}

			result := &asc.AppEventLocalizationDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return printOutput(result, *output, *pretty)
		},
	}
}
