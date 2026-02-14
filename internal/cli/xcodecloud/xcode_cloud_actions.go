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
  asc xcode-cloud actions get --id "ACTION_ID"
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
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Build action ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud actions get --id \"ACTION_ID\"",
		ShortHelp:  "Get details for a build action.",
		LongHelp: `Get details for a build action.

Examples:
  asc xcode-cloud actions get --id "ACTION_ID"
  asc xcode-cloud actions get --id "ACTION_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud actions get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiBuildAction(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud actions get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func XcodeCloudActionsBuildRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("build-run", flag.ExitOnError)

	id := fs.String("id", "", "Build action ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "build-run",
		ShortUsage: "asc xcode-cloud actions build-run --id \"ACTION_ID\"",
		ShortHelp:  "Get the build run for a build action.",
		LongHelp: `Get the build run for a build action.

Examples:
  asc xcode-cloud actions build-run --id "ACTION_ID"
  asc xcode-cloud actions build-run --id "ACTION_ID" --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud actions build-run: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiBuildActionBuildRun(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud actions build-run: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

func xcodeCloudActionsList(ctx context.Context, runID string, limit int, next string, paginate bool, output string, pretty bool) error {
	if limit != 0 && (limit < 1 || limit > 200) {
		return fmt.Errorf("xcode-cloud actions: --limit must be between 1 and 200")
	}
	if err := shared.ValidateNextURL(next); err != nil {
		return fmt.Errorf("xcode-cloud actions: %w", err)
	}

	resolvedRunID := strings.TrimSpace(runID)
	if resolvedRunID == "" && strings.TrimSpace(next) == "" {
		fmt.Fprintln(os.Stderr, "Error: --run-id is required")
		return flag.ErrHelp
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return fmt.Errorf("xcode-cloud actions: %w", err)
	}

	requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
	defer cancel()

	opts := []asc.CiBuildActionsOption{
		asc.WithCiBuildActionsLimit(limit),
		asc.WithCiBuildActionsNextURL(next),
	}

	if paginate {
		paginateOpts := append(opts, asc.WithCiBuildActionsLimit(200))
		firstPage, err := client.GetCiBuildActions(requestCtx, resolvedRunID, paginateOpts...)
		if err != nil {
			return fmt.Errorf("xcode-cloud actions: failed to fetch: %w", err)
		}

		resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetCiBuildActions(ctx, resolvedRunID, asc.WithCiBuildActionsNextURL(nextURL))
		})
		if err != nil {
			return fmt.Errorf("xcode-cloud actions: %w", err)
		}

		return shared.PrintOutput(resp, output, pretty)
	}

	resp, err := client.GetCiBuildActions(requestCtx, resolvedRunID, opts...)
	if err != nil {
		return fmt.Errorf("xcode-cloud actions: %w", err)
	}

	return shared.PrintOutput(resp, output, pretty)
}
