package publish

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// publishTestClientWithTimeout is the client a test's commands get from their
// deps' ClientWithTimeout; overridePublishCommandTestHooks restores it.
var publishTestClientWithTimeout = shared.GetASCClientWithTimeout

// newPublishTestDeps returns deps that write to buffers, with the process
// clock, timers, timeouts and working directory, and the client from
// publishTestClientWithTimeout.
func newPublishTestDeps() *shared.Deps {
	return &shared.Deps{
		ClientWithTimeout: func(timeout time.Duration) (*asc.Client, error) {
			return publishTestClientWithTimeout(timeout)
		},
		IO:             &shared.IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
		Now:            time.Now,
		Timeout:        shared.ContextWithTimeout,
		UploadTimeout:  shared.ContextWithUploadTimeout,
		ResolveTimeout: shared.ResolveTimeoutSetting,
		After:          time.After,
		Getwd:          os.Getwd,
	}
}
