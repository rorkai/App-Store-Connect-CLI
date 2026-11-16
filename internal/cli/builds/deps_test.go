package builds

import (
	"bytes"
	"net/http"
	"os"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBuildsTestDeps returns deps that write to buffers and take their client
// from client, with the process clock, timeouts, HTTP transport and working
// directory. It returns the stderr buffer.
func newBuildsTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:        client,
		IO:            &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:           time.Now,
		Timeout:       shared.ContextWithTimeout,
		UploadTimeout: shared.ContextWithUploadTimeout,
		LookupEnv:     os.LookupEnv,
		HTTPTransport: func() http.RoundTripper { return http.DefaultTransport },
		Getwd:         os.Getwd,
	}, &stderr
}

// dsymDownloadTestDeps is newBuildsTestDeps whose HTTP clients send to
// transport.
func dsymDownloadTestDeps(transport http.RoundTripper) *shared.Deps {
	d, _ := newBuildsTestDeps(nil)
	d.HTTPTransport = func() http.RoundTripper { return transport }
	return d
}
