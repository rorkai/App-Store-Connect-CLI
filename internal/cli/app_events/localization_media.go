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

// AppEventLocalizationScreenshotsCommand returns the app event localization screenshots group.
func AppEventLocalizationScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshots",
		ShortUsage: "asc app-events localizations screenshots <subcommand> [flags]",
		ShortHelp:  "Manage localization screenshots for in-app events.",
		LongHelp: `Manage localization screenshots for in-app events.

Examples:
  asc app-events localizations screenshots list --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppEventLocalizationScreenshotsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEventLocalizationScreenshotsListCommand returns the localization screenshots list subcommand.
func AppEventLocalizationScreenshotsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
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
		ErrorPrefix:          "app-events localizations screenshots list",
		FetchErrorPrefix:     "app-events localizations screenshots list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventScreenshots(ctx, parentID,
				asc.Limit[asc.AppEventScreenshotsOption](limit),
				asc.NextURL[asc.AppEventScreenshotsOption](next),
			)
		},
	})
}

// AppEventLocalizationVideoClipsCommand returns the app event localization video clips group.
func AppEventLocalizationVideoClipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations video-clips", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "video-clips",
		ShortUsage: "asc app-events localizations video-clips <subcommand> [flags]",
		ShortHelp:  "Manage localization video clips for in-app events.",
		LongHelp: `Manage localization video clips for in-app events.

Examples:
  asc app-events localizations video-clips list --localization-id "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppEventLocalizationVideoClipsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEventLocalizationVideoClipsListCommand returns the localization video clips list subcommand.
func AppEventLocalizationVideoClipsListCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
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
		ErrorPrefix:          "app-events localizations video-clips list",
		FetchErrorPrefix:     "app-events localizations video-clips list: failed to fetch",
		FetchPage: func(ctx context.Context, client *asc.Client, parentID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAppEventVideoClips(ctx, parentID,
				asc.Limit[asc.AppEventVideoClipsOption](limit),
				asc.NextURL[asc.AppEventVideoClipsOption](next),
			)
		},
	})
}

// AppEventLocalizationScreenshotsRelationshipsCommand returns the screenshot links subcommand.
func AppEventLocalizationScreenshotsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations screenshots-links", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "screenshots-links",
		ShortUsage: "asc app-events localizations screenshots-links --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "List screenshot relationships for an in-app event localization.",
		LongHelp: `List screenshot relationships for an in-app event localization.

Examples:
  asc app-events localizations screenshots-links --localization-id "LOC_ID"
  asc app-events localizations screenshots-links --localization-id "LOC_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("app-events localizations screenshots-links: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("app-events localizations screenshots-links: %v", err)
			}

			id := strings.TrimSpace(*localizationID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--localization-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations screenshots-links: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				firstPage, err := client.GetAppEventScreenshotsRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations screenshots-links: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshotsRelationships(ctx, id, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations screenshots-links: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventScreenshotsRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations screenshots-links: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventLocalizationVideoClipsRelationshipsCommand returns the video clip links subcommand.
func AppEventLocalizationVideoClipsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations video-clips-links", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "video-clips-links",
		ShortUsage: "asc app-events localizations video-clips-links --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "List video clip relationships for an in-app event localization.",
		LongHelp: `List video clip relationships for an in-app event localization.

Examples:
  asc app-events localizations video-clips-links --localization-id "LOC_ID"
  asc app-events localizations video-clips-links --localization-id "LOC_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("app-events localizations video-clips-links: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("app-events localizations video-clips-links: %v", err)
			}

			id := strings.TrimSpace(*localizationID)
			if id == "" && strings.TrimSpace(*next) == "" {
				return shared.RequiredFlag("--localization-id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations video-clips-links: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				firstPage, err := client.GetAppEventVideoClipsRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations video-clips-links: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventVideoClipsRelationships(ctx, id, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations video-clips-links: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventVideoClipsRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations video-clips-links: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
