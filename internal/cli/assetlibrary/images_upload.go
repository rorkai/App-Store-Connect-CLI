package assetlibrary

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func imagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images upload", flag.ExitOnError)
	libraryID := shared.BindResourceIDFlag(fs, "library-id", "appAssetLibraries", "Asset Library ID")
	path := fs.String("file", "", "Image file to upload")
	category := fs.String("category", "CREATIVE_ASSETS", "Asset category: CREATIVE_ASSETS or APP_SCREENSHOTS_AND_PREVIEWS")
	referenceName := bindUploadReferenceNameFlag(fs)
	progress := fs.Bool("progress", false, "Emit structured upload progress as JSONL on stderr")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "upload", ShortUsage: "asc asset-library images upload --library-id ID --file image.png [flags]",
		ShortHelp: "Upload an image to the Asset Library and wait for processing.",
		LongHelp: `Upload an image to the Asset Library without assigning it to a placement or submitting review.

The command waits for processed image data under ASC_UPLOAD_TIMEOUT. On failure
after reservation, it prints a partial receipt with the image ID for inspection
or cleanup. It never automatically deletes the reserved image.

Examples:
  asc asset-library images upload --library-id LIBRARY_ID --file ./header.png
  asc asset-library images upload --library-id LIBRARY_ID --file ./cms-8f3a21.png --reference-name "Hero en-US 3.14"`,
		FlagSet: fs, UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				return shared.UsageErrorf("asset-library images upload: unexpected argument %q", args[0])
			}
			if err := shared.ValidateBoundOutputFlags(fs); err != nil {
				return shared.UsageErrorf("asset-library images upload: %v", err)
			}
			selectedCategory, categoryErr := normalizeAssetCategory(*category)
			if categoryErr != nil {
				return shared.UsageErrorf("asset-library images upload: %v", categoryErr)
			}
			selectedReferenceName, referenceNameErr := uploadReferenceName(fs, *referenceName)
			if referenceNameErr != nil {
				return shared.UsageErrorf("asset-library images upload: %v", referenceNameErr)
			}
			id := strings.TrimSpace(*libraryID)
			if id == "" {
				return shared.UsageError("asset-library images upload: --library-id is required")
			}
			if !resourceIDPattern.MatchString(id) {
				return shared.UsageError("asset-library images upload: --library-id must be a resource ID")
			}
			filePath := strings.TrimSpace(*path)
			if filePath == "" {
				return shared.UsageError("asset-library images upload: --file is required")
			}
			file, size, err := openLibraryImage("asset-library images upload", "--file", filePath)
			if err != nil {
				return err
			}
			defer file.Close()
			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("asset-library images upload: %w", err)
			}
			uploadCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()
			if *progress {
				uploadCtx = asc.WithAssetLibraryImageUploadProgress(uploadCtx, func(event asc.AssetLibraryImageUploadEvent) { _ = json.NewEncoder(os.Stderr).Encode(event) })
			}
			result, uploadErr := client.UploadAssetLibraryImageWithCategory(uploadCtx, id, filepath.Base(filePath), file, size, selectedCategory, selectedReferenceName, shared.ContextWithTimeout)
			if uploadErr == nil || result.ImageID != "" {
				if err := shared.PrintOutput(&result, *output.Output, *output.Pretty); err != nil {
					return err
				}
			}
			if uploadErr != nil {
				return fmt.Errorf("asset-library images upload: %w", uploadErr)
			}
			return nil
		},
	}
}

// openLibraryImage validates an image and returns it rewound for upload.
func openLibraryImage(command, flagName, filePath string) (*os.File, int64, error) {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".png", ".jpg", ".jpeg":
	default:
		return nil, 0, shared.UsageErrorf("%s: %s must have a .png, .jpg, or .jpeg extension", command, flagName)
	}
	file, err := rootfs.OpenFile(filePath)
	if err != nil {
		return nil, 0, shared.UsageErrorf("%s: %s: %v", command, flagName, err)
	}
	size, err := validateLibraryImage(command, flagName, filePath, file)
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, size, nil
}

func validateLibraryImage(command, flagName, filePath string, file *os.File) (int64, error) {
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("%s: stat file: %w", command, err)
	}
	if err := asc.ValidateAssetFileInfo(filePath, info); err != nil {
		return 0, shared.UsageErrorf("%s: %s: %v", command, flagName, err)
	}
	if info.Size() > 524288000 {
		return 0, shared.UsageErrorf("%s: file exceeds 500 MiB", command)
	}
	format, err := asc.ReadAppStoreImageFormatFrom(file)
	if err != nil {
		return 0, shared.UsageErrorf("%s: %s: %v", command, flagName, err)
	}
	if format != "png" && format != "jpeg" {
		return 0, shared.UsageErrorf("%s: %s must be a PNG or JPEG image", command, flagName)
	}
	if err := asc.ValidateImageFormatMatchesExtension(filePath, format); err != nil {
		return 0, shared.UsageErrorf("%s: %s: %v", command, flagName, err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("%s: rewind file: %w", command, err)
	}
	return info.Size(), nil
}
