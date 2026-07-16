package assets

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AssetsPreviewsListCommand returns the previews list subcommand.
func AssetsPreviewsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc video-previews list --version-localization \"VERSION_LOCALIZATION_ID\"",
		ShortHelp:  "List previews for a localization.",
		LongHelp: `List previews for a localization.

--version-localization is the App Store version localization resource ID
returned as data[].id by "asc localizations list --version VERSION_ID --output json".
It is not the locale code such as en-US.

Examples:
  asc localizations list --version "VERSION_ID" --output json --locale "en-US"
  asc video-previews list --version-localization "VERSION_LOCALIZATION_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-localization is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("video-previews list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			setsResp, err := client.GetAppPreviewSets(requestCtx, locID)
			if err != nil {
				return fmt.Errorf("video-previews list: failed to fetch sets: %w", err)
			}

			result := asc.AppPreviewListResult{
				VersionLocalizationID: locID,
				Sets:                  make([]asc.AppPreviewSetWithPreviews, 0, len(setsResp.Data)),
			}

			for _, set := range setsResp.Data {
				previews, err := client.GetAppPreviews(requestCtx, set.ID)
				if err != nil {
					return fmt.Errorf("video-previews list: failed to fetch previews for set %s: %w", set.ID, err)
				}
				result.Sets = append(result.Sets, asc.AppPreviewSetWithPreviews{
					Set:      set,
					Previews: previews.Data,
				})
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// AssetsPreviewsUploadCommand returns the previews upload subcommand.
func AssetsPreviewsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	skipExisting := fs.Bool("skip-existing", false, "Skip files whose MD5 checksum already exists in the target preview set")
	replace := fs.Bool("replace", false, "Delete all existing previews from the target set before uploading")
	dryRun := fs.Bool("dry-run", false, "Show what would be uploaded, skipped, or deleted without making changes")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc video-previews upload --version-localization \"VERSION_LOCALIZATION_ID\" --path \"./previews\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload previews for a localization.",
		LongHelp: `Upload previews for a localization.

--version-localization is the App Store version localization resource ID
returned as data[].id by "asc localizations list --version VERSION_ID --output json".
It is not the locale code such as en-US.

Examples:
  asc localizations list --version "VERSION_ID" --output json --locale "en-US"
  asc video-previews upload --version-localization "VERSION_LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65"
  asc video-previews upload --version-localization "VERSION_LOCALIZATION_ID" --path "./previews/preview.mov" --device-type "IPHONE_65"
  asc video-previews upload --version-localization "VERSION_LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65" --skip-existing
  asc video-previews upload --version-localization "VERSION_LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65" --replace
  asc video-previews upload --version-localization "VERSION_LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_65" --skip-existing --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				fmt.Fprintln(os.Stderr, "Error: --version-localization is required")
				return shared.MissingRequiredUsageError()
			}
			pathValue := strings.TrimSpace(*path)
			if pathValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --path is required")
				return shared.MissingRequiredUsageError()
			}
			deviceValue := strings.TrimSpace(*deviceType)
			if deviceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --device-type is required")
				return shared.MissingRequiredUsageError()
			}
			if *skipExisting && *replace {
				fmt.Fprintln(os.Stderr, "Error: --skip-existing and --replace are mutually exclusive")
				return flag.ErrHelp
			}

			previewType, err := normalizePreviewType(deviceValue)
			if err != nil {
				return fmt.Errorf("video-previews upload: %w", err)
			}

			files, err := collectAssetFiles(pathValue)
			if err != nil {
				return fmt.Errorf("video-previews upload: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("video-previews upload: %w", err)
			}

			result, err := uploadPreviews(ctx, client, locID, previewType, files, *skipExisting, *replace, *dryRun)
			if err != nil {
				return fmt.Errorf("video-previews upload: %w", err)
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

type previewDownloadItem struct {
	ID          string `json:"id"`
	PreviewType string `json:"previewType,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	URL         string `json:"url,omitempty"`
	OutputPath  string `json:"outputPath"`

	ContentType  string `json:"contentType,omitempty"`
	BytesWritten int64  `json:"bytesWritten,omitempty"`
}

type previewDownloadFailure struct {
	ID          string `json:"id,omitempty"`
	PreviewType string `json:"previewType,omitempty"`
	URL         string `json:"url,omitempty"`
	OutputPath  string `json:"outputPath,omitempty"`
	Error       string `json:"error"`

	cause error
}

type previewDownloadResult struct {
	VersionLocalizationID string `json:"versionLocalizationId,omitempty"`
	OutputDir             string `json:"outputDir,omitempty"`
	Overwrite             bool   `json:"overwrite"`

	Total      int `json:"total"`
	Downloaded int `json:"downloaded"`
	Failed     int `json:"failed"`

	Items    []previewDownloadItem    `json:"items,omitempty"`
	Failures []previewDownloadFailure `json:"failures,omitempty"`
}

// AssetsPreviewsDownloadCommand returns the previews download subcommand.
func AssetsPreviewsDownloadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("download", flag.ExitOnError)

	id := fs.String("id", "", "Preview ID to download")
	localizationID := fs.String("version-localization", "", "App Store version localization ID (download all previews)")
	outputPath := fs.String("output", "", "Output file path (required with --id)")
	outputDir := fs.String("output-dir", "", "Output directory (required with --version-localization)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing files")
	format := shared.BindOutputFlagsWith(fs, "format", "json", "Summary output format: json (default), table, markdown")

	return &ffcli.Command{
		Name:       "download",
		ShortUsage: "asc video-previews download (--id \"PREVIEW_ID\" --output \"./preview.mov\") | (--version-localization \"VERSION_LOCALIZATION_ID\" --output-dir \"./previews\")",
		ShortHelp:  "Download App Store app preview videos to disk.",
		LongHelp: `Download App Store app preview videos to disk.

--version-localization is the App Store version localization resource ID
returned as data[].id by "asc localizations list --version VERSION_ID --output json".
It is not the locale code such as en-US.

Examples:
  asc video-previews download --id "PREVIEW_ID" --output "./preview.mov"
  asc localizations list --version "VERSION_ID" --output json --locale "en-US"
  asc video-previews download --version-localization "VERSION_LOCALIZATION_ID" --output-dir "./previews"
  asc video-previews download --version-localization "VERSION_LOCALIZATION_ID" --output-dir "./previews" --overwrite`,
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
				return fmt.Errorf("video-previews download: %w", err)
			}

			cleanOutputDir := ""
			if outputDirValue != "" {
				cleanOutputDir = filepath.Clean(outputDirValue)
			}
			result := &previewDownloadResult{
				VersionLocalizationID: locID,
				OutputDir:             cleanOutputDir,
				Overwrite:             *overwrite,
			}

			items := make([]previewDownloadItem, 0, 8)
			var lookupErrs []error

			if idValue != "" {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				resp, err := client.GetAppPreview(requestCtx, idValue)
				cancel()
				if err != nil {
					return fmt.Errorf("video-previews download: failed to fetch preview: %w", err)
				}

				downloadURL := strings.TrimSpace(resp.Data.Attributes.VideoURL)
				if downloadURL == "" {
					items = append(items, previewDownloadItem{
						ID:         idValue,
						FileName:   strings.TrimSpace(resp.Data.Attributes.FileName),
						OutputPath: outputFile,
					})
					result.Items = items
					result.Failures = append(result.Failures, previewDownloadFailure{
						ID:         idValue,
						OutputPath: outputFile,
						Error:      "preview has no videoUrl",
					})
					result.Total = 1
					result.Failed = 1

					if err := shared.PrintOutputWithRenderers(
						result,
						*format.Output,
						*format.Pretty,
						func() error { return renderPreviewDownloadResult(result, false) },
						func() error { return renderPreviewDownloadResult(result, true) },
					); err != nil {
						return err
					}
					return shared.NewReportedError(fmt.Errorf("video-previews download: 1 file failed"))
				}

				items = append(items, previewDownloadItem{
					ID:         idValue,
					FileName:   strings.TrimSpace(resp.Data.Attributes.FileName),
					URL:        downloadURL,
					OutputPath: outputFile,
				})
			} else {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				setsResp, err := client.GetAppPreviewSets(requestCtx, locID)
				cancel()
				if err != nil {
					return fmt.Errorf("video-previews download: failed to fetch sets: %w", err)
				}

				sets := make([]asc.Resource[asc.AppPreviewSetAttributes], 0, len(setsResp.Data))
				sets = append(sets, setsResp.Data...)
				sort.Slice(sets, func(i, j int) bool {
					ti := strings.ToUpper(strings.TrimSpace(sets[i].Attributes.PreviewType))
					tj := strings.ToUpper(strings.TrimSpace(sets[j].Attributes.PreviewType))
					if ti == tj {
						return sets[i].ID < sets[j].ID
					}
					return ti < tj
				})

				for _, set := range sets {
					previewType := strings.TrimSpace(set.Attributes.PreviewType)

					requestCtx, cancel := shared.ContextWithTimeout(ctx)
					previewsResp, err := client.GetAppPreviews(requestCtx, set.ID)
					cancel()
					if err != nil {
						return fmt.Errorf("video-previews download: failed to fetch previews for set %s: %w", set.ID, err)
					}

					previews := make([]asc.Resource[asc.AppPreviewAttributes], 0, len(previewsResp.Data))
					previews = append(previews, previewsResp.Data...)
					sort.Slice(previews, func(i, j int) bool {
						fi := strings.ToLower(strings.TrimSpace(previews[i].Attributes.FileName))
						fj := strings.ToLower(strings.TrimSpace(previews[j].Attributes.FileName))
						if fi == fj {
							return previews[i].ID < previews[j].ID
						}
						return fi < fj
					})

					for idx, preview := range previews {
						base := sanitizeBaseFileName(preview.Attributes.FileName)
						if base == "" {
							base = strings.TrimSpace(preview.ID)
						}
						if base == "" {
							base = fmt.Sprintf("preview-%d.mov", idx+1)
						}

						destDir := filepath.Join(outputDirValue, previewType)
						destName := fmt.Sprintf("%02d_%s_%s", idx+1, strings.TrimSpace(preview.ID), base)
						destPath := filepath.Join(destDir, destName)

						items = append(items, previewDownloadItem{
							ID:          strings.TrimSpace(preview.ID),
							PreviewType: previewType,
							FileName:    strings.TrimSpace(preview.Attributes.FileName),
							URL:         strings.TrimSpace(preview.Attributes.VideoURL),
							OutputPath:  destPath,
						})
					}
				}

				lookupErrs = resolvePreviewDownloadURLs(ctx, client, items)
			}

			downloaded, downloadFailures := downloadPreviewItems(ctx, items, *overwrite)
			result.Downloaded = downloaded
			result.Failures = orderPreviewDownloadFailures(items, lookupErrs, downloadFailures)

			result.Items = items
			result.Total = len(items)
			result.Failed = len(result.Failures)

			if err := shared.PrintOutputWithRenderers(
				result,
				*format.Output,
				*format.Pretty,
				func() error { return renderPreviewDownloadResult(result, false) },
				func() error { return renderPreviewDownloadResult(result, true) },
			); err != nil {
				return err
			}

			if result.Failed > 0 {
				return shared.NewReportedError(previewDownloadReportedError(result.Failed, result.Failures))
			}
			return nil
		},
	}
}

func orderPreviewDownloadFailures(items []previewDownloadItem, lookupErrs []error, downloadFailures []previewDownloadFailure) []previewDownloadFailure {
	failuresByPath := make(map[string]previewDownloadFailure, len(downloadFailures))
	for _, failure := range downloadFailures {
		failuresByPath[failure.OutputPath] = failure
	}

	ordered := make([]previewDownloadFailure, 0, len(downloadFailures))
	for idx, item := range items {
		if idx < len(lookupErrs) && lookupErrs[idx] != nil {
			lookupErr := fmt.Errorf("failed to fetch preview: %w", lookupErrs[idx])
			ordered = append(ordered, previewDownloadFailure{
				ID:          item.ID,
				PreviewType: item.PreviewType,
				OutputPath:  item.OutputPath,
				Error:       lookupErr.Error(),
				cause:       lookupErr,
			})
			continue
		}
		if strings.TrimSpace(item.URL) == "" {
			ordered = append(ordered, previewDownloadFailure{
				ID:          item.ID,
				PreviewType: item.PreviewType,
				OutputPath:  item.OutputPath,
				Error:       "preview has no videoUrl",
			})
			continue
		}
		if failure, ok := failuresByPath[item.OutputPath]; ok {
			ordered = append(ordered, failure)
		}
	}
	return ordered
}

func previewDownloadReportedError(failed int, failures []previewDownloadFailure) error {
	message := fmt.Sprintf("video-previews download: %d file(s) failed", failed)
	causes := make([]error, 0, len(failures))
	for _, failure := range failures {
		if failure.cause != nil {
			causes = append(causes, failure.cause)
		}
	}
	if len(causes) == 0 {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %w", message, errors.Join(causes...))
}

func renderPreviewDownloadResult(result *previewDownloadResult, markdown bool) error {
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
				item.PreviewType,
				item.FileName,
				item.OutputPath,
				fmt.Sprintf("%d", item.BytesWritten),
			})
		}
		render([]string{"ID", "Preview Type", "File Name", "Output Path", "Bytes"}, rows)
	}

	if len(result.Failures) > 0 {
		rows := make([][]string, 0, len(result.Failures))
		for _, f := range result.Failures {
			rows = append(rows, []string{
				f.ID,
				f.PreviewType,
				f.OutputPath,
				f.Error,
			})
		}
		render([]string{"ID", "Preview Type", "Output Path", "Error"}, rows)
	}

	return nil
}

// resolvePreviewDownloadURLs fills in missing download URLs by fetching each
// preview's details concurrently. It returns one error slot per item so lookup
// failures remain distinct from successful responses with an empty videoUrl.
func resolvePreviewDownloadURLs(ctx context.Context, client *asc.Client, items []previewDownloadItem) []error {
	lookupErrs := make([]error, len(items))
	missing := make([]int, 0, len(items))
	for idx := range items {
		if strings.TrimSpace(items[idx].URL) == "" {
			missing = append(missing, idx)
		}
	}
	if len(missing) == 0 {
		return lookupErrs
	}

	taskErrs := forEachAssetTask(ctx, len(missing), false, func(taskCtx context.Context, i int) error {
		item := &items[missing[i]]
		requestCtx, cancel := shared.ContextWithTimeout(taskCtx)
		defer cancel()
		full, err := client.GetAppPreview(requestCtx, item.ID)
		if err != nil {
			return err
		}
		item.URL = strings.TrimSpace(full.Data.Attributes.VideoURL)
		return nil
	})
	for idx, itemIdx := range missing {
		lookupErrs[itemIdx] = taskErrs[idx]
	}
	return lookupErrs
}

// downloadPreviewItems downloads every item with a resolved URL concurrently.
// Items without a URL are skipped. Byte counts are written back into the
// index-keyed items, and the returned failures preserve item order.
func downloadPreviewItems(ctx context.Context, items []previewDownloadItem, overwrite bool) (int, []previewDownloadFailure) {
	type downloadOutcome struct {
		bytesWritten int64
		contentType  string
		err          error
	}

	outcomes := make([]downloadOutcome, len(items))
	taskErrs := forEachAssetTask(ctx, len(items), false, func(taskCtx context.Context, idx int) error {
		if strings.TrimSpace(items[idx].URL) == "" {
			return nil
		}
		downloadCtx, cancel := shared.ContextWithTimeout(taskCtx)
		defer cancel()
		written, contentType, err := downloadURLToFile(downloadCtx, items[idx].URL, items[idx].OutputPath, overwrite)
		outcomes[idx] = downloadOutcome{bytesWritten: written, contentType: contentType, err: err}
		return err
	})

	downloaded := 0
	failures := make([]previewDownloadFailure, 0)
	for idx := range items {
		if strings.TrimSpace(items[idx].URL) == "" {
			continue
		}
		outcomeErr := outcomes[idx].err
		if outcomeErr == nil {
			outcomeErr = taskErrs[idx]
		}
		if outcomeErr != nil {
			failures = append(failures, previewDownloadFailure{
				ID:          items[idx].ID,
				PreviewType: items[idx].PreviewType,
				URL:         items[idx].URL,
				OutputPath:  items[idx].OutputPath,
				Error:       outcomeErr.Error(),
				cause:       outcomeErr,
			})
			continue
		}
		items[idx].BytesWritten = outcomes[idx].bytesWritten
		items[idx].ContentType = outcomes[idx].contentType
		downloaded++
	}
	return downloaded, failures
}

// AssetsPreviewsDeleteCommand returns the preview delete subcommand.
func AssetsPreviewsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Preview ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc video-previews delete --id \"PREVIEW_ID\" --confirm",
		ShortHelp:  "Delete a preview by ID.",
		LongHelp: `Delete a preview by ID.

Examples:
  asc video-previews delete --id "PREVIEW_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			assetID := strings.TrimSpace(*id)
			if assetID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("video-previews delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppPreview(requestCtx, assetID); err != nil {
				return fmt.Errorf("video-previews delete: %w", err)
			}

			result := asc.AssetDeleteResult{
				ID:      assetID,
				Deleted: true,
			}

			return shared.PrintOutput(&result, *output.Output, *output.Pretty)
		},
	}
}

// AssetsPreviewsSetPosterFrameCommand returns the set-poster-frame subcommand.
func AssetsPreviewsSetPosterFrameCommand() *ffcli.Command {
	fs := flag.NewFlagSet("set-poster-frame", flag.ExitOnError)

	id := fs.String("id", "", "Preview ID")
	timeCode := fs.String("time-code", "", "Poster frame timecode (e.g., 00:00:05:00 or 00:00:05.000)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "set-poster-frame",
		ShortUsage: `asc video-previews set-poster-frame --id "PREVIEW_ID" --time-code "00:00:05:00"`,
		ShortHelp:  "Set the poster frame timecode for a preview.",
		LongHelp: `Set the poster frame timecode for a preview.

The poster frame is the still image shown before the video plays on the
App Store product page. Accepted timecode formats are HH:MM:SS:FF (frames)
and HH:MM:SS.mmm (milliseconds).

Examples:
  asc video-previews set-poster-frame --id "PREVIEW_ID" --time-code "00:00:05:00"
  asc video-previews set-poster-frame --id "PREVIEW_ID" --time-code "00:00:05.000"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("video-previews set-poster-frame does not accept positional arguments")
			}

			previewID := strings.TrimSpace(*id)
			if previewID == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError()
			}
			tc := strings.TrimSpace(*timeCode)
			if tc == "" {
				fmt.Fprintln(os.Stderr, "Error: --time-code is required")
				return shared.MissingRequiredUsageError()
			}
			if !isValidPreviewFrameTimeCode(tc) {
				fmt.Fprintln(os.Stderr, "Error: --time-code must be in HH:MM:SS:FF or HH:MM:SS.mmm format (e.g., 00:00:05:00 or 00:00:05.000)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("video-previews set-poster-frame: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			result, err := client.SetAppPreviewFrameTimeCode(requestCtx, previewID, tc)
			if err != nil {
				return fmt.Errorf("video-previews set-poster-frame: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

func normalizePreviewType(input string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "", fmt.Errorf("device type is required")
	}
	value = strings.TrimPrefix(value, "APP_")
	if !asc.IsValidPreviewType(value) {
		return "", fmt.Errorf("unsupported preview type %q", value)
	}
	return value, nil
}

// isValidPreviewFrameTimeCode checks supported poster frame timecode formats.
func isValidPreviewFrameTimeCode(tc string) bool {
	return isValidFrameTimeCode(tc) || isValidMillisecondTimeCode(tc)
}

func isValidFrameTimeCode(tc string) bool {
	parts := strings.Split(tc, ":")
	if len(parts) != 4 {
		return false
	}

	return parseFixedWidthInt(parts[0], 2, 99) &&
		parseFixedWidthInt(parts[1], 2, 59) &&
		parseFixedWidthInt(parts[2], 2, 59) &&
		parseFixedWidthInt(parts[3], 2, 29)
}

func isValidMillisecondTimeCode(tc string) bool {
	parts := strings.Split(tc, ":")
	if len(parts) != 3 {
		return false
	}

	secondParts := strings.Split(parts[2], ".")
	if len(secondParts) != 2 {
		return false
	}

	return parseFixedWidthInt(parts[0], 2, 99) &&
		parseFixedWidthInt(parts[1], 2, 59) &&
		parseFixedWidthInt(secondParts[0], 2, 59) &&
		parseFixedWidthInt(secondParts[1], 3, 999)
}

func parseFixedWidthInt(value string, width int, max int) bool {
	if len(value) != width {
		return false
	}

	result := 0
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
		result = result*10 + int(c-'0')
	}

	return result <= max
}

// NormalizePreviewType normalizes and validates a preview type.
func NormalizePreviewType(input string) (string, error) {
	return normalizePreviewType(input)
}

func findPreviewSet(ctx context.Context, client *asc.Client, localizationID, previewType string) (asc.Resource[asc.AppPreviewSetAttributes], error) {
	resp, err := client.GetAppPreviewSets(ctx, localizationID)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.PreviewType, previewType) {
			return set, nil
		}
	}
	return asc.Resource[asc.AppPreviewSetAttributes]{
		Attributes: asc.AppPreviewSetAttributes{PreviewType: previewType},
	}, nil
}

func ensurePreviewSet(ctx context.Context, client *asc.Client, localizationID, previewType string) (asc.Resource[asc.AppPreviewSetAttributes], error) {
	resp, err := client.GetAppPreviewSets(ctx, localizationID)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, err
	}
	for _, set := range resp.Data {
		if strings.EqualFold(set.Attributes.PreviewType, previewType) {
			return set, nil
		}
	}
	created, err := client.CreateAppPreviewSet(ctx, localizationID, previewType)
	if err != nil {
		return asc.Resource[asc.AppPreviewSetAttributes]{}, err
	}
	return created.Data, nil
}

func uploadPreviewAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	if err := asc.ValidateImageFile(filePath); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	mimeType, err := detectPreviewMimeType(filePath)
	if err != nil {
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
	item := asc.AssetUploadResultItem{
		FileName: info.Name(),
		FilePath: filePath,
	}

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return item, err
	}

	created, err := client.CreateAppPreview(ctx, setID, info.Name(), info.Size(), mimeType)
	if err != nil {
		return item, err
	}
	item.AssetID = created.Data.ID
	if len(created.Data.Attributes.UploadOperations) == 0 {
		return item, fmt.Errorf("no upload operations returned for %q", info.Name())
	}

	if err := asc.UploadAssetFromFile(ctx, file, info.Size(), created.Data.Attributes.UploadOperations); err != nil {
		return item, err
	}

	if _, err := client.UpdateAppPreview(ctx, created.Data.ID, true, checksum.Hash); err != nil {
		return item, err
	}

	state, err := waitForPreviewDelivery(ctx, client, created.Data.ID)
	if err != nil {
		return item, err
	}
	item.State = state
	return item, nil
}

// UploadPreviewAsset uploads a preview file to a set.
func UploadPreviewAsset(ctx context.Context, client *asc.Client, setID, filePath string) (asc.AssetUploadResultItem, error) {
	return uploadPreviewAsset(ctx, client, setID, filePath)
}

func detectPreviewMimeType(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return "", fmt.Errorf("preview file %q is missing an extension", path)
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "", fmt.Errorf("unsupported preview file extension %q", ext)
	}
	if idx := strings.Index(mimeType, ";"); idx > 0 {
		mimeType = mimeType[:idx]
	}
	return mimeType, nil
}

func uploadPreviews(ctx context.Context, client *asc.Client, localizationID, previewType string, files []string, skipExisting, replace, dryRun bool) (asc.AppPreviewUploadResult, error) {
	if client == nil {
		return asc.AppPreviewUploadResult{}, fmt.Errorf("client is required")
	}

	requestCtx, reqCancel := shared.ContextWithTimeout(ctx)
	var set asc.Resource[asc.AppPreviewSetAttributes]
	var err error
	if dryRun {
		set, err = findPreviewSet(requestCtx, client, localizationID, previewType)
	} else {
		set, err = ensurePreviewSet(requestCtx, client, localizationID, previewType)
	}
	reqCancel()
	if err != nil {
		return asc.AppPreviewUploadResult{}, err
	}

	existingPreviews := make([]asc.Resource[asc.AppPreviewAttributes], 0)
	if (skipExisting || replace) && set.ID != "" {
		fetchCtx, fetchCancel := shared.ContextWithTimeout(ctx)
		existingResp, err := client.GetAppPreviews(fetchCtx, set.ID)
		fetchCancel()
		if err != nil {
			return asc.AppPreviewUploadResult{}, err
		}
		existingPreviews = existingResp.Data
	}

	skippedResults := make([]asc.AssetUploadResultItem, 0)
	if skipExisting {
		files, skippedResults, err = filterExistingPreviewFiles(files, existingPreviews)
		if err != nil {
			return asc.AppPreviewUploadResult{}, err
		}
	}

	if dryRun {
		results := make([]asc.AssetUploadResultItem, 0, len(skippedResults)+len(files)+len(existingPreviews))
		if replace {
			for _, p := range existingPreviews {
				results = append(results, asc.AssetUploadResultItem{
					FileName: p.Attributes.FileName,
					AssetID:  p.ID,
					State:    "would-delete",
				})
			}
		}
		for _, filePath := range files {
			results = append(results, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				State:    "would-upload",
			})
		}
		results = append(results, skippedResults...)

		return asc.AppPreviewUploadResult{
			VersionLocalizationID: localizationID,
			SetID:                 set.ID,
			PreviewType:           set.Attributes.PreviewType,
			DryRun:                true,
			Results:               results,
		}, nil
	}

	uploadCtx, cancel := contextWithAssetUploadTimeout(ctx)
	defer cancel()

	if replace {
		if err := deleteExistingPreviews(uploadCtx, client, existingPreviews); err != nil {
			return asc.AppPreviewUploadResult{}, err
		}
	}

	results := make([]asc.AssetUploadResultItem, 0, len(skippedResults)+len(files))
	if len(files) > 0 {
		items := make([]asc.AssetUploadResultItem, len(files))
		taskErrs := forEachAssetTask(uploadCtx, len(files), true, func(taskCtx context.Context, idx int) error {
			item, err := uploadPreviewAsset(taskCtx, client, set.ID, files[idx])
			items[idx] = item
			if err != nil {
				return err
			}
			return nil
		})
		if uploadErr := aggregateAssetTaskErrors(taskErrs); uploadErr != nil {
			return asc.AppPreviewUploadResult{}, rollbackCreatedPreviewsAfterError(uploadCtx, client, items, uploadErr)
		}
		if len(items) > 1 {
			if err := reorderUploadedPreviews(uploadCtx, client, set.ID, items); err != nil {
				return asc.AppPreviewUploadResult{}, rollbackCreatedPreviewsAfterError(uploadCtx, client, items, err)
			}
		}
		results = append(results, items...)
	}
	results = append(skippedResults, results...)

	return asc.AppPreviewUploadResult{
		VersionLocalizationID: localizationID,
		SetID:                 set.ID,
		PreviewType:           set.Attributes.PreviewType,
		Results:               results,
	}, nil
}

func rollbackCreatedPreviewsAfterError(ctx context.Context, client *asc.Client, items []asc.AssetUploadResultItem, operationErr error) error {
	if rollbackErr := deleteCreatedPreviews(ctx, client, items); rollbackErr != nil {
		return fmt.Errorf("preview batch failed and rollback was incomplete: %w", errors.Join(operationErr, rollbackErr))
	}
	return operationErr
}

func deleteCreatedPreviews(ctx context.Context, client *asc.Client, items []asc.AssetUploadResultItem) error {
	var rollbackErrs []error
	for _, item := range items {
		if strings.TrimSpace(item.AssetID) == "" {
			continue
		}
		// Rollback attempts every created preview so one failed cleanup does not
		// leave the rest of a parallel batch attached in completion order.
		if err := client.DeleteAppPreview(ctx, item.AssetID); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("delete preview %q: %w", item.AssetID, err))
		}
	}
	return errors.Join(rollbackErrs...)
}

func deleteExistingPreviews(ctx context.Context, client *asc.Client, previews []asc.Resource[asc.AppPreviewAttributes]) error {
	// Keep destructive replacement serial so one failure stops before later
	// previews are removed.
	for _, preview := range previews {
		if err := client.DeleteAppPreview(ctx, preview.ID); err != nil {
			return err
		}
	}
	return nil
}

func reorderUploadedPreviews(ctx context.Context, client *asc.Client, setID string, uploaded []asc.AssetUploadResultItem) error {
	// ASC can attach concurrent completions in network order. Preserve every
	// pre-existing relationship and force only this batch back to file order.
	firstPage, err := client.GetAppPreviewSetAppPreviewsRelationships(ctx, setID, asc.WithLinkagesLimit(200))
	if err != nil {
		return err
	}

	currentIDs := make([]string, 0, len(firstPage.Data))
	err = asc.PaginateEach(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
		return client.GetAppPreviewSetAppPreviewsRelationships(ctx, "", asc.WithLinkagesNextURL(nextURL))
	}, func(page asc.PaginatedResponse) error {
		linkages, ok := page.(*asc.LinkagesResponse)
		if !ok {
			return fmt.Errorf("unexpected preview relationship response type %T", page)
		}
		for _, item := range linkages.Data {
			currentIDs = append(currentIDs, item.ID)
		}
		return nil
	})
	if err != nil {
		return err
	}

	uploadedIDs := make(map[string]struct{}, len(uploaded))
	for _, item := range uploaded {
		uploadedIDs[item.AssetID] = struct{}{}
	}
	orderedIDs := make([]string, 0, len(currentIDs)+len(uploaded))
	for _, id := range currentIDs {
		if _, justUploaded := uploadedIDs[id]; !justUploaded {
			orderedIDs = append(orderedIDs, id)
		}
	}
	for _, item := range uploaded {
		orderedIDs = append(orderedIDs, item.AssetID)
	}
	return client.UpdateAppPreviewSetAppPreviewsRelationship(ctx, setID, orderedIDs)
}

func filterExistingPreviewFiles(files []string, previews []asc.Resource[asc.AppPreviewAttributes]) ([]string, []asc.AssetUploadResultItem, error) {
	existingChecksums := make(map[string]struct{}, len(previews))
	for _, preview := range previews {
		checksum := strings.TrimSpace(preview.Attributes.SourceFileChecksum)
		if checksum == "" {
			continue
		}
		existingChecksums[checksum] = struct{}{}
	}

	filtered := make([]string, 0, len(files))
	skipped := make([]asc.AssetUploadResultItem, 0)
	for _, filePath := range files {
		checksum, err := computeFileChecksum(filePath)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := existingChecksums[checksum]; exists {
			skipped = append(skipped, asc.AssetUploadResultItem{
				FileName: filepath.Base(filePath),
				FilePath: filePath,
				State:    "skipped",
				Skipped:  true,
			})
			continue
		}
		filtered = append(filtered, filePath)
	}

	return filtered, skipped, nil
}

func waitForPreviewDelivery(ctx context.Context, client *asc.Client, previewID string) (string, error) {
	return waitForAssetDeliveryState(ctx, previewID, func(ctx context.Context) (*asc.AssetDeliveryState, error) {
		resp, err := client.GetAppPreview(ctx, previewID)
		if err != nil {
			return nil, err
		}
		return resp.Data.Attributes.AssetDeliveryState, nil
	})
}
