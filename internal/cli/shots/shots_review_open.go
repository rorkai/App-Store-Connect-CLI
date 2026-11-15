package shots

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// NewShotsReviewOpenCommand returns screenshots review-open subcommand.
func NewShotsReviewOpenCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("review-open", flag.ExitOnError)
	outputDir := fs.String("output-dir", defaultShotsReviewOutputDir, "Directory containing review artifacts")
	htmlPath := fs.String("html-path", "", "Optional HTML path (default: <output-dir>/index.html)")
	dryRun := fs.Bool("dry-run", false, "Resolve path and print output without opening browser")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "review-open",
		ShortUsage: "asc screenshots review-open [flags]",
		ShortHelp:  "Open review HTML report in the default browser.",
		FlagSet:    fs,
		UsageFunc:  d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*outputDir) == "" && strings.TrimSpace(*htmlPath) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: --output-dir or --html-path is required")
				return shared.MissingRequiredUsageError("")
			}

			result, err := screenshots.OpenReview(ctx, screenshots.ReviewOpenRequest{
				OutputDir: userPath(d, *outputDir),
				HTMLPath:  *htmlPath,
				DryRun:    *dryRun,
			})
			if err != nil {
				return fmt.Errorf("screenshots review-open: %w", err)
			}
			return d.PrintOutput(result, *output.Output, *output.Pretty)
		},
	}
}
