package builds

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

// BuildsIndividualTestersCommand returns the individual-testers command group.
func BuildsIndividualTestersCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "individual-testers",
		ShortUsage: "asc builds individual-testers <subcommand> [flags]",
		ShortHelp:  "Manage individual testers for a build.",
		LongHelp: `Manage individual testers for a build.

Examples:
  asc builds individual-testers list --build-id "BUILD_ID"
  asc builds individual-testers list --app "123456789" --latest
  asc builds individual-testers add --build-id "BUILD_ID" --tester "TESTER_ID"
  asc builds individual-testers add --app "123456789" --latest --tester "TESTER_ID"
  asc builds individual-testers remove --build-id "BUILD_ID" --tester "TESTER_ID" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
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

	selectors := bindBuildSelectorFlags(fs, buildSelectorFlagOptions{})
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc builds individual-testers list (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER [--version VERSION] [--platform PLATFORM]) [flags]",
		ShortHelp:  "List individual testers assigned to a build.",
		LongHelp: `List individual testers assigned to a build.

Examples:
  asc builds individual-testers list --build-id "BUILD_ID"
  asc builds individual-testers list --app "123456789" --latest
  asc builds individual-testers list --app "123456789" --latest --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds individual-testers list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("builds individual-testers list: %w", err)
			}

			nextValue := strings.TrimSpace(*next)
			if nextValue == "" {
				if err := selectors.validate(); err != nil {
					return err
				}
			} else if err := selectors.validateNextPageSelectorFlags(); err != nil {
				return fmt.Errorf("builds individual-testers list: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds individual-testers list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID := ""
			if nextValue == "" {
				buildID, err = selectors.resolveBuildID(requestCtx, client)
				if err != nil {
					return fmt.Errorf("builds individual-testers list: %w", err)
				}
			}

			opts := []asc.BuildIndividualTestersOption{
				asc.WithBuildIndividualTestersLimit(*limit),
				asc.WithBuildIndividualTestersNextURL(*next),
			}

			if *paginate {
				paginateOpts := append(opts, asc.WithBuildIndividualTestersLimit(200))
				resp, err := shared.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBuildIndividualTesters(ctx, buildID, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBuildIndividualTesters(ctx, buildID, asc.WithBuildIndividualTestersNextURL(nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("builds individual-testers list: %w", err)
				}

				return shared.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBuildIndividualTesters(requestCtx, buildID, opts...)
			if err != nil {
				return fmt.Errorf("builds individual-testers list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BuildsIndividualTestersAddCommand returns the individual-testers add subcommand.
func BuildsIndividualTestersAddCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers add", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(fs, buildSelectorFlagOptions{})
	testers := fs.String("tester", "", "Comma-separated tester IDs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add",
		ShortUsage: "asc builds individual-testers add (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER [--version VERSION] [--platform PLATFORM]) --tester \"TESTER_ID[,TESTER_ID...]\"",
		ShortHelp:  "Add individual testers to a build.",
		LongHelp: `Add individual testers to a build.

Examples:
  asc builds individual-testers add --build-id "BUILD_ID" --tester "TESTER_ID"
  asc builds individual-testers add --app "123456789" --latest --tester "TESTER_ID"
  asc builds individual-testers add --build-id "BUILD_ID" --tester "TESTER_ID1,TESTER_ID2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if err := selectors.validate(); err != nil {
				return err
			}

			testerIDs := shared.SplitCSV(*testers)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds individual-testers add: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds individual-testers add: %w", err)
			}

			if err := client.AddIndividualTestersToBuild(requestCtx, buildID, testerIDs); err != nil {
				return fmt.Errorf("builds individual-testers add: failed to add testers: %w", err)
			}

			result := &asc.BuildIndividualTestersUpdateResult{
				BuildID:   buildID,
				TesterIDs: testerIDs,
				Action:    "added",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BuildsIndividualTestersRemoveCommand returns the individual-testers remove subcommand.
func BuildsIndividualTestersRemoveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("individual-testers remove", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(fs, buildSelectorFlagOptions{})
	testers := fs.String("tester", "", "Comma-separated tester IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove",
		ShortUsage: "asc builds individual-testers remove (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER [--version VERSION] [--platform PLATFORM]) --tester \"TESTER_ID[,TESTER_ID...]\" --confirm",
		ShortHelp:  "Remove individual testers from a build.",
		LongHelp: `Remove individual testers from a build.

Examples:
  asc builds individual-testers remove --build-id "BUILD_ID" --tester "TESTER_ID" --confirm
  asc builds individual-testers remove --app "123456789" --latest --tester "TESTER_ID" --confirm
  asc builds individual-testers remove --build-id "BUILD_ID" --tester "TESTER_ID1,TESTER_ID2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.applyLegacyAliases(); err != nil {
				return err
			}
			if err := selectors.validate(); err != nil {
				return err
			}

			testerIDs := shared.SplitCSV(*testers)
			if len(testerIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --tester is required")
				return shared.MissingRequiredUsageError()
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds individual-testers remove: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds individual-testers remove: %w", err)
			}

			if err := client.RemoveIndividualTestersFromBuild(requestCtx, buildID, testerIDs); err != nil {
				return fmt.Errorf("builds individual-testers remove: failed to remove testers: %w", err)
			}

			result := &asc.BuildIndividualTestersUpdateResult{
				BuildID:   buildID,
				TesterIDs: testerIDs,
				Action:    "removed",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
