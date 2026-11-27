package diagnosticoutput

import (
	"io"
	"sync/atomic"
)

var suppressed atomic.Bool

// Suppress silences legacy process diagnostics for the active CLI invocation.
func Suppress() func() {
	previous := suppressed.Swap(true)
	return func() { suppressed.Store(previous) }
}

// Writer wraps a legacy diagnostic destination. Interactive prompts and child
// streams use their own destinations.
func Writer(destination io.Writer) io.Writer {
	return writer{destination: destination}
}

type writer struct {
	destination io.Writer
}

func (w writer) Write(p []byte) (int, error) {
	if suppressed.Load() {
		return len(p), nil
	}
	return w.destination.Write(p)
}
