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

// newBuildsMetricsCommand returns the builds metrics command group.
func newBuildsMetricsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "metrics",
		ShortUsage: "asc builds metrics <subcommand> [flags]",
		ShortHelp:  "Fetch build metrics.",
		LongHelp: `Fetch build metrics.

Examples:
  asc builds metrics beta-usages --build-id "BUILD_ID"
  asc builds metrics beta-usages --app "123456789" --latest`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newBuildsMetricsBetaUsagesCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newBuildsMetricsBetaUsagesCommand returns the beta usages metrics subcommand.
func newBuildsMetricsBetaUsagesCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("metrics beta-usages", flag.ExitOnError)

	selectors := bindBuildSelectorFlags(d, fs, buildSelectorFlagOptions{})
	limit := fs.Int("limit", 0, shared.LimitFlagUsage(200))
	next := fs.String("next", "", shared.NextFlagUsage)
	output := d.BindOutputFlags(fs)

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
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := d.ValidatePaginationFlags(ctx, fs, "builds metrics beta-usages", 200); err != nil {
				return err
			}

			nextValue := strings.TrimSpace(*next)
			if nextValue == "" {
				if err := selectors.validate(); err != nil {
					return err
				}
			} else if err := selectors.validateNextPageSelectorFlags(); err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			client, err := d.Client()
			if err != nil {
				return fmt.Errorf("builds metrics beta-usages: %w", err)
			}

			requestCtx, cancel := d.Timeout(ctx)
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

			return d.PrintOutput(resp, *output.Output, *output.Pretty)
		},
	}
}
