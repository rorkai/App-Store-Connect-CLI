package testflight

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBetaGroupsAppCommand returns the beta-groups app command group built on d.
func newBetaGroupsAppCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc testflight beta-groups app <subcommand> [flags]",
		ShortHelp:  "View the app related to a beta group.",
		LongHelp: `View the app related to a beta group.

Examples:
  asc testflight beta-groups app view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaGroupsAppGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaGroupsAppGetCommand returns the beta-groups app get subcommand built on d.
func newBetaGroupsAppGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("app view", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "betaGroups", "Beta group ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaGroups", "Beta group ID (alias of --group-id)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups app view --group-id \"GROUP_ID\"",
		ShortHelp:  "View the app for a beta group.",
		LongHelp: `View the app for a beta group.

Examples:
  asc testflight beta-groups app view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-groups app view: --group-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			if groupValue == "" {
				return d.RequiredFlag("--group-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-groups app view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupApp(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups app view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaGroupsRecruitmentCriteriaCommand returns the beta-groups beta-recruitment-criteria command group built on d.
func newBetaGroupsRecruitmentCriteriaCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-recruitment-criteria", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-recruitment-criteria",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criteria <subcommand> [flags]",
		ShortHelp:  "View beta recruitment criteria for a beta group.",
		LongHelp: `View beta recruitment criteria for a beta group.

Examples:
  asc testflight beta-groups beta-recruitment-criteria view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaGroupsRecruitmentCriteriaGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaGroupsRecruitmentCriteriaGetCommand returns the beta-recruitment-criteria get subcommand built on d.
func newBetaGroupsRecruitmentCriteriaGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-recruitment-criteria view", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "betaGroups", "Beta group ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaGroups", "Beta group ID (alias of --group-id)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criteria view --group-id \"GROUP_ID\"",
		ShortHelp:  "View beta recruitment criteria for a beta group.",
		LongHelp: `View beta recruitment criteria for a beta group.

Examples:
  asc testflight beta-groups beta-recruitment-criteria view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-groups beta-recruitment-criteria view: --group-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			if groupValue == "" {
				return d.RequiredFlag("--group-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupBetaRecruitmentCriteria(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaGroupsRecruitmentCriterionCompatibleBuildCheckCommand returns the compatible-build-check command group built on d.
func newBetaGroupsRecruitmentCriterionCompatibleBuildCheckCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-recruitment-criterion-compatible-build-check", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-recruitment-criterion-compatible-build-check",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criterion-compatible-build-check <subcommand> [flags]",
		ShortHelp:  "Check beta recruitment compatible build status for a group.",
		LongHelp: `Check beta recruitment compatible build status for a group.

Examples:
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand returns the compatible-build-check get subcommand built on d.
func newBetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-recruitment-criterion-compatible-build-check view", flag.ExitOnError)

	groupID := shared.BindResourceIDFlag(fs, "group-id", "betaGroups", "Beta group ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaGroups", "Beta group ID (alias of --group-id)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id \"GROUP_ID\"",
		ShortHelp:  "View compatible build status for beta recruitment criteria.",
		LongHelp: `View compatible build status for beta recruitment criteria.

Examples:
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-groups beta-recruitment-criterion-compatible-build-check view: --group-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			if groupValue == "" {
				return d.RequiredFlag("--group-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check view: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check view: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
