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
  asc shots capture --bundle-id "com.example.app" --name home --output-dir ./screenshots/raw
  asc shots capture --provider simctl --bundle-id "com.example.app" --name home --udid booted`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ShotsCaptureCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
