package screenshots

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/assets"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shots"
)

// ScreenshotsCommand returns the top-level screenshots command.
func ScreenshotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("screenshots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "screenshots",
		ShortUsage: "asc screenshots <subcommand> [flags]",
		ShortHelp:  "Capture, frame, review, and upload App Store screenshots.",
		LongHelp: `Manage the full screenshot workflow from local capture to App Store upload.

Local workflow:
  asc screenshots run --plan .asc/screenshots.json
  asc screenshots capture --bundle-id "com.example.app" --name home
  asc screenshots frame --input ./screenshots/raw/home.png --device iphone-air
  asc screenshots review-generate --framed-dir ./screenshots/framed
  asc screenshots review-open --output-dir ./screenshots/review
  asc screenshots review-approve --all-ready --output-dir ./screenshots/review
  asc screenshots list-frame-devices --output json

App Store workflow:
  asc screenshots list --version-localization "LOC_ID"
  asc screenshots sizes --display-type "APP_IPHONE_69"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots" --device-type "IPHONE_69"
  asc screenshots delete --id "SCREENSHOT_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			shots.ShotsRunCommand(),
			shots.ShotsCaptureCommand(),
			shots.ShotsFrameCommand(),
			shots.ShotsFramesListDevicesCommand(),
			shots.ShotsReviewGenerateCommand(),
			shots.ShotsReviewOpenCommand(),
			shots.ShotsReviewApproveCommand(),
			assets.AssetsScreenshotsListCommand(),
			assets.AssetsScreenshotsSizesCommand(),
			assets.AssetsScreenshotsUploadCommand(),
			assets.AssetsScreenshotsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// Command returns the screenshots command group.
func Command() *ffcli.Command {
	return ScreenshotsCommand()
}
