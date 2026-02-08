package gamecenter

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

	localizationID := fs.String("localization-id", "", "Leaderboard set localization ID")
	filePath := fs.String("file", "", "Path to image file (PNG)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

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
				fmt.Fprintln(os.Stderr, "Error: --localization-id is required")
				return flag.ErrHelp
			}

			file := strings.TrimSpace(*filePath)
			if file == "" {
				fmt.Fprintln(os.Stderr, "Error: --file is required")
				return flag.ErrHelp
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

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}

// GameCenterLeaderboardSetImagesDeleteCommand returns the images delete subcommand.
func GameCenterLeaderboardSetImagesDeleteCommand() *ffcli.Command {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)

	imageID := fs.String("id", "", "Leaderboard set image ID")
	confirm := fs.Bool("confirm", false, "Confirm deletion")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "delete",
		ShortUsage: "asc game-center leaderboard-sets images delete --id \"IMAGE_ID\" --confirm",
		ShortHelp:  "Delete a leaderboard set image.",
		LongHelp: `Delete a leaderboard set image.

Examples:
  asc game-center leaderboard-sets images delete --id "IMAGE_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*imageID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("game-center leaderboard-sets images delete: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.DeleteGameCenterLeaderboardSetImage(requestCtx, id); err != nil {
				return fmt.Errorf("game-center leaderboard-sets images delete: failed to delete: %w", err)
			}

			result := &asc.GameCenterLeaderboardSetImageDeleteResult{
				ID:      id,
				Deleted: true,
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
