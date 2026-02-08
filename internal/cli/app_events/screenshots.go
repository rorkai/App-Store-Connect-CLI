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

// AppEventScreenshotsCommand returns the app event screenshots command group.
func AppEventScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshots",
		ShortUsage: "asc app-events screenshots <subcommand> [flags]",
		ShortHelp:  "Manage in-app event screenshots.",
		LongHelp: `Manage in-app event screenshots.

Examples:
  asc app-events screenshots list --event-id "EVENT_ID"
  asc app-events screenshots create --localization-id "LOC_ID" --path "./event.png" --asset-type EVENT_CARD`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppEventScreenshotsListCommand(),
			AppEventScreenshotsRelationshipsCommand(),
			AppEventScreenshotsGetCommand(),
			AppEventScreenshotsCreateCommand(),
			AppEventScreenshotsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppEventScreenshotsRelationshipsCommand returns the app event screenshots relationships subcommand.
func AppEventScreenshotsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots relationships", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	localizationID := fs.String("localization-id", "", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc app-events screenshots relationships [flags]",
		ShortHelp:  "List screenshot relationships for an in-app event localization.",
		LongHelp: `List screenshot relationships for an in-app event localization.

Examples:
  asc app-events screenshots relationships --localization-id "LOC_ID"
  asc app-events screenshots relationships --event-id "EVENT_ID" --locale "en-US"
  asc app-events screenshots relationships --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events screenshots relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-events screenshots relationships: %w", err)
			}

			trimmedNext := strings.TrimSpace(*next)
			if trimmedNext == "" && strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id or --localization-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events screenshots relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedLocalizationID := strings.TrimSpace(*localizationID)
			if trimmedNext == "" {
				resolvedLocalizationID, err = resolveAppEventLocalizationID(requestCtx, client, *eventID, resolvedLocalizationID, *locale)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(200))
				firstPage, err := client.GetAppEventScreenshotsRelationships(requestCtx, resolvedLocalizationID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events screenshots relationships: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshotsRelationships(ctx, resolvedLocalizationID, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events screenshots relationships: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventScreenshotsRelationships(requestCtx, resolvedLocalizationID, opts...)
			if err != nil {
				return fmt.Errorf("app-events screenshots relationships: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppEventScreenshotsListCommand returns the app event screenshots list subcommand.
func AppEventScreenshotsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots list", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	localizationID := fs.String("localization-id", "", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc app-events screenshots list [flags]",
		ShortHelp:  "List screenshots for an in-app event localization.",
		LongHelp: `List screenshots for an in-app event localization.

Examples:
  asc app-events screenshots list --localization-id "LOC_ID"
  asc app-events screenshots list --event-id "EVENT_ID" --locale "en-US"
  asc app-events screenshots list --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("app-events screenshots list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("app-events screenshots list: %w", err)
			}
			if strings.TrimSpace(*next) == "" && strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id or --localization-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events screenshots list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedLocalizationID := strings.TrimSpace(*localizationID)
			if strings.TrimSpace(*next) == "" {
				resolvedLocalizationID, err = resolveAppEventLocalizationID(requestCtx, client, *eventID, resolvedLocalizationID, *locale)
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err.Error())
					return flag.ErrHelp
				}
			}

			opts := []asc.AppEventScreenshotsOption{
				asc.WithAppEventScreenshotsLimit(*limit),
				asc.WithAppEventScreenshotsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithAppEventScreenshotsLimit(200))
				firstPage, err := client.GetAppEventScreenshots(requestCtx, resolvedLocalizationID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events screenshots list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshots(ctx, resolvedLocalizationID, asc.WithAppEventScreenshotsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events screenshots list: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAppEventScreenshots(requestCtx, resolvedLocalizationID, opts...)
			if err != nil {
				return fmt.Errorf("app-events screenshots list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppEventScreenshotsGetCommand returns the app event screenshots get subcommand.
func AppEventScreenshotsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots get", flag.ExitOnError)

	screenshotID := fs.String("screenshot-id", "", "App event screenshot ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-events screenshots get --screenshot-id \"SHOT_ID\"",
		ShortHelp:  "Get an in-app event screenshot by ID.",
		LongHelp: `Get an in-app event screenshot by ID.

Examples:
  asc app-events screenshots get --screenshot-id "SHOT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events screenshots get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppEventScreenshot(requestCtx, id)
			if err != nil {
				return fmt.Errorf("app-events screenshots get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppEventScreenshotsCreateCommand returns the app event screenshots create subcommand.
func AppEventScreenshotsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots create", flag.ExitOnError)

	eventID := fs.String("event-id", "", "App event ID")
	localizationID := fs.String("localization-id", "", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	path := fs.String("path", "", "Path to screenshot file")
	assetType := fs.String("asset-type", "", "Asset type: "+strings.Join(asc.ValidAppEventAssetTypes, ", "))
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-events screenshots create [flags]",
		ShortHelp:  "Upload a screenshot for an in-app event localization.",
		LongHelp: `Upload a screenshot for an in-app event localization.

Examples:
  asc app-events screenshots create --localization-id "LOC_ID" --path "./event.png" --asset-type EVENT_CARD
  asc app-events screenshots create --event-id "EVENT_ID" --locale "en-US" --path "./event.png" --asset-type EVENT_DETAILS_PAGE`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id or --localization-id is required")
				return flag.ErrHelp
			}

			normalizedAssetType, err := normalizeAppEventAssetType(*assetType)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events screenshots create: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			resolvedLocalizationID, err := resolveAppEventLocalizationID(requestCtx, client, *eventID, *localizationID, *locale)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err.Error())
				return flag.ErrHelp
			}

			file, info, err := openAssetFile(pathValue)
			if err != nil {
				return fmt.Errorf("app-events screenshots create: %w", err)
			}
			defer file.Close()

			resp, err := client.CreateAppEventScreenshot(requestCtx, resolvedLocalizationID, info.Name(), info.Size(), normalizedAssetType)
			if err != nil {
				return fmt.Errorf("app-events screenshots create: failed to create: %w", err)
			}
			if resp == nil || len(resp.Data.Attributes.UploadOperations) == 0 {
				return fmt.Errorf("app-events screenshots create: no upload operations returned")
			}

			if err := asc.UploadAssetFromFile(requestCtx, file, info.Size(), resp.Data.Attributes.UploadOperations); err != nil {
				return fmt.Errorf("app-events screenshots create: upload failed: %w", err)
			}

			_, err = client.UpdateAppEventScreenshot(requestCtx, resp.Data.ID, true)
			if err != nil {
				return fmt.Errorf("app-events screenshots create: failed to commit upload: %w", err)
			}

			finalResp, err := waitForAppEventScreenshotDelivery(requestCtx, client, resp.Data.ID)
			if err != nil {
				return fmt.Errorf("app-events screenshots create: %w", err)
			}
			if finalResp != nil {
				return shared.PrintOutput(finalResp, *output, *pretty)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AppEventScreenshotsDeleteCommand returns the app event screenshots delete subcommand.
func AppEventScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots delete", flag.ExitOnError)

	screenshotID := fs.String("screenshot-id", "", "App event screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-events screenshots delete --screenshot-id \"SHOT_ID\" --confirm",
		ShortHelp:  "Delete an in-app event screenshot.",
		LongHelp: `Delete an in-app event screenshot.

Examples:
  asc app-events screenshots delete --screenshot-id "SHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*screenshotID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --screenshot-id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppEventScreenshot(requestCtx, id); err != nil {
				return fmt.Errorf("app-events screenshots delete: failed to delete: %w", err)
			}

			result := &asc.AssetDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
