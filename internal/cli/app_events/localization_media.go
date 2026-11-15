package app_events

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppEventLocalizationScreenshotsCommand returns the app event localization screenshots group.
func newAppEventLocalizationScreenshotsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshots",
		ShortUsage: "asc app-events localizations screenshots <subcommand> [flags]",
		ShortHelp:  "Manage localization screenshots for in-app events.",
		LongHelp: `Manage localization screenshots for in-app events.

Examples:
  asc app-events localizations screenshots list --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppEventLocalizationScreenshotsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppEventLocalizationScreenshotsListCommand returns the localization screenshots list subcommand.
func newAppEventLocalizationScreenshotsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "localizations screenshots list",
		Name:        "list",
		ShortUsage:  "asc app-events localizations screenshots list --localization-id \"LOC_ID\" [flags]",
		ShortHelp:   "List screenshots for an in-app event localization.",
		LongHelp: `List screenshots for an in-app event localization.

Examples:
  asc app-events localizations screenshots list --localization-id "LOC_ID"
  asc app-events localizations screenshots list --localization-id "LOC_ID" --paginate`,
		ParentFlag:           "localization-id",
		ParentUsage:          "App event localization ID",
		ParentType:           "appEventLocalizations",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventScreenshots(ctx, parentID,
				asc.Limit[asc.AppEventScreenshotsOption](limit),
				asc.NextURL[asc.AppEventScreenshotsOption](next),
			)
		},
	})
}

// newAppEventLocalizationVideoClipsCommand returns the app event localization video clips group.
func newAppEventLocalizationVideoClipsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("localizations video-clips", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "video-clips",
		ShortUsage: "asc app-events localizations video-clips <subcommand> [flags]",
		ShortHelp:  "Manage localization video clips for in-app events.",
		LongHelp: `Manage localization video clips for in-app events.

Examples:
  asc app-events localizations video-clips list --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppEventLocalizationVideoClipsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppEventLocalizationVideoClipsListCommand returns the localization video clips list subcommand.
func newAppEventLocalizationVideoClipsListCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "localizations video-clips list",
		Name:        "list",
		ShortUsage:  "asc app-events localizations video-clips list --localization-id \"LOC_ID\" [flags]",
		ShortHelp:   "List video clips for an in-app event localization.",
		LongHelp: `List video clips for an in-app event localization.

Examples:
  asc app-events localizations video-clips list --localization-id "LOC_ID"
  asc app-events localizations video-clips list --localization-id "LOC_ID" --paginate`,
		ParentFlag:           "localization-id",
		ParentUsage:          "App event localization ID",
		ParentType:           "appEventLocalizations",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventVideoClips(ctx, parentID,
				asc.Limit[asc.AppEventVideoClipsOption](limit),
				asc.NextURL[asc.AppEventVideoClipsOption](next),
			)
		},
	})
}

// newAppEventLocalizationScreenshotsRelationshipsCommand returns the screenshot links subcommand.
func newAppEventLocalizationScreenshotsRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "localizations screenshots-links",
		Name:        "screenshots-links",
		ShortUsage:  "asc app-events localizations screenshots-links --localization-id \"LOC_ID\" [flags]",
		ShortHelp:   "List screenshot relationships for an in-app event localization.",
		LongHelp: `List screenshot relationships for an in-app event localization.

Examples:
  asc app-events localizations screenshots-links --localization-id "LOC_ID"
  asc app-events localizations screenshots-links --localization-id "LOC_ID" --paginate`,
		ParentFlag:           "localization-id",
		ParentUsage:          "App event localization ID",
		ParentType:           "appEventLocalizations",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventScreenshotsRelationships(ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
}

// newAppEventLocalizationVideoClipsRelationshipsCommand returns the video clip links subcommand.
func newAppEventLocalizationVideoClipsRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:        d,
		FlagSetName: "localizations video-clips-links",
		Name:        "video-clips-links",
		ShortUsage:  "asc app-events localizations video-clips-links --localization-id \"LOC_ID\" [flags]",
		ShortHelp:   "List video clip relationships for an in-app event localization.",
		LongHelp: `List video clip relationships for an in-app event localization.

Examples:
  asc app-events localizations video-clips-links --localization-id "LOC_ID"
  asc app-events localizations video-clips-links --localization-id "LOC_ID" --paginate`,
		ParentFlag:           "localization-id",
		ParentUsage:          "App event localization ID",
		ParentType:           "appEventLocalizations",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		NoSpinner:            true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventVideoClipsRelationships(ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
}
