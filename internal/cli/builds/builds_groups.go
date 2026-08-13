package builds

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/testflight"
)

// BuildsGroupsCommand returns the build beta-groups command group.
func BuildsGroupsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("groups", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "groups",
		ShortUsage: "asc builds groups <subcommand> [flags]",
		ShortHelp:  "View TestFlight beta groups for builds.",
		LongHelp: `View TestFlight beta groups for builds.

Examples:
  asc builds groups list --build-id "BUILD_ID"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			testflight.BuildGroupsListCommand(testflight.BuildGroupsListCommandConfig{
				ShortUsage: "asc builds groups list --build-id \"BUILD_ID\" [flags]",
				ShortHelp:  "[experimental] List TestFlight beta groups that contain a build.",
				LongHelp: `List TestFlight beta groups that contain a build.

The membership lookup is experimental. It resolves the build's app and
automatically fetches all required pages. The lookup uses the documented
betaGroups build filter and checks inverse group-to-build relationships for
all-build access. The command returns the same structured membership result as
asc testflight groups list --build-id.

Examples:
  asc builds groups list --build-id "BUILD_ID"
  asc builds groups list --build-id "BUILD_ID" --output table`,
				ErrorPrefix: "builds groups list",
			}),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}
