package assets

import (
	"context"
	"flag"
	"fmt"
	"mime"
	"os"
	"path/filepath"
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
  asc video-previews delete --id "PREVIEW_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AssetsPreviewsListCommand(),
			AssetsPreviewsUploadCommand(),
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
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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

			return shared.PrintOutput(&result, *output, *pretty)
		},
	}
}

// AssetsPreviewsUploadCommand returns the previews upload subcommand.
func AssetsPreviewsUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := fs.String("version-localization", "", "App Store version localization ID")
	path := fs.String("path", "", "Path to preview file or directory")
	deviceType := fs.String("device-type", "", "Device type (e.g., IPHONE_65)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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

			return shared.PrintOutput(&result, *output, *pretty)
		},
	}
}

// AssetsPreviewsDeleteCommand returns the preview delete subcommand.
func AssetsPreviewsDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	id := fs.String("id", "", "Preview ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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

			return shared.PrintOutput(&result, *output, *pretty)
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
