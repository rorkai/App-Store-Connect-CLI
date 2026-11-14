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
		UsageFunc: shared.VisibleUsageFunc,
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

// AppEventScreenshotsRelationshipsCommand returns the app event screenshots links subcommand.
func AppEventScreenshotsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots links", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "links",
		ShortUsage: "asc app-events screenshots links [flags]",
		ShortHelp:  "List screenshot relationships for an in-app event localization.",
		LongHelp: `List screenshot relationships for an in-app event localization.

Examples:
  asc app-events screenshots links --localization-id "LOC_ID"
  asc app-events screenshots links --event-id "EVENT_ID" --locale "en-US"
  asc app-events screenshots links --event-id "EVENT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return shared.UsageError("app-events screenshots links: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("app-events screenshots links: %v", err)
			}

			trimmedNext := strings.TrimSpace(*next)
			if trimmedNext == "" && strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-events screenshots links: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedLocalizationID := strings.TrimSpace(*localizationID)
			if trimmedNext == "" {
				resolvedLocalizationID, err = resolveAppEventLocalizationID(requestCtx, client, *eventID, resolvedLocalizationID, *locale)
				if err != nil {
					return fmt.Errorf("app-events screenshots links: %w", err)
				}
			}

			opts := []asc.LinkagesOption{
				asc.Limit[asc.LinkagesOption](*limit),
				asc.NextURL[asc.LinkagesOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.LinkagesOption](200))
				firstPage, err := client.GetAppEventScreenshotsRelationships(requestCtx, resolvedLocalizationID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events screenshots links: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshotsRelationships(ctx, resolvedLocalizationID, asc.NextURL[asc.LinkagesOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events screenshots links: %w", err)
				}
				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventScreenshotsRelationships(requestCtx, resolvedLocalizationID, opts...)
			if err != nil {
				return fmt.Errorf("app-events screenshots links: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventScreenshotsListCommand returns the app event screenshots list subcommand.
func AppEventScreenshotsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots list", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

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
				return shared.UsageError("app-events screenshots list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("app-events screenshots list: %v", err)
			}
			if strings.TrimSpace(*next) == "" && strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
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
					return fmt.Errorf("app-events screenshots list: %w", err)
				}
			}

			opts := []asc.AppEventScreenshotsOption{
				asc.Limit[asc.AppEventScreenshotsOption](*limit),
				asc.NextURL[asc.AppEventScreenshotsOption](*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.Limit[asc.AppEventScreenshotsOption](200))
				firstPage, err := client.GetAppEventScreenshots(requestCtx, resolvedLocalizationID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("app-events screenshots list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAppEventScreenshots(ctx, resolvedLocalizationID, asc.NextURL[asc.AppEventScreenshotsOption](nextURL))
				})
				if err != nil {
					return fmt.Errorf("app-events screenshots list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAppEventScreenshots(requestCtx, resolvedLocalizationID, opts...)
			if err != nil {
				return fmt.Errorf("app-events screenshots list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventScreenshotsGetCommand returns the app event screenshots get subcommand.
func AppEventScreenshotsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "screenshots view",
		Name:        "view",
		ShortUsage:  "asc app-events screenshots view --screenshot-id \"SHOT_ID\"",
		ShortHelp:   "View an in-app event screenshot by ID.",
		LongHelp: `View an in-app event screenshot by ID.

Examples:
  asc app-events screenshots view --screenshot-id "SHOT_ID"`,
		IDFlag:             "screenshot-id",
		IDUsage:            "App event screenshot ID",
		IDType:             "appEventScreenshots",
		RequiredDiagnostic: true,
		ErrorPrefix:        "app-events screenshots view",
		FetchErrorPrefix:   "app-events screenshots view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppEventScreenshot(ctx, id)
		},
	})
}

// AppEventScreenshotsCreateCommand returns the app event screenshots create subcommand.
func AppEventScreenshotsCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots create", flag.ExitOnError)

	eventID := shared.BindResourceIDFlag(fs, "event-id", "appEvents", "App event ID")
	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appEventLocalizations", "App event localization ID")
	locale := fs.String("locale", "", "Locale (e.g., en-US) when resolving localization")
	path := fs.String("path", "", "Path to screenshot file")
	assetType := fs.String("asset-type", "", "Asset type: "+strings.Join(asc.ValidAppEventAssetTypes, ", "))
	output := shared.BindOutputFlags(fs)

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
				return shared.RequiredFlag("--path")
			}
			if strings.TrimSpace(*localizationID) == "" && strings.TrimSpace(*eventID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --event-id or --localization-id is required")
				return shared.MissingRequiredUsageError("")
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
				return fmt.Errorf("app-events screenshots create: %w", err)
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
				return shared.PrintOutput(finalResp, *output.Output, *output.Pretty)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppEventScreenshotsDeleteCommand returns the app event screenshots delete subcommand.
func AppEventScreenshotsDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "screenshots delete",
		Name:        "delete",
		ShortUsage:  "asc app-events screenshots delete --screenshot-id \"SHOT_ID\" --confirm",
		ShortHelp:   "Delete an in-app event screenshot.",
		LongHelp: `Delete an in-app event screenshot.

Examples:
  asc app-events screenshots delete --screenshot-id "SHOT_ID" --confirm`,
		IDFlag:             "screenshot-id",
		IDUsage:            "App event screenshot ID",
		IDType:             "appEventScreenshots",
		RequiredDiagnostic: true,
		ErrorPrefix:        "app-events screenshots delete",
		DeleteErrorPrefix:  "app-events screenshots delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppEventScreenshot(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AssetDeleteResult{ID: id, Deleted: true}
		},
	})
}
