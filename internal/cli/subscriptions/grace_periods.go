package subscriptions

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// SubscriptionsGracePeriodsCommand returns the grace periods command group.
func SubscriptionsGracePeriodsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("grace-periods", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "grace-periods",
		ShortUsage: "asc subscriptions grace-periods <subcommand> [flags]",
		ShortHelp:  "Inspect subscription grace periods.",
		LongHelp: `Inspect subscription grace periods.

Examples:
  asc subscriptions grace-periods get --id "GRACE_PERIOD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			SubscriptionsGracePeriodsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SubscriptionsGracePeriodsGetCommand returns the grace period get subcommand.
func SubscriptionsGracePeriodsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("grace-periods get", flag.ExitOnError)

	gracePeriodID := fs.String("id", "", "Subscription grace period ID")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc subscriptions grace-periods get --id \"GRACE_PERIOD_ID\"",
		ShortHelp:  "Get a subscription grace period by ID.",
		LongHelp: `Get a subscription grace period by ID.

Examples:
  asc subscriptions grace-periods get --id "GRACE_PERIOD_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*gracePeriodID)
			if id == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods get: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetSubscriptionGracePeriod(requestCtx, id)
			if err != nil {
				return fmt.Errorf("subscriptions grace-periods get: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
