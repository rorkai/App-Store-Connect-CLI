package cmdtest

import (
	"github.com/peterbourgon/ff/v3/ffcli"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func resetCmdtestState() {
	asc.ResetConfigCacheForTest()
	auth.ResetInvalidBypassKeychainWarningsForTest()
	shared.ResetDefaultOutputFormat()
	shared.ResetTierCacheForTest()
}

func RootCommand(version string) *ffcli.Command {
	resetCmdtestState()
	return cmd.RootCommand(version)
}

type ReportedError = shared.ReportedError
