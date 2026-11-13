package cmdtest

import (
	"net/http"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func installDefaultTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()

	asctest.SwapDefaultTransport(t, transport)
}

type lockedCounter struct {
	mu sync.Mutex
	n  int
}

func (c *lockedCounter) Inc() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.n++
	return c.n
}

func (c *lockedCounter) Load() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.n
}

type requestLog struct {
	mu      sync.Mutex
	entries []string
}

func newRequestLog(capacity int) *requestLog {
	return &requestLog{entries: make([]string, 0, capacity)}
}

func (l *requestLog) Add(entry string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)
}

func (l *requestLog) Snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.entries...)
}
