package assets

import (
	"context"
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

// AssetsPreviewsCommand returns the previews subcommand group.
func AssetsPreviewsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("previews", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "previews",
		ShortUsage: "asc video-previews <subcommand> [flags]",
		ShortHelp:  "Manage App Store app preview videos.",
		LongHelp: `Manage App Store app preview videos.

Examples:
  asc video-previews list --version-localization "LOC_ID"
  asc video-previews upload --version-localization "LOC_ID" --path "./previews" --device-type "IPHONE_65"
  asc video-previews download --version-localization "LOC_ID" --output-dir "./previews/downloaded"
  asc video-previews delete --id "PREVIEW_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AssetsPreviewsListCommand(),
			AssetsPreviewsUploadCommand(),
			AssetsPreviewsDownloadCommand(),
			AssetsPreviewsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AssetsPreviewsListCommand returns the previews list subcommand.
func AssetsPreviewsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc video-previews list --version-localization \"LOC_ID\"",
		ShortHelp:  "List previews for a localization.",
		LongHelp: `List previews for a localization.

Examples:
  asc video-previews list --version-localization "LOC_ID"`,
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc video-previews upload --version-localization \"LOC_ID\" --path \"./previews\" --device-type \"IPHONE_65\"",
		ShortHelp:  "Upload previews for a localization.",
		LongHelp: `Upload previews for a localization.

Examples:
  asc video-previews upload --version-localization "LOC_ID" --path "./previews" --device-type "IPHONE_65"
  asc video-previews upload --version-localization "LOC_ID" --path "./previews/preview.mov" --device-type "IPHONE_65"`,
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

			requestCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()

			set, err := ensurePreviewSet(requestCtx, client, locID, previewType)
			if err != nil {
				return fmt.Errorf("video-previews upload: %w", err)
			}

			results := make([]asc.AssetUploadResultItem, 0, len(files))
			for _, filePath := range files {
				item, err := uploadPreviewAsset(requestCtx, client, set.ID, filePath)
				if err != nil {
					return fmt.Errorf("video-previews upload: %w", err)
				}
				results = append(results, item)
			}

			result := asc.AppPreviewUploadResult{
				VersionLocalizationID: locID,
				SetID:                 set.ID,
				PreviewType:           set.Attributes.PreviewType,
				Results:               results,
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
		ShortUsage: "asc video-previews download (--id \"PREVIEW_ID\" --output \"./preview.mov\") | (--version-localization \"LOC_ID\" --output-dir \"./previews\")",
		ShortHelp:  "Download App Store app preview videos to disk.",
		LongHelp: `Download App Store app preview videos to disk.

Examples:
  asc video-previews download --id "PREVIEW_ID" --output "./preview.mov"
  asc video-previews download --version-localization "LOC_ID" --output-dir "./previews"
  asc video-previews download --version-localization "LOC_ID" --output-dir "./previews" --overwrite`,
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

						videoURL := strings.TrimSpace(preview.Attributes.VideoURL)
						if videoURL == "" {
							requestCtx, cancel := shared.ContextWithTimeout(ctx)
							full, err := client.GetAppPreview(requestCtx, preview.ID)
							cancel()
							if err == nil {
								videoURL = strings.TrimSpace(full.Data.Attributes.VideoURL)
							}
						}

						if videoURL == "" {
							items = append(items, previewDownloadItem{
								ID:          strings.TrimSpace(preview.ID),
								PreviewType: previewType,
								FileName:    strings.TrimSpace(preview.Attributes.FileName),
								OutputPath:  destPath,
							})
							result.Failures = append(result.Failures, previewDownloadFailure{
								ID:          strings.TrimSpace(preview.ID),
								PreviewType: previewType,
								OutputPath:  destPath,
								Error:       "preview has no videoUrl",
							})
							continue
						}

						items = append(items, previewDownloadItem{
							ID:          strings.TrimSpace(preview.ID),
							PreviewType: previewType,
							FileName:    strings.TrimSpace(preview.Attributes.FileName),
							URL:         videoURL,
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
					result.Failures = append(result.Failures, previewDownloadFailure{
						ID:          item.ID,
						PreviewType: item.PreviewType,
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
				func() error { return renderPreviewDownloadResult(result, false) },
				func() error { return renderPreviewDownloadResult(result, true) },
			); err != nil {
				return err
			}

			if result.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("video-previews download: %d file(s) failed", result.Failed))
			}
			return nil
		},
	}
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
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
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

// NormalizePreviewType normalizes and validates a preview type.
func NormalizePreviewType(input string) (string, error) {
	return normalizePreviewType(input)
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
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	checksum, err := asc.ComputeChecksumFromReader(file, asc.ChecksumAlgorithmMD5)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	created, err := client.CreateAppPreview(ctx, setID, info.Name(), info.Size(), mimeType)
	if err != nil {
		return asc.AssetUploadResultItem{}, err
	}
	if len(created.Data.Attributes.UploadOperations) == 0 {
		return asc.AssetUploadResultItem{}, fmt.Errorf("no upload operations returned for %q", info.Name())
	}

	if err := asc.UploadAssetFromFile(ctx, file, info.Size(), created.Data.Attributes.UploadOperations); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	if _, err := client.UpdateAppPreview(ctx, created.Data.ID, true, checksum.Hash); err != nil {
		return asc.AssetUploadResultItem{}, err
	}

	state, err := waitForPreviewDelivery(ctx, client, created.Data.ID)
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

func waitForPreviewDelivery(ctx context.Context, client *asc.Client, previewID string) (string, error) {
	return waitForAssetDeliveryState(ctx, previewID, func(ctx context.Context) (*asc.AssetDeliveryState, error) {
		resp, err := client.GetAppPreview(ctx, previewID)
		if err != nil {
			return nil, err
		}
		return resp.Data.Attributes.AssetDeliveryState, nil
	})
}
