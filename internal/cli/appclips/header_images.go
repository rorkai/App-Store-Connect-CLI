package appclips

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

// AppClipHeaderImagesCommand returns the header images command group.
func AppClipHeaderImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("header-images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "header-images",
		ShortUsage: "asc app-clips header-images <subcommand> [flags]",
		ShortHelp:  "Manage App Clip header images.",
		LongHelp: `Manage App Clip header images.

Examples:
  asc app-clips header-images get --id "IMAGE_ID"
  asc app-clips header-images create --localization-id "LOC_ID" --file path/to/image.png
  asc app-clips header-images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipHeaderImagesGetCommand(),
			AppClipHeaderImagesCreateCommand(),
			AppClipHeaderImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipHeaderImagesGetCommand retrieves a header image by ID.
func AppClipHeaderImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Header image ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips header-images get --id \"IMAGE_ID\"",
		ShortHelp:  "Get a header image by ID.",
		LongHelp: `Get a header image by ID.

Examples:
  asc app-clips header-images get --id "IMAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*imageID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips header-images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipHeaderImage(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("app-clips header-images get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipHeaderImagesCreateCommand uploads a header image.
func AppClipHeaderImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	localizationID := fs.String("localization-id", "", "Default experience localization ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips header-images create --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload a header image for a localization.",
		LongHelp: `Upload a header image for a localization.

The upload process reserves an upload slot, uploads the image, and commits the upload.

Examples:
  asc app-clips header-images create --localization-id "LOC_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips header-images create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadAppClipHeaderImage(requestCtx, locValue, fileValue)
			if err != nil {
				return fmt.Errorf("app-clips header-images create: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AppClipHeaderImagesDeleteCommand deletes a header image.
func AppClipHeaderImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Header image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips header-images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a header image.",
		LongHelp: `Delete a header image.

Examples:
  asc app-clips header-images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*imageID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required to delete")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips header-images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppClipHeaderImage(requestCtx, idValue); err != nil {
				return fmt.Errorf("app-clips header-images delete: failed to delete: %w", err)
			}

			result := &asc.AppClipHeaderImageDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
