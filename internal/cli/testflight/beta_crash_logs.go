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

func DeprecatedBetaCrashLogsAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-crash-logs", flag.ExitOnError)

	cmd := &ffcli.Command{
		Name:       "beta-crash-logs",
		ShortUsage: "asc testflight crashes log --crash-log-id \"CRASH_LOG_ID\"",
		ShortHelp:  "Compatibility alias for direct crash-log lookups.",
		LongHelp:   "Compatibility alias: use `asc testflight crashes log --crash-log-id CRASH_LOG_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Subcommands: []*ffcli.Command{
			deprecatedBetaCrashLogsGetAliasCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}

	return hideTestFlightCommand(cmd)
}

func deprecatedBetaCrashLogsGetAliasCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Beta crash log ID")
	crashLogID := fs.String("crash-log-id", "", "Crash log ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight crashes log --crash-log-id \"CRASH_LOG_ID\"",
		ShortHelp:  "Compatibility alias: use `asc testflight crashes log`.",
		LongHelp:   "Compatibility alias: use `asc testflight crashes log --crash-log-id CRASH_LOG_ID`.",
		FlagSet:    fs,
		UsageFunc:  shared.DeprecatedUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue, err := resolveLegacyAliasID(strings.TrimSpace(*id), strings.TrimSpace(*crashLogID), "--crash-log-id")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
				return flag.ErrHelp
			}
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --crash-log-id is required")
				return flag.ErrHelp
			}
			return runCrashLogByCrashLogID(ctx, idValue, output)
		},
	}
}

func runCrashLogByCrashLogID(ctx context.Context, crashLogID string, output shared.OutputFlags) error {
	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("testflight crashes log: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	resp, err := client.GetBetaCrashLog(requestCtx, crashLogID)
	if err != nil {
		return fmt.Errorf("testflight crashes log: failed to fetch: %w", err)
	}

	return shared.PrintOutput(resp, *output.Output, *output.Pretty)
}
