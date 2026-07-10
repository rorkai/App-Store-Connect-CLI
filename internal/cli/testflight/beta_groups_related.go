package testflight

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// BetaGroupsAppCommand returns the beta-groups app command group.
func BetaGroupsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc testflight beta-groups app <subcommand> [flags]",
		ShortHelp:  "View the app related to a beta group.",
		LongHelp: `View the app related to a beta group.

Examples:
  asc testflight beta-groups app view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaGroupsAppGetCommand returns the beta-groups app get subcommand.
func BetaGroupsAppGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app view", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups app view --group-id \"GROUP_ID\"",
		ShortHelp:  "View the app for a beta group.",
		LongHelp: `View the app for a beta group.

Examples:
  asc testflight beta-groups app view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups app view: --group-id and --id must match")
			}
			if groupValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups app view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupApp(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups app view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaGroupsRecruitmentCriteriaCommand returns the beta-groups beta-recruitment-criteria command group.
func BetaGroupsRecruitmentCriteriaCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-recruitment-criteria", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-recruitment-criteria",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criteria <subcommand> [flags]",
		ShortHelp:  "View beta recruitment criteria for a beta group.",
		LongHelp: `View beta recruitment criteria for a beta group.

Examples:
  asc testflight beta-groups beta-recruitment-criteria view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsRecruitmentCriteriaGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaGroupsRecruitmentCriteriaGetCommand returns the beta-recruitment-criteria get subcommand.
func BetaGroupsRecruitmentCriteriaGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-recruitment-criteria view", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criteria view --group-id \"GROUP_ID\"",
		ShortHelp:  "View beta recruitment criteria for a beta group.",
		LongHelp: `View beta recruitment criteria for a beta group.

Examples:
  asc testflight beta-groups beta-recruitment-criteria view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria view: --group-id and --id must match")
			}
			if groupValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupBetaRecruitmentCriteria(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// BetaGroupsRecruitmentCriterionCompatibleBuildCheckCommand returns the compatible-build-check command group.
func BetaGroupsRecruitmentCriterionCompatibleBuildCheckCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-recruitment-criterion-compatible-build-check", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-recruitment-criterion-compatible-build-check",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criterion-compatible-build-check <subcommand> [flags]",
		ShortHelp:  "Check beta recruitment compatible build status for a group.",
		LongHelp: `Check beta recruitment compatible build status for a group.

Examples:
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand returns the compatible-build-check get subcommand.
func BetaGroupsRecruitmentCriterionCompatibleBuildCheckGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-recruitment-criterion-compatible-build-check view", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "view",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id \"GROUP_ID\"",
		ShortHelp:  "View compatible build status for beta recruitment criteria.",
		LongHelp: `View compatible build status for beta recruitment criteria.

Examples:
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check view --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check view: --group-id and --id must match")
			}
			if groupValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return shared.MissingRequiredUsageError()
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check view: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check view: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
