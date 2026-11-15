package xcode

import (
	"bytes"
	"os"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newXcodeTestDeps returns deps that write to buffers and take their client
// from client, with the process clock, timeouts, timer and working directory.
func newXcodeTestDeps(client func() (*asc.Client, error)) *shared.Deps {
	return &shared.Deps{
		Client:         client,
		IO:             &shared.IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}},
		Now:            time.Now,
		Timeout:        shared.ContextWithTimeout,
		ResolveTimeout: shared.ResolveTimeoutSetting,
		After:          time.After,
		Getwd:          os.Getwd,
	}
}

// depsOutput returns what d's stdout and stderr buffers hold and empties them.
func depsOutput(d *shared.Deps) (string, string) {
	stdout, stderr := d.IO.Out.(*bytes.Buffer), d.IO.ErrOut.(*bytes.Buffer)
	defer stdout.Reset()
	defer stderr.Reset()
	return stdout.String(), stderr.String()
}
