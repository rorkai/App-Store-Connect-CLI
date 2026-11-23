package analytics

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAnalyticsInstancesCommand returns the analytics instances command group.
func newAnalyticsInstancesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("instances", flag.ExitOnError)

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
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newAnalyticsInstancesGetCommand(d),
			newAnalyticsInstancesRelationshipsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newAnalyticsInstancesGetCommand retrieves a specific analytics report instance.
func newAnalyticsInstancesGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Deps:       d,
		Name:       "view",
		ShortUsage: "asc analytics instances view --instance-id \"INSTANCE_ID\" [flags]",
		ShortHelp:  "View an analytics report instance by ID.",
		LongHelp: `View an analytics report instance by ID.

Examples:
  asc analytics instances view --instance-id "INSTANCE_ID"`,
		IDFlag:             "instance-id",
		IDUsage:            "Analytics report instance ID",
		IDType:             "analyticsReportInstances",
		RequiredDiagnostic: true,
		Flags: func(fs *flag.FlagSet) func() error {
			instanceID := fs.Lookup("instance-id").Value
			return func() error {
				if _, err := asc.ValidateResourcePathSegment(strings.TrimSpace(instanceID.String())); err != nil {
					return d.UsageErrorf("analytics instances view: --instance-id: %v", err)
				}
				return nil
			}
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			instance, err := client.GetAnalyticsReportInstance(ctx, id)
			if apiErr, ok := errors.AsType[*asc.APIError](err); ok && apiErr.HTTPStatusCode() == http.StatusNotFound {
				return nil, &shared.HintedError{
					Err: err,
					Hint: "Find current instance IDs with `asc analytics reports links --report-id REPORT_ID --paginate --output json` (use `data[].id`) " +
						"or `asc analytics view --request-id REQUEST_ID --paginate --output json` (use `data[].instances[].id`).",
				}
			}
			return instance, err
		},
	})
}

// newAnalyticsInstancesRelationshipsCommand lists segment links for an instance.
func newAnalyticsInstancesRelationshipsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Deps:       d,
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
						return d.UsageErrorf("analytics instances links: --instance-id: %v", err)
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
