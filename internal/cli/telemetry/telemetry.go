package telemetrycmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TelemetryCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "telemetry",
		ShortUsage: "asc telemetry <subcommand>",
		ShortHelp:  "Manage anonymous CLI telemetry settings.",
		LongHelp: `Manage anonymous CLI telemetry settings.

Telemetry is enabled by default and sends anonymous command-level usage events
to help improve asc. It does not collect raw arguments, Apple account
identifiers, app identifiers, bundle identifiers, usernames, hostnames, repo
names, or paths. Use "asc telemetry disable" to opt out.

Examples:
  asc telemetry status
  asc telemetry disable
  asc telemetry enable
  asc telemetry reset-id`,
		FlagSet:   flag.NewFlagSet("telemetry", flag.ExitOnError),
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			statusCommand(),
			enableCommand(),
			disableCommand(),
			resetIDCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", args[0])
			}
			return flag.ErrHelp
		},
	}
}

func statusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("telemetry status", flag.ExitOnError)
	output := shared.BindOutputFlagsWithAllowed(fs, "output", defaultStatusOutputFormat(), "Output format: table, json", "table", "json")
	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc telemetry status [flags]",
		ShortHelp:  "Show anonymous telemetry status.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return flag.ErrHelp
			}
			normalizedOutput, err := shared.ValidateOutputFormatAllowed(*output.Output, *output.Pretty, "table", "json")
			if err != nil {
				return shared.UsageError(err.Error())
			}
			status, err := telemetry.ReadStatus()
			if err != nil {
				return err
			}
			if normalizedOutput == "json" {
				return shared.PrintOutput(status, "json", *output.Pretty)
			}
			printStatus(status)
			return nil
		},
	}
}

func defaultStatusOutputFormat() string {
	if shared.DefaultOutputFormat() == "json" {
		return "json"
	}
	return "table"
}

func enableCommand() *ffcli.Command {
	return stateCommand("enable", "Enable anonymous telemetry.", true)
}

func disableCommand() *ffcli.Command {
	return stateCommand("disable", "Disable anonymous telemetry.", false)
}

func stateCommand(name, help string, enabled bool) *ffcli.Command {
	fs := flag.NewFlagSet("telemetry "+name, flag.ExitOnError)
	return &ffcli.Command{
		Name:       name,
		ShortUsage: "asc telemetry " + name,
		ShortHelp:  help,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return flag.ErrHelp
			}
			if err := telemetry.SetEnabled(enabled); err != nil {
				return err
			}
			if enabled {
				fmt.Fprintln(os.Stdout, "Telemetry enabled")
			} else {
				fmt.Fprintln(os.Stdout, "Telemetry disabled")
			}
			return nil
		},
	}
}

func resetIDCommand() *ffcli.Command {
	fs := flag.NewFlagSet("telemetry reset-id", flag.ExitOnError)
	return &ffcli.Command{
		Name:       "reset-id",
		ShortUsage: "asc telemetry reset-id",
		ShortHelp:  "Reset the anonymous local install ID.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return flag.ErrHelp
			}
			if _, err := telemetry.ResetInstallID(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Telemetry install ID reset")
			return nil
		},
	}
}

func printStatus(status telemetry.Status) {
	enabled := "false"
	if status.Enabled {
		enabled = "true"
	}
	rows := [][]string{
		{"Enabled", enabled},
		{"Reason", shared.OrNA(status.Reason)},
		{"Install ID", shared.OrNA(status.InstallID)},
		{"Endpoint", shared.OrNA(status.Endpoint)},
		{"Path", status.Path},
	}
	shared.RenderSection("Telemetry", []string{"Field", "Value"}, rows, false)
}
