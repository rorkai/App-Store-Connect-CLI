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

// AppClipAdvancedExperienceImagesCommand returns the images command group.
func AppClipAdvancedExperienceImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc app-clips advanced-experiences images <subcommand> [flags]",
		ShortHelp:  "Manage App Clip advanced experience images.",
		LongHelp: `Manage App Clip advanced experience images.

Examples:
  asc app-clips advanced-experiences images get --id "IMAGE_ID"
  asc app-clips advanced-experiences images create --experience-id "EXP_ID" --file path/to/image.png
  asc app-clips advanced-experiences images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipAdvancedExperienceImagesGetCommand(),
			AppClipAdvancedExperienceImagesCreateCommand(),
			AppClipAdvancedExperienceImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipAdvancedExperienceImagesGetCommand retrieves an image by ID.
func AppClipAdvancedExperienceImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	imageID := fs.String("id", "", "Image ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc app-clips advanced-experiences images get --id \"IMAGE_ID\"",
		ShortHelp:  "Get an advanced experience image by ID.",
		LongHelp: `Get an advanced experience image by ID.

Examples:
  asc app-clips advanced-experiences images get --id "IMAGE_ID"`,
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
				return fmt.Errorf("app-clips advanced-experiences images get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipAdvancedExperienceImage(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipAdvancedExperienceImagesCreateCommand uploads an image.
func AppClipAdvancedExperienceImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	experienceID := fs.String("experience-id", "", "Advanced experience ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips advanced-experiences images create --experience-id \"EXP_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for an advanced experience.",
		LongHelp: `Upload an image for an advanced experience.

The upload process reserves an upload slot, uploads the image, commits the upload,
and associates the image with the experience.

Examples:
  asc app-clips advanced-experiences images create --experience-id "EXP_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --experience-id is required")
				return flag.ErrHelp
			}

			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadAppClipAdvancedExperienceImage(requestCtx, fileValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images create: %w", err)
			}

			if _, err := client.UpdateAppClipAdvancedExperience(requestCtx, experienceValue, nil, "", result.ID, nil); err != nil {
				return fmt.Errorf("app-clips advanced-experiences images create: failed to attach image: %w", err)
			}

			result.ExperienceID = experienceValue
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// AppClipAdvancedExperienceImagesDeleteCommand deletes an image.
func AppClipAdvancedExperienceImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc app-clips advanced-experiences images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete an advanced experience image.",
		LongHelp: `Delete an advanced experience image.

Examples:
  asc app-clips advanced-experiences images delete --id "IMAGE_ID" --confirm`,
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
				return fmt.Errorf("app-clips advanced-experiences images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteAppClipAdvancedExperienceImage(requestCtx, idValue); err != nil {
				return fmt.Errorf("app-clips advanced-experiences images delete: failed to delete: %w", err)
			}

			result := &asc.AppClipAdvancedExperienceImageDeleteResult{
				ID:      idValue,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
