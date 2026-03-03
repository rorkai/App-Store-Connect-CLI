package docs

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// DocsShowCommand returns the docs show subcommand.
func DocsShowCommand() *ffcli.Command {
	fs := flag.NewFlagSet("docs show", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "show",
		ShortUsage: "asc docs show <workflows|api-notes|reference>",
		ShortHelp:  "Print an embedded documentation guide.",
		LongHelp: `Print an embedded documentation guide.

Examples:
  asc docs show workflows
  asc docs show api-notes
  asc docs show reference`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			if len(args) == 0 {
				return flag.ErrHelp
			}
			if len(args) > 1 {
				fmt.Fprintln(os.Stderr, "Error: docs show accepts exactly one guide name")
				return flag.ErrHelp
			}

			guideName := strings.TrimSpace(args[0])
			guide, ok := findGuide(guideName)
			if !ok {
				safeName := shared.SanitizeTerminal(guideName)
				_, _ = fmt.Fprintf(os.Stderr, "Error: unknown guide %q\n", safeName)
				_, _ = fmt.Fprintf(os.Stderr, "Available guides: %s\n", strings.Join(guideSlugs(), ", "))
				return flag.ErrHelp
			}

			content := guide.Content
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			_, _ = fmt.Fprint(os.Stdout, content)
			return nil
		},
	}
}
