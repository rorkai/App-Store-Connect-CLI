package assetlibrary

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func normalizeAssetCategory(value string) (string, error) {
	category := strings.ToUpper(strings.TrimSpace(value))
	if category != "CREATIVE_ASSETS" && category != "APP_SCREENSHOTS_AND_PREVIEWS" {
		return "", fmt.Errorf("--category must be CREATIVE_ASSETS or APP_SCREENSHOTS_AND_PREVIEWS")
	}
	return category, nil
}

func videosUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("videos upload", flag.ExitOnError)
	library := shared.BindResourceIDFlag(fs, "library-id", "appAssetLibraries", "Asset Library ID")
	path := fs.String("file", "", "Video file to upload (.mp4, .m4v, or .mov)")
	category := fs.String("category", "CREATIVE_ASSETS", "Asset category: CREATIVE_ASSETS or APP_SCREENSHOTS_AND_PREVIEWS")
	output := shared.BindOutputFlags(fs)
	return &ffcli.Command{Name: "upload", ShortUsage: "asc asset-library videos upload --library-id ID --file video.mp4 [flags]", ShortHelp: "Upload a video to the Asset Library and wait for processing.", LongHelp: `Upload a video independently of an app version, placement or review submission.

Requires ffprobe (FFmpeg) to inspect the file. Dimensions, frame rate, duration,
audio and file size must match Apple's current asset reference specifications.
ASC_UPLOAD_TIMEOUT bounds upload and processing (default 30m for videos, since
Apple commonly takes 10 to 15 minutes to process one). Failures after reservation print
a partial receipt with the video ID; no automatic deletion occurs.

Examples:
  asc asset-library videos upload --library-id LIBRARY_ID --file ./header.mp4
  asc asset-library videos upload --library-id LIBRARY_ID --file ./duo-preview.mp4 --category APP_SCREENSHOTS_AND_PREVIEWS`, FlagSet: fs, UsageFunc: shared.DefaultUsageFunc, Exec: func(ctx context.Context, args []string) error {
		if len(args) > 0 {
			return shared.UsageErrorf("asset-library videos upload: unexpected argument %q", args[0])
		}
		if err := shared.ValidateBoundOutputFlags(fs); err != nil {
			return shared.UsageErrorf("asset-library videos upload: %v", err)
		}
		selected, err := normalizeAssetCategory(*category)
		if err != nil {
			return shared.UsageErrorf("asset-library videos upload: %v", err)
		}
		id := strings.TrimSpace(*library)
		if id == "" || !resourceIDPattern.MatchString(id) {
			return shared.UsageError("asset-library videos upload: --library-id must be a resource ID")
		}
		source := strings.TrimSpace(*path)
		if source == "" {
			return shared.UsageError("asset-library videos upload: --file is required")
		}
		switch strings.ToLower(filepath.Ext(source)) {
		case ".mp4", ".m4v", ".mov":
		default:
			return shared.UsageError("asset-library videos upload: --file must have a .mp4, .m4v, or .mov extension")
		}
		file, err := rootfs.OpenFile(source)
		if err != nil {
			return shared.UsageErrorf("asset-library videos upload: --file: %v", err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			return fmt.Errorf("asset-library videos upload: %w", err)
		}
		if err := asc.ValidateAssetFileInfo(source, info); err != nil {
			return shared.UsageErrorf("asset-library videos upload: %v", err)
		}
		if info.Size() > 524288000 {
			return shared.UsageError("asset-library videos upload: file exceeds 500 MiB")
		}
		// Inspect a private staged copy, then upload that same copy so a source-path
		// replacement cannot change the bytes validated by ffprobe.
		dir, err := os.MkdirTemp("", "asc-library-video-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		stagedRoot, err := rootfs.New(dir)
		if err != nil {
			return err
		}
		defer stagedRoot.Close()
		stagedName := "video" + strings.ToLower(filepath.Ext(source))
		copied, err := stagedRoot.WriteFrom(stagedName, io.LimitReader(file, info.Size()+1), 0o600)
		if err != nil {
			return err
		}
		if copied != info.Size() {
			return fmt.Errorf("asset-library videos upload: source changed while staging")
		}
		staged, err := stagedRoot.OpenFile(stagedName)
		if err != nil {
			return err
		}
		defer staged.Close()
		properties, err := inspectLibraryVideo(ctx, filepath.Join(dir, stagedName))
		if err != nil {
			return shared.UsageErrorf("asset-library videos upload: %v", err)
		}
		client, err := shared.GetASCClient()
		if err != nil {
			return fmt.Errorf("asset-library videos upload: %w", err)
		}
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		reference, err := client.RawRequest(requestCtx, "GET", "/v1/appAssetLibraryRefData", nil)
		cancel()
		if err != nil {
			return fmt.Errorf("asset-library videos upload: read specifications: %w", err)
		}
		if err := matchLibraryVideoSpec(reference, selected, properties, info.Size(), filepath.Ext(source)); err != nil {
			return shared.UsageErrorf("asset-library videos upload: %v", err)
		}
		if _, err := staged.Seek(0, io.SeekStart); err != nil {
			return err
		}
		uploadCtx, cancel := libraryVideoUploadContext(ctx)
		defer cancel()
		result, uploadErr := client.UploadAssetLibraryVideoWithCategory(uploadCtx, id, filepath.Base(source), staged, info.Size(), selected, shared.ContextWithTimeout)
		if uploadErr == nil || result.VideoID != "" {
			if err := shared.PrintOutput(&result, *output.Output, *output.Pretty); err != nil {
				return err
			}
		}
		if uploadErr != nil {
			if errors.Is(uploadCtx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("asset-library videos upload operation: %w", uploadErr)
			}
			return fmt.Errorf("asset-library videos upload: %w", uploadErr)
		}
		return nil
	}}
}

// libraryVideoUploadTimeout bounds a library video upload and Apple's
// processing of it. Processing commonly takes 10 to 15 minutes, so the general
// 5 minute upload default would expire before most videos finish.
const libraryVideoUploadTimeout = 30 * time.Minute

func libraryVideoUploadContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return shared.ContextWithResolvedUploadTimeout(ctx, libraryVideoUploadTimeout)
}
