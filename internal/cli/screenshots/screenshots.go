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
		ShortHelp:  "Capture, frame, review (experimental local workflow), and upload App Store screenshots.",
		LongHelp: `Manage the full screenshot workflow from local capture to App Store upload.

Local screenshot automation commands are experimental.
If you face issues, please file feedback at:
https://github.com/rudrankriyam/App-Store-Connect-CLI/issues/new/choose

Local workflow (experimental):
  asc screenshots run --plan .asc/screenshots.json
  asc screenshots capture --bundle-id "com.example.app" --name home
  asc screenshots frame --input ./screenshots/raw/home.png --device iphone-air
  asc screenshots review-generate --framed-dir ./screenshots/framed
  asc screenshots review-open --output-dir ./screenshots/review
  asc screenshots review-approve --all-ready --output-dir ./screenshots/review
  asc screenshots list-frame-devices --output json

App Store workflow:
  asc screenshots list --version-localization "LOC_ID"
  asc screenshots sizes
  asc screenshots sizes --all
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/iphone" --device-type "IPHONE_65"
  asc screenshots upload --version-localization "LOC_ID" --path "./screenshots/ipad" --device-type "IPAD_PRO_3GEN_129"
  asc screenshots download --version-localization "LOC_ID" --output-dir "./screenshots/downloaded"
  asc screenshots delete --id "SCREENSHOT_ID" --confirm

For most iOS submissions, one iPhone set (IPHONE_65) and one iPad set
(IPAD_PRO_3GEN_129) are enough. "asc screenshots sizes" focuses on these by
default; use --all only when you need the full matrix.`,
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
			assets.AssetsScreenshotsDownloadCommand(),
			assets.AssetsScreenshotsDeleteCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
