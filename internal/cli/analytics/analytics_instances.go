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

// AnalyticsInstancesCommand returns the analytics instances command group.
func AnalyticsInstancesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("instances", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "instances",
		ShortUsage: "asc analytics instances <subcommand> [flags]",
		ShortHelp:  "Get analytics report instances or relationships.",
		LongHelp: `Get analytics report instances or relationships.

Examples:
  asc analytics instances get --instance-id "INSTANCE_ID"
  asc analytics instances relationships --instance-id "INSTANCE_ID"
  asc analytics instances relationships --instance-id "INSTANCE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			AnalyticsInstancesGetCommand(),
			AnalyticsInstancesRelationshipsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// AnalyticsInstancesGetCommand retrieves a specific analytics report instance.
func AnalyticsInstancesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	instanceID := fs.String("instance-id", "", "Analytics report instance ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc analytics instances get --instance-id \"INSTANCE_ID\" [flags]",
		ShortHelp:  "Get an analytics report instance by ID.",
		LongHelp: `Get an analytics report instance by ID.

Examples:
  asc analytics instances get --instance-id "INSTANCE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*instanceID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --instance-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics instances get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAnalyticsReportInstance(requestCtx, strings.TrimSpace(*instanceID))
			if err != nil {
				return fmt.Errorf("analytics instances get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AnalyticsInstancesRelationshipsCommand lists segment relationships for an instance.
func AnalyticsInstancesRelationshipsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("relationships", flag.ExitOnError)

	instanceID := fs.String("instance-id", "", "Analytics report instance ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "relationships",
		ShortUsage: "asc analytics instances relationships --instance-id \"INSTANCE_ID\" [flags]",
		ShortHelp:  "List analytics report segment relationships.",
		LongHelp: `List analytics report segment relationships.

Examples:
  asc analytics instances relationships --instance-id "INSTANCE_ID"
  asc analytics instances relationships --instance-id "INSTANCE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > analyticsMaxLimit) {
				return fmt.Errorf("analytics instances relationships: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("analytics instances relationships: %w", err)
			}

			id := strings.TrimSpace(*instanceID)
			if id != "" {
				if err := validateUUIDFlag("--instance-id", id); err != nil {
					return fmt.Errorf("analytics instances relationships: %w", err)
				}
			}
			if id == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --instance-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics instances relationships: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.LinkagesOption{
				asc.WithLinkagesLimit(*limit),
				asc.WithLinkagesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithLinkagesLimit(analyticsMaxLimit))
				firstPage, err := client.GetAnalyticsReportInstanceSegmentsRelationships(requestCtx, id, paginateOpts...)
				if err != nil {
					return fmt.Errorf("analytics instances relationships: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetAnalyticsReportInstanceSegmentsRelationships(ctx, id, asc.WithLinkagesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("analytics instances relationships: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetAnalyticsReportInstanceSegmentsRelationships(requestCtx, id, opts...)
			if err != nil {
				return fmt.Errorf("analytics instances relationships: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
