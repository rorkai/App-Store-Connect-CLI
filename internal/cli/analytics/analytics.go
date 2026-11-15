package analytics

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewAnalyticsCommand returns the analytics command with subcommands.
func NewAnalyticsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("analytics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "analytics",
		ShortUsage: "asc analytics <subcommand> [flags]",
		ShortHelp:  "Request and download analytics and sales reports.",
		LongHelp: `Request and download analytics and sales reports.

Examples:
  asc analytics sales --vendor "12345678" --type SALES --subtype SUMMARY --frequency DAILY --date "2024-01-20"
  asc analytics request --app "APP_ID" --access-type ONGOING
  asc analytics request --app "APP_ID" --access-type ONGOING --reuse-existing
  asc analytics requests --app "APP_ID"
  asc analytics view --request-id "REQUEST_ID"
  asc analytics reports view --report-id "REPORT_ID"
  asc analytics instances links --instance-id "INSTANCE_ID"
  asc analytics download --request-id "REQUEST_ID" --instance-id "INSTANCE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newAnalyticsSalesCommand(d),
			newAnalyticsRequestCommand(d),
			newAnalyticsRequestsCommand(d),
			newAnalyticsViewCommand(d),
			newAnalyticsReportsCommand(d),
			newAnalyticsInstancesCommand(d),
			newAnalyticsSegmentsCommand(d),
			newAnalyticsDownloadCommand(d),
			newAnalyticsCompareCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
