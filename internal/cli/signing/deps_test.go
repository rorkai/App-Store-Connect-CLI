package signing

import (
	"bytes"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSigningTestDeps returns deps that take their client from client and
// write to the returned stdout and stderr buffers, with the process clock and
// timer.
func newSigningTestDeps(client func() (*asc.Client, error)) (*shared.Deps, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	return &shared.Deps{
		Client: client,
		IO:     &shared.IO{In: strings.NewReader(""), Out: &stdout, ErrOut: &stderr},
		Now:    time.Now,
		After:  time.After,
	}, &stdout, &stderr
}
