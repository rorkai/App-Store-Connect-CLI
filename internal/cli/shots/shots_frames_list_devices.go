package shots

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

type frameDevicesOutput struct {
	Default string                          `json:"default"`
	Devices []screenshots.FrameDeviceOption `json:"devices"`
}

// ShotsFramesListDevicesCommand returns the screenshots list-frame-devices subcommand.
func ShotsFramesListDevicesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list-frame-devices", flag.ExitOnError)
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list-frame-devices",
		ShortUsage: "asc screenshots list-frame-devices [--output json]",
		ShortHelp:  "List supported frame devices and the default.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result := frameDevicesOutput{
				Default: string(screenshots.DefaultFrameDevice()),
				Devices: screenshots.FrameDeviceOptions(),
			}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
