package xcode

import (
	"context"
	"errors"
	"flag"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var runListDevices = localxcode.ListDevices

func newXcodeDevicesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode devices", flag.ExitOnError)
	availableOnly := fs.Bool("available-only", false, "Only paired, connected or available physical iOS devices")
	timeout := fs.Duration("timeout", 20*time.Second, "Maximum discovery duration (5s through 10m)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "devices", ShortUsage: "asc xcode devices [flags]", ShortHelp: "List physical iOS devices with identifiers accepted by xcode install.",
		LongHelp: `List physical iOS devices known to the active Xcode CoreDevice tool.

Use the returned deviceId with asc xcode install --device-id. These are
CoreDevice identifiers, not hardware UDIDs. Device names and identifiers are
intentionally included for local selection. No device state is changed.

By default, disconnected devices are included with their current state.
--available-only selects paired, connected or available physical iOS devices
whose hardware identity is known. It excludes simulators. App-specific signing
and provisioning eligibility are checked separately by xcode install.
Requires macOS with Xcode devicectl; no App Store Connect authentication.

Examples:
  asc xcode devices --available-only --output json
  asc xcode devices --output table`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			opts := localxcode.DevicesOptions{AvailableOnly: *availableOnly, Timeout: *timeout}
			if err := localxcode.ValidateDevicesOptions(opts); err != nil {
				return d.UsageError(err.Error())
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			result, err := runListDevices(ctx, opts)
			if err != nil {
				return shared.NewErrorWithCause(errors.New("xcode devices failed"), err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
