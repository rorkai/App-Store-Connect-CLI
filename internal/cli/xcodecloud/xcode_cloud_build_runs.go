package xcodecloud

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

func xcodeCloudBuildRunsListFlags(fs *flag.FlagSet) (workflowID *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	workflowID = fs.String("workflow-id", "", "Workflow ID to list build runs for")
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

	workflowID, limit, next, paginate, output, pretty := xcodeCloudBuildRunsListFlags(fs)

	return &ffcli.Command{
		Name:       "build-runs",
		ShortUsage: "asc xcode-cloud build-runs [flags]",
		ShortHelp:  "Manage Xcode Cloud build runs.",
		LongHelp: `Manage Xcode Cloud build runs.

Examples:
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID"
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID"
  asc xcode-cloud build-runs builds --run-id "BUILD_RUN_ID"
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --limit 50
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudBuildRunsListCommand(),
			XcodeCloudBuildRunsBuildsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudBuildRunsList(ctx, *workflowID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudBuildRunsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	workflowID, limit, next, paginate, output, pretty := xcodeCloudBuildRunsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud build-runs list [flags]",
		ShortHelp:  "List Xcode Cloud build runs for a workflow.",
		LongHelp: `List Xcode Cloud build runs for a workflow.

Examples:
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID"
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID" --limit 50
  asc xcode-cloud build-runs list --workflow-id "WORKFLOW_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudBuildRunsList(ctx, *workflowID, *limit, *next, *paginate, *output, *pretty)
		},
	}
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
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud build-runs builds: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud build-runs builds: %w", err)
			}

			runIDValue := strings.TrimSpace(*runID)
			if runIDValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --run-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud build-runs builds: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiBuildRunBuildsOption{
				asc.WithCiBuildRunBuildsLimit(*limit),
				asc.WithCiBuildRunBuildsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiBuildRunBuildsLimit(200))
				firstPage, err := client.GetCiBuildRunBuilds(requestCtx, runIDValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud build-runs builds: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiBuildRunBuilds(ctx, runIDValue, asc.WithCiBuildRunBuildsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud build-runs builds: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiBuildRunBuilds(requestCtx, runIDValue, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud build-runs builds: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func xcodeCloudBuildRunsList(ctx context.Context, workflowID string, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud build-runs: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("xcode-cloud build-runs: %w", err)
	}

	resolvedWorkflowID := strings.TrimSpace(workflowID)
	if resolvedWorkflowID == "" && strings.TrimSpace(next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --workflow-id is required")
		return flag.ErrHelp
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud build-runs: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	opts := []asc.CiBuildRunsOption{
		asc.WithCiBuildRunsLimit(limit),
		asc.WithCiBuildRunsNextURL(next),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithCiBuildRunsLimit(200))
		firstPage, err := client.GetCiBuildRuns(requestCtx, resolvedWorkflowID, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud build-runs: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildRuns(ctx, resolvedWorkflowID, asc.WithCiBuildRunsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("xcode-cloud build-runs: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetCiBuildRuns(requestCtx, resolvedWorkflowID, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud build-runs: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}
