package app_events

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppEventLocalizationsCommand returns the app event localizations command group.
func newAppEventLocalizationsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations", flag.ExitOnError)

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
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppEventLocalizationsListCommand(d),
			newAppEventLocalizationsGetCommand(d),
			newAppEventLocalizationScreenshotsCommand(d),
			newAppEventLocalizationVideoClipsCommand(d),
			newAppEventLocalizationScreenshotsRelationshipsCommand(d),
			newAppEventLocalizationVideoClipsRelationshipsCommand(d),
			newAppEventLocalizationsCreateCommand(d),
			newAppEventLocalizationsUpdateCommand(d),
			newAppEventLocalizationsDeleteCommand(d),
			d.CreativePlacementsCommand("appEventLocalizations", "app-events localizations", "an in-app event localization"),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppEventLocalizationsListCommand returns the app event localizations list subcommand.
func newAppEventLocalizationsListCommand(d *shared.Deps) *ffcli.Command {
	var (
		includeValues   []string
		placementsLimit int
	)

	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "localizations list",
		Name:        "list",
		ShortUsage:  "asc app-events localizations list [flags]",
		ShortHelp:   "List localizations for an in-app event.",
		LongHelp: `List localizations for an in-app event.

Examples:
  asc app-events localizations list --event-id "EVENT_ID"
  asc app-events localizations list --event-id "EVENT_ID" --include placements --placements-limit 50
  asc app-events localizations list --event-id "EVENT_ID" --paginate`,
		ParentFlag:           "event-id",
		ParentUsage:          "App event ID",
		ParentType:           "appEvents",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		NextConflicts:        []string{"include", "placements-limit"},
		Flags: func(fs *flag.FlagSet) func() error {
			include := shared.BindLocalizationIncludeFlags(fs, "Include related resources", appEventLocalizationIncludeList())
			return func() error {
				var err error
				includeValues, placementsLimit, err = include.ResolveWith(d)
				return err
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventLocalizations(ctx, id,
				asc.Limit[asc.AppEventLocalizationsOption](limit),
				asc.NextURL[asc.AppEventLocalizationsOption](next),
				asc.WithAppEventLocalizationsInclude(includeValues),
				asc.WithAppEventLocalizationsPlacementsLimit(placementsLimit),
			)
		},
	})
}

// newAppEventLocalizationsGetCommand returns the app event localizations get subcommand.
func newAppEventLocalizationsGetCommand(d *shared.Deps) *ffcli.Command {
	var (
		includeValues   []string
		placementsLimit int
	)

	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:        d,
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
				includeValues, placementsLimit, err = include.ResolveWith(d)
				return err
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEventLocalization(ctx, id, asc.WithLocalizationReadInclude(includeValues), asc.WithLocalizationReadPlacementsLimit(placementsLimit))
		},
	})
}

// App Store Connect's character limits for an event localization.
const (
	appEventNameMaxLength             = 30
	appEventShortDescriptionMaxLength = 50
	appEventLongDescriptionMaxLength  = 120
)

// newAppEventLocalizationsCreateCommand returns the app event localizations create subcommand.
func newAppEventLocalizationsCreateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.AppEventLocalizationCreateAttributes
	return shared.BuildCreateCommand(shared.WriteCommandConfig{
		Deps:        d,
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
			shared.StringFlag(&attrs.Name, "name", "Localized name, up to 30 characters").WithMaxLength(appEventNameMaxLength),
			shared.StringFlag(&attrs.ShortDescription, "short-description", "Short description shown on the event card, up to 50 characters").WithMaxLength(appEventShortDescriptionMaxLength),
			shared.StringFlag(&attrs.LongDescription, "long-description", "Long description shown on the event details page, up to 120 characters").WithMaxLength(appEventLongDescriptionMaxLength),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.CreateAppEventLocalization(ctx, id, attrs)
		},
	})
}

// newAppEventLocalizationsUpdateCommand returns the app event localizations update subcommand.
func newAppEventLocalizationsUpdateCommand(d *shared.Deps) *ffcli.Command {
	var attrs asc.AppEventLocalizationUpdateAttributes
	return shared.BuildUpdateCommand(shared.WriteCommandConfig{
		Deps:        d,
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
			shared.OptionalStringFlag(&attrs.Name, "name", "Localized name, up to 30 characters").WithMaxLength(appEventNameMaxLength),
			shared.OptionalStringFlag(&attrs.ShortDescription, "short-description", "Short description shown on the event card, up to 50 characters").WithMaxLength(appEventShortDescriptionMaxLength),
			shared.OptionalStringFlag(&attrs.LongDescription, "long-description", "Long description shown on the event details page, up to 120 characters").WithMaxLength(appEventLongDescriptionMaxLength),
		},
		Send: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.UpdateAppEventLocalization(ctx, id, attrs)
		},
	})
}

// newAppEventLocalizationsDeleteCommand returns the app event localizations delete subcommand.
func newAppEventLocalizationsDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Deps:        d,
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
