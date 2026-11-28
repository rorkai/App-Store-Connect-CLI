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

var runInstallSimulator = localxcode.InstallSimulator

func newXcodeInstallSimulatorCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode install-simulator", flag.ExitOnError)
	app := fs.String("app", "", "Path to a built iOS Simulator .app (required)")
	udid := fs.String("udid", "", "Exact UUID of an available, booted iOS simulator (required)")
	timeout := fs.Duration("timeout", time.Minute, "Maximum installation and verification duration (5s through 10m)")
	output := d.BindOutputFlags(fs)
	return &ffcli.Command{
		Name: "install-simulator", ShortUsage: "asc xcode install-simulator --app PATH.app --udid UUID [flags]",
		ShortHelp: "Install and verify a local app on one booted iOS simulator.",
		LongHelp: `Install a built iOS Simulator .app before screenshot capture.

The app must declare a bundle identifier, version, build and executable.
The exact --udid must identify one available, already booted iOS simulator.
This command never creates or boots a simulator, launches an app, or captures
screenshots. It verifies the installed bundle identifier, version and build by
reading the app container returned by simctl. A failed verification returns a
receipt with installed=true and verified=false. Local paths and raw simulator
identifiers are omitted from output. Requires macOS and Xcode.

Example:
  asc xcode install-simulator --app .asc/artifacts/App.app --udid SIMULATOR_UUID --output json`,
		FlagSet: fs, UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			opts := localxcode.SimulatorInstallOptions{AppPath: osPath(d, *app), UDID: *udid, Timeout: *timeout}
			if err := localxcode.ValidateSimulatorInstallOptions(opts); err != nil {
				return d.UsageError(err.Error())
			}
			if _, err := shared.ValidateOutputFormat(*output.Output, *output.Pretty); err != nil {
				return d.UsageError(err.Error())
			}
			result, err := runInstallSimulator(ctx, opts)
			var inputErr *localxcode.InstallInputError
			if errors.As(err, &inputErr) {
				return d.UsageError(inputErr.Error())
			}
			if result != nil {
				if outputErr := d.PrintOutput(result, *output.Output, *output.Pretty); outputErr != nil {
					return errors.Join(outputErr, err)
				}
			}
			return err
		},
	}
}
