package builds

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
		UsageFunc: DefaultUsageFunc,
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
	output := fs.String("output", "json", "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "beta-usages",
		ShortUsage: "asc builds metrics beta-usages --build \"BUILD_ID\" [flags]",
		ShortHelp:  "Fetch beta build usage metrics for a build.",
		LongHelp: `Fetch beta build usage metrics for a build.

Examples:
  asc builds metrics beta-usages --build "BUILD_ID"
  asc builds metrics beta-usages --build "BUILD_ID" --limit 50`,
		FlagSet:   fs,
		UsageFunc: DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if *limit != 0 && (*limit < 1 || *limit > 200) {
				return fmt.Errorf("builds metrics beta-usages: --limit must be between 1 and 200")
			}
			if err := validateNextURL(*next); err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			buildValue := strings.TrimSpace(*buildID)
			if buildValue == "" && strings.TrimSpace(*next) == "" {
				fmt.Fprintln(os.Stderr, "Error: --build is required")
				return flag.ErrHelp
			}

			client, err := getASCClient()
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			requestCtx, cancel := contextWithTimeout(ctx)
			defer cancel()

			opts := []asc.BetaBuildUsagesOption{
				asc.WithBetaBuildUsagesLimit(*limit),
				asc.WithBetaBuildUsagesNextURL(*next),
			}

			resp, err := client.GetBuildBetaUsagesMetrics(requestCtx, buildValue, opts...)
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: failed to fetch: %w", err)
			}

			return printOutput(resp, *output, *pretty)
		},
	}
}
