package diffcmd

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// NewDiffCommand returns the root diff command group built on d.
func NewDiffCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("diff", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "diff",
		ShortUsage: "asc diff <subcommand> [flags]",
		ShortHelp:  "Generate deterministic non-mutating diff plans.",
		LongHelp: `Generate deterministic non-mutating diff plans.

Examples:
  asc diff localizations --app "APP_ID" --path "./metadata/localizations" --version "VERSION_ID"
  asc diff localizations --app "APP_ID" --from-version "VERSION_ID_A" --to-version "VERSION_ID_B"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newDiffLocalizationsCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
