package cmdtest

import (
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// appsDeps returns test deps whose client answers from reg, a root built on
// them, and capture, which runs fn and returns what it wrote to the deps'
// stdout and stderr.
func appsDeps(t *testing.T, reg *asctest.Registry) (*shared.Deps, *ffcli.Command, func(func()) (string, string)) {
	t.Helper()

	d, stdout, stderr := newTestDeps(t, stubClient(t, reg))
	capture := func(fn func()) (string, string) {
		stdout.Reset()
		stderr.Reset()
		fn()
		return stdout.String(), stderr.String()
	}
	return d, rootcmd.RootCommandWithDeps("1.2.3", d), capture
}
