package xcodecloud

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func xcodeCloudActionsListFlags(fs *flag.FlagSet) (runID *string, limit *int, next *string, paginate *bool, output *string, pretty *bool) {
	runID = fs.String("run-id", "", "Build run ID to get actions for (required)")
	limit = fs.Int("limit", 0, "Maximum results per page (1-200)")
	next = fs.String("next", "", "Fetch next page using a links.next URL")
	paginate = fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	outputFlags := shared.BindOutputFlags(fs)
	output = outputFlags.Output
	pretty = outputFlags.Pretty
	return
}

// XcodeCloudActionsCommand returns the xcode-cloud actions subcommand.
func XcodeCloudActionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("actions", flag.ExitOnError)

	runID, limit, next, paginate, output, pretty := xcodeCloudActionsListFlags(fs)

	return &ffcli.Command{
		Name:       "actions",
		ShortUsage: "asc xcode-cloud actions [flags]",
		ShortHelp:  "Manage build actions for an Xcode Cloud build run.",
		LongHelp: `Manage build actions for an Xcode Cloud build run.

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
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudActionsListCommand(),
			XcodeCloudActionsGetCommand(),
			XcodeCloudActionsBuildRunCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudActionsList(ctx, *runID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudActionsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	runID, limit, next, paginate, output, pretty := xcodeCloudActionsListFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud actions list [flags]",
		ShortHelp:  "List build actions for an Xcode Cloud build run.",
		LongHelp: `List build actions for an Xcode Cloud build run.

Examples:
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID"
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID" --output table
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID" --limit 50
  asc xcode-cloud actions list --run-id "BUILD_RUN_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			return xcodeCloudActionsList(ctx, *runID, *limit, *next, *paginate, *output, *pretty)
		},
	}
}

func XcodeCloudActionsGetCommand() *ffcli.Command {
	return shared.BuildIDGetCommand(shared.IDGetCommandConfig{
		FlagSetName: "view",
		Name:        "view",
		ShortUsage:  "asc xcode-cloud actions view --id \"ACTION_ID\"",
		ShortHelp:   "View details for a build action.",
		LongHelp: `View details for a build action.

Examples:
  asc xcode-cloud actions view --id "ACTION_ID"
  asc xcode-cloud actions view --id "ACTION_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Build action ID",
		ErrorPrefix: "xcode-cloud actions view",
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
		FlagSetName: "build-run",
		Name:        "build-run",
		ShortUsage:  "asc xcode-cloud actions build-run --id \"ACTION_ID\"",
		ShortHelp:   "Get the build run for a build action.",
		LongHelp: `Get the build run for a build action.

Examples:
  asc xcode-cloud actions build-run --id "ACTION_ID"
  asc xcode-cloud actions build-run --id "ACTION_ID" --output table`,
		IDFlag:      "id",
		IDUsage:     "Build action ID",
		ErrorPrefix: "xcode-cloud actions build-run",
		ContextTimeout: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return contextWithXcodeCloudTimeout(ctx, 0)
		},
		Fetch: func(ctx context.Context, client *asc.Client, id string) (any, error) {
			return client.GetCiBuildActionBuildRun(ctx, id)
		},
	})
}

func xcodeCloudActionsList(ctx context.Context, runID string, limit int, next string, paginate bool, output string, pretty bool) error {
	return runXcodeCloudPaginatedParentList(
		ctx,
		runID,
		"run-id",
		limit,
		next,
		paginate,
		output,
		pretty,
		"xcode-cloud actions",
		func(ctx context.Context, client *asc.Client, runID string, limit int, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildActions(
				ctx,
				runID,
				asc.WithCiBuildActionsLimit(limit),
				asc.WithCiBuildActionsNextURL(next),
			)
		},
		func(ctx context.Context, client *asc.Client, runID string, next string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildActions(ctx, runID, asc.WithCiBuildActionsNextURL(next))
		},
	)
}
