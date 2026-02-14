package xcodecloud

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// XcodeCloudCommand returns the xcode-cloud command with subcommands.
func XcodeCloudCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode-cloud", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "xcode-cloud",
		ShortUsage: "asc xcode-cloud <subcommand> [flags]",
		ShortHelp:  "Trigger and monitor Xcode Cloud workflows.",
		LongHelp: `Trigger and monitor Xcode Cloud workflows.

Examples:
  asc xcode-cloud workflows --app "APP_ID"
  asc xcode-cloud build-runs --workflow-id "WORKFLOW_ID"
  asc xcode-cloud actions --run-id "BUILD_RUN_ID"
  asc xcode-cloud scm providers list
  asc xcode-cloud run --app "APP_ID" --workflow "WorkflowName" --branch "main"
  asc xcode-cloud run --workflow-id "WORKFLOW_ID" --git-reference-id "REF_ID"
  asc xcode-cloud run --app "APP_ID" --workflow "Deploy" --branch "main" --wait
  asc xcode-cloud status --run-id "BUILD_RUN_ID"
  asc xcode-cloud status --run-id "BUILD_RUN_ID" --wait`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudRunCommand(),
			XcodeCloudStatusCommand(),
			XcodeCloudProductsCommand(),
			XcodeCloudWorkflowsCommand(),
			XcodeCloudScmCommand(),
			XcodeCloudBuildRunsCommand(),
			XcodeCloudActionsCommand(),
			XcodeCloudArtifactsCommand(),
			XcodeCloudTestResultsCommand(),
			XcodeCloudIssuesCommand(),
			XcodeCloudMacOSVersionsCommand(),
			XcodeCloudXcodeVersionsCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeCloudRunCommand returns the xcode-cloud run subcommand.
func XcodeCloudRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("run", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	workflowName := fs.String("workflow", "", "Workflow name to trigger")
	workflowID := fs.String("workflow-id", "", "Workflow ID to trigger (alternative to --workflow)")
	branch := fs.String("branch", "", "Branch or tag name to build")
	gitReferenceID := fs.String("git-reference-id", "", "Git reference ID to build (alternative to --branch)")
	wait := fs.Bool("wait", false, "Wait for build to complete")
	pollInterval := fs.Duration("poll-interval", 10*time.Second, "Poll interval when waiting")
	timeout := fs.Duration("timeout", 0, "Timeout for Xcode Cloud requests (0 = use ASC_TIMEOUT or 30m default)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc xcode-cloud run [flags]",
		ShortHelp:  "Trigger an Xcode Cloud workflow build.",
		LongHelp: `Trigger an Xcode Cloud workflow build.

You can specify the workflow by name (requires --app) or by ID (--workflow-id).
You can specify the branch/tag by name (--branch) or by ID (--git-reference-id).

Examples:
  asc xcode-cloud run --app "123456789" --workflow "CI" --branch "main"
  asc xcode-cloud run --workflow-id "WORKFLOW_ID" --git-reference-id "REF_ID"
  asc xcode-cloud run --app "123456789" --workflow "Deploy" --branch "release/1.0" --wait
  asc xcode-cloud run --app "123456789" --workflow "CI" --branch "main" --wait --poll-interval 30s --timeout 1h`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			// Validate input combinations
			hasWorkflowName := strings.TrimSpace(*workflowName) != ""
			hasWorkflowID := strings.TrimSpace(*workflowID) != ""
			hasBranch := strings.TrimSpace(*branch) != ""
			hasGitRefID := strings.TrimSpace(*gitReferenceID) != ""

			if hasWorkflowName && hasWorkflowID {
				return shared.UsageError("--workflow and --workflow-id are mutually exclusive")
			}
			if !hasWorkflowName && !hasWorkflowID {
				fmt.Fprintln(os.Stderr, "Error: --workflow or --workflow-id is required")
				return flag.ErrHelp
			}
			if hasBranch && hasGitRefID {
				return shared.UsageError("--branch and --git-reference-id are mutually exclusive")
			}
			if !hasBranch && !hasGitRefID {
				fmt.Fprintln(os.Stderr, "Error: --branch or --git-reference-id is required")
				return flag.ErrHelp
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than or equal to 0")
			}
			if *wait && *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if hasWorkflowName && resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required when using --workflow (or set ASC_APP_ID)")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud run: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, *timeout)
			defer cancel()

			// Resolve workflow ID
			resolvedWorkflowID := strings.TrimSpace(*workflowID)
			var workflowNameForOutput string
			if resolvedWorkflowID == "" {
				// Need to resolve workflow by name
				product, err := client.ResolveCiProductForApp(requestCtx, resolvedAppID)
				if err != nil {
					return fmt.Errorf("xcode-cloud run: %w", err)
				}

				workflow, err := client.ResolveCiWorkflowByName(requestCtx, product.ID, strings.TrimSpace(*workflowName))
				if err != nil {
					return fmt.Errorf("xcode-cloud run: %w", err)
				}

				resolvedWorkflowID = workflow.ID
				workflowNameForOutput = workflow.Attributes.Name
			}

			// Resolve git reference ID
			resolvedGitRefID := strings.TrimSpace(*gitReferenceID)
			var refNameForOutput string
			if resolvedGitRefID == "" {
				// Need to resolve git reference by name
				// First get the repository from the workflow
				repo, err := client.GetCiWorkflowRepository(requestCtx, resolvedWorkflowID)
				if err != nil {
					return fmt.Errorf("xcode-cloud run: failed to get workflow repository: %w", err)
				}

				gitRef, err := client.ResolveGitReferenceByName(requestCtx, repo.ID, strings.TrimSpace(*branch))
				if err != nil {
					return fmt.Errorf("xcode-cloud run: %w", err)
				}

				resolvedGitRefID = gitRef.ID
				refNameForOutput = gitRef.Attributes.Name
			}

			// Create the build run
			req := asc.CiBuildRunCreateRequest{
				Data: asc.CiBuildRunCreateData{
					Type: asc.ResourceTypeCiBuildRuns,
					Relationships: &asc.CiBuildRunCreateRelationships{
						Workflow: &asc.Relationship{
							Data: asc.ResourceData{Type: asc.ResourceTypeCiWorkflows, ID: resolvedWorkflowID},
						},
						SourceBranchOrTag: &asc.Relationship{
							Data: asc.ResourceData{Type: asc.ResourceTypeScmGitReferences, ID: resolvedGitRefID},
						},
					},
				},
			}

			resp, err := client.CreateCiBuildRun(requestCtx, req)
			if err != nil {
				return fmt.Errorf("xcode-cloud run: failed to trigger build: %w", err)
			}

			result := &asc.XcodeCloudRunResult{
				BuildRunID:        resp.Data.ID,
				BuildNumber:       resp.Data.Attributes.Number,
				WorkflowID:        resolvedWorkflowID,
				WorkflowName:      workflowNameForOutput,
				GitReferenceID:    resolvedGitRefID,
				GitReferenceName:  refNameForOutput,
				ExecutionProgress: string(resp.Data.Attributes.ExecutionProgress),
				CompletionStatus:  string(resp.Data.Attributes.CompletionStatus),
				StartReason:       resp.Data.Attributes.StartReason,
				CreatedDate:       resp.Data.Attributes.CreatedDate,
				StartedDate:       resp.Data.Attributes.StartedDate,
				FinishedDate:      resp.Data.Attributes.FinishedDate,
			}

			if !*wait {
				return shared.PrintOutput(result, *output, *pretty)
			}

			// Wait for completion
			return waitForBuildCompletion(requestCtx, client, resp.Data.ID, *pollInterval, *output, *pretty)
		},
	}
}

// XcodeCloudStatusCommand returns the xcode-cloud status subcommand.
func XcodeCloudStatusCommand() *ffcli.Command {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	runID := fs.String("run-id", "", "Build run ID to check")
	wait := fs.Bool("wait", false, "Wait for build to complete")
	pollInterval := fs.Duration("poll-interval", 10*time.Second, "Poll interval when waiting")
	timeout := fs.Duration("timeout", 0, "Timeout for Xcode Cloud requests (0 = use ASC_TIMEOUT or 30m default)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "status",
		ShortUsage: "asc xcode-cloud status [flags]",
		ShortHelp:  "Check the status of an Xcode Cloud build run.",
		LongHelp: `Check the status of an Xcode Cloud build run.

Examples:
  asc xcode-cloud status --run-id "BUILD_RUN_ID"
  asc xcode-cloud status --run-id "BUILD_RUN_ID" --output table
  asc xcode-cloud status --run-id "BUILD_RUN_ID" --wait
  asc xcode-cloud status --run-id "BUILD_RUN_ID" --wait --poll-interval 30s --timeout 1h`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*runID) == "" {
				fmt.Fprintln(os.Stderr, "Error: --run-id is required")
				return flag.ErrHelp
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than or equal to 0")
			}
			if *wait && *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud status: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, *timeout)
			defer cancel()

			if *wait {
				return waitForBuildCompletion(requestCtx, client, strings.TrimSpace(*runID), *pollInterval, *output, *pretty)
			}

			// Single status check
			resp, err := getCiBuildRunWithRetry(requestCtx, client, strings.TrimSpace(*runID))
			if err != nil {
				return fmt.Errorf("xcode-cloud status: %w", err)
			}

			result := buildStatusResult(resp)
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
