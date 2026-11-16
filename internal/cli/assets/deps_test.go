package assets

import (
	"bytes"
	"net/http"
	"os"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newAssetsTestDeps returns deps that write to buffers and take their client
// from client, with the process clock, timers, timeouts, working directory
// and HTTP transport. It returns the stdout and stderr buffers.
func newAssetsTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:         client,
		IO:             &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:            time.Now,
		Timeout:        shared.ContextWithTimeout,
		UploadTimeout:  shared.ContextWithUploadTimeout,
		ResolveTimeout: shared.ResolveTimeoutSetting,
		After:          time.After,
		Getwd:          os.Getwd,
		HTTPTransport:  func() http.RoundTripper { return http.DefaultTransport },
		LookupEnv:      os.LookupEnv,
	}, &stdout, &stderr
}

// newAssetsUploadTestDeps is newAssetsTestDeps for helpers that take their
// client as an argument.
func newAssetsUploadTestDeps() *shared.Deps {
	d, _, _ := newAssetsTestDeps(nil)
	return d
}

// newAssetsClientTestDeps is newAssetsTestDeps without the buffers.
func newAssetsClientTestDeps(client func() (*asc.Client, error)) *shared.Deps {
	d, _, _ := newAssetsTestDeps(client)
	return d
}
