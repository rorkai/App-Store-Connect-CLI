package shots

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// ShotsRunCommand returns the screenshots run subcommand.
func ShotsRunCommand() *ffcli.Command {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	planPath := fs.String("plan", ".asc/screenshots.json", "Path to screenshot run plan JSON")
	bundleID := fs.String("bundle-id", "", "Override app bundle ID from plan")
	udid := fs.String("udid", "", "Override simulator UDID from plan")
	outputDir := fs.String("output-dir", "", "Override output directory from plan")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "run",
		ShortUsage: "asc screenshots run [--plan .asc/screenshots.json] [flags]",
		ShortHelp:  "Run a deterministic screenshot sequence from JSON.",
		LongHelp: `Run a deterministic screenshot automation sequence.

By default it loads .asc/screenshots.json from the current project root.
Supported actions: launch, tap, type, wait, wait_for (polling), screenshot.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			planPathVal := strings.TrimSpace(*planPath)
			if planPathVal == "" {
				fmt.Fprintln(os.Stderr, "Error: --plan is required")
				return flag.ErrHelp
			}

			absPlanPath, err := filepath.Abs(planPathVal)
			if err != nil {
				return fmt.Errorf("screenshots run: resolve plan path: %w", err)
			}

			plan, err := screenshots.LoadPlanUnvalidated(absPlanPath)
			if err != nil {
				return fmt.Errorf("screenshots run: %w", err)
			}

			if override := strings.TrimSpace(*bundleID); override != "" {
				plan.App.BundleID = override
			}
			if override := strings.TrimSpace(*udid); override != "" {
				plan.App.UDID = override
			}
			if override := strings.TrimSpace(*outputDir); override != "" {
				plan.App.OutputDir = override
			}

			result, err := screenshots.RunPlan(ctx, plan)
			if err != nil {
				return fmt.Errorf("screenshots run: %w", err)
			}

			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
