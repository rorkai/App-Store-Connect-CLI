package shots

import (
	"bytes"
	"os"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newShotsTestDeps returns deps that write to buffers, with the process
// clock, timers, timeouts and working directory and no App Store Connect
// client. It returns the stdout and stderr buffers.
func newShotsTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		IO:      &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:     time.Now,
		Timeout: shared.ContextWithTimeout,
		After:   time.After,
		Getwd:   os.Getwd,
	}, &stdout, &stderr
}

// newShotsTestDepsOnly is newShotsTestDeps without the buffers.
func newShotsTestDepsOnly() *shared.Deps {
	d, _, _ := newShotsTestDeps()
	return d
}
