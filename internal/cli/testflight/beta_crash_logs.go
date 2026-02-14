package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaCrashLogsCommand returns the beta-crash-logs command group.
func BetaCrashLogsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-crash-logs", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-crash-logs",
		ShortUsage: "asc testflight beta-crash-logs <subcommand> [flags]",
		ShortHelp:  "Fetch TestFlight beta crash logs.",
		LongHelp: `Fetch TestFlight beta crash logs.

Examples:
  asc testflight beta-crash-logs get --id "CRASH_LOG_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaCrashLogsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaCrashLogsGetCommand returns the beta-crash-logs get subcommand.
func BetaCrashLogsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-crash-logs get", flag.ExitOnError)

	id := fs.String("id", "", "Beta crash log ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-crash-logs get --id \"CRASH_LOG_ID\"",
		ShortHelp:  "Get a beta crash log by ID.",
		LongHelp: `Get a beta crash log by ID.

Examples:
  asc testflight beta-crash-logs get --id "CRASH_LOG_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-crash-logs get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaCrashLog(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("testflight beta-crash-logs get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
