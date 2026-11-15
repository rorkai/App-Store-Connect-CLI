package xcodecloud

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newXcodeCloudIssuesCommand returns the xcode-cloud issues command with subcommands.
func newXcodeCloudIssuesCommand(d *shared.Deps) *ffcli.Command {
	return newXcodeCloudActionResourceCommand(d, xcodeCloudIssuesCommandConfig)
}
