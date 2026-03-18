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
	groups := fs.String("group", "", "Comma-separated beta group IDs or names")
	skipInternal := fs.Bool("skip-internal", false, "Skip internal beta groups instead of adding them")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc builds add-groups --build BUILD_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Add beta groups to a build for TestFlight distribution.",
		LongHelp: `Add beta groups to a build for TestFlight distribution.

Examples:
  asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds add-groups --build "BUILD_ID" --group "External Testers"
  asc builds add-groups --build "BUILD_ID" --group "GROUP1,GROUP2"
  asc builds add-groups --build "BUILD_ID" --group "INTERNAL_ID,EXTERNAL_ID" --skip-internal`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			groupInputs := shared.SplitCSV(*groups)
			if len(groupInputs) == 0 {
				fmt.Fprintln(os.Stderr, "Error: --group is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resolvedGroups, err := resolveBuildBetaGroups(requestCtx, client, trimmedBuildID, groupInputs, *skipInternal)
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

			addResult, err := shared.AddBuildBetaGroups(requestCtx, client, trimmedBuildID, resolvedGroups, shared.AddBuildBetaGroupsOptions{
				SkipInternal: *skipInternal,
			})
			if err != nil {
				return fmt.Errorf("builds add-groups: failed to add groups: %w", err)
			}

			for _, group := range addResult.SkippedInternalGroups {
				fmt.Fprintf(
					os.Stderr,
					"Skipped internal group %q (%s) because --skip-internal was set\n",
					group.NameForDisplay(),
					group.ID,
				)
			}

			if len(addResult.AddedGroupIDs) == 0 {
				fmt.Fprintf(os.Stderr, "No groups to add for build %s after applying filters\n", trimmedBuildID)
				result := &asc.BuildBetaGroupsUpdateResult{
					BuildID:  trimmedBuildID,
					GroupIDs: []string{},
					Action:   "added",
				}
				return shared.PrintOutput(result, *output.Output, *output.Pretty)
			}

			fmt.Fprintf(os.Stderr, "Successfully added %d group(s) to build %s\n", len(addResult.AddedGroupIDs), trimmedBuildID)
			result := &asc.BuildBetaGroupsUpdateResult{
				BuildID:  trimmedBuildID,
				GroupIDs: addResult.AddedGroupIDs,
				Action:   "added",
			}

			return shared.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

type resolvedBuildBetaGroup = shared.ResolvedBetaGroup

func resolveBuildBetaGroups(ctx context.Context, client *asc.Client, buildID string, groups []string, skipInternal bool) ([]resolvedBuildBetaGroup, error) {
	buildApp, err := client.GetBuildApp(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve app for build %q: %w", buildID, err)
	}
	appID := strings.TrimSpace(buildApp.Data.ID)
	if appID == "" {
		return nil, fmt.Errorf("build %q is missing related app ID", buildID)
	}

	return shared.ResolveBetaGroups(ctx, client, appID, groups, shared.ResolveBetaGroupsOptions{
		SkipInternal:            skipInternal,
		IncludeSkipInternalHint: true,
	})
}

func resolveBuildBetaGroupsFromList(inputGroups []string, groups *asc.BetaGroupsResponse, skipInternal bool) ([]resolvedBuildBetaGroup, error) {
	return shared.ResolveBetaGroupsFromList(inputGroups, groups, shared.ResolveBetaGroupsOptions{
		SkipInternal:            skipInternal,
		IncludeSkipInternalHint: true,
	})
}

func resolveBuildBetaGroupIDsFromList(inputGroups []string, groups *asc.BetaGroupsResponse, skipInternal bool) ([]string, error) {
	resolvedGroups, err := resolveBuildBetaGroupsFromList(inputGroups, groups, skipInternal)
	if err != nil {
		return nil, err
	}
	resolvedIDs := make([]string, 0, len(resolvedGroups))
	for _, group := range resolvedGroups {
		resolvedIDs = append(resolvedIDs, group.ID)
	}
	return resolvedIDs, nil
}

// BuildsUpdateCommand returns the builds update subcommand.
func BuildsUpdateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds update", flag.ExitOnError)

	buildID := fs.String("build", "", "Build ID (required)")
	usesNonExemptEncryption := fs.String("uses-non-exempt-encryption", "", "Set encryption compliance: true or false")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc builds update --build BUILD_ID [flags]",
		ShortHelp:  "Update build attributes.",
		LongHelp: `Update build attributes such as encryption compliance.

Examples:
  asc builds update --build "BUILD_ID" --uses-non-exempt-encryption=false
  asc builds update --build "BUILD_ID" --uses-non-exempt-encryption=true`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedBuildID := strings.TrimSpace(*buildID)
			if trimmedBuildID == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			trimmedEncryption := strings.TrimSpace(strings.ToLower(*usesNonExemptEncryption))
			if trimmedEncryption == "" {
				fmt.Fprintln(os.Stderr, "Error: at least one update flag is required (e.g. --uses-non-exempt-encryption)")
				return flag.ErrHelp
			}

			attrs := asc.BuildUpdateAttributes{}
			switch trimmedEncryption {
			case "true":
				v := true
				attrs.UsesNonExemptEncryption = &v
			case "false":
				v := false
				attrs.UsesNonExemptEncryption = &v
			default:
				return shared.UsageError("--uses-non-exempt-encryption must be 'true' or 'false'")
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds update: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.UpdateBuild(requestCtx, trimmedBuildID, attrs)
			if err != nil {
				return fmt.Errorf("builds update: %w", err)
			}

			fmt.Fprintf(os.Stderr, "Updated build %s\n", trimmedBuildID)
			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
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
