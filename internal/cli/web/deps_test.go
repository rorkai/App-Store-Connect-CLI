package web

import (
	"bytes"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func newWebTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		IO:      &shared.IO{Out: &stdout, ErrOut: &stderr},
		Timeout: shared.ContextWithTimeout,
		After:   time.After,
	}, &stdout, &stderr
}

// runWithWebTestDeps runs fn and returns what it wrote to the deps streams.
func runWithWebTestDeps(stdout, stderr *bytes.Buffer, fn func()) (string, string) {
	fn()
	return stdout.String(), stderr.String()
}
