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

// newAppClipHeaderImagesCommand returns the header images command group.
func newAppClipHeaderImagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("header-images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "header-images",
		ShortUsage: "asc app-clips header-images <subcommand> [flags]",
		ShortHelp:  "Manage App Clip header images.",
		LongHelp: `Manage App Clip header images.

Examples:
  asc app-clips header-images view --id "IMAGE_ID"
  asc app-clips header-images create --localization-id "LOC_ID" --file path/to/image.png
  asc app-clips header-images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAppClipHeaderImagesGetCommand(d),
			newAppClipHeaderImagesCreateCommand(d),
			newAppClipHeaderImagesDeleteCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAppClipHeaderImagesGetCommand retrieves a header image by ID.
func newAppClipHeaderImagesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc app-clips header-images view --id \"IMAGE_ID\"",
		ShortHelp:  "View a header image by ID.",
		LongHelp: `View a header image by ID.

Examples:
  asc app-clips header-images view --id "IMAGE_ID"`,
		IDUsage:            "Header image ID",
		IDType:             "appClipHeaderImages",
		RequiredDiagnostic: true,
		Deps:               d,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipHeaderImage(ctx, id)
		},
	})
}

// newAppClipHeaderImagesCreateCommand uploads a header image.
func newAppClipHeaderImagesCreateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("create", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appClipDefaultExperienceLocalizations", "Default experience localization ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "create",
		ShortUsage: "asc app-clips header-images create --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload a header image for a localization.",
		LongHelp: `Upload a header image for a localization.

The upload process reserves an upload slot, uploads the image, and commits the upload.

Examples:
  asc app-clips header-images create --localization-id "LOC_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locValue := strings.TrimSpace(*localizationID)
			if locValue == "" {
				return d.RequiredFlag("--localization-id")
			}

			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				return d.RequiredFlag("--file")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("app-clips header-images create: %w", err)
			}

			requestCtx, cancel := d.UploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadAppClipHeaderImage(requestCtx, locValue, fileValue)
			if err != nil {
				return fmt.Errorf("app-clips header-images create: %w", err)
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// newAppClipHeaderImagesDeleteCommand deletes a header image.
func newAppClipHeaderImagesDeleteCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		Name:       "delete",
		ShortUsage: "asc app-clips header-images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a header image.",
		LongHelp: `Delete a header image.

Examples:
  asc app-clips header-images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Header image ID",
		IDType:             "appClipHeaderImages",
		RequiredDiagnostic: true,
		Deps:               d,
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppClipHeaderImage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.DeleteResult{ID: id, Deleted: true}
		},
	})
}
