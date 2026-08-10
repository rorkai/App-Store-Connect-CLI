package release

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// ReleaseCommand returns the top-level release command group.
func ReleaseCommand() *ffcli.Command {
	fs := flag.NewFlagSet("release", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "release",
		ShortUsage: "asc release <subcommand> [flags]",
		ShortHelp:  "Run high-level App Store release workflows.",
		LongHelp: `Run high-level App Store release workflows.

release stage prepares a version for review without submitting it. It orchestrates:
  1. Ensure/create version
  2. Apply metadata and localizations
  3. Reconcile optional routing app coverage
  4. Attach selected build
  5. Run readiness checks

For the canonical App Store shipping command, use:
  asc publish appstore --app "APP_ID" --ipa app.ipa --version "VERSION" --submit --confirm

After submission, monitor progress with:
  asc status --app "APP_ID"

For lower-level submission lifecycle control, use:
  asc validate --app "APP_ID" --version "VERSION"
  asc submit status --version-id "VERSION_ID"
  asc submit cancel --version-id "VERSION_ID" --confirm

Examples:
  asc release stage --app "APP_ID" --version "2.4.0" --build "BUILD_ID" --copy-metadata-from "2.3.2" --dry-run
  asc publish appstore --app "APP_ID" --ipa app.ipa --version "2.4.0" --submit --confirm
  asc status --app "APP_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.VisibleUsageFunc,
		Subcommands: []*ffcli.Command{
			ReleaseStageCommand(),
		},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}
}
