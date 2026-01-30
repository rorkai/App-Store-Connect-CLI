package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// BuildsIndividualTestersCommand returns the individual-testers command group.
func BuildsIndividualTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "individual-testers",
		ShortUsage: "asc builds individual-testers <subcommand> [flags]",
		ShortHelp:  "Manage individual testers for a build.",
		LongHelp: `Manage individual testers for a build.

Examples:
  asc builds individual-testers list --build "BUILD_ID"
  asc builds individual-testers add --build "BUILD_ID" --tester "TESTER_ID"
  asc builds individual-testers remove --build "BUILD_ID" --tester "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsIndividualTestersListCommand(),
			BuildsIndividualTestersAddCommand(),
			BuildsIndividualTestersRemoveCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsIndividualTestersListCommand returns the individual-testers list subcommand.
func BuildsIndividualTestersListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers list", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds individual-testers list [flags]",
		ShortHelp:  "List individual testers assigned to a build.",
		LongHelp: `List individual testers assigned to a build.

Examples:
  asc builds individual-testers list --build "BUILD_ID"
  asc builds individual-testers list --build "BUILD_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds individual-testers list: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("builds individual-testers list: %w", err)
			}

			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds individual-testers list: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BuildIndividualTestersOption{
				asc.WithBuildIndividualTestersLimit(*limit),
				asc.WithBuildIndividualTestersNextURL(*next),
			}

			if *paginate {
				if buildValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --build is required")
					return flag.ErrHelp
				}

				paginateOpts := append(opts, asc.WithBuildIndividualTestersLimit(200))
				firstPage, err := client.GetBuildIndividualTesters(requestCtx, buildValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("builds individual-testers list: failed to fetch: %w", err)
				}

				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBuildIndividualTesters(ctx, buildValue, asc.WithBuildIndividualTestersNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("builds individual-testers list: %w", err)
				}

				return printOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBuildIndividualTesters(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("builds individual-testers list: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}

// BuildsIndividualTestersAddCommand returns the individual-testers add subcommand.
func BuildsIndividualTestersAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers add", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	testers := fs.String("tester", "", "Comma-separated tester IDs")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc builds individual-testers add --build \"BUILD_ID\" --tester \"TESTER_ID[,TESTER_ID...]\"",
		ShortHelp:  "Add individual testers to a build.",
		LongHelp: `Add individual testers to a build.

Examples:
  asc builds individual-testers add --build "BUILD_ID" --tester "TESTER_ID"
  asc builds individual-testers add --build "BUILD_ID" --tester "TESTER_ID1,TESTER_ID2"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			testerIDs := parseCommaSeparatedIDs(*testers)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds individual-testers add: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.AddIndividualTestersToBuild(requestCtx, buildValue, testerIDs); err != nil {
				return fmt.Errorf("builds individual-testers add: failed to add testers: %w", err)
			}

			result := &asc.BuildIndividualTestersUpdateResult{
				BuildID:   buildValue,
				TesterIDs: testerIDs,
				Action:    "added",
			}

			return printOutput(result, *output, *pretty)
		},
	}
}

// BuildsIndividualTestersRemoveCommand returns the individual-testers remove subcommand.
func BuildsIndividualTestersRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers remove", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	testers := fs.String("tester", "", "Comma-separated tester IDs")
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "remove",
		ShortUsage: "asc builds individual-testers remove --build \"BUILD_ID\" --tester \"TESTER_ID[,TESTER_ID...]\"",
		ShortHelp:  "Remove individual testers from a build.",
		LongHelp: `Remove individual testers from a build.

Examples:
  asc builds individual-testers remove --build "BUILD_ID" --tester "TESTER_ID"
  asc builds individual-testers remove --build "BUILD_ID" --tester "TESTER_ID1,TESTER_ID2"`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			testerIDs := parseCommaSeparatedIDs(*testers)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds individual-testers remove: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveIndividualTestersFromBuild(requestCtx, buildValue, testerIDs); err != nil {
				return fmt.Errorf("builds individual-testers remove: failed to remove testers: %w", err)
			}

			result := &asc.BuildIndividualTestersUpdateResult{
				BuildID:   buildValue,
				TesterIDs: testerIDs,
				Action:    "removed",
			}

			return printOutput(result, *output, *pretty)
		},
	}
}
