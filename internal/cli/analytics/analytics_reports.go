package analytics

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// AnalyticsReportsCommand returns the analytics reports command group.
func AnalyticsReportsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("reports", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "reports",
		ShortUsage: "asc analytics reports <subcommand> [flags]",
		ShortHelp:  "Get analytics reports by ID or relationships.",
		LongHelp: `Get analytics reports by ID or relationships.

Examples:
  asc analytics reports view --report-id "REPORT_ID"
  asc analytics reports links --report-id "REPORT_ID"
  asc analytics reports links --report-id "REPORT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			AnalyticsReportsGetCommand(),
			AnalyticsReportsRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AnalyticsReportsGetCommand retrieves a specific analytics report.
func AnalyticsReportsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc analytics reports view --report-id \"REPORT_ID\" [flags]",
		ShortHelp:  "View an analytics report by ID.",
		LongHelp: `View an analytics report by ID.

Examples:
  asc analytics reports view --report-id "REPORT_ID"`,
		IDFlag:             "report-id",
		IDUsage:            "Analytics report ID",
		IDType:             "analyticsReports",
		RequiredDiagnostic: true,
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetAnalyticsReport(ctx, id)
		},
	})
}

// AnalyticsReportsRelationshipsCommand lists instance links for a report.
func AnalyticsReportsRelationshipsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "links",
		ShortUsage: "asc analytics reports links --report-id \"REPORT_ID\" [flags]",
		ShortHelp:  "List analytics report instance relationships.",
		LongHelp: `List analytics report instance relationships.

Examples:
  asc analytics reports links --report-id "REPORT_ID"
  asc analytics reports links --report-id "REPORT_ID" --paginate`,
		ParentFlag:           "report-id",
		ParentUsage:          "Analytics report ID",
		ParentType:           "analyticsReports",
		RequiredDiagnostic:   true,
		LimitMax:             analyticsMaxLimit,
		PaginateIgnoresLimit: true,
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAnalyticsReportInstancesRelationships(ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
}
