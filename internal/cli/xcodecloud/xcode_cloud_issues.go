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

// XcodeCloudIssuesCommand returns the xcode-cloud issues command with subcommands.
func XcodeCloudIssuesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("issues", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "issues",
		ShortUsage: "asc xcode-cloud issues <subcommand> [flags]",
		ShortHelp:  "List Xcode Cloud build issues.",
		LongHelp: `List Xcode Cloud build issues.

Examples:
  asc xcode-cloud issues list --action-id "ACTION_ID"
  asc xcode-cloud issues get --id "ISSUE_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudIssuesListCommand(),
			XcodeCloudIssuesGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeCloudIssuesListCommand returns the xcode-cloud issues list subcommand.
func XcodeCloudIssuesListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	actionID := fs.String("action-id", "", "Build action ID to list issues for")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud issues list [flags]",
		ShortHelp:  "List issues for a build action.",
		LongHelp: `List issues for a build action.

Examples:
  asc xcode-cloud issues list --action-id "ACTION_ID"
  asc xcode-cloud issues list --action-id "ACTION_ID" --output table
  asc xcode-cloud issues list --action-id "ACTION_ID" --limit 50
  asc xcode-cloud issues list --action-id "ACTION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud issues list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud issues list: %w", err)
			}

			resolvedActionID := strings.TrimSpace(*actionID)
			if resolvedActionID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --action-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud issues list: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiIssuesOption{
				asc.WithCiIssuesLimit(*limit),
				asc.WithCiIssuesNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiIssuesLimit(200))
				firstPage, err := client.GetCiBuildActionIssues(requestCtx, resolvedActionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud issues list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiBuildActionIssues(ctx, resolvedActionID, asc.WithCiIssuesNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud issues list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiBuildActionIssues(requestCtx, resolvedActionID, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud issues list: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// XcodeCloudIssuesGetCommand returns the xcode-cloud issues get subcommand.
func XcodeCloudIssuesGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Issue ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud issues get --id \"ISSUE_ID\"",
		ShortHelp:  "Get details for a build issue.",
		LongHelp: `Get details for a build issue.

Examples:
  asc xcode-cloud issues get --id "ISSUE_ID"
  asc xcode-cloud issues get --id "ISSUE_ID" --output table`,
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
				return fmt.Errorf("xcode-cloud issues get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiIssue(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud issues get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
