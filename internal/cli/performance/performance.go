package performance

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewPerformanceCommand returns the performance command group.
func NewPerformanceCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("performance", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "performance",
		ShortUsage: "asc performance <subcommand> [flags]",
		ShortHelp:  "Access performance metrics and diagnostic logs.",
		LongHelp: `Access performance metrics and diagnostic logs.

Examples:
  asc performance metrics list --app "APP_ID"
  asc performance metrics view --build-id "BUILD_ID"
  asc performance diagnostics list --build-id "BUILD_ID"
  asc performance diagnostics view --id "SIGNATURE_ID"
  asc performance download --build-id "BUILD_ID" --output ./metrics.json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newPerformanceMetricsCommand(d),
			newPerformanceOverviewCommand(d),
			newPerformanceDiagnosticsCommand(d),
			newPerformanceDownloadCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
