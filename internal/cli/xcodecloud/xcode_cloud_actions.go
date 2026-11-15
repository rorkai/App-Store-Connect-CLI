package xcodecloud

import (
	"context"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// XcodeCloudActionsCommand returns the xcode-cloud actions subcommand.
func XcodeCloudActionsCommand() *ffcli.Command {
	cmd := xcodeCloudActionsListCommand(
		"actions",
		"asc xcode-cloud actions [flags]",
		"Manage build actions for an Xcode Cloud build run.",
		`Manage build actions for an Xcode Cloud build run.

Build actions show the individual steps of a build run (e.g., "Resolve Dependencies",
"Archive", "Upload") and their status, which helps diagnose why builds failed.

Examples:
  asc xcode-cloud actions --run-id "BUILD_RUN_ID"
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID"
  asc xcode-cloud actions view --id "ACTION_ID"
  asc xcode-cloud actions build-run --id "ACTION_ID"
  asc xcode-cloud actions --run-id "BUILD_RUN_ID" --output table
  asc xcode-cloud actions --run-id "BUILD_RUN_ID" --limit 50
  asc xcode-cloud actions --run-id "BUILD_RUN_ID" --paginate`,
	)
	cmd.Subcommands = []*ffcli.Command{
		XcodeCloudActionsListCommand(),
		XcodeCloudActionsGetCommand(),
		XcodeCloudActionsBuildRunCommand(),
	}
	return cmd
}

func XcodeCloudActionsListCommand() *ffcli.Command {
	return xcodeCloudActionsListCommand(
		"list",
		"asc xcode-cloud actions list [flags]",
		"List build actions for an Xcode Cloud build run.",
		`List build actions for an Xcode Cloud build run.

Examples:
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID"
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID" --output table
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID" --limit 50
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID" --paginate`,
	)
}

func xcodeCloudActionsListCommand(name, shortUsage, shortHelp, longHelp string) *ffcli.Command {
	return shared.BuildPaginatedListCommand(shared.PaginatedListCommandConfig{
		Name:                 name,
		ShortUsage:           shortUsage,
		ShortHelp:            shortHelp,
		LongHelp:             longHelp,
		ParentFlag:           "run-id",
		ParentUsage:          "Build run ID to get actions for (required)",
		ParentType:           "ciBuildRuns",
		RequiredDiagnostic:   true,
		PaginateIgnoresLimit: true,
		ErrorPrefix:          "xcode-cloud actions",
		FetchErrorPrefix:     "xcode-cloud actions",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		FetchPage: func(ctx context.Context, client *asc.Client, runID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildActions(ctx, runID, asc.Limit[asc.CiBuildActionsOption](limit), asc.NextURL[asc.CiBuildActionsOption](next))
		},
	})
}

func XcodeCloudActionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "view",
		ShortUsage: "asc xcode-cloud actions view --id \"ACTION_ID\"",
		ShortHelp:  "View details for a build action.",
		LongHelp: `View details for a build action.

Examples:
  asc xcode-cloud actions view --id "ACTION_ID"
  asc xcode-cloud actions view --id "ACTION_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Build action ID",
		IDType:           "ciBuildActions",
		FetchErrorPrefix: "xcode-cloud actions view",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiBuildAction(ctx, id)
		},
	})
}

func XcodeCloudActionsBuildRunCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		Name:       "build-run",
		ShortUsage: "asc xcode-cloud actions build-run --id \"ACTION_ID\"",
		ShortHelp:  "Get the build run for a build action.",
		LongHelp: `Get the build run for a build action.

Examples:
  asc xcode-cloud actions build-run --id "ACTION_ID"
  asc xcode-cloud actions build-run --id "ACTION_ID" --output table`,
		IDFlag:           "id",
		IDUsage:          "Build action ID",
		IDType:           "ciBuildActions",
		FetchErrorPrefix: "xcode-cloud actions build-run",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiBuildActionBuildRun(ctx, id)
		},
	})
}
