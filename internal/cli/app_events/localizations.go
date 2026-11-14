package app_events

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
  asc app-events localizations view --localization-id "LOC_ID"
  asc app-events localizations screenshots list --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
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
			shared.CreativePlacementsCommand("appEventLocalizations", "app-events localizations", "an in-app event localization"),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEventLocalizationsListCommand returns the app event localizations list subcommand.
func AppEventLocalizationsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations list", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", appEventLocalizationIncludeList())
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events localizations list [flags]",
		ShortHelp:  "List localizations for an in-app event.",
		LongHelp: `List localizations for an in-app event.

Examples:
  asc app-events localizations list --event-id "EVENT_ID"
  asc app-events localizations list --event-id "EVENT_ID" --include placements --placements-limit 50
  asc app-events localizations list --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--event-id")
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("app-events localizations list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("app-events localizations list: %v", err)
			}
			if err := shared.RejectNextFlagConflicts(fs, *next, "app-events localizations list", "include", "placements-limit"); err != nil {
				return err
			}
			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEventLocalizationsOption{
				asc.Limit[asc.AppEventLocalizationsOption](*limit),
				asc.NextURL[asc.AppEventLocalizationsOption](*next),
				asc.WithAppEventLocalizationsInclude(includeValues),
				asc.WithAppEventLocalizationsPlacementsLimit(placementsLimit),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppEventLocalizationsOption](200))
				firstPage, err := client.GetAppEventLocalizations(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventLocalizations(ctx, id, asc.NextURL[asc.AppEventLocalizationsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventLocalizations(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventLocalizationsGetCommand returns the app event localizations get subcommand.
func AppEventLocalizationsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations view", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", appEventLocalizationIncludeList())
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc app-events localizations view --localization-id \"LOC_ID\"",
		ShortHelp:  "View an in-app event localization by ID.",
		LongHelp: `View an in-app event localization by ID.

Examples:
  asc app-events localizations view --localization-id "LOC_ID"
  asc app-events localizations view --localization-id "LOC_ID" --include placements --placements-limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				return shared.RequiredFlag("--localization-id")
			}
			includeValues, placementsLimit, err := include.Resolve()
			if err != nil {
				return err
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEventLocalization(requestCtx, id, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
			if err != nil {
				return fmt.Errorf("app-events localizations view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventLocalizationsCreateCommand returns the app event localizations create subcommand.
func AppEventLocalizationsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations create", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US)")
	name := fs.String("name", "", "Localized name")
	shortDescription := fs.String("short-description", "", "Short description")
	longDescription := fs.String("long-description", "", "Long description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events localizations create [flags]",
		ShortHelp:  "Create an in-app event localization.",
		LongHelp: `Create an in-app event localization.

Examples:
  asc app-events localizations create --event-id "EVENT_ID" --locale "en-US" --name "Summer Challenge"
  asc app-events localizations create --event-id "EVENT_ID" --locale "ja-JP" --short-description "Short text"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*eventID)
			if id == "" {
				return shared.RequiredFlag("--event-id")
			}

			localeValue := strings.TrimSpace(*locale)
			if localeValue == "" {
				return shared.RequiredFlag("--locale")
			}

			attrs := asc.AppEventLocalizationCreateAttributes{
				Locale:           localeValue,
				Name:             strings.TrimSpace(*name),
				ShortDescription: strings.TrimSpace(*shortDescription),
				LongDescription:  strings.TrimSpace(*longDescription),
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.CreateAppEventLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events localizations create: failed to create: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventLocalizationsUpdateCommand returns the app event localizations update subcommand.
func AppEventLocalizationsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations update", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	name := fs.String("name", "", "Localized name")
	shortDescription := fs.String("short-description", "", "Short description")
	longDescription := fs.String("long-description", "", "Long description")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc app-events localizations update [flags]",
		ShortHelp:  "Update an in-app event localization.",
		LongHelp: `Update an in-app event localization.

Examples:
  asc app-events localizations update --localization-id "LOC_ID" --name "New Name"
  asc app-events localizations update --localization-id "LOC_ID" --short-description "Updated text"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*localizationID)
			if id == "" {
				return shared.RequiredFlag("--localization-id")
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
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateAppEventLocalization(requestCtx, id, attrs)
			if err != nil {
				return fmt.Errorf("app-events localizations update: failed to update: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventLocalizationsDeleteCommand returns the app event localizations delete subcommand.
func AppEventLocalizationsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "localizations delete",
		Name:        "delete",
		ShortUsage:  "asc app-events localizations delete --localization-id \"LOC_ID\" --confirm",
		ShortHelp:   "Delete an in-app event localization.",
		LongHelp: `Delete an in-app event localization.

Examples:
  asc app-events localizations delete --localization-id "LOC_ID" --confirm`,
		IDFlag:             "localization-id",
		IDUsage:            "App event localization ID",
		IDType:             "appEventLocalizations",
		RequiredDiagnostic: true,
		ErrorPrefix:        "app-events localizations delete",
		DeleteErrorPrefix:  "app-events localizations delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppEventLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppEventLocalizationDeleteResult{ID: id, Deleted: true}
		},
	})
}
