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
		ShortUsage: "asc screenshots download (--id \"SCREENSHOT_ID\" --output \"./screenshot.png\") | (--version-localization \"VERSION_LOCALIZATION_ID\" --output-dir \"./screenshots\")",
		ShortHelp:  "Download App Store screenshots to disk.",
		LongHelp: `Download App Store screenshots to disk.

--version-localization is the App Store version localization resource ID
returned as data[].id by "asc localizations list --version VERSION_ID --output json".
It is not the locale code such as en-US.

Examples:
  asc screenshots download --id "SCREENSHOT_ID" --output "./screenshot.png"
  asc localizations list --version "VERSION_ID" --output json --locale "en-US"
  asc screenshots download --version-localization "VERSION_LOCALIZATION_ID" --output-dir "./screenshots"
  asc screenshots download --version-localization "VERSION_LOCALIZATION_ID" --output-dir "./screenshots" --overwrite`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			locID := strings.TrimSpace(*localizationID)

			if idValue == "" && locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id or --version-localization is required")
				return shared.MissingRequiredUsageError()
			}
			if idValue != "" && locID != "" {
				return shared.UsageError("--id and --version-localization are mutually exclusive")
			}

			outputFile := strings.TrimSpace(*outputPath)
			outputDirValue := strings.TrimSpace(*outputDir)
			if idValue != "" {
				if outputFile == "" {
					fmt.Fprintln(os.Stderr, "Error: --output is required with --id")
					return shared.MissingRequiredUsageError()
				}
				if strings.HasSuffix(outputFile, string(filepath.Separator)) {
					return shared.UsageError("--output must be a file path")
				}
			}
			if locID != "" {
				if outputDirValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --output-dir is required with --version-localization")
					return shared.MissingRequiredUsageError()
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

					requestCtx, cancel = shared.ContextWithTimeout(ctx)
					orderedIDs, err := GetOrderedAppScreenshotIDs(requestCtx, client, set.ID)
					cancel()
					if err != nil {
						return fmt.Errorf("screenshots download: failed to fetch screenshot order for set %s: %w", set.ID, err)
					}
					shots := orderScreenshotsForDownload(shotsResp.Data, orderedIDs)

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

						requestCtx, cancel := shared.ContextWithTimeout(ctx)
						downloadURL, err := resolveScreenshotDownloadURL(requestCtx, client, shot)
						cancel()
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

func resolveScreenshotDownloadURL(ctx context.Context, client *asc.Client, shot asc.Resource[asc.AppScreenshotAttributes]) (string, error) {
	imageAsset := shot.Attributes.ImageAsset
	if imageAsset == nil || strings.TrimSpace(imageAsset.TemplateURL) == "" {
		full, err := client.GetAppScreenshot(ctx, shot.ID)
		if err != nil {
			return "", fmt.Errorf("fetch screenshot metadata: %w", err)
		}
		imageAsset = full.Data.Attributes.ImageAsset
	}
	return resolveImageAssetDownloadURL(imageAsset, shot.Attributes.FileName)
}

func orderScreenshotsForDownload(shots []asc.Resource[asc.AppScreenshotAttributes], orderedIDs []string) []asc.Resource[asc.AppScreenshotAttributes] {
	ordered := append([]asc.Resource[asc.AppScreenshotAttributes](nil), shots...)
	orderByID := make(map[string]int, len(orderedIDs))
	for idx, id := range orderedIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, exists := orderByID[id]; exists {
			continue
		}
		orderByID[id] = idx
	}

	sort.SliceStable(ordered, func(i, j int) bool {
		iOrder, iOK := orderByID[strings.TrimSpace(ordered[i].ID)]
		jOrder, jOK := orderByID[strings.TrimSpace(ordered[j].ID)]
		switch {
		case iOK && jOK:
			return iOrder < jOrder
		case iOK:
			return true
		case jOK:
			return false
		}

		fi := strings.ToLower(strings.TrimSpace(ordered[i].Attributes.FileName))
		fj := strings.ToLower(strings.TrimSpace(ordered[j].Attributes.FileName))
		if fi == fj {
			return ordered[i].ID < ordered[j].ID
		}
		return fi < fj
	})

	return ordered
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
