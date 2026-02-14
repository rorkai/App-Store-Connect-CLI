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

// BuildsAddGroupsCommand returns the builds add-groups subcommand.
func BuildsAddGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("add-groups", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	groups := fs.String("group", "", "Comma-separated beta group IDs")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc builds add-groups --build BUILD_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Add beta groups to a build for TestFlight distribution.",
		LongHelp: `Add beta groups to a build for TestFlight distribution.

Examples:
  asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds add-groups --build "BUILD_ID" --group "GROUP1,GROUP2"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			groupIDs := shared.SplitCSV(*groups)
			if len(groupIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.AddBetaGroupsToBuild(requestCtx, trimmedBuildID, groupIDs); err != nil {
				return fmt.Errorf("builds add-groups: failed to add groups: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully added %d group(s) to build %s\n", len(groupIDs), trimmedBuildID)
			result := &asc.BuildBetaGroupsUpdateResult{
				BuildID:  trimmedBuildID,
				GroupIDs: groupIDs,
				Action:   "added",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

// BuildsRemoveGroupsCommand returns the builds remove-groups subcommand.
func BuildsRemoveGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("remove-groups", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID")
	groups := fs.String("group", "", "Comma-separated beta group IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-groups",
		ShortUsage: "asc builds remove-groups --build BUILD_ID --group GROUP_ID[,GROUP_ID...] --confirm",
		ShortHelp:  "Remove beta groups from a build.",
		LongHelp: `Remove beta groups from a build.

Examples:
  asc builds remove-groups --build "BUILD_ID" --group "GROUP_ID" --confirm
  asc builds remove-groups --build "BUILD_ID" --group "GROUP1,GROUP2" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			groupIDs := shared.SplitCSV(*groups)
			if len(groupIDs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}
			if !*confirm {
				fmt.Fprintln(os.Stderr, "Error: --confirm is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds remove-groups: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			if err := client.RemoveBetaGroupsFromBuild(requestCtx, trimmedBuildID, groupIDs); err != nil {
				return fmt.Errorf("builds remove-groups: failed to remove groups: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Successfully removed %d group(s) from build %s\n", len(groupIDs), trimmedBuildID)
			result := &asc.BuildBetaGroupsUpdateResult{
				BuildID:  trimmedBuildID,
				GroupIDs: groupIDs,
				Action:   "removed",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
