package prerelease

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

// PreReleaseVersionsAppCommand returns the app command group.
func PreReleaseVersionsAppCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "app",
		ShortUsage: "asc pre-release-versions app <subcommand> [flags]",
		ShortHelp:  "View the app for a pre-release version.",
		LongHelp: `View the app for a pre-release version.

Examples:
  asc pre-release-versions app get --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PreReleaseVersionsAppGetCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PreReleaseVersionsAppGetCommand returns the app get subcommand.
func PreReleaseVersionsAppGetCommand() *ffcli.Command {
	fs := flag.NewFlagSet("app get", flag.ExitOnError)

	id := fs.String("id", "", "Pre-release version ID")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "get",
		ShortUsage: "asc pre-release-versions app get --id \"PR_ID\"",
		ShortHelp:  "Get the app for a pre-release version.",
		LongHelp: `Get the app for a pre-release version.

Examples:
  asc pre-release-versions app get --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			idValue := strings.TrimSpace(*id)
			if idValue == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-release-versions app get: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			resp, err := client.GetPreReleaseVersionApp(requestCtx, idValue)
			if err != nil {
				return fmt.Errorf("pre-release-versions app get: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}

// PreReleaseVersionsBuildsCommand returns the builds command group.
func PreReleaseVersionsBuildsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "builds",
		ShortUsage: "asc pre-release-versions builds <subcommand> [flags]",
		ShortHelp:  "List builds for a pre-release version.",
		LongHelp: `List builds for a pre-release version.

Examples:
  asc pre-release-versions builds list --id "PR_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			PreReleaseVersionsBuildsListCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// PreReleaseVersionsBuildsListCommand returns the builds list subcommand.
func PreReleaseVersionsBuildsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("builds list", flag.ExitOnError)

	id := fs.String("id", "", "Pre-release version ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	paginate := fs.Bool("paginate", false, "Automatically fetch all pages (aggregate results)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc pre-release-versions builds list [flags]",
		ShortHelp:  "List builds for a pre-release version.",
		LongHelp: `List builds for a pre-release version.

Examples:
  asc pre-release-versions builds list --id "PR_ID"
  asc pre-release-versions builds list --id "PR_ID" --paginate`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("pre-release-versions builds list: --limit must be between 1 and 200")
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("pre-release-versions builds list: %w", err)
			}

			idValue := strings.TrimSpace(*id)
			if idValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --id is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("pre-release-versions builds list: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.PreReleaseVersionBuildsOption{
				asc.WithPreReleaseVersionBuildsLimit(*limit),
				asc.WithPreReleaseVersionBuildsNextURL(*next),
			}

			if *paginate {
				if idValue == "" {
					fmt.Fprintln(os.Stderr, "Error: --id is required")
					return flag.ErrHelp
				}
				paginateOpts := append(opts, asc.WithPreReleaseVersionBuildsLimit(200))
				firstPage, err := client.GetPreReleaseVersionBuilds(requestCtx, idValue, paginateOpts...)
				if err != nil {
					return fmt.Errorf("pre-release-versions builds list: failed to fetch: %w", err)
				}
				resp, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
					return client.GetPreReleaseVersionBuilds(ctx, idValue, asc.WithPreReleaseVersionBuildsNextURL(nextURL))
				})
				if err != nil {
					return fmt.Errorf("pre-release-versions builds list: %w", err)
				}
				return shared.PrintOutput(resp, *output, *pretty)
			}

			resp, err := client.GetPreReleaseVersionBuilds(requestCtx, idValue, opts...)
			if err != nil {
				return fmt.Errorf("pre-release-versions builds list: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output, *pretty)
		},
	}
}
