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
  asc builds metrics beta-usages --build "BUILD_ID"`,
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

	buildID := fs.String("build", "", "Build ID")
	limit := fs.Int("limit", 0, "Maximum results per page (1-200)")
	next := fs.String("next", "", "Fetch next page using a links.next URL")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "beta-usages",
		ShortUsage: "asc builds metrics beta-usages --build \"BUILD_ID\" [flags]",
		ShortHelp:  "Fetch beta build usage metrics for a build.",
		LongHelp: `Fetch beta build usage metrics for a build.

Examples:
  asc builds metrics beta-usages --build "BUILD_ID"
  asc builds metrics beta-usages --build "BUILD_ID" --limit 50`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				fmt.Fprintln(os.Stderr, "Error: --limit must be between 1 and 200")
				return flag.ErrHelp
			}
			if err := shared.ValidateNextURL(*next); err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaBuildUsagesOption{
				asc.WithBetaBuildUsagesLimit(*limit),
				asc.WithBetaBuildUsagesNextURL(*next),
			}

			resp, err := client.GetBuildBetaUsagesMetrics(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: failed to fetch: %w", err)
			}

			return shared.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
