package analytics

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AnalyticsCommand returns the analytics command with subcommands.
func AnalyticsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("analytics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "analytics",
		ShortUsage: "asc analytics <subcommand> [flags]",
		ShortHelp:  "Request and download analytics and sales reports.",
		LongHelp: `Request and download analytics and sales reports.

Examples:
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"
  asc analytics request --app "APP_ID" --access-type ONGOING
  asc analytics requests --app "APP_ID"
  asc analytics get --request-id "REQUEST_ID"
  asc analytics reports get --report-id "REPORT_ID"
  asc analytics instances links --instance-id "INSTANCE_ID"
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AnalyticsSalesCommand(),
			AnalyticsRequestCommand(),
			AnalyticsRequestsCommand(),
			AnalyticsGetCommand(),
			AnalyticsReportsCommand(),
			AnalyticsInstancesCommand(),
			AnalyticsSegmentsCommand(),
			AnalyticsDownloadCommand(),
			AnalyticsCompareCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
