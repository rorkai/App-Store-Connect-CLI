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

const (
	defaultShotsReviewRawDir    = "./screenshots/raw"
	defaultShotsReviewFramedDir = "./screenshots/framed"
	defaultShotsReviewOutputDir = "./screenshots/review"
)

// ShotsReviewGenerateCommand returns screenshots review-generate subcommand.
func ShotsReviewGenerateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review-generate", flag.ExitOnError)
	rawDir := fs.String("raw-dir", defaultShotsReviewRawDir, "Directory containing raw screenshots (optional)")
	framedDir := fs.String("framed-dir", defaultShotsReviewFramedDir, "Directory containing framed screenshots (required)")
	outputDir := fs.String("output-dir", defaultShotsReviewOutputDir, "Directory to write HTML and JSON review artifacts")
	approvalPath := fs.String("approval-path", "", "Optional approvals file path (default: <output-dir>/approved.json)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "review-generate",
		ShortUsage: "asc screenshots review-generate [flags]",
		ShortHelp:  "Generate HTML side-by-side review and JSON manifest.",
		LongHelp: `Generate review artifacts for screenshots:

- HTML report for visual QA (raw vs framed side-by-side)
- JSON manifest for agent checks (size, locale/device grouping, approval state)`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			framed := strings.TrimSpace(*framedDir)
			if framed == "" {
				fmt.Fprintln(os.Stderr, "Error: --framed-dir is required")
				return flag.ErrHelp
			}

			result, err := screenshots.GenerateReview(ctx, screenshots.ReviewRequest{
				RawDir:       strings.TrimSpace(*rawDir),
				FramedDir:    framed,
				OutputDir:    strings.TrimSpace(*outputDir),
				ApprovalPath: strings.TrimSpace(*approvalPath),
			})
			if err != nil {
				return fmt.Errorf("screenshots review-generate: %w", err)
			}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
