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

// BuildsMetricsCommand returns the builds metrics command group.
func BuildsMetricsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc builds metrics <subcommand> [flags]",
		ShortHelp:  "Fetch build metrics.",
		LongHelp: `Fetch build metrics.

Examples:
  asc builds metrics beta-usages --build-id "BUILD_ID"
  asc builds metrics beta-usages --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			BuildsMetricsBetaUsagesCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// BuildsMetricsBetaUsagesCommand returns the beta usages metrics subcommand.
func BuildsMetricsBetaUsagesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metrics beta-usages", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(fs, buildSelectorFlagOptions{})
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "beta-usages",
		ShortUsage: "asc builds metrics beta-usages (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION]) [flags]",
		ShortHelp:  "Fetch beta build usage metrics for a build.",
		LongHelp: `Fetch beta build usage metrics for a build.

Examples:
  asc builds metrics beta-usages --build-id "BUILD_ID"
  asc builds metrics beta-usages --app "123456789" --latest
  asc builds metrics beta-usages --app "123456789" --latest --limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return shared.UsageErrorf("builds metrics beta-usages: %v", err)
			}

			nextValue := strings.TrimSpace(*next)
			if nextValue == "" {
				if err := selectors.validate(); err != nil {
					return err
				}
			} else if err := selectors.validateNextPageSelectorFlags(); err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			buildID := ""
			if nextValue == "" {
				buildID, err = selectors.resolveBuildID(requestCtx, client)
				if err != nil {
					return fmt.Errorf("builds metrics beta-usages: %w", err)
				}
			}

			opts := []asc.BetaBuildUsagesOption{
				asc.Limit[asc.BetaBuildUsagesOption](*limit),
				asc.NextURL[asc.BetaBuildUsagesOption](*next),
			}

			resp, err := client.GetBuildBetaUsagesMetrics(requestCtx, buildID, opts...)
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
