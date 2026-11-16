package distribute

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newDistributeTestDeps returns deps that write to buffers, with the process
// clock, timers, timeouts and working directory and no App Store Connect
// client.
func newDistributeTestDeps() *shared.Deps {
	return &shared.Deps{
		IO:             &shared.IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
		Now:            time.Now,
		Timeout:        shared.ContextWithTimeout,
		UploadTimeout:  shared.ContextWithUploadTimeout,
		ResolveTimeout: shared.ResolveTimeoutSetting,
		After:          time.After,
		Getwd:          os.Getwd,
	}
}

// distributionTestNow is the clock of the orchestration test dependencies.
var distributionTestNow = time.Date(2026, 8, 13, 8, 0, 0, 0, time.UTC)

// distributionTestDepsAt returns newDistributeTestDeps with its clock at now.
func distributionTestDepsAt(now time.Time) *shared.Deps {
	d := newDistributeTestDeps()
	d.Now = func() time.Time { return now }
	return d
}
