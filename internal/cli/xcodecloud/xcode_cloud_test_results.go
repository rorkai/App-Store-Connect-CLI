package xcodecloud

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newXcodeCloudTestResultsCommand returns the xcode-cloud test-results command with subcommands.
func newXcodeCloudTestResultsCommand(d *shared.Deps) *ffcli.Command {
	return newXcodeCloudActionResourceCommand(d, xcodeCloudTestResultsCommandConfig)
}
