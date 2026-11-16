package appclips

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAppClipAdvancedExperienceImagesCommand returns the images command group.
func newAppClipAdvancedExperienceImagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("images", flag.ExitOnError)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipAdvancedExperienceImagesGetCommand(d),
			newAppClipAdvancedExperienceImagesCreateCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipAdvancedExperienceImagesGetCommand retrieves an image by ID.
func newAppClipAdvancedExperienceImagesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips advanced-experiences images view --id \"IMAGE_ID\"",
		ShortHelp:  "View an advanced experience image by ID.",
		LongHelp: `View an advanced experience image by ID.

Examples:
  asc app-clips advanced-experiences images view --id "IMAGE_ID"`,
		IDUsage:            "Image ID",
		IDType:             "appClipAdvancedExperienceImages",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipAdvancedExperienceImage(ctx, id)
		},
	})
}

// newAppClipAdvancedExperienceImagesCreateCommand uploads an image.
func newAppClipAdvancedExperienceImagesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	experienceID := shared.BindResourceIDFlag(fs, "experience-id", "appClipAdvancedExperiences", "Advanced experience ID to attach after upload (optional)")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				return d.RequiredFlag("--file")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips advanced-experiences images create: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
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

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
