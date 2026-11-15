package app_events

import (
	"context"
	"flag"
	"fmt"
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
			if err := shared.ValidatePaginationFlags(ctx, fs, "app-events localizations list", 200, "include", "placements-limit"); err != nil {
				return err
			}
			id := strings.TrimSpace(*eventID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--event-id")
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
	var (
		includeValues   []string
		placementsLimit int
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "localizations view",
		Name:        "view",
		ShortUsage:  "asc app-events localizations view --localization-id \"LOC_ID\"",
		ShortHelp:   "View an in-app event localization by ID.",
		LongHelp: `View an in-app event localization by ID.

Examples:
  asc app-events localizations view --localization-id "LOC_ID"
  asc app-events localizations view --localization-id "LOC_ID" --include placements --placements-limit 50`,
		IDFlag:             "localization-id",
		IDUsage:            "App event localization ID",
		IDType:             "appEventLocalizations",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", appEventLocalizationIncludeList())
			return func() (err error) {
				includeValues, placementsLimit, err = include.Resolve()
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEventLocalization(ctx, id, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
		},
	})
}

// AppEventLocalizationsCreateCommand returns the app event localizations create subcommand.
func AppEventLocalizationsCreateCommand() *ffcli.Command {
	var attrs asc.AppEventLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		FlagSetName: "localizations create",
		Name:        "create",
		ShortUsage:  "asc app-events localizations create [flags]",
		ShortHelp:   "Create an in-app event localization.",
		LongHelp: `Create an in-app event localization.

Examples:
  asc app-events localizations create --event-id "EVENT_ID" --locale "en-US" --name "Summer Challenge"
  asc app-events localizations create --event-id "EVENT_ID" --locale "ja-JP" --short-description "Short text"`,
		IDFlag:  "event-id",
		IDUsage: "App event ID",
		IDType:  "appEvents",
		Fields: []shared.AttributeFlag{
			shared.RequiredStringFlag(&attrs.Locale, "locale", "Locale (e.g., en-US)"),
			shared.StringFlag(&attrs.Name, "name", "Localized name"),
			shared.StringFlag(&attrs.ShortDescription, "short-description", "Short description"),
			shared.StringFlag(&attrs.LongDescription, "long-description", "Long description"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateAppEventLocalization(ctx, id, attrs)
		},
	})
}

// AppEventLocalizationsUpdateCommand returns the app event localizations update subcommand.
func AppEventLocalizationsUpdateCommand() *ffcli.Command {
	var attrs asc.AppEventLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		FlagSetName: "localizations update",
		Name:        "update",
		ShortUsage:  "asc app-events localizations update [flags]",
		ShortHelp:   "Update an in-app event localization.",
		LongHelp: `Update an in-app event localization.

Examples:
  asc app-events localizations update --localization-id "LOC_ID" --name "New Name"
  asc app-events localizations update --localization-id "LOC_ID" --short-description "Updated text"`,
		IDFlag:  "localization-id",
		IDUsage: "App event localization ID",
		IDType:  "appEventLocalizations",
		Fields: []shared.AttributeFlag{
			shared.OptionalStringFlag(&attrs.Name, "name", "Localized name"),
			shared.OptionalStringFlag(&attrs.ShortDescription, "short-description", "Short description"),
			shared.OptionalStringFlag(&attrs.LongDescription, "long-description", "Long description"),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateAppEventLocalization(ctx, id, attrs)
		},
	})
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
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppEventLocalization(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
