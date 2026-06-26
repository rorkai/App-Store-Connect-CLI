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

const xcodeCloudAppFlagUsage = "App Store Connect app ID, bundle ID, or exact app name (or ASC_APP_ID env)"

func resolveXcodeCloudAppID(ctx context.Context, client *asc.Client, appID string) (string, error) {
	resolvedAppID := shared.ResolveAppID(appID)
	if resolvedAppID == "" {
		return "", nil
	}

	return shared.ResolveAppIDWithExactLookup(ctx, client, resolvedAppID)
}

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
  asc xcode-cloud run --workflow-id "WORKFLOW_ID" --pull-request-id "PR_ID"
  asc xcode-cloud run --source-run-id "BUILD_RUN_ID" --clean
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

	appID := fs.String("app", "", xcodeCloudAppFlagUsage)
	workflowName := fs.String("workflow", "", "Workflow name to trigger")
	workflowID := fs.String("workflow-id", "", "Workflow ID to trigger (alternative to --workflow)")
	branch := fs.String("branch", "", "Branch or tag name to build")
	gitReferenceID := fs.String("git-reference-id", "", "Git reference ID to build (alternative to --branch)")
	pullRequestID := fs.String("pull-request-id", "", "Pull request ID to build")
	sourceRunID := fs.String("source-run-id", "", "Source build run ID to rerun")
	clean := fs.Bool("clean", false, "Request a clean build")
	wait := fs.Bool("wait", false, "Wait for build to complete")
	pollInterval := fs.Duration("poll-interval", 10*time.Second, "Poll interval when waiting")
	timeout := fs.Duration("timeout", 0, "Timeout for Xcode Cloud requests (0 = use ASC_TIMEOUT or 30m default)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc xcode-cloud run [flags]",
		ShortHelp:  "Trigger an Xcode Cloud workflow build.",
		LongHelp: `Trigger an Xcode Cloud workflow build.

Standard mode:
- Specify workflow by name (requires --app) or by ID (--workflow-id)
- Specify source by branch/tag (--branch or --git-reference-id) or pull request (--pull-request-id)

Rerun mode:
- Use --source-run-id to rerun from an existing build run (without workflow/source selectors)

Examples:
  asc xcode-cloud run --app "123456789" --workflow "CI" --branch "main"
  asc xcode-cloud run --workflow-id "WORKFLOW_ID" --git-reference-id "REF_ID"
  asc xcode-cloud run --workflow-id "WORKFLOW_ID" --pull-request-id "PR_ID"
  asc xcode-cloud run --source-run-id "BUILD_RUN_ID" --clean
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
			hasPullRequestID := strings.TrimSpace(*pullRequestID) != ""
			hasSourceRunID := strings.TrimSpace(*sourceRunID) != ""

			if hasWorkflowName && hasWorkflowID {
				return shared.UsageError("--workflow and --workflow-id are mutually exclusive")
			}
			if hasBranch && hasGitRefID {
				return shared.UsageError("--branch and --git-reference-id are mutually exclusive")
			}
			if (hasBranch || hasGitRefID) && hasPullRequestID {
				return shared.UsageError("--branch, --git-reference-id, and --pull-request-id are mutually exclusive")
			}
			if hasSourceRunID {
				if hasWorkflowName || hasWorkflowID {
					return shared.UsageError("--source-run-id is mutually exclusive with --workflow and --workflow-id")
				}
				if hasBranch || hasGitRefID || hasPullRequestID {
					return shared.UsageError("--source-run-id is mutually exclusive with --branch, --git-reference-id, and --pull-request-id")
				}
			} else {
				if !hasWorkflowName && !hasWorkflowID {
					fmt.Fprintln(os.Stderr, "Error: --workflow or --workflow-id is required")
					return shared.MissingRequiredUsageError()
				}
				if !hasBranch && !hasGitRefID && !hasPullRequestID {
					fmt.Fprintln(os.Stderr, "Error: --branch, --git-reference-id, or --pull-request-id is required")
					return shared.MissingRequiredUsageError()
				}
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be greater than or equal to 0")
			}
			if *wait && *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}

			resolvedAppID := shared.ResolveAppID(*appID)
			if hasWorkflowName && !hasSourceRunID && resolvedAppID == "" {
				fmt.Fprintln(os.Stderr, "Error: --app is required when using --workflow (or set ASC_APP_ID)")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud run: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, *timeout)
			defer cancel()

			resolvedWorkflowID := strings.TrimSpace(*workflowID)
			var workflowNameForOutput string
			resolvedGitRefID := strings.TrimSpace(*gitReferenceID)
			resolvedPullRequestID := strings.TrimSpace(*pullRequestID)
			resolvedSourceRunID := strings.TrimSpace(*sourceRunID)
			var refNameForOutput string
			triggerSource := ""

			if !hasSourceRunID {
				// Resolve workflow ID if needed.
				if resolvedWorkflowID == "" {
					resolvedAppID, err = resolveXcodeCloudAppID(requestCtx, client, resolvedAppID)
					if err != nil {
						return fmt.Errorf("xcode-cloud run: %w", err)
					}

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

				if resolvedPullRequestID != "" {
					triggerSource = "pull-request"
				} else {
					// Resolve git reference by name if needed.
					if resolvedGitRefID == "" {
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
						triggerSource = "branch"
					} else {
						triggerSource = "git-reference"
					}
				}
			} else {
				sourceRunResp, err := getCiBuildRun(
					requestCtx,
					client,
					resolvedSourceRunID,
					asc.WithCiBuildRunInclude("workflow"),
					asc.WithCiBuildRunFields("workflow"),
				)
				if err != nil {
					return fmt.Errorf("xcode-cloud run: failed to resolve source run workflow: %w", err)
				}
				if sourceRunResp.Data.Relationships == nil || sourceRunResp.Data.Relationships.Workflow == nil || strings.TrimSpace(sourceRunResp.Data.Relationships.Workflow.Data.ID) == "" {
					return fmt.Errorf("xcode-cloud run: source run %q does not contain a workflow relationship", resolvedSourceRunID)
				}
				resolvedWorkflowID = sourceRunResp.Data.Relationships.Workflow.Data.ID
				triggerSource = "source-run"
			}

			relationships := &asc.CiBuildRunCreateRelationships{}
			switch {
			case hasSourceRunID:
				relationships.Workflow = &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeCiWorkflows, ID: resolvedWorkflowID},
				}
				relationships.BuildRun = &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeCiBuildRuns, ID: resolvedSourceRunID},
				}
			case resolvedPullRequestID != "":
				relationships.Workflow = &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeCiWorkflows, ID: resolvedWorkflowID},
				}
				relationships.PullRequest = &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeScmPullRequests, ID: resolvedPullRequestID},
				}
			default:
				relationships.Workflow = &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeCiWorkflows, ID: resolvedWorkflowID},
				}
				relationships.SourceBranchOrTag = &asc.Relationship{
					Data: asc.ResourceData{Type: asc.ResourceTypeScmGitReferences, ID: resolvedGitRefID},
				}
			}

			req := asc.CiBuildRunCreateRequest{
				Data: asc.CiBuildRunCreateData{
					Type:          asc.ResourceTypeCiBuildRuns,
					Relationships: relationships,
				},
			}
			if *clean {
				cleanValue := true
				req.Data.Attributes = &asc.CiBuildRunCreateAttributes{Clean: &cleanValue}
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
				TriggerSource:     triggerSource,
				GitReferenceID:    resolvedGitRefID,
				GitReferenceName:  refNameForOutput,
				PullRequestID:     resolvedPullRequestID,
				SourceRunID:       resolvedSourceRunID,
				Clean:             *clean,
				ExecutionProgress: string(resp.Data.Attributes.ExecutionProgress),
				CompletionStatus:  string(resp.Data.Attributes.CompletionStatus),
				StartReason:       resp.Data.Attributes.StartReason,
				CreatedDate:       resp.Data.Attributes.CreatedDate,
				StartedDate:       resp.Data.Attributes.StartedDate,
				FinishedDate:      resp.Data.Attributes.FinishedDate,
			}
			if resp.Data.Relationships != nil {
				if result.WorkflowID == "" && resp.Data.Relationships.Workflow != nil {
					result.WorkflowID = resp.Data.Relationships.Workflow.Data.ID
				}
				if result.GitReferenceID == "" && resp.Data.Relationships.SourceBranchOrTag != nil {
					result.GitReferenceID = resp.Data.Relationships.SourceBranchOrTag.Data.ID
				}
				if result.PullRequestID == "" && resp.Data.Relationships.PullRequest != nil {
					result.PullRequestID = resp.Data.Relationships.PullRequest.Data.ID
				}
			}

			if !*wait {
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			// Wait for completion
			return waitForBuildCompletion(requestCtx, client, resp.Data.ID, *pollInterval, *output.Output, *output.Pretty)
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
	output := shared.BindOutputFlags(fs)

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
				return shared.MissingRequiredUsageError()
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
				return waitForBuildCompletion(requestCtx, client, strings.TrimSpace(*runID), *pollInterval, *output.Output, *output.Pretty)
			}

			// Single status check
			resp, err := getCiBuildRun(requestCtx, client, strings.TrimSpace(*runID))
			if err != nil {
				return fmt.Errorf("xcode-cloud status: %w", err)
			}

			result := buildStatusResult(resp)
			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
