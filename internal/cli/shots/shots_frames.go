package shots

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ShotsFramesCommand returns the shots frames command group.
func ShotsFramesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("frames", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "frames",
		ShortUsage: "asc shots frames <subcommand> [flags]",
		ShortHelp:  "Discover and inspect screenshot frame devices.",
		LongHelp: `Discover available frame devices for asc shots frame.

Use list-devices to print all supported --device values and the default.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ShotsFramesListDevicesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
