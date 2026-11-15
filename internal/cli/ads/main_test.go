package ads

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestMain disables retry backoff by default; retry tests opt back in with t.Setenv.
func TestMain(m *testing.M) {
	_ = os.Setenv("ASC_MAX_RETRIES", "0")
	os.Exit(m.Run())
}

// newAdsTestDeps returns deps with an empty stdin and buffered stdout and
// stderr. Members a test does not need are nil.
func newAdsTestDeps() *shared.Deps {
	return &shared.Deps{IO: &shared.IO{In: strings.NewReader(""), Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}}
}
