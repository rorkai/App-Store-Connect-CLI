package analytics

import (
	"context"
	"flag"
	"fmt"
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
		ShortHelp:  "View analytics report instances or relationships.",
		LongHelp: `View analytics report instances or relationships.

Examples:
  asc analytics instances view --instance-id "INSTANCE_ID"
  asc analytics instances links --instance-id "INSTANCE_ID"
  asc analytics instances links --instance-id "INSTANCE_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
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
	fs := flag.NewFlagSet("view", flag.ExitOnError)

	instanceID := shared.BindResourceIDFlag(fs, "instance-id", "analyticsReportInstances", "Analytics report instance ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc analytics instances view --instance-id \"INSTANCE_ID\" [flags]",
		ShortHelp:  "View an analytics report instance by ID.",
		LongHelp: `View an analytics report instance by ID.

Examples:
  asc analytics instances view --instance-id "INSTANCE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			id := strings.TrimSpace(*instanceID)
			if id == "" {
				return shared.RequiredFlag("--instance-id")
			}
			var err error
			id, err = asc.ValidateResourcePathSegment(id)
			if err != nil {
				return shared.UsageErrorf("analytics instances view: --instance-id: %v", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("analytics instances view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetAnalyticsReportInstance(requestCtx, id)
			if err != nil {
				return fmt.Errorf("analytics instances view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// AnalyticsInstancesRelationshipsCommand lists segment links for an instance.
func AnalyticsInstancesRelationshipsCommand() *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "links",
		ShortUsage: "asc analytics instances links --instance-id \"INSTANCE_ID\" [flags]",
		ShortHelp:  "List analytics report segment relationships.",
		LongHelp: `List analytics report segment relationships.

Examples:
  asc analytics instances links --instance-id "INSTANCE_ID"
  asc analytics instances links --instance-id "INSTANCE_ID" --paginate`,
		ParentFlag:           "instance-id",
		ParentUsage:          "Analytics report instance ID",
		ParentType:           "analyticsReportInstances",
		RequiredDiagnostic:   true,
		LimitMax:             analyticsMaxLimit,
		PaginateIgnoresLimit: true,
		Flags: func(fs *flag.FlagSet) func() error {
			instanceID := fs.Lookup("instance-id").Value
			return func() error {
				if id := strings.TrimSpace(instanceID.String()); id != "" {
					if _, err := asc.ValidateResourcePathSegment(id); err != nil {
						return shared.UsageErrorf("analytics instances links: --instance-id: %v", err)
					}
				}
				return nil
			}
		},
		FetchPage: func(ctx context.Context, client *asc.Client, id string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetAnalyticsReportInstanceSegmentsRelationships(ctx, id, asc.Limit[asc.LinkagesOption](limit), asc.NextURL[asc.LinkagesOption](next))
		},
	})
}
