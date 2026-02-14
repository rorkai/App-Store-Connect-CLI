package videopreviews

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// VideoPreviewsCommand returns the top-level video previews command.
func VideoPreviewsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("video-previews", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "video-previews",
		ShortUsage: "asc video-previews <subcommand> [flags]",
		ShortHelp:  "Manage App Store app preview videos.",
		LongHelp: `Manage App Store app preview videos for a version localization.

Examples:
  asc video-previews list --version-localization "LOC_ID"
  asc video-previews upload --version-localization "LOC_ID" --path "./previews" --device-type "IPHONE_69"
  asc video-previews delete --id "PREVIEW_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			assets.AssetsPreviewsListCommand(),
			assets.AssetsPreviewsUploadCommand(),
			assets.AssetsPreviewsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// Command returns the video-previews command group.
func Command() *ffcli.Command {
	return VideoPreviewsCommand()
}
