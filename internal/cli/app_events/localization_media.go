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
		UsageFunc: DefaultUsageFunc,
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
	fs := flag.NewFlagSet("localizations screenshots list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events localizations screenshots list --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "List screenshots for an in-app event localization.",
		LongHelp: `List screenshots for an in-app event localization.

Examples:
  asc app-events localizations screenshots list --localization-id "LOC_ID"
  asc app-events localizations screenshots list --localization-id "LOC_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events localizations screenshots list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events localizations screenshots list: %w", err)
			}

			id := strings.TrimSpace(*localizationID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations screenshots list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEventScreenshotsOption{
				asc.WithAppEventScreenshotsLimit(*limit),
				asc.WithAppEventScreenshotsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEventScreenshotsLimit(200))
				firstPage, err := client.GetAppEventScreenshots(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations screenshots list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshots(ctx, id, asc.WithAppEventScreenshotsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations screenshots list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventScreenshots(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations screenshots list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
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
		UsageFunc: DefaultUsageFunc,
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
	fs := flag.NewFlagSet("localizations video-clips list", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events localizations video-clips list --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "List video clips for an in-app event localization.",
		LongHelp: `List video clips for an in-app event localization.

Examples:
  asc app-events localizations video-clips list --localization-id "LOC_ID"
  asc app-events localizations video-clips list --localization-id "LOC_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events localizations video-clips list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events localizations video-clips list: %w", err)
			}

			id := strings.TrimSpace(*localizationID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations video-clips list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.AppEventVideoClipsOption{
				asc.WithAppEventVideoClipsLimit(*limit),
				asc.WithAppEventVideoClipsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEventVideoClipsLimit(200))
				firstPage, err := client.GetAppEventVideoClips(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations video-clips list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventVideoClips(ctx, id, asc.WithAppEventVideoClipsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations video-clips list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventVideoClips(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations video-clips list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventLocalizationScreenshotsRelationshipsCommand returns the screenshot relationships subcommand.
func AppEventLocalizationScreenshotsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations screenshots-relationships", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "screenshots-relationships",
		ShortUsage: "asc app-events localizations screenshots-relationships --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "List screenshot relationships for an in-app event localization.",
		LongHelp: `List screenshot relationships for an in-app event localization.

Examples:
  asc app-events localizations screenshots-relationships --localization-id "LOC_ID"
  asc app-events localizations screenshots-relationships --localization-id "LOC_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events localizations screenshots-relationships: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events localizations screenshots-relationships: %w", err)
			}

			id := strings.TrimSpace(*localizationID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations screenshots-relationships: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppEventScreenshotsRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations screenshots-relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshotsRelationships(ctx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations screenshots-relationships: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventScreenshotsRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations screenshots-relationships: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// AppEventLocalizationVideoClipsRelationshipsCommand returns the video clip relationships subcommand.
func AppEventLocalizationVideoClipsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("localizations video-clips-relationships", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "App event localization ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "video-clips-relationships",
		ShortUsage: "asc app-events localizations video-clips-relationships --localization-id \"LOC_ID\" [flags]",
		ShortHelp:  "List video clip relationships for an in-app event localization.",
		LongHelp: `List video clip relationships for an in-app event localization.

Examples:
  asc app-events localizations video-clips-relationships --localization-id "LOC_ID"
  asc app-events localizations video-clips-relationships --localization-id "LOC_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events localizations video-clips-relationships: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("app-events localizations video-clips-relationships: %w", err)
			}

			id := strings.TrimSpace(*localizationID)
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("app-events localizations video-clips-relationships: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppEventVideoClipsRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events localizations video-clips-relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventVideoClipsRelationships(ctx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events localizations video-clips-relationships: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventVideoClipsRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("app-events localizations video-clips-relationships: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
