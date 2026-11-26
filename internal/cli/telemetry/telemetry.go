package telemetrycmd

import (
	"context"
	"flag"
	"fmt"
	"strconv"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

// NewTelemetryCommand returns the telemetry command group built on d.
func NewTelemetryCommand(d *shared.Deps) *ffcli.Command {
	return &ffcli.Command{
		Name:       "telemetry",
		ShortUsage: "asc telemetry <subcommand>",
		ShortHelp:  "Manage CLI telemetry settings.",
		LongHelp: `Manage CLI telemetry settings.

Telemetry is enabled by default and sends command-level usage events to help
improve asc. asc prints a one-time notice before it sends the first event.
Events carry no persistent ID unless you opt in with
"asc telemetry enable --install-id", which stores a random installation ID so
activity from one installation can be grouped over time. For "asc api" it also
sends the HTTP method, the request path with every identifier replaced by {id}
(or "other"), and the names of the query parameters; never their values,
identifiers, headers, or bodies. It does not collect raw arguments, Apple
account identifiers, app identifiers, bundle identifiers, usernames, hostnames,
repo names, or file paths.

Turn telemetry off with "asc telemetry disable" (alias "off"), DO_NOT_TRACK=1,
ASC_TELEMETRY_DISABLED=1, or "asc --no-telemetry <command>" for one run.

Examples:
  asc telemetry status
  asc telemetry disable
  asc telemetry enable
  asc telemetry enable --install-id
  asc telemetry reset-id`,
		FlagSet:   d.NewFlagSet("telemetry", flag.ExitOnError),
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			statusCommand(d),
			enableCommand(d),
			disableCommand(d),
			resetIDCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintf(d.IO.ErrOut, "Unknown subcommand: %s\n\n", args[0])
			}
			return flag.ErrHelp
		},
	}
}

func statusCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("telemetry status", flag.ExitOnError)
	output := shared.BindOutputFlagsWithAllowed(fs, "output", defaultStatusOutputFormat(d), "Output format: table, json", "table", "json")
	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc telemetry status [flags]",
		ShortHelp:  "Show telemetry status.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "table", "json")
			if err != nil {
				return d.UsageError(err.Error())
			}
			status, err := telemetry.ReadStatus()
			if err != nil {
				return err
			}
			if normalizedOutput == "json" {
				return d.PrintOutput(status, "json", *output.Pretty)
			}
			printStatus(d, status)
			return nil
		},
	}
}

func defaultStatusOutputFormat(d *shared.Deps) string {
	if d.DefaultOutputFormat() == "json" {
		return "json"
	}
	return "table"
}

func enableCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("telemetry enable", flag.ExitOnError)
	installID := fs.Bool("install-id", false, "Also send a random installation ID that groups events from this installation; --install-id=false deletes it")
	return &ffcli.Command{
		Name:       "enable",
		ShortUsage: "asc telemetry enable [flags]",
		ShortHelp:  "Enable telemetry.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := telemetry.SetEnabled(true); err != nil {
				return err
			}
			installIDSet := false
			fs.Visit(func(f *flag.Flag) { installIDSet = installIDSet || f.Name == "install-id" })
			if !installIDSet {
				fmt.Fprintln(d.IO.Out, "Telemetry enabled")
				return nil
			}
			if _, err := telemetry.SetInstallIDEnabled(*installID); err != nil {
				return err
			}
			if *installID {
				fmt.Fprintln(d.IO.Out, "Telemetry enabled with install ID")
			} else {
				fmt.Fprintln(d.IO.Out, "Telemetry enabled without install ID")
			}
			return nil
		},
	}
}

func disableCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("telemetry disable", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "disable",
		ShortUsage: "asc telemetry disable",
		ShortHelp:  "Disable telemetry and delete the install ID.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := telemetry.SetEnabled(false); err != nil {
				return err
			}
			fmt.Fprintln(d.IO.Out, "Telemetry disabled")
			return nil
		},
	}
}

func resetIDCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("telemetry reset-id", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "reset-id",
		ShortUsage: "asc telemetry reset-id",
		ShortHelp:  "Reset the random local install ID.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id, err := telemetry.ResetInstallID()
			if err != nil {
				return err
			}
			if id == "" {
				fmt.Fprintln(d.IO.Out, "No install ID is stored; telemetry events carry none")
				return nil
			}
			fmt.Fprintln(d.IO.Out, "Telemetry install ID reset")
			return nil
		},
	}
}

func printStatus(d *shared.Deps, status telemetry.Status) {
	rows := [][]string{
		{"Enabled", strconv.FormatBool(status.Enabled)},
		{"Reason", shared.OrNA(status.Reason)},
		{"Install ID enabled", strconv.FormatBool(status.InstallIDEnabled)},
		{"Install ID", shared.OrNA(status.InstallID)},
		{"Endpoint", shared.OrNA(status.Endpoint)},
		{"Path", status.Path},
	}
	d.RenderSection("Telemetry", []string{"Field", "Value"}, rows, false)
}
