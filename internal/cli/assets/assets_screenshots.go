package assets

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var focusedScreenshotDisplayTypes = []string{
	"APP_IPHONE_65",
	"APP_IPAD_PRO_3GEN_129",
}

func focusedScreenshotSizeCatalog() []asc.ScreenshotSizeEntry {
	focused := make([]asc.ScreenshotSizeEntry, 0, len(focusedScreenshotDisplayTypes))
	for _, displayType := range focusedScreenshotDisplayTypes {
		entry, ok := asc.ScreenshotSizeEntryForDisplayType(displayType)
		if !ok {
			continue
		}
		focused = append(focused, entry)
	}
	if len(focused) == 0 {
		return asc.ScreenshotSizeCatalog()
	}
	return focused
}

// AssetsScreenshotsCommand returns the screenshots subcommand group.
func AssetsScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshots",
		ShortUsage: "asc screenshots <subcommand> [flags]",
		ShortHelp:  "Manage App Store screenshots.",
		LongHelp: `Manage App Store screenshots.

Examples:
  asc screenshots list --version-localization "LOC_ID"
  asc screenshots sizes
  asc screenshots sizes --all
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/iphone" --device-type "IPHONE_65"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots download --version-localization "LOC_ID" --output-dir "./screenshots/downloaded"
  asc screenshots delete --id "SCREENSHOT_ID" --confirm

By default, "asc screenshots sizes" focuses on one iPhone set (IPHONE_65)
and one iPad set (IPAD_PRO_3GEN_129). Use --all to list every supported
display type.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AssetsScreenshotsListCommand(),
			AssetsScreenshotsSizesCommand(),
			AssetsScreenshotsUploadCommand(),
			AssetsScreenshotsDownloadCommand(),
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc screenshots list --version-localization \"LOC_ID\"",
		ShortHelp:  "List screenshots for a localization.",
		LongHelp: `List screenshots for a localization.

Examples:
  asc screenshots list --version-localization "LOC_ID"`,
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
				return fmt.Errorf("screenshots list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			setsResp, err := client.GetAppScreenshotSets(requestCtx, locID)
			if err != nil {
				return fmt.Errorf("screenshots list: failed to fetch sets: %w", err)
			}

			result := asc.AppScreenshotListResult{
				VersionLocalizationID: locID,
				Sets:                  make([]asc.AppScreenshotSetWithScreenshots, 0, len(setsResp.Data)),
			}

			for _, set := range setsResp.Data {
				screenshots, err := client.GetAppScreenshots(requestCtx, set.ID)
				if err != nil {
					return fmt.Errorf("screenshots list: failed to fetch screenshots for set %s: %w", set.ID, err)
				}
				result.Sets = append(result.Sets, asc.AppScreenshotSetWithScreenshots{
					Set:         set,
					Screenshots: screenshots.Data,
				})
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// AssetsScreenshotsSizesCommand returns the screenshots sizes subcommand.
func AssetsScreenshotsSizesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("sizes", flag.ExitOnError)

	displayType := fs.String("display-type", "", "Filter by screenshot display type (e.g., APP_IPHONE_65)")
	all := fs.Bool("all", false, "List all supported screenshot display types")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sizes",
		ShortUsage: "asc screenshots sizes [--display-type \"APP_IPHONE_65\" | --all]",
		ShortHelp:  "List supported screenshot display sizes.",
		LongHelp: `List supported screenshot display sizes.

By default this command focuses on common iOS submission slots:
APP_IPHONE_65 and APP_IPAD_PRO_3GEN_129.

Examples:
  asc screenshots sizes
  asc screenshots sizes --all
  asc screenshots sizes --display-type "APP_IPHONE_65"
  asc screenshots sizes --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			filter := strings.TrimSpace(*displayType)
			if filter != "" && *all {
				return shared.UsageError("--display-type and --all are mutually exclusive")
			}

			result := asc.ScreenshotSizesResult{}

			if filter != "" {
				normalized, err := normalizeScreenshotDisplayType(filter)
				if err != nil {
					return fmt.Errorf("screenshots sizes: %w", err)
				}
				entry, ok := asc.ScreenshotSizeEntryForDisplayType(normalized)
				if !ok {
					return fmt.Errorf("screenshots sizes: unsupported screenshot display type %q", normalized)
				}
				result.Sizes = []asc.ScreenshotSizeEntry{entry}
			} else if *all {
				result.Sizes = asc.ScreenshotSizeCatalog()
			} else {
				result.Sizes = focusedScreenshotSizeCatalog()
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// AssetsScreenshotsUploadCommand returns the screenshots upload subcommand.
func AssetsScreenshotsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	path := fs.String("path", "", "Path to screenshot file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65 or IPAD_PRO_3GEN_129)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc screenshots upload --version-localization \"LOC_ID\" --path \"./screenshots\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload screenshots for a localization.",
		LongHelp: `Upload screenshots for a localization.

Examples:
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_65"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/en-US.png" --device-type "IPHONE_65"`,
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
				return fmt.Errorf("screenshots upload: %w", err)
			}
			apiDisplayType := asc.CanonicalScreenshotDisplayTypeForAPI(displayType)

			files, err := collectAssetFiles(pathValue)
			if err != nil {
				return fmt.Errorf("screenshots upload: %w", err)
			}

			if err := validateScreenshotDimensions(files, apiDisplayType); err != nil {
				return fmt.Errorf("screenshots upload: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots upload: %w", err)
			}

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			set, err := ensureScreenshotSet(requestCtx, client, locID, apiDisplayType)
			if err != nil {
				return fmt.Errorf("screenshots upload: %w", err)
			}

			results := make([]asc.AssetUploadResultItem, 0, len(files))
			for _, filePath := range files {
				item, err := uploadScreenshotAsset(requestCtx, client, set.ID, filePath)
				if err != nil {
					return fmt.Errorf("screenshots upload: %w", err)
				}
				results = append(results, item)
			}

			result := asc.AppScreenshotUploadResult{
				VersionLocalizationID: locID,
				SetID:                 set.ID,
				DisplayType:           set.Attributes.ScreenshotDisplayType,
				Results:               results,
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

type screenshotDownloadItem struct {
	ID          string `json:"id"`
	DisplayType string `json:"displayType,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	URL         string `json:"url,omitempty"`
	OutputPath  string `json:"outputPath"`

	ContentType  string `json:"contentType,omitempty"`
	BytesWritten int64  `json:"bytesWritten,omitempty"`
}

type screenshotDownloadFailure struct {
	ID          string `json:"id,omitempty"`
	DisplayType string `json:"displayType,omitempty"`
	URL         string `json:"url,omitempty"`
	OutputPath  string `json:"outputPath,omitempty"`
	Error       string `json:"error"`
}

type screenshotDownloadResult struct {
	VersionLocalizationID string `json:"versionLocalizationId,omitempty"`
	OutputDir             string `json:"outputDir,omitempty"`
	Overwrite             bool   `json:"overwrite"`

	Total      int `json:"total"`
	Downloaded int `json:"downloaded"`
	Failed     int `json:"failed"`

	Items    []screenshotDownloadItem    `json:"items,omitempty"`
	Failures []screenshotDownloadFailure `json:"failures,omitempty"`
}

// AssetsScreenshotsDownloadCommand returns the screenshots download subcommand.
func AssetsScreenshotsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	id := fs.String("id", "", "Screenshot ID to download")
	localizationID := fs.String("version-localization", "", "App Store version localization ID (download all screenshots)")
	outputPath := fs.String("output", "", "Output file path (required with --id)")
	outputDir := fs.String("output-dir", "", "Output directory (required with --version-localization)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files")
	format := shared.BindOutputFlagsWith(fs, "format", "json", "Summary output format: json (default), table, markdown")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc screenshots download (--id \"SCREENSHOT_ID\" --output \"./screenshot.png\") | (--version-localization \"LOC_ID\" --output-dir \"./screenshots\")",
		ShortHelp:  "Download App Store screenshots to disk.",
		LongHelp: `Download App Store screenshots to disk.

Examples:
  asc screenshots download --id "SCREENSHOT_ID" --output "./screenshot.png"
  asc screenshots download --version-localization "LOC_ID" --output-dir "./screenshots"
  asc screenshots download --version-localization "LOC_ID" --output-dir "./screenshots" --overwrite`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			locID := strings.TrimSpace(*localizationID)

			if idValue == "" && locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-localization is required")
				return flag.ErrHelp
			}
			if idValue != "" && locID != "" {
				return shared.UsageError("--id and --version-localization are mutually exclusive")
			}

			outputFile := strings.TrimSpace(*outputPath)
			outputDirValue := strings.TrimSpace(*outputDir)
			if idValue != "" {
				if outputFile == "" {
					fmt.Fprintln(os.Stderr, "Error: --output is required with --id")
					return flag.ErrHelp
				}
				if strings.HasSuffix(outputFile, string(filepath.Separator)) {
					return shared.UsageError("--output must be a file path")
				}
			}
			if locID != "" {
				if outputDirValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --output-dir is required with --version-localization")
					return flag.ErrHelp
				}
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("screenshots download: %w", err)
			}

			cleanOutputDir := ""
			if outputDirValue != "" {
				cleanOutputDir = filepath.Clean(outputDirValue)
			}
			result := &screenshotDownloadResult{
				VersionLocalizationID: locID,
				OutputDir:             cleanOutputDir,
				Overwrite:             *overwrite,
			}

			items := make([]screenshotDownloadItem, 0, 8)

			if idValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				resp, err := client.GetAppScreenshot(requestCtx, idValue)
				cancel()
				if err != nil {
					return fmt.Errorf("screenshots download: failed to fetch screenshot: %w", err)
				}

				downloadURL, err := resolveImageAssetDownloadURL(resp.Data.Attributes.ImageAsset, resp.Data.Attributes.FileName)
				if err != nil {
					items = append(items, screenshotDownloadItem{
						ID:         idValue,
						FileName:   strings.TrimSpace(resp.Data.Attributes.FileName),
						OutputPath: outputFile,
					})
					result.Items = items
					result.Failures = append(result.Failures, screenshotDownloadFailure{
						ID:         idValue,
						OutputPath: outputFile,
						Error:      err.Error(),
					})
					result.Total = 1
					result.Failed = 1

					if err := shared.PrintOutputWithRenderers(
						result,
						*format.Output,
						*format.Pretty,
						func() error { return renderScreenshotDownloadResult(result, false) },
						func() error { return renderScreenshotDownloadResult(result, true) },
					); err != nil {
						return err
					}
					return shared.NewReportedError(fmt.Errorf("screenshots download: 1 file failed"))
				}

				items = append(items, screenshotDownloadItem{
					ID:         idValue,
					FileName:   strings.TrimSpace(resp.Data.Attributes.FileName),
					URL:        downloadURL,
					OutputPath: outputFile,
				})
			} else {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				setsResp, err := client.GetAppScreenshotSets(requestCtx, locID)
				cancel()
				if err != nil {
					return fmt.Errorf("screenshots download: failed to fetch sets: %w", err)
				}

				sets := make([]asc.Resource[asc.AppScreenshotSetAttributes], 0, len(setsResp.Data))
				sets = append(sets, setsResp.Data...)
				sort.Slice(sets, func(i, j int) bool {
					di := strings.ToUpper(strings.TrimSpace(sets[i].Attributes.ScreenshotDisplayType))
					dj := strings.ToUpper(strings.TrimSpace(sets[j].Attributes.ScreenshotDisplayType))
					if di == dj {
						return sets[i].ID < sets[j].ID
					}
					return di < dj
				})

				for _, set := range sets {
					displayType := strings.TrimSpace(set.Attributes.ScreenshotDisplayType)

					requestCtx, cancel := shared.ContextWithTimeout(ctx)
					shotsResp, err := client.GetAppScreenshots(requestCtx, set.ID)
					cancel()
					if err != nil {
						return fmt.Errorf("screenshots download: failed to fetch screenshots for set %s: %w", set.ID, err)
					}

					shots := make([]asc.Resource[asc.AppScreenshotAttributes], 0, len(shotsResp.Data))
					shots = append(shots, shotsResp.Data...)
					sort.Slice(shots, func(i, j int) bool {
						fi := strings.ToLower(strings.TrimSpace(shots[i].Attributes.FileName))
						fj := strings.ToLower(strings.TrimSpace(shots[j].Attributes.FileName))
						if fi == fj {
							return shots[i].ID < shots[j].ID
						}
						return fi < fj
					})

					for idx, shot := range shots {
						base := sanitizeBaseFileName(shot.Attributes.FileName)
						if base == "" {
							base = strings.TrimSpace(shot.ID)
						}
						if base == "" {
							base = fmt.Sprintf("screenshot-%d", idx+1)
						}

						destDir := filepath.Join(outputDirValue, displayType)
						destName := fmt.Sprintf("%02d_%s_%s", idx+1, strings.TrimSpace(shot.ID), base)
						destPath := filepath.Join(destDir, destName)

						imageAsset := shot.Attributes.ImageAsset
						if imageAsset == nil || strings.TrimSpace(imageAsset.TemplateURL) == "" {
							requestCtx, cancel := shared.ContextWithTimeout(ctx)
							full, err := client.GetAppScreenshot(requestCtx, shot.ID)
							cancel()
							if err == nil {
								imageAsset = full.Data.Attributes.ImageAsset
							}
						}

						downloadURL, err := resolveImageAssetDownloadURL(imageAsset, shot.Attributes.FileName)
						if err != nil {
							items = append(items, screenshotDownloadItem{
								ID:          strings.TrimSpace(shot.ID),
								DisplayType: displayType,
								FileName:    strings.TrimSpace(shot.Attributes.FileName),
								OutputPath:  destPath,
							})
							result.Failures = append(result.Failures, screenshotDownloadFailure{
								ID:          strings.TrimSpace(shot.ID),
								DisplayType: displayType,
								OutputPath:  destPath,
								Error:       err.Error(),
							})
							continue
						}

						items = append(items, screenshotDownloadItem{
							ID:          strings.TrimSpace(shot.ID),
							DisplayType: displayType,
							FileName:    strings.TrimSpace(shot.Attributes.FileName),
							URL:         downloadURL,
							OutputPath:  destPath,
						})
					}
				}
			}

			for i := range items {
				item := &items[i]
				if strings.TrimSpace(item.URL) == "" {
					continue
				}

				downloadCtx, cancel := shared.ContextWithTimeout(ctx)
				written, contentType, err := downloadURLToFile(downloadCtx, item.URL, item.OutputPath, *overwrite)
				cancel()
				if err != nil {
					result.Failures = append(result.Failures, screenshotDownloadFailure{
						ID:          item.ID,
						DisplayType: item.DisplayType,
						URL:         item.URL,
						OutputPath:  item.OutputPath,
						Error:       err.Error(),
					})
					continue
				}

				item.BytesWritten = written
				item.ContentType = contentType
				result.Downloaded++
			}

			result.Items = items
			result.Total = len(items)
			result.Failed = len(result.Failures)

			if err := shared.PrintOutputWithRenderers(
				result,
				*format.Output,
				*format.Pretty,
				func() error { return renderScreenshotDownloadResult(result, false) },
				func() error { return renderScreenshotDownloadResult(result, true) },
			); err != nil {
				return err
			}

			if result.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("screenshots download: %d file(s) failed", result.Failed))
			}
			return nil
		},
	}
}

func renderScreenshotDownloadResult(result *screenshotDownloadResult, markdown bool) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	render := asc.RenderTable
	if markdown {
		render = asc.RenderMarkdown
	}

	render(
		[]string{"Version Localization", "Output Dir", "Overwrite", "Total", "Downloaded", "Failed"},
		[][]string{{
			result.VersionLocalizationID,
			result.OutputDir,
			fmt.Sprintf("%t", result.Overwrite),
			fmt.Sprintf("%d", result.Total),
			fmt.Sprintf("%d", result.Downloaded),
			fmt.Sprintf("%d", result.Failed),
		}},
	)

	if len(result.Items) > 0 {
		rows := make([][]string, 0, len(result.Items))
		for _, item := range result.Items {
			rows = append(rows, []string{
				item.ID,
				item.DisplayType,
				item.FileName,
				item.OutputPath,
				fmt.Sprintf("%d", item.BytesWritten),
			})
		}
		render([]string{"ID", "Display Type", "File Name", "Output Path", "Bytes"}, rows)
	}

	if len(result.Failures) > 0 {
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{
				f.ID,
				f.DisplayType,
				f.OutputPath,
				f.Error,
			})
		}
		render([]string{"ID", "Display Type", "Output Path", "Error"}, rows)
	}

	return nil
}

// AssetsScreenshotsDeleteCommand returns the screenshot delete subcommand.
func AssetsScreenshotsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Screenshot ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc screenshots delete --id \"SCREENSHOT_ID\" --confirm",
		ShortHelp:  "Delete a screenshot by ID.",
		LongHelp: `Delete a screenshot by ID.

Examples:
  asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
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
				return fmt.Errorf("screenshots delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppScreenshot(requestCtx, assetID); err != nil {
				return fmt.Errorf("screenshots delete: %w", err)
			}

			result := asc.AssetDeleteResult{
				ID:      assetID,
				Deleted: true,
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

func normalizeScreenshotDisplayType(input string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("device type is required")
	}
	if !strings.HasPrefix(value, "APP_") && !strings.HasPrefix(value, "IMESSAGE_") {
		value = "APP_" + value
	}
	value = normalizeScreenshotDisplayTypeAlias(value)
	if !asc.IsValidScreenshotDisplayType(value) {
		return "", fmt.Errorf("unsupported screenshot display type %q", value)
	}
	return value, nil
}

// NormalizeScreenshotDisplayType normalizes and validates a screenshot display type.
func NormalizeScreenshotDisplayType(input string) (string, error) {
	return normalizeScreenshotDisplayType(input)
}

func normalizeScreenshotDisplayTypeAlias(value string) string {
	return value
}

func validateScreenshotDimensions(files []string, displayType string) error {
	for _, filePath := range files {
		if err := asc.ValidateScreenshotDimensions(filePath, displayType); err != nil {
			return err
		}
	}
	return nil
}

// ValidateScreenshotDimensions validates screenshot dimensions for all files.
func ValidateScreenshotDimensions(files []string, displayType string) error {
	return validateScreenshotDimensions(files, displayType)
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

// UploadScreenshotAsset uploads a screenshot file to a set.
func UploadScreenshotAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	return uploadScreenshotAsset(ctx, client, setID, filePath)
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
