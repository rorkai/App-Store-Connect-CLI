package testflight

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBetaTestersAppsCommand returns the beta-testers apps command group built on d.
func newBetaTestersAppsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc testflight beta-testers apps <subcommand> [flags]",
		ShortHelp:  "List apps for a beta tester.",
		LongHelp: `List apps for a beta tester.

Examples:
  asc testflight beta-testers apps list --tester-id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaTestersAppsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaTestersAppsListCommand returns the beta-testers apps list subcommand built on d.
func newBetaTestersAppsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("apps list", flag.ExitOnError)

	testerID := shared.BindResourceIDFlag(fs, "tester-id", "betaTesters", "Beta tester ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID (alias of --tester-id)")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers apps list [flags]",
		ShortHelp:  "List apps for a beta tester.",
		LongHelp: `List apps for a beta tester.

Examples:
  asc testflight beta-testers apps list --tester-id "TESTER_ID"
  asc testflight beta-testers apps list --tester-id "TESTER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight beta-testers apps list", 200); err != nil {
				return err
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-testers apps list: --tester-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			if testerValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--tester-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-testers apps list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterAppsOption{
				asc.Limit[asc.BetaTesterAppsOption](*limit),
				asc.NextURL[asc.BetaTesterAppsOption](*next),
			}

			if *paginate {
				if testerValue == "" {
					return d.RequiredFlag("--tester-id")
				}
				paginateOpts := append(opts, asc.Limit[asc.BetaTesterAppsOption](200))
				resp, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaTesterApps(ctx, testerValue, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaTesterApps(ctx, testerValue, asc.NextURL[asc.BetaTesterAppsOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("testflight beta-testers apps list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaTesterApps(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers apps list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaTestersBetaGroupsCommand returns the beta-testers beta-groups command group built on d.
func newBetaTestersBetaGroupsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-groups",
		ShortUsage: "asc testflight beta-testers beta-groups <subcommand> [flags]",
		ShortHelp:  "List beta groups for a beta tester.",
		LongHelp: `List beta groups for a beta tester.

Examples:
  asc testflight beta-testers beta-groups list --tester-id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaTestersBetaGroupsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaTestersBetaGroupsListCommand returns the beta-testers beta-groups list subcommand built on d.
func newBetaTestersBetaGroupsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("beta-groups list", flag.ExitOnError)

	testerID := shared.BindResourceIDFlag(fs, "tester-id", "betaTesters", "Beta tester ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID (alias of --tester-id)")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers beta-groups list [flags]",
		ShortHelp:  "List beta groups for a beta tester.",
		LongHelp: `List beta groups for a beta tester.

Examples:
  asc testflight beta-testers beta-groups list --tester-id "TESTER_ID"
  asc testflight beta-testers beta-groups list --tester-id "TESTER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight beta-testers beta-groups list", 200); err != nil {
				return err
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-testers beta-groups list: --tester-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			if testerValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--tester-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-testers beta-groups list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterBetaGroupsOption{
				asc.Limit[asc.BetaTesterBetaGroupsOption](*limit),
				asc.NextURL[asc.BetaTesterBetaGroupsOption](*next),
			}

			if *paginate {
				if testerValue == "" {
					return d.RequiredFlag("--tester-id")
				}
				paginateOpts := append(opts, asc.Limit[asc.BetaTesterBetaGroupsOption](200))
				resp, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaTesterBetaGroups(ctx, testerValue, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaTesterBetaGroups(ctx, testerValue, asc.NextURL[asc.BetaTesterBetaGroupsOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("testflight beta-testers beta-groups list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaTesterBetaGroups(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers beta-groups list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}

// newBetaTestersBuildsCommand returns the beta-testers builds command group built on d.
func newBetaTestersBuildsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("builds", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "builds",
		ShortUsage: "asc testflight beta-testers builds <subcommand> [flags]",
		ShortHelp:  "List builds for a beta tester.",
		LongHelp: `List builds for a beta tester.

Examples:
  asc testflight beta-testers builds list --tester-id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBetaTestersBuildsListCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBetaTestersBuildsListCommand returns the beta-testers builds list subcommand built on d.
func newBetaTestersBuildsListCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("builds list", flag.ExitOnError)

	testerID := shared.BindResourceIDFlag(fs, "tester-id", "betaTesters", "Beta tester ID")
	aliasID := shared.BindResourceIDFlag(fs, "id", "betaTesters", "Beta tester ID (alias of --tester-id)")
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	paginate := fs.Bool("paginate", false, shared.PaginateFlagUsage)
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers builds list [flags]",
		ShortHelp:  "List builds for a beta tester.",
		LongHelp: `List builds for a beta tester.

Examples:
  asc testflight beta-testers builds list --tester-id "TESTER_ID"
  asc testflight beta-testers builds list --tester-id "TESTER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "testflight beta-testers builds list", 200); err != nil {
				return err
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return shared.WithDiagnostic(
					d.UsageErrorCtx(ctx, "testflight beta-testers builds list: --tester-id and --id must match"),
					shared.DiagnosticConflictingInput,
					"",
				)
			}
			if testerValue == "" && strings.TrimSpace(*next) == "" {
				return d.RequiredFlag("--tester-id")
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("testflight beta-testers builds list: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterBuildsOption{
				asc.Limit[asc.BetaTesterBuildsOption](*limit),
				asc.NextURL[asc.BetaTesterBuildsOption](*next),
			}

			if *paginate {
				if testerValue == "" {
					return d.RequiredFlag("--tester-id")
				}
				paginateOpts := append(opts, asc.Limit[asc.BetaTesterBuildsOption](200))
				resp, err := d.PaginateWithSpinner(
					requestCtx,
					func(ctx context.Context) (asc.PaginatedResponse, error) {
						return client.GetBetaTesterBuilds(ctx, testerValue, paginateOpts...)
					},
					func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
						return client.GetBetaTesterBuilds(ctx, testerValue, asc.NextURL[asc.BetaTesterBuildsOption](nextURL))
					},
				)
				if err != nil {
					return fmt.Errorf("testflight beta-testers builds list: %w", err)
				}
				return d.PrintOutput(resp, *output.Output, *output.Pretty)
			}

			resp, err := client.GetBetaTesterBuilds(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers builds list: failed to fetch: %w", err)
			}

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
