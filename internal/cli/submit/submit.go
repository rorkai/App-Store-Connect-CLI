package submit

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func SubmitCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc submit <subcommand> [flags]",
		ShortHelp:  "Submission lifecycle tools; use `publish appstore --submit` to ship.",
		LongHelp: `Submission lifecycle tools for App Store review.

Use:
  - asc publish appstore --submit for the canonical high-level App Store shipping path
  - asc validate for canonical readiness checks before submission
  - asc submit status/cancel for lower-level review submission lifecycle work`,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			SubmitStatusCommand(),
			SubmitCancelCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				switch strings.TrimSpace(args[0]) {
				case "create":
					fmt.Fprintln(os.Stderr, "Error: `asc submit create` was removed. Use `asc review submit` for already-uploaded builds, or `asc publish appstore --submit` for the full shipping path.")
					return flag.ErrHelp
				case "preflight":
					fmt.Fprintln(os.Stderr, "Error: `asc submit preflight` was removed. Use `asc validate` instead.")
					return flag.ErrHelp
				}
			}
			return flag.ErrHelp
		},
	}
}
