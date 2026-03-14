package docs

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// DocsCommand returns the docs command group.
func DocsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("docs", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "docs",
		ShortUsage: "asc docs <subcommand> [flags]",
		ShortHelp:  "Access embedded documentation guides and reference helpers.",
		LongHelp: `Access embedded documentation guides and reference helpers.

Examples:
  asc docs list
  asc docs show api-notes
  asc docs init
  asc docs init --path ./ASC.md
  asc docs init --force --link=false`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			DocsListCommand(),
			DocsShowCommand(),
			DocsInitCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", args[0])
			return flag.ErrHelp
		},
	}
}
