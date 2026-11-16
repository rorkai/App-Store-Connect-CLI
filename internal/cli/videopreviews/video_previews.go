package videopreviews

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewVideoPreviewsCommand returns the top-level video previews command.
func NewVideoPreviewsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("video-previews", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "video-previews",
		ShortUsage: "asc video-previews <subcommand> [flags]",
		ShortHelp:  "Manage App Store app preview videos.",
		LongHelp: `Manage App Store app preview videos for a version localization.

Examples:
  asc video-previews list --version-localization "VERSION_LOCALIZATION_ID"
  asc video-previews upload --version-localization "VERSION_LOCALIZATION_ID" --path "./previews" --device-type "IPHONE_69"
  asc video-previews download --version-localization "VERSION_LOCALIZATION_ID" --output-dir "./previews/downloaded"
  asc video-previews delete --id "PREVIEW_ID" --confirm
  asc video-previews set-poster-frame --id "PREVIEW_ID" --time-code "00:00:05:00"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			assets.NewAssetsPreviewsListCommand(d),
			assets.NewAssetsPreviewsUploadCommand(d),
			assets.NewAssetsPreviewsDownloadCommand(d),
			assets.NewAssetsPreviewsDeleteCommand(d),
			assets.NewAssetsPreviewsSetPosterFrameCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
