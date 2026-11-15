package signing

import (
	"bytes"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSigningTestDeps returns deps that write to the returned stdout and stderr
// buffers, with the process clock and timer and no client.
func newSigningTestDeps() (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		IO:    &shared.IO{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
		Now:   time.Now,
		After: time.After,
	}, &stdout, &stderr
}
