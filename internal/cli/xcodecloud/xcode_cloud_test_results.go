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

// XcodeCloudTestResultsCommand returns the xcode-cloud test-results command with subcommands.
func XcodeCloudTestResultsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("test-results", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "test-results",
		ShortUsage: "asc xcode-cloud test-results <subcommand> [flags]",
		ShortHelp:  "List Xcode Cloud test results.",
		LongHelp: `List Xcode Cloud test results.

Examples:
  asc xcode-cloud test-results list --action-id "ACTION_ID"
  asc xcode-cloud test-results get --id "TEST_RESULT_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeCloudTestResultsListCommand(),
			XcodeCloudTestResultsGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeCloudTestResultsListCommand returns the xcode-cloud test-results list subcommand.
func XcodeCloudTestResultsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("list", flag.ExitOnError)

	actionID := fs.String("action-id", "", "Build action ID to list test results for")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc xcode-cloud test-results list [flags]",
		ShortHelp:  "List test results for a build action.",
		LongHelp: `List test results for a build action.

Examples:
  asc xcode-cloud test-results list --action-id "ACTION_ID"
  asc xcode-cloud test-results list --action-id "ACTION_ID" --output table
  asc xcode-cloud test-results list --action-id "ACTION_ID" --limit 50
  asc xcode-cloud test-results list --action-id "ACTION_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("xcode-cloud test-results list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("xcode-cloud test-results list: %w", err)
			}

			resolvedActionID := strings.TrimSpace(*actionID)
			if resolvedActionID == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --action-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("xcode-cloud test-results list: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			opts := []asc.CiTestResultsOption{
				asc.WithCiTestResultsLimit(*limit),
				asc.WithCiTestResultsNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithCiTestResultsLimit(200))
				firstPage, err := client.GetCiBuildActionTestResults(requestCtx, resolvedActionID, paginateOpts...)
				if err != nil {
					return fmt.Errorf("xcode-cloud test-results list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetCiBuildActionTestResults(ctx, resolvedActionID, asc.WithCiTestResultsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("xcode-cloud test-results list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetCiBuildActionTestResults(requestCtx, resolvedActionID, opts...)
			if err != nil {
				return fmt.Errorf("xcode-cloud test-results list: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// XcodeCloudTestResultsGetCommand returns the xcode-cloud test-results get subcommand.
func XcodeCloudTestResultsGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("get", flag.ExitOnError)

	id := fs.String("id", "", "Test result ID")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc xcode-cloud test-results get --id \"TEST_RESULT_ID\"",
		ShortHelp:  "Get details for a test result.",
		LongHelp: `Get details for a test result.

Examples:
  asc xcode-cloud test-results get --id "TEST_RESULT_ID"
  asc xcode-cloud test-results get --id "TEST_RESULT_ID" --output table`,
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
				return fmt.Errorf("xcode-cloud test-results get: %w", err)
			}

			requestCtx, cancel := contextWithXcodeCloudTimeout(ctx, 0)
			defer cancel()

			resp, err := client.GetCiTestResult(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("xcode-cloud test-results get: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
