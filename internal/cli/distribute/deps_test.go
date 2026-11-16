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
