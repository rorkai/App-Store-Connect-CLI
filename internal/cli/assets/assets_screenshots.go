package assets

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

// AssetsScreenshotsCommand returns the screenshots subcommand group.
func AssetsScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshots",
		ShortUsage: "asc assets screenshots <subcommand> [flags]",
		ShortHelp:  "Manage App Store screenshots.",
		LongHelp: `Manage App Store screenshots.

Examples:
  asc assets screenshots list --version-localization "LOC_ID"
  asc assets screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc assets screenshots delete --id "SCREENSHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AssetsScreenshotsListCommand(),
			AssetsScreenshotsUploadCommand(),
			AssetsScreenshotsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AssetsScreenshotsListCommand returns the screenshots list subcommand.
func AssetsScreenshotsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc assets screenshots list --version-localization \"LOC_ID\"",
		ShortHelp:  "List screenshots for a localization.",
		LongHelp: `List screenshots for a localization.

Examples:
  asc assets screenshots list --version-localization "LOC_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-localization is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("assets screenshots list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			setsResp, err := client.GetAppScreenshotSets(requestCtx, locID)
			if err != nil {
				return fmt.Errorf("assets screenshots list: failed to fetch sets: %w", err)
			}

			result := asc.AppScreenshotListResult{
				VersionLocalizationID: locID,
				Sets:                  make([]asc.AppScreenshotSetWithScreenshots, 0, len(setsResp.Data)),
			}

			for _, set := range setsResp.Data {
				screenshots, err := client.GetAppScreenshots(requestCtx, set.ID)
				if err != nil {
					return fmt.Errorf("assets screenshots list: failed to fetch screenshots for set %s: %w", set.ID, err)
				}
				result.Sets = append(result.Sets, asc.AppScreenshotSetWithScreenshots{
					Set:         set,
					Screenshots: screenshots.Data,
				})
			}

			return shared.PrintOutput(&result, *output, *pretty)
		},
	}
}

// AssetsScreenshotsUploadCommand returns the screenshots upload subcommand.
func AssetsScreenshotsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc assets screenshots upload --version-localization \"LOC_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for a localization.",
		LongHelp: `Upload screenshots for a localization.

Examples:
  asc assets screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc assets screenshots upload --version-localization "LOC_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-localization is required")
				return flag.ErrHelp
			}
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return flag.ErrHelp
			}
			deviceValue := strings.TrimSpace(*deviceType)
			if deviceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --device-type is required")
				return flag.ErrHelp
			}

			displayType, err := normalizeScreenshotDisplayType(deviceValue)
			if err != nil {
				return fmt.Errorf("assets screenshots upload: %w", err)
			}

			files, err := collectAssetFiles(pathValue)
			if err != nil {
				return fmt.Errorf("assets screenshots upload: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("assets screenshots upload: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			set, err := ensureScreenshotSet(requestCtx, client, locID, displayType)
			if err != nil {
				return fmt.Errorf("assets screenshots upload: %w", err)
			}

			results := make([]asc.AssetUploadResultItem, 0, len(files))
			for _, filePath := range files {
				item, err := uploadScreenshotAsset(requestCtx, client, set.ID, filePath)
				if err != nil {
					return fmt.Errorf("assets screenshots upload: %w", err)
				}
				results = append(results, item)
			}

			result := asc.AppScreenshotUploadResult{
				VersionLocalizationID: locID,
				SetID:                 set.ID,
				DisplayType:           set.Attributes.ScreenshotDisplayType,
				Results:               results,
			}

			return shared.PrintOutput(&result, *output, *pretty)
		},
	}
}

// AssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func AssetsScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc assets screenshots delete --id \"SCREENSHOT_ID\" --confirm",
		ShortHelp:  "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Examples:
  asc assets screenshots delete --id "SCREENSHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetID := strings.TrimSpace(*id)
			if assetID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("assets screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppScreenshot(requestCtx, assetID); err != nil {
				return fmt.Errorf("assets screenshots delete: %w", err)
			}

			result := asc.AssetDeleteResult{
				ID:      assetID,
				Deleted: true,
			}

			return shared.PrintOutput(&result, *output, *pretty)
		},
	}
}

func normalizeScreenshotDisplayType(input string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("device type is required")
	}
	if !strings.HasPrefix(value, "APP_") {
		value = "APP_" + value
	}
	if !asc.IsValidScreenshotDisplayType(value) {
		return "", fmt.Errorf("unsupported screenshot display type %q", value)
	}
	return value, nil
}

func ensureScreenshotSet(ctx context.Context, client *asc.Client, localizationID, displayType string) (asc.Resource[asc.AppScreenshotSetAttributes], error) {
	resp, err := client.GetAppScreenshotSets(ctx, localizationID)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.ScreenshotDisplayType, displayType) {
			return set, nil
		}
	}
	created, err := client.CreateAppScreenshotSet(ctx, localizationID, displayType)
	if err != nil {
		return asc.Resource[asc.AppScreenshotSetAttributes]{}, err
	}
	return created.Data, nil
}

func uploadScreenshotAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	if err := asc.ValidateImageFile(filePath); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	file, err := shared.OpenExistingNoFollow(filePath)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	created, err := client.CreateAppScreenshot(ctx, setID, info.Name(), info.Size())
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}
	if len(created.Data.Attributes.UploadOperations) == 0 {
		return asc.AssetUploadResultItem{}, fmt.Errorf("no upload operations returned for %q", info.Name())
	}

	if err := asc.UploadAssetFromFile(ctx, file, info.Size(), created.Data.Attributes.UploadOperations); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	if _, err := client.UpdateAppScreenshot(ctx, created.Data.ID, true, checksum.Hash); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	state, err := waitForScreenshotDelivery(ctx, client, created.Data.ID)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	return asc.AssetUploadResultItem{
		FileName: info.Name(),
		FilePath: filePath,
		AssetID:  created.Data.ID,
		State:    state,
	}, nil
}

func waitForScreenshotDelivery(ctx context.Context, client *asc.Client, screenshotID string) (string, error) {
	return waitForAssetDeliveryState(ctx, screenshotID, func(ctx context.Context) (*asc.AssetDeliveryState, error) {
		resp, err := client.GetAppScreenshot(ctx, screenshotID)
		if err != nil {
			return nil, err
		}
		return resp.Data.Attributes.AssetDeliveryState, nil
	})
}
