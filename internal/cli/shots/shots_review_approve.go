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

// ShotsReviewApproveCommand returns shots review approve subcommand.
func ShotsReviewApproveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("approve", flag.ExitOnError)
	outputDir := fs.String("output-dir", defaultShotsReviewOutputDir, "Directory containing review artifacts")
	manifestPath := fs.String("manifest-path", "", "Optional manifest path (default: <output-dir>/manifest.json)")
	approvalPath := fs.String("approval-path", "", "Optional approvals path (default: <output-dir>/approved.json)")
	allReady := fs.Bool("all-ready", false, "Approve all entries with status=ready")
	key := fs.String("key", "", "Review key(s) to approve, comma-separated (locale|device|screenshot_id)")
	screenshotID := fs.String("id", "", "Screenshot ID to approve")
	locale := fs.String("locale", "", "Optional locale filter (works with --all-ready or --id)")
	device := fs.String("device", "", "Optional device filter (works with --all-ready or --id)")
	output := fs.String("output", shared.DefaultOutputFormat(), "Output format: json (default), table, markdown")
	pretty := fs.Bool("pretty", false, "Pretty-print JSON output")

	return &ffcli.Command{
		Name:       "approve",
		ShortUsage: "asc shots review approve [--all-ready | --key key1,key2 | --id home] [flags]",
		ShortHelp:  "Write/update approved.json from review manifest selectors.",
		LongHelp: `Approve review entries and persist to approved.json.

Selectors:
- --all-ready: approve all status=ready entries
- --key: approve exact review key(s), comma-separated
- --id: approve by screenshot ID (optionally narrowed by --locale/--device)`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			keys := shared.SplitCSV(*key)
			id := strings.TrimSpace(*screenshotID)
			if !*allReady && len(keys) == 0 && id == "" {
				fmt.Fprintln(os.Stderr, "Error: provide at least one selector: --all-ready, --key, or --id")
				return flag.ErrHelp
			}

			result, err := screenshots.ApproveReview(ctx, screenshots.ReviewApproveRequest{
				OutputDir:    strings.TrimSpace(*outputDir),
				ManifestPath: strings.TrimSpace(*manifestPath),
				ApprovalPath: strings.TrimSpace(*approvalPath),
				AllReady:     *allReady,
				Keys:         keys,
				ScreenshotID: id,
				Locale:       strings.TrimSpace(*locale),
				Device:       strings.TrimSpace(*device),
			})
			if err != nil {
				return fmt.Errorf("shots review approve: %w", err)
			}
			return shared.PrintOutput(result, *output, *pretty)
		},
	}
}
