package xcodecloud

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func xcodeCloudBuildRunsListFlags(d *shared.Deps, fs *flag.FlagSet) (workflowID *string, sort *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	workflowID = shared.BindResourceIDFlag(fs, "workflow-id", "ciWorkflows", "Workflow ID to list build runs for")
	sort = fs.String("sort", "", "Sort by number or -number")
	limit = fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next = fs.String("next", "", shared.NextFlagUsage)
	paginate = fs.Bool("paginate", false, shared.PaginateFlagUsage)
	outputFlags := d.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// newXcodeCloudBuildRunsCommand returns the xcode-cloud build-runs subcommand.
func newXcodeCloudBuildRunsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("build-runs", flag.ExitOnError)

	workflowID, sort, limit, next, paginate, output, pretty := xcodeCloudBuildRunsListFlags(d, fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newXcodeCloudBuildRunsListCommand(d),
			newXcodeCloudBuildRunsGetCommand(d),
			newXcodeCloudBuildRunsBuildsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "xcode-cloud build-runs", 200, "sort"); err != nil {
				return err
			}
			return xcodeCloudBuildRunsList(ctx, d, *workflowID, *sort, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func newXcodeCloudBuildRunsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("list", flag.ExitOnError)

	workflowID, sort, limit, next, paginate, output, pretty := xcodeCloudBuildRunsListFlags(d, fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "xcode-cloud build-runs list", 200, "sort"); err != nil {
				return err
			}
			return xcodeCloudBuildRunsList(ctx, d, *workflowID, *sort, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func newXcodeCloudBuildRunsGetCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud build-runs view --id \"BUILD_RUN_ID\"",
		ShortHelp:  "View details for a build run.",
		LongHelp: `View details for a build run.

Examples:
  asc xcode-cloud build-runs view --id "BUILD_RUN_ID"
  asc xcode-cloud build-runs view --id "BUILD_RUN_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Build run ID",
		IDType:           "ciBuildRuns",
		FetchErrorPrefix: "xcode-cloud build-runs view",
		Deps:             d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiBuildRun(ctx, id)
		},
	})
}

func newXcodeCloudBuildRunsBuildsCommand(d *shared.Deps) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:       "builds",
		ShortUsage: "asc xcode-cloud build-runs builds [flags]",
		ShortHelp:  "List builds for a build run.",
		LongHelp: `List builds for a build run.

Examples:
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID"
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID" --output table
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID" --limit 50
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID" --paginate`,
		ParentFlag:           "run-id",
		ParentUsage:          "Build run ID to list builds for",
		ParentType:           "ciBuildRuns",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		FetchErrorPrefix:     "xcode-cloud build-runs builds",
		Deps:                 d,
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(d, ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, runID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildRunBuilds(ctx, runID, asc.Limit[asc.CiBuildRunBuildsOption](limit), asc.NextURL[asc.CiBuildRunBuildsOption](next))
		},
	})
}

func xcodeCloudBuildRunsList(ctx context.Context, d *shared.Deps, workflowID string, sort string, limit int, next string, paginate bool, output string, pretty bool) error {
	const workflowIDHint = `Find workflow IDs with: asc xcode-cloud workflows list --app "APP_ID"`
	if err := requireXcodeCloudParent(d, workflowID, "workflow-id", workflowIDHint, next); err != nil {
		return err
	}
	if err := shared.ValidateSort(sort, "number", "-number"); err != nil {
		return d.UsageError(err.Error())
	}

	return runXcodeCloudPaginatedParentList(
		ctx,
		d,
		workflowID,
		"workflow-id",
		workflowIDHint,
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
				asc.Limit[asc.CiBuildRunsOption](limit),
				asc.NextURL[asc.CiBuildRunsOption](next),
			)
		},
		func(ctx context.Context, client *asc.Client, workflowID string, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildRuns(ctx, workflowID, asc.NextURL[asc.CiBuildRunsOption](next))
		},
	)
}
