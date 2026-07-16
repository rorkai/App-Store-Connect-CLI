package xcodecloud

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func xcodeCloudBuildRunsListFlags(fs *flag.FlagSet) (workflowID *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	workflowID = fs.String("workflow-id", "", "Workflow ID to list build runs for")
	sort = fs.String("sort", "", "Sort by number or -number")
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// XcodeCloudBuildRunsCommand returns the xcode-cloud build-runs subcommand.
func XcodeCloudBuildRunsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-runs", flag.ExitOnError)

	workflowID, sort, limit, next, paginate, output, pretty := xcodeCloudBuildRunsListFlags(fs)

	return &ffcli.Command{
		Name:       "build-runs",
		ShortUsage: "asc xcode-cloud build-runs [flags]",
		ShortHelp:  "Manage Xcode Cloud build runs.",
		LongHelp: `Manage Xcode Cloud build runs.

Examples:
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID"
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --sort "-number"
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID"
  asc xcode-cloud build-runs view --id "BUILD_RUN_ID"
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID"
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --limit 50
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudBuildRunsListCommand(),
			XcodeCloudBuildRunsGetCommand(),
			XcodeCloudBuildRunsBuildsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudBuildRunsList(ctx, *workflowID, *sort, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudBuildRunsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	workflowID, sort, limit, next, paginate, output, pretty := xcodeCloudBuildRunsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud build-runs list [flags]",
		ShortHelp:  "List Xcode Cloud build runs for a workflow.",
		LongHelp: `List Xcode Cloud build runs for a workflow.

Examples:
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID"
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID" --sort "-number"
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID" --limit 50
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudBuildRunsList(ctx, *workflowID, *sort, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudBuildRunsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc xcode-cloud build-runs view --id \"BUILD_RUN_ID\"",
		ShortHelp:   "View details for a build run.",
		LongHelp: `View details for a build run.

Examples:
  asc xcode-cloud build-runs view --id "BUILD_RUN_ID"
  asc xcode-cloud build-runs view --id "BUILD_RUN_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Build run ID",
		ErrorPrefix: "xcode-cloud build-runs view",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiBuildRun(ctx, id)
		},
	})
}

func XcodeCloudBuildRunsBuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds", flag.ExitOnError)

	runID := fs.String("run-id", "", "Build run ID to list builds for")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "builds",
		ShortUsage: "asc xcode-cloud build-runs builds [flags]",
		ShortHelp:  "List builds for a build run.",
		LongHelp: `List builds for a build run.

Examples:
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID"
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID" --output table
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID" --limit 50
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return runXcodeCloudPaginatedParentList(
				ctx,
				*runID,
				"run-id",
				*limit,
				*next,
				*paginate,
				*output.Output,
				*output.Pretty,
				"xcode-cloud build-runs builds",
				func(ctx context.Context, client *asc.Client, runID string, limit int, next string) (asc.PaginatedResponse, error) {
					return client.GetCiBuildRunBuilds(
						ctx,
						runID,
						asc.WithCiBuildRunBuildsLimit(limit),
						asc.WithCiBuildRunBuildsNextURL(next),
					)
				},
				func(ctx context.Context, client *asc.Client, runID string, next string) (asc.PaginatedResponse, error) {
					return client.GetCiBuildRunBuilds(ctx, runID, asc.WithCiBuildRunBuildsNextURL(next))
				},
			)
		},
	}
}

func xcodeCloudBuildRunsList(ctx context.Context, workflowID string, sort string, limit int, next string, paginate bool, output string, pretty bool) error {
	if err := shared.ValidateSort(sort, "number", "-number"); err != nil {
		return shared.UsageError(err.Error())
	}

	return runXcodeCloudPaginatedParentList(
		ctx,
		workflowID,
		"workflow-id",
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud build-runs",
		func(ctx context.Context, client *asc.Client, workflowID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildRuns(
				ctx,
				workflowID,
				asc.WithCiBuildRunsSort(sort),
				asc.WithCiBuildRunsLimit(limit),
				asc.WithCiBuildRunsNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, workflowID string, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildRuns(ctx, workflowID, asc.WithCiBuildRunsNextURL(next))
		},
	)
}
