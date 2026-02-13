package shots

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ShotsCommand returns the shots command group.
func ShotsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("shots", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "shots",
		ShortUsage: "asc shots <subcommand> [flags]",
		ShortHelp:  "Capture and manage screenshots for App Store.",
		LongHelp: `Capture simulator screenshots and prepare them for App Store submission.

Examples:
  asc shots run
  asc shots run --plan .asc/screenshots.json
  asc shots capture --bundle-id "com.example.app" --name home --output-dir ./screenshots/raw
  asc shots capture --provider axe --bundle-id "com.example.app" --name home --udid booted
  asc shots frame --input ./screenshots/raw/home.png --device iphone-air
  asc shots frame --config .asc/koubou.yaml --output-dir ./screenshots/framed
  asc shots review generate --raw-dir ./screenshots/raw --framed-dir ./screenshots/framed
  asc shots review open --output-dir ./screenshots/review
  asc shots review approve --all-ready --output-dir ./screenshots/review
  asc assets screenshots upload --version-localization "LOC_ID" --path "./screenshots/framed/en/iPhone_Air" --device-type "IPHONE_69"
  asc shots frames list-devices --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ShotsRunCommand(),
			ShotsCaptureCommand(),
			ShotsFrameCommand(),
			ShotsFramesCommand(),
			ShotsReviewCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
