package appclips

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

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
  asc app-clips advanced-experiences images view --id "IMAGE_ID"
  asc app-clips advanced-experiences images create --file path/to/image.png
  asc app-clips advanced-experiences images create --file path/to/image.png --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AppClipAdvancedExperienceImagesGetCommand(),
			AppClipAdvancedExperienceImagesCreateCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AppClipAdvancedExperienceImagesGetCommand retrieves an image by ID.
func AppClipAdvancedExperienceImagesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	imageID := fs.String("id", "", "Image ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc app-clips advanced-experiences images view --id \"IMAGE_ID\"",
		ShortHelp:  "View an advanced experience image by ID.",
		LongHelp: `View an advanced experience image by ID.

Examples:
  asc app-clips advanced-experiences images view --id "IMAGE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*imageID)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return shared.MissingRequiredUsageError("--id")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAppClipAdvancedExperienceImage(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AppClipAdvancedExperienceImagesCreateCommand uploads an image.
func AppClipAdvancedExperienceImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipAdvancedExperiences", "Advanced experience ID to attach after upload (optional)")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips advanced-experiences images create --file path/to/image.png [--experience-id \"EXP_ID\"]",
		ShortHelp:  "Upload an image for an advanced experience.",
		LongHelp: `Upload an image for an advanced experience.

The upload process reserves an upload slot, uploads the image, and commits the upload.
The returned image ID can be passed to advanced-experiences create with --header-image-id.
When --experience-id is provided, the command also attaches the uploaded image to
that existing experience.

Examples:
  asc app-clips advanced-experiences images create --file path/to/image.png
  asc app-clips advanced-experiences images create --file path/to/image.png --experience-id "EXP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return shared.MissingRequiredUsageError("--file")
			}

			client, err := appClipsClientFactory()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images create: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadAppClipAdvancedExperienceImage(requestCtx, fileValue)
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images create: %w", err)
			}

			experienceValue := strings.TrimSpace(*experienceID)
			if experienceValue != "" {
				if _, err := client.UpdateAppClipAdvancedExperience(requestCtx, experienceValue, nil, "", result.ID, nil); err != nil {
					return fmt.Errorf("app-clips advanced-experiences images create: failed to attach uploaded image %q: %w", result.ID, err)
				}
				result.ExperienceID = experienceValue
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
