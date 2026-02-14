package shots

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// ShotsReviewOpenCommand returns screenshots review-open subcommand.
func ShotsReviewOpenCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-open", flag.ExitOnError)
	outputDir := fs.String("output-dir", defaultShotsReviewOutputDir, "Directory containing review artifacts")
	htmlPath := fs.String("html-path", "", "Optional HTML path (default: <output-dir>/index.html)")
	dryRun := fs.Bool("dry-run", false, "Resolve path and print output without opening browser")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "review-open",
		ShortUsage: "asc screenshots review-open [flags]",
		ShortHelp:  "Open review HTML report in the default browser.",
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*outputDir) == "" && strings.TrimSpace(*htmlPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --output-dir or --html-path is required")
				return flag.ErrHelp
			}

			result, err := screenshots.OpenReview(ctx, screenshots.ReviewOpenRequest{
				OutputDir: strings.TrimSpace(*outputDir),
				HTMLPath:  strings.TrimSpace(*htmlPath),
				DryRun:    *dryRun,
			})
			if err != nil {
				return fmt.Errorf("screenshots review-open: %w", err)
			}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
