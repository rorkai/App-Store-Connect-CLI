package gamecenter

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// GameCenterLeaderboardSetImagesCommand returns the images command group for leaderboard sets.
func GameCenterLeaderboardSetImagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("images", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "images",
		ShortUsage: "asc game-center leaderboard-sets images <subcommand> [flags]",
		ShortHelp:  "Manage Game Center leaderboard set images.",
		LongHelp: `Manage Game Center leaderboard set images. Images are attached to leaderboard set localizations.

Examples:
  asc game-center leaderboard-sets images upload --localization-id "LOC_ID" --file path/to/image.png
  asc game-center leaderboard-sets images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			GameCenterLeaderboardSetImagesUploadCommand(),
			GameCenterLeaderboardSetImagesDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// GameCenterLeaderboardSetImagesUploadCommand returns the images upload subcommand.
func GameCenterLeaderboardSetImagesUploadCommand() *ffcli.Command {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)

	localizationID := shared.BindResourceIDFlag(fs, "localization-id", "gameCenterLeaderboardSetLocalizations", "Leaderboard set localization ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		ShortUsage: "asc game-center leaderboard-sets images upload --localization-id \"LOC_ID\" --file path/to/image.png",
		ShortHelp:  "Upload an image for a leaderboard set localization.",
		LongHelp: `Upload an image for a leaderboard set localization.

The upload process reserves an upload slot, uploads the image file, and commits the upload.

Examples:
  asc game-center leaderboard-sets images upload --localization-id "LOC_ID" --file path/to/image.png`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			locID := strings.TrimSpace(*localizationID)
			if locID == "" {
				return shared.RequiredFlag("--localization-id")
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				return shared.RequiredFlag("--file")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets images upload: %w", err)
			}

			requestCtx, cancel := shared.ContextWithUploadTimeout(ctx)
			defer cancel()

			result, err := client.UploadGameCenterLeaderboardSetImage(requestCtx, locID, file)
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets images upload: %w", err)
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// GameCenterLeaderboardSetImagesDeleteCommand returns the images delete subcommand.
func GameCenterLeaderboardSetImagesDeleteCommand() *ffcli.Command {
	return shared.BuildConfirmDeleteCommand(shared.ConfirmDeleteCommandConfig{
		FlagSetName: "delete",
		Name:        "delete",
		ShortUsage:  "asc game-center leaderboard-sets images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:   "Delete a leaderboard set image.",
		LongHelp: `Delete a leaderboard set image.

Examples:
  asc game-center leaderboard-sets images delete --id "IMAGE_ID" --confirm`,
		IDFlag:             "id",
		IDUsage:            "Leaderboard set image ID",
		IDType:             "gameCenterLeaderboardSetImages",
		RequiredDiagnostic: true,
		ErrorPrefix:        "game-center leaderboard-sets images delete",
		DeleteErrorPrefix:  "game-center leaderboard-sets images delete: failed to delete",
		Delete: func(ctx context.Context, client *asc.Client, id string) error {
			return client.DeleteGameCenterLeaderboardSetImage(ctx, id)
		},
		Result: func(id string) any {
			return &asc.GameCenterLeaderboardSetImageDeleteResult{ID: id, Deleted: true}
		},
	})
}
