package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
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
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc builds add-groups --build BUILD_ID --group GROUP_ID[,GROUP_ID...]",
		ShortHelp:  "Add beta groups to a build for TestFlight distribution.",
		LongHelp: `Add beta groups to a build for TestFlight distribution.

Examples:
  asc builds add-groups --build "BUILD_ID" --group "GROUP_ID"
  asc builds add-groups --build "BUILD_ID" --group "External Testers"
  asc builds add-groups --build "BUILD_ID" --group "GROUP1,GROUP2"`,
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

			groupIDs, err := resolveBuildBetaGroupIDs(requestCtx, client, trimmedBuildID, groupInputs)
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

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

func resolveBuildBetaGroupIDs(ctx context.Context, client *asc.Client, buildID string, groups []string) ([]string, error) {
	buildApp, err := client.GetBuildApp(ctx, buildID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve app for build %q: %w", buildID, err)
	}
	appID := strings.TrimSpace(buildApp.Data.ID)
	if appID == "" {
		return nil, fmt.Errorf("build %q is missing related app ID", buildID)
	}

	firstPage, err := client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsLimit(200))
	if err != nil {
		return nil, fmt.Errorf("failed to list beta groups: %w", err)
	}
	allGroups := firstPage
	if firstPage != nil && firstPage.Links.Next != "" {
		paginated, err := asc.PaginateAll(ctx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
			return client.GetBetaGroups(ctx, appID, asc.WithBetaGroupsNextURL(nextURL))
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list beta groups: %w", err)
		}
		var ok bool
		allGroups, ok = paginated.(*asc.BetaGroupsResponse)
		if !ok {
			return nil, fmt.Errorf("unexpected beta groups pagination type %T", paginated)
		}
	}
	return resolveBuildBetaGroupIDsFromList(groups, allGroups)
}

func resolveBuildBetaGroupIDsFromList(inputGroups []string, groups *asc.BetaGroupsResponse) ([]string, error) {
	if groups == nil {
		return nil, fmt.Errorf("no beta groups returned for app")
	}

	groupIDs := make(map[string]struct{}, len(groups.Data))
	groupNameToIDs := make(map[string][]string)
	for _, item := range groups.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		groupIDs[id] = struct{}{}

		name := strings.TrimSpace(item.Attributes.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if !slices.Contains(groupNameToIDs[key], id) {
			groupNameToIDs[key] = append(groupNameToIDs[key], id)
		}
	}

	resolved := make([]string, 0, len(inputGroups))
	seen := make(map[string]struct{}, len(inputGroups))
	for _, raw := range inputGroups {
		group := strings.TrimSpace(raw)
		if group == "" {
			continue
		}

		resolvedID := ""
		if _, ok := groupIDs[group]; ok {
			resolvedID = group
		} else {
			matches := groupNameToIDs[strings.ToLower(group)]
			switch len(matches) {
			case 0:
				return nil, fmt.Errorf("beta group %q not found", group)
			case 1:
				resolvedID = matches[0]
			default:
				return nil, fmt.Errorf("multiple beta groups named %q; use group ID", group)
			}
		}

		if _, ok := seen[resolvedID]; ok {
			continue
		}
		seen[resolvedID] = struct{}{}
		resolved = append(resolved, resolvedID)
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("at least one beta group is required")
	}
	return resolved, nil
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
