package docs

import (
	"context"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// DocsListCommand returns the docs list subcommand.
func DocsListCommand() *ffcli.Command {
	fs := flag.NewFlagSet("docs list", flag.ExitOnError)
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "list",
		ShortUsage: "asc docs list [flags]",
		ShortHelp:  "List available embedded documentation guides.",
		LongHelp: `List available embedded documentation guides.

Examples:
  asc docs list
  asc docs list --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			if err := shared.PrintOutputWithRenderers(
				listGuideSummaries(),
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"slug", "description"}, guideRows())
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"slug", "description"}, guideRows())
					return nil
				},
			); err != nil {
				return fmt.Errorf("docs list: %w", err)
			}
			return nil
		},
	}
}
