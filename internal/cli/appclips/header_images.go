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

// AppClipHeaderImagesCommand returns the header images command group.
func AppClipHeaderImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("header-images", flag.ExitOnError)

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
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc app-clips header-images view --id \"IMAGE_ID\"",
		ShortHelp:   "View a header image by ID.",
		LongHelp: `View a header image by ID.

Examples:
  asc app-clips header-images view --id "IMAGE_ID"`,
		IDUsage:            "Header image ID",
		IDType:             "appClipHeaderImages",
		RequiredDiagnostic: true,
		ErrorPrefix:        "app-clips header-images view",
		FetchErrorPrefix:   "app-clips header-images view: failed to fetch",
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAppClipHeaderImage(ctx, id)
		},
	})
}

// AppClipHeaderImagesCreateCommand uploads a header image.
func AppClipHeaderImagesCreateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("create", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "appClipDefaultExperienceLocalizations", "Default experience localization ID")
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
				return shared.RequiredFlag("--localization-id")
			}

			fileValue := strings.TrimSpace(*filePath)
			if fileValue == "" {
				return shared.RequiredFlag("--file")
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
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc app-clips header-images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:   "Delete a header image.",
		LongHelp: `Delete a header image.

Examples:
  asc app-clips header-images delete --id "IMAGE_ID" --confirm`,
		IDFlag:                 "id",
		IDUsage:                "Header image ID",
		IDType:                 "appClipHeaderImages",
		RequiredDiagnostic:     true,
		ConfirmRequiredMessage: "--confirm is required to delete",
		ErrorPrefix:            "app-clips header-images delete",
		DeleteErrorPrefix:      "app-clips header-images delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteAppClipHeaderImage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.AppClipHeaderImageDeleteResult{ID: id, Deleted: true}
		},
	})
}
