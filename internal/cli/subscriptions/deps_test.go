package subscriptions

import (
	"bytes"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// newSubscriptionsTestDeps returns deps whose streams are buffers, and the
// stderr buffer.
func newSubscriptionsTestDeps() (*shared.Deps, *bytes.Buffer) {
	var stderr bytes.Buffer
	return &shared.Deps{IO: &shared.IO{Out: &bytes.Buffer{}, ErrOut: &stderr}}, &stderr
}
