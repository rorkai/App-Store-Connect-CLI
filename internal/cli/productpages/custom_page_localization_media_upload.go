package productpages

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var customPageMediaClientFactory = shared.GetASCClient

// CustomPageLocalizationsScreenshotSetsUploadCommand returns the screenshot sets upload subcommand.
func CustomPageLocalizationsScreenshotSetsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations screenshot-sets upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets upload --localization-id \"LOCALIZATION_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for a custom product page localization.",
		LongHelp: `Upload screenshots for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc product-pages custom-pages localizations screenshot-sets upload --localization-id "LOCALIZATION_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := executeCustomPageScreenshotUpload(ctx, *localizationID, *path, *deviceType, false)
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets upload: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsScreenshotSetsSyncCommand returns the screenshot sets sync subcommand.
func CustomPageLocalizationsScreenshotSetsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations screenshot-sets sync", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	confirm := fs.Bool("confirm", false, "Confirm sync (deletes existing media in the matching set before upload)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc product-pages custom-pages localizations screenshot-sets sync --localization-id \"LOCALIZATION_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\" --confirm",
		ShortHelp:  "Sync screenshots for a custom product page localization.",
		LongHelp: `Sync screenshots for a custom product page localization.

This replaces existing screenshots in the matching display-type set with files from --path.

Examples:
  asc product-pages custom-pages localizations screenshot-sets sync --localization-id "LOCALIZATION_ID" --path "./screenshots" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to sync")
				return flag.ErrHelp
			}

			result, err := executeCustomPageScreenshotUpload(ctx, *localizationID, *path, *deviceType, true)
			if err != nil {
				return fmt.Errorf("custom-pages localizations screenshot-sets sync: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsPreviewSetsUploadCommand returns the preview sets upload subcommand.
func CustomPageLocalizationsPreviewSetsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations preview-sets upload", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets upload --localization-id \"LOCALIZATION_ID\" --path \"./previews\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload previews for a custom product page localization.",
		LongHelp: `Upload previews for a custom product page localization.

Examples:
  asc product-pages custom-pages localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65"
  asc product-pages custom-pages localizations preview-sets upload --localization-id "LOCALIZATION_ID" --path "./previews/en-US.mov" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := executeCustomPagePreviewUpload(ctx, *localizationID, *path, *deviceType, false)
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets upload: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// CustomPageLocalizationsPreviewSetsSyncCommand returns the preview sets sync subcommand.
func CustomPageLocalizationsPreviewSetsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("custom-page-localizations preview-sets sync", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Custom product page localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	confirm := fs.Bool("confirm", false, "Confirm sync (deletes existing media in the matching set before upload)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc product-pages custom-pages localizations preview-sets sync --localization-id \"LOCALIZATION_ID\" --path \"./previews\" --device-type \"IPHONE_65\" --confirm",
		ShortHelp:  "Sync previews for a custom product page localization.",
		LongHelp: `Sync previews for a custom product page localization.

This replaces existing previews in the matching preview-type set with files from --path.

Examples:
  asc product-pages custom-pages localizations preview-sets sync --localization-id "LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to sync")
				return flag.ErrHelp
			}

			result, err := executeCustomPagePreviewUpload(ctx, *localizationID, *path, *deviceType, true)
			if err != nil {
				return fmt.Errorf("custom-pages localizations preview-sets sync: %w", err)
			}
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func executeCustomPageScreenshotUpload(
	ctx context.Context,
	localizationID, path, deviceType string,
	sync bool,
) (*asc.CustomProductPageScreenshotUploadResult, error) {
	trimmedLocalizationID := strings.TrimSpace(localizationID)
	if trimmedLocalizationID == "" {
		fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
		return nil, flag.ErrHelp
	}
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --path is required")
		return nil, flag.ErrHelp
	}
	trimmedDeviceType := strings.TrimSpace(deviceType)
	if trimmedDeviceType == "" {
		fmt.Fprintln(os.Stderr, "Error: --device-type is required")
		return nil, flag.ErrHelp
	}

	displayType, err := assets.NormalizeScreenshotDisplayType(trimmedDeviceType)
	if err != nil {
		return nil, err
	}
	files, err := collectCustomPageMediaFiles(trimmedPath)
	if err != nil {
		return nil, err
	}
	if err := assets.ValidateScreenshotDimensions(files, displayType); err != nil {
		return nil, err
	}

	client, err := customPageMediaClientFactory()
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := contextWithCustomPageMediaUploadTimeout(ctx)
	defer cancel()

	set, err := ensureCustomPageLocalizationScreenshotSet(requestCtx, client, trimmedLocalizationID, displayType)
	if err != nil {
		return nil, err
	}
	if sync {
		if err := deleteAllScreenshotsInSet(requestCtx, client, set.ID); err != nil {
			return nil, err
		}
	}

	results, err := assets.UploadScreenshotsToSet(requestCtx, client, set.ID, files, !sync)
	if err != nil {
		return nil, err
	}

	return &asc.CustomProductPageScreenshotUploadResult{
		CustomProductPageLocalizationID: trimmedLocalizationID,
		SetID:                           set.ID,
		DisplayType:                     set.Attributes.ScreenshotDisplayType,
		Results:                         results,
	}, nil
}

func executeCustomPagePreviewUpload(
	ctx context.Context,
	localizationID, path, deviceType string,
	sync bool,
) (*asc.CustomProductPagePreviewUploadResult, error) {
	trimmedLocalizationID := strings.TrimSpace(localizationID)
	if trimmedLocalizationID == "" {
		fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
		return nil, flag.ErrHelp
	}
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		fmt.Fprintln(os.Stderr, "Error: --path is required")
		return nil, flag.ErrHelp
	}
	trimmedDeviceType := strings.TrimSpace(deviceType)
	if trimmedDeviceType == "" {
		fmt.Fprintln(os.Stderr, "Error: --device-type is required")
		return nil, flag.ErrHelp
	}

	previewType, err := assets.NormalizePreviewType(trimmedDeviceType)
	if err != nil {
		return nil, err
	}
	files, err := collectCustomPageMediaFiles(trimmedPath)
	if err != nil {
		return nil, err
	}

	client, err := customPageMediaClientFactory()
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := contextWithCustomPageMediaUploadTimeout(ctx)
	defer cancel()

	set, err := ensureCustomPageLocalizationPreviewSet(requestCtx, client, trimmedLocalizationID, previewType)
	if err != nil {
		return nil, err
	}
	if sync {
		if err := deleteAllPreviewsInSet(requestCtx, client, set.ID); err != nil {
			return nil, err
		}
	}

	results := make([]asc.AssetUploadResultItem, 0, len(files))
	for _, filePath := range files {
		item, err := uploadCustomPagePreviewAsset(requestCtx, client, set.ID, filePath)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return &asc.CustomProductPagePreviewUploadResult{
		CustomProductPageLocalizationID: trimmedLocalizationID,
		SetID:                           set.ID,
		PreviewType:                     set.Attributes.PreviewType,
		Results:                         results,
	}, nil
}

func contextWithCustomPageMediaUploadTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return assets.ContextWithAssetUploadTimeout(ctx)
}

func collectCustomPageMediaFiles(path string) ([]string, error) {
	return assets.CollectAssetFiles(path)
}

func ensureCustomPageLocalizationScreenshotSet(ctx context.Context, client *asc.Client, localizationID, displayType string) (asc.Resource[asc.AppScreenshotSetAttributes], error) {
	resp, err := client.GetAppCustomProductPageLocalizationScreenshotSets(ctx, localizationID)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.ScreenshotDisplayType, displayType) {
			return set, nil
		}
	}
	created, err := client.CreateAppScreenshotSetForCustomProductPageLocalization(ctx, localizationID, displayType)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	return created.Data, nil
}

func ensureCustomPageLocalizationPreviewSet(ctx context.Context, client *asc.Client, localizationID, previewType string) (asc.Resource[asc.AppPreviewSetAttributes], error) {
	resp, err := client.GetAppCustomProductPageLocalizationPreviewSets(ctx, localizationID)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.PreviewType, previewType) {
			return set, nil
		}
	}
	created, err := client.CreateAppPreviewSetForCustomProductPageLocalization(ctx, localizationID, previewType)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, err
	}
	return created.Data, nil
}

func deleteAllScreenshotsInSet(ctx context.Context, client *asc.Client, setID string) error {
	resp, err := client.GetAppScreenshots(ctx, setID)
	if err != nil {
		return err
	}
	for _, screenshot := range resp.Data {
		if err := client.DeleteAppScreenshot(ctx, screenshot.ID); err != nil {
			return err
		}
	}
	return nil
}

func deleteAllPreviewsInSet(ctx context.Context, client *asc.Client, setID string) error {
	resp, err := client.GetAppPreviews(ctx, setID)
	if err != nil {
		return err
	}
	for _, preview := range resp.Data {
		if err := client.DeleteAppPreview(ctx, preview.ID); err != nil {
			return err
		}
	}
	return nil
}

func uploadCustomPagePreviewAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	return assets.UploadPreviewAsset(ctx, client, setID, filePath)
}
