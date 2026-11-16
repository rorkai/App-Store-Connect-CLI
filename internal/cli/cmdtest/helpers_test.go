package cmdtest

import (
	"errors"
	"flag"
	"testing"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/auth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// isUsageClassError reports whether err maps to usage exit code 2. Commands
// reach that classification either by wrapping flag.ErrHelp, which makes ffcli
// render the full help page, or by returning a concise reported usage error
// whose message already stands on its own.
func isUsageClassError(err error) bool {
	return errors.Is(err, flag.ErrHelp) || shared.IsReportedUsageError(err)
}

func resetCmdtestState() {
	asc.ResetConfigCacheForTest()
	auth.ResetInvalidBypassKeychainWarningsForTest()
	shared.ResetDefaultOutputFormat()
	shared.ResetTierCacheForTest()
}

func setCmdtestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// rootNow is the clock of the roots RootCommand builds while a test has set
// one with setRootNow; nil means the process dependencies' clock.
var rootNow func() time.Time

// setRootNow makes RootCommand build roots whose clock is now and returns a
// function that restores the previous clock.
func setRootNow(now func() time.Time) func() {
	previous := rootNow
	rootNow = now
	return func() { rootNow = previous }
}

func RootCommand(version string) *ffcli.Command {
	resetCmdtestState()
	if rootNow != nil {
		d := shared.DefaultDeps()
		d.Now = rootNow
		return cmd.RootCommandWithDeps(version, d)
	}
	return cmd.RootCommand(version)
}

type ReportedError = shared.ReportedError
