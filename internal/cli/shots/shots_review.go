package shots

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ShotsReviewCommand returns the shots review command group.
func ShotsReviewCommand() *ffcli.Command {
	fs := flag.NewFlagSet("review", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "review",
		ShortUsage: "asc shots review <subcommand> [flags]",
		ShortHelp:  "Generate review artifacts for screenshot QA.",
		LongHelp: `Generate side-by-side review artifacts for screenshot quality checks.

Use "generate" to write both a static HTML report and a JSON manifest.
Use "open" to open the HTML report in your browser.
Use "approve" to update approved.json from CLI selectors.`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			ShotsReviewGenerateCommand(),
			ShotsReviewOpenCommand(),
			ShotsReviewApproveCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
