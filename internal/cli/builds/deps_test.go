package builds

import (
	"bytes"
	"net/http"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newBuildsTestDeps returns deps that write to buffers and take their client
// from client, with the process clock, timeouts and HTTP transport. It
// returns the stderr buffer.
func newBuildsTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client:        client,
		IO:            &shared.IO{Out: &stdout, ErrOut: &stderr},
		Now:           time.Now,
		Timeout:       shared.ContextWithTimeout,
		UploadTimeout: shared.ContextWithUploadTimeout,
		HTTPTransport: func() http.RoundTripper { return http.DefaultTransport },
	}, &stderr
}
