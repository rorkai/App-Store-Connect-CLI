package submit

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewSubmitCommand returns the submit command group, built on d.
func NewSubmitCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("submit", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "submit",
		ShortUsage: "asc submit <subcommand> [flags]",
		ShortHelp:  "Submission lifecycle tools; use `review submit` for an existing build.",
		LongHelp: `Submission lifecycle tools for App Store review.

Use:
  - asc review submit for an already-uploaded build and prepared App Store version
  - asc publish appstore --submit for the canonical high-level path with a new upload
  - asc validate for canonical readiness checks before submission
  - asc submit status/cancel for lower-level review submission lifecycle work

Submit an existing version and build:
  asc review submit --app "APP_ID" --version-id "VERSION_ID" --build-id "BUILD_ID" --platform "IOS" --confirm`,
		FlagSet:   fs,
		UsageFunc: d.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			newSubmitStatusCommand(d),
			newSubmitCancelCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				switch strings.TrimSpace(args[0]) {
				case "create":
					fmt.Fprintln(d.IO.ErrOut, "Error: `asc submit create` was removed. Use `asc review submit` for already-uploaded builds, or `asc publish appstore --submit` for the full shipping path.")
					return flag.ErrHelp
				case "preflight":
					fmt.Fprintln(d.IO.ErrOut, "Error: `asc submit preflight` was removed. Use `asc validate` instead.")
					return flag.ErrHelp
				}
			}
			return flag.ErrHelp
		},
	}
}
