package builds

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBuildsAddGroupsCommand returns the builds add-groups subcommand.
func newBuildsAddGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("add-groups", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{})
	groups := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group IDs or names")
	shared.BindFlagAlias(fs, "group-id", "group")
	skipInternal := fs.Bool("skip-internal", false, "Skip internal beta groups instead of adding them")
	dryRun := fs.Bool("dry-run", false, "Preview beta group assignment without adding groups")
	submit := fs.Bool("submit", false, "Submit build for beta app review after adding external groups")
	confirm := fs.Bool("confirm", false, "Confirm beta app review submission (required with --submit)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "add-groups",
		ShortUsage: "asc builds add-groups (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION]) --group GROUP_ID[,GROUP_ID...] [--dry-run | --submit --confirm]",
		ShortHelp:  "Add beta groups to a build for TestFlight distribution.",
		LongHelp: `Add beta groups to a build for TestFlight distribution.

Examples:
  asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID"
  asc builds add-groups --app "123456789" --latest --group "GROUP_ID"
  asc builds add-groups --build-id "BUILD_ID" --group "External Testers"
  asc builds add-groups --build-id "BUILD_ID" --group "GROUP1,GROUP2"
  asc builds add-groups --build-id "BUILD_ID" --group "INTERNAL_ID,EXTERNAL_ID" --skip-internal
  asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID" --dry-run
  asc builds add-groups --build-id "BUILD_ID" --group "GROUP_ID" --submit --confirm

Use --dry-run to resolve the build and groups and preview the relationship
without adding groups or submitting a review. Any state shown by its
best-effort reads is observational and advisory, and does not predict whether
a later assignment will be accepted.

Normal assignments verify the current build processing and expiry before
sending the relationship request. Assignments that include an external group
also verify encryption, audience, and beta-review state. If App Store Connect
does not provide enough state to prove readiness, the command stops without
sending that request.`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			groupInputs := shared.SplitCSV(groups.String())
			if len(groupInputs) == 0 {
				return d.RequiredFlag("--group")
			}
			if *submit && *dryRun {
				return shared.WithDiagnostic(
					d.UsageError("--submit cannot be used with --dry-run"),
					shared.DiagnosticConflictingInput,
					"--dry-run",
				)
			}
			if *submit && !*confirm {
				fmt.Fprintln(d.IO.ErrOut, "Error: --confirm is required with --submit")
				return shared.MissingRequiredUsageError("--confirm")
			}
			if *confirm && !*submit {
				fmt.Fprintln(d.IO.ErrOut, "Error: --confirm requires --submit")
				return flag.ErrHelp
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

			resolvedGroups, err := resolveBuildBetaGroups(requestCtx, client, buildID, groups.String(), *skipInternal)
			if err != nil {
				return fmt.Errorf("builds add-groups: %w", err)
			}

			plan := shared.PlanBuildBetaGroupAssignment(resolvedGroups, shared.AddBuildBetaGroupsOptions{
				SkipInternal: *skipInternal,
			})
			if *dryRun {
				return reportBuildBetaGroupAssignmentDryRun(requestCtx, d, client, buildID, plan, output)
			}
			if err := d.PreflightBuildBetaGroupAssignment(requestCtx, client, buildID, plan, shared.BuildBetaGroupPreflightOptions{
				OperationName: "builds add-groups",
			}); err != nil {
				return err
			}

			addResult, err := shared.AddBuildBetaGroups(requestCtx, client, buildID, resolvedGroups, shared.AddBuildBetaGroupsOptions{
				SkipInternal: *skipInternal,
			})
			if err != nil {
				return reportBuildBetaGroupAssignmentFailure(requestCtx, d, client, buildID, plan.GroupsToAdd, err)
			}

			submissionResult, err := shared.SubmitBuildBetaReviewIfNeeded(requestCtx, client, buildID, resolvedGroups, addResult.AddedGroupIDs, *submit, "builds add-groups")
			if err != nil {
				return err
			}

			for _, group := range addResult.SkippedInternalGroups {
				fmt.Fprintf(
					d.IO.ErrOut,
					"Skipped internal group %q (%s) because --skip-internal was set\n",
					group.NameForDisplay(),
					shared.SanitizeTerminal(group.ID),
				)
			}

			if len(addResult.AddedGroupIDs) == 0 {
				fmt.Fprintf(d.IO.ErrOut, "No groups to add for build %s after applying filters\n", buildID)
			} else {
				fmt.Fprintf(d.IO.ErrOut, "Successfully added %d group(s) to build %s\n", len(addResult.AddedGroupIDs), buildID)
			}
			if submissionResult.Message != "" {
				fmt.Fprintln(d.IO.ErrOut, submissionResult.Message)
			}

			if len(addResult.AddedGroupIDs) == 0 {
				result := &asc.BuildBetaGroupsUpdateResult{
					BuildID:  buildID,
					GroupIDs: []string{},
					Action:   "added",
				}
				return d.PrintOutput(result, *output.Output, *output.Pretty)
			}

			result := &asc.BuildBetaGroupsUpdateResult{
				BuildID:  buildID,
				GroupIDs: addResult.AddedGroupIDs,
				Action:   "added",
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}

type resolvedBuildBetaGroup = shared.ResolvedBetaGroup

func resolveBuildBetaGroups(ctx context.Context, client *asc.Client, buildID, groups string, skipInternal bool) ([]resolvedBuildBetaGroup, error) {
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

// newBuildsUpdateCommand returns the builds update subcommand.
func newBuildsUpdateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("builds update", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{buildIDUsage: "Build ID"})
	usesNonExemptEncryption := fs.String("uses-non-exempt-encryption", "", "Set encryption compliance: true or false")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "update",
		ShortUsage: "asc builds update (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION]) --uses-non-exempt-encryption [true|false] [flags]",
		ShortHelp:  "Update build attributes.",
		LongHelp: `Update build attributes such as encryption compliance.

Examples:
  asc builds update --build-id "BUILD_ID" --uses-non-exempt-encryption=false
  asc builds update --app "123456789" --latest --uses-non-exempt-encryption=false
  asc builds update --build-id "BUILD_ID" --uses-non-exempt-encryption=true`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			trimmedEncryption := strings.TrimSpace(strings.ToLower(*usesNonExemptEncryption))
			if trimmedEncryption == "" {
				return d.UpdateFlagRequired(fs, usesNonExemptEncryption)
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
				return d.UsageError("--uses-non-exempt-encryption must be 'true' or 'false'")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds update: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds update: %w", err)
			}

			resp, err := client.UpdateBuild(requestCtx, buildID, attrs)
			if err != nil {
				return fmt.Errorf("builds update: %w", err)
			}

			fmt.Fprintf(d.IO.ErrOut, "Updated build %s\n", buildID)
			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBuildsRemoveGroupsCommand returns the builds remove-groups subcommand.
func newBuildsRemoveGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("remove-groups", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{})
	groups := shared.BindOnceCSVFlag(fs, "group", "Comma-separated beta group IDs")
	confirm := fs.Bool("confirm", false, "Confirm removal")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "remove-groups",
		ShortUsage: "asc builds remove-groups (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION]) --group GROUP_ID[,GROUP_ID...] --confirm",
		ShortHelp:  "Remove beta groups from a build.",
		LongHelp: `Remove beta groups from a build.

Examples:
  asc builds remove-groups --build-id "BUILD_ID" --group "GROUP_ID" --confirm
  asc builds remove-groups --app "123456789" --latest --group "GROUP_ID" --confirm
  asc builds remove-groups --build-id "BUILD_ID" --group "GROUP1,GROUP2" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := selectors.validate(); err != nil {
				return err
			}

			groupIDs := shared.SplitCSV(groups.String())
			if len(groupIDs) == 0 {
				return d.RequiredFlag("--group")
			}
			if err := d.RequireConfirm(*confirm); err != nil {
				return err
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds remove-groups: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			buildID, err := selectors.resolveBuildID(requestCtx, client)
			if err != nil {
				return fmt.Errorf("builds remove-groups: %w", err)
			}

			if err := client.RemoveBetaGroupsFromBuild(requestCtx, buildID, groupIDs); err != nil {
				return fmt.Errorf("builds remove-groups: failed to remove groups: %w", err)
			}

			fmt.Fprintf(d.IO.ErrOut, "Successfully removed %d group(s) from build %s\n", len(groupIDs), buildID)
			result := &asc.BuildBetaGroupsUpdateResult{
				BuildID:  buildID,
				GroupIDs: groupIDs,
				Action:   "removed",
			}

			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
