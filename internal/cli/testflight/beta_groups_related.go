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
  asc testflight beta-groups app get --group-id "GROUP_ID"`,
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
	fs := flag.NewFlagSet("app get", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-groups app get --group-id \"GROUP_ID\"",
		ShortHelp:  "Get the app for a beta group.",
		LongHelp: `Get the app for a beta group.

Examples:
  asc testflight beta-groups app get --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups app get: --group-id and --id must match")
			}
			if groupValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups app get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupApp(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups app get: failed to fetch: %w", err)
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
  asc testflight beta-groups beta-recruitment-criteria get --group-id "GROUP_ID"`,
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
	fs := flag.NewFlagSet("beta-recruitment-criteria get", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criteria get --group-id \"GROUP_ID\"",
		ShortHelp:  "Get beta recruitment criteria for a beta group.",
		LongHelp: `Get beta recruitment criteria for a beta group.

Examples:
  asc testflight beta-groups beta-recruitment-criteria get --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria get: --group-id and --id must match")
			}
			if groupValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupBetaRecruitmentCriteria(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criteria get: failed to fetch: %w", err)
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
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check get --group-id "GROUP_ID"`,
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
	fs := flag.NewFlagSet("beta-recruitment-criterion-compatible-build-check get", flag.ExitOnError)

	groupID := fs.String("group-id", "", "Beta group ID")
	aliasID := fs.String("id", "", "Beta group ID (alias of --group-id)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc testflight beta-groups beta-recruitment-criterion-compatible-build-check get --group-id \"GROUP_ID\"",
		ShortHelp:  "Get compatible build status for beta recruitment criteria.",
		LongHelp: `Get compatible build status for beta recruitment criteria.

Examples:
  asc testflight beta-groups beta-recruitment-criterion-compatible-build-check get --group-id "GROUP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			groupValue := strings.TrimSpace(*groupID)
			aliasValue := strings.TrimSpace(*aliasID)
			if groupValue == "" {
				groupValue = aliasValue
			} else if aliasValue != "" && aliasValue != groupValue {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check get: --group-id and --id must match")
			}
			if groupValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --group-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetBetaGroupBetaRecruitmentCriterionCompatibleBuildCheck(requestCtx, groupValue)
			if err != nil {
				return fmt.Errorf("testflight beta-groups beta-recruitment-criterion-compatible-build-check get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
