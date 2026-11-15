package docs

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newShowCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("docs show", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "show",
		ShortUsage: "asc docs show <guide>",
		ShortHelp:  "Print an embedded documentation guide.",
		LongHelp: `Print an embedded documentation guide.

Examples:
  asc docs show api-notes
  asc docs show reference
  asc docs show third-party-notices`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			_ = ctx
			if len(args) == 0 {
				return flag.ErrHelp
			}
			if len(args) > 1 {
				fmt.Fprintln(d.IO.ErrOut, "Error: docs show accepts exactly one guide name")
				return flag.ErrHelp
			}

			guideName := strings.TrimSpace(args[0])
			guide, ok := findGuide(guideName)
			if !ok {
				safeName := shared.SanitizeTerminal(guideName)
				fmt.Fprintf(d.IO.ErrOut, "Error: unknown guide %q\n", safeName)
				fmt.Fprintf(d.IO.ErrOut, "Available guides: %s\n", strings.Join(guideSlugs(), ", "))
				return flag.ErrHelp
			}

			content := guide.Content
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			fmt.Fprint(d.IO.Out, content)
			return nil
		},
	}
}
