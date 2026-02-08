package analytics

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

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
  asc analytics reports get --report-id "REPORT_ID"
  asc analytics reports relationships --report-id "REPORT_ID"
  asc analytics reports relationships --report-id "REPORT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	reportID := fs.String("report-id", "", "Analytics report ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc analytics reports get --report-id \"REPORT_ID\" [flags]",
		ShortHelp:  "Get an analytics report by ID.",
		LongHelp: `Get an analytics report by ID.

Examples:
  asc analytics reports get --report-id "REPORT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*reportID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --report-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics reports get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAnalyticsReport(requestCtx, strings.TrimSpace(*reportID))
			if err != nil {
				return fmt.Errorf("analytics reports get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// AnalyticsReportsRelationshipsCommand lists instance relationships for a report.
func AnalyticsReportsRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	reportID := fs.String("report-id", "", "Analytics report ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc analytics reports relationships --report-id \"REPORT_ID\" [flags]",
		ShortHelp:  "List analytics report instance relationships.",
		LongHelp: `List analytics report instance relationships.

Examples:
  asc analytics reports relationships --report-id "REPORT_ID"
  asc analytics reports relationships --report-id "REPORT_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > analyticsMaxLimit) {
				return fmt.Errorf("analytics reports relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("analytics reports relationships: %w", err)
			}

			id := strings.TrimSpace(*reportID)
			if id != "" {
				if err := validateUUIDFlag("--report-id", id); err != nil {
					return fmt.Errorf("analytics reports relationships: %w", err)
				}
			}
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --report-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics reports relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(analyticsMaxLimit))
				firstPage, err := client.GetAnalyticsReportInstancesRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("analytics reports relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAnalyticsReportInstancesRelationships(ctx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("analytics reports relationships: %w", err)
				}

				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetAnalyticsReportInstancesRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("analytics reports relationships: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
