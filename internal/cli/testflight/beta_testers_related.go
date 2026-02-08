package testflight

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

// BetaTestersAppsCommand returns the beta-testers apps command group.
func BetaTestersAppsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "apps",
		ShortUsage: "asc testflight beta-testers apps <subcommand> [flags]",
		ShortHelp:  "List apps for a beta tester.",
		LongHelp: `List apps for a beta tester.

Examples:
  asc testflight beta-testers apps list --tester-id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaTestersAppsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaTestersAppsListCommand returns the beta-testers apps list subcommand.
func BetaTestersAppsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps list", flag.ExitOnError)

	testerID := fs.String("tester-id", "", "Beta tester ID")
	aliasID := fs.String("id", "", "Beta tester ID (alias of --tester-id)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers apps list [flags]",
		ShortHelp:  "List apps for a beta tester.",
		LongHelp: `List apps for a beta tester.

Examples:
  asc testflight beta-testers apps list --tester-id "TESTER_ID"
  asc testflight beta-testers apps list --tester-id "TESTER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight beta-testers apps list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-testers apps list: %w", err)
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return fmt.Errorf("testflight beta-testers apps list: --tester-id and --id must match")
			}
			if testerValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-testers apps list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterAppsOption{
				asc.WithBetaTesterAppsLimit(*limit),
				asc.WithBetaTesterAppsNextURL(*next),
			}

			if *paginate {
				if testerValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithBetaTesterAppsLimit(200))
				firstPage, err := client.GetBetaTesterApps(requestCtx, testerValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight beta-testers apps list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaTesterApps(ctx, testerValue, asc.WithBetaTesterAppsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("testflight beta-testers apps list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBetaTesterApps(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers apps list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BetaTestersBetaGroupsCommand returns the beta-testers beta-groups command group.
func BetaTestersBetaGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "beta-groups",
		ShortUsage: "asc testflight beta-testers beta-groups <subcommand> [flags]",
		ShortHelp:  "List beta groups for a beta tester.",
		LongHelp: `List beta groups for a beta tester.

Examples:
  asc testflight beta-testers beta-groups list --tester-id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaTestersBetaGroupsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaTestersBetaGroupsListCommand returns the beta-testers beta-groups list subcommand.
func BetaTestersBetaGroupsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("beta-groups list", flag.ExitOnError)

	testerID := fs.String("tester-id", "", "Beta tester ID")
	aliasID := fs.String("id", "", "Beta tester ID (alias of --tester-id)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers beta-groups list [flags]",
		ShortHelp:  "List beta groups for a beta tester.",
		LongHelp: `List beta groups for a beta tester.

Examples:
  asc testflight beta-testers beta-groups list --tester-id "TESTER_ID"
  asc testflight beta-testers beta-groups list --tester-id "TESTER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight beta-testers beta-groups list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-testers beta-groups list: %w", err)
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return fmt.Errorf("testflight beta-testers beta-groups list: --tester-id and --id must match")
			}
			if testerValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-testers beta-groups list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterBetaGroupsOption{
				asc.WithBetaTesterBetaGroupsLimit(*limit),
				asc.WithBetaTesterBetaGroupsNextURL(*next),
			}

			if *paginate {
				if testerValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithBetaTesterBetaGroupsLimit(200))
				firstPage, err := client.GetBetaTesterBetaGroups(requestCtx, testerValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight beta-testers beta-groups list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaTesterBetaGroups(ctx, testerValue, asc.WithBetaTesterBetaGroupsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("testflight beta-testers beta-groups list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBetaTesterBetaGroups(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers beta-groups list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// BetaTestersBuildsCommand returns the beta-testers builds command group.
func BetaTestersBuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "builds",
		ShortUsage: "asc testflight beta-testers builds <subcommand> [flags]",
		ShortHelp:  "List builds for a beta tester.",
		LongHelp: `List builds for a beta tester.

Examples:
  asc testflight beta-testers builds list --tester-id "TESTER_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BetaTestersBuildsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BetaTestersBuildsListCommand returns the beta-testers builds list subcommand.
func BetaTestersBuildsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds list", flag.ExitOnError)

	testerID := fs.String("tester-id", "", "Beta tester ID")
	aliasID := fs.String("id", "", "Beta tester ID (alias of --tester-id)")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc testflight beta-testers builds list [flags]",
		ShortHelp:  "List builds for a beta tester.",
		LongHelp: `List builds for a beta tester.

Examples:
  asc testflight beta-testers builds list --tester-id "TESTER_ID"
  asc testflight beta-testers builds list --tester-id "TESTER_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("testflight beta-testers builds list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("testflight beta-testers builds list: %w", err)
			}

			testerValue := strings.TrimSpace(*testerID)
			aliasValue := strings.TrimSpace(*aliasID)
			if testerValue == "" {
				testerValue = aliasValue
			} else if aliasValue != "" && aliasValue != testerValue {
				return fmt.Errorf("testflight beta-testers builds list: --tester-id and --id must match")
			}
			if testerValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("testflight beta-testers builds list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaTesterBuildsOption{
				asc.WithBetaTesterBuildsLimit(*limit),
				asc.WithBetaTesterBuildsNextURL(*next),
			}

			if *paginate {
				if testerValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --tester-id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithBetaTesterBuildsLimit(200))
				firstPage, err := client.GetBetaTesterBuilds(requestCtx, testerValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("testflight beta-testers builds list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetBetaTesterBuilds(ctx, testerValue, asc.WithBetaTesterBuildsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("testflight beta-testers builds list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetBetaTesterBuilds(requestCtx, testerValue, opts...)
			if err != nil {
				return fmt.Errorf("testflight beta-testers builds list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
