package shared

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/diagnosticoutput"
)

const spinnerDisabledEnvVar = "ASC_SPINNER_DISABLED"

var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

const spinnerTickRate = 120 * time.Millisecond

// SpinnerEnabled reports whether the CLI should render an indeterminate spinner
// on stderr for the current run.
//
// Requirements:
//   - stderr-only (handled by WithSpinner)
//   - TTY-gated (stdout + stderr)
//   - opt-out via ASC_SPINNER_DISABLED (invalid => disabled)
//   - disabled when stderr is expected to be noisy (debug/retry logs)
func SpinnerEnabled() bool {
	return spinnerEnabledFor(isTerminal(int(os.Stdout.Fd())), isTerminal(int(os.Stderr.Fd())))
}

// SpinnerEnabled is SpinnerEnabled decided from d's stdout and stderr.
func (d *Deps) SpinnerEnabled() bool {
	return !d.IO.quietDiagnostics && spinnerEnabledFor(d.IO.StdoutTTY, d.IO.StderrTTY)
}

func spinnerEnabledFor(stdoutTTY, stderrTTY bool) bool {
	// Reuse the existing “safe to emit progress” gate (stderr TTY + tests can suppress).
	if !progressEnabledFor(stderrTTY) {
		return false
	}
	// If stdout is piped, keep stderr quiet to preserve clean stdout contracts (often JSON).
	if !stdoutTTY {
		return false
	}
	if spinnerDisabledByEnv() {
		return false
	}
	// Avoid interleaving spinner updates with debug/retry logs.
	if debugOrRetryLogsEnabled() {
		return false
	}
	return true
}

// WithSpinner runs fn while rendering an indeterminate spinner on d's stderr,
// shown only when d's stdout and stderr are terminals.
func (d *Deps) WithSpinner(label string, fn func() error) error {
	return withSpinnerTo(d.IO.ErrOut, d.SpinnerEnabled, label, fn)
}

func withSpinnerTo(w io.Writer, enabled func() bool, label string, fn func() error) (err error) {
	if fn == nil {
		return nil
	}
	if !enabled() {
		return fn()
	}

	s := newSpinner(w)
	s.Start(label)
	defer func() {
		s.Stop()
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	return fn()
}

// WithSpinnerDelayed runs fn while rendering a gh-style indeterminate spinner on stderr,
// but only starts the spinner after delay has elapsed.
//
// This avoids spinner “flicker” for fast operations while still providing feedback
// for slower operations (e.g., keychain prompts).
func WithSpinnerDelayed(label string, delay time.Duration, fn func() error) (err error) {
	return withSpinnerDelayedTo(diagnosticoutput.Writer(os.Stderr), SpinnerEnabled, time.After, label, delay, fn)
}

// WithSpinnerDelayed is WithSpinnerDelayed with the spinner on d's stderr,
// shown only when d's stdout and stderr are terminals, after d.After(delay).
func (d *Deps) WithSpinnerDelayed(label string, delay time.Duration, fn func() error) error {
	return withSpinnerDelayedTo(d.IO.ErrOut, d.SpinnerEnabled, d.After, label, delay, fn)
}

func withSpinnerDelayedTo(w io.Writer, enabled func() bool, after func(time.Duration) <-chan time.Time, label string, delay time.Duration, fn func() error) (err error) {
	if fn == nil {
		return nil
	}
	if delay <= 0 {
		return withSpinnerTo(w, enabled, label, fn)
	}
	if !enabled() {
		return fn()
	}

	s := newSpinner(w)
	done := make(chan struct{})
	started := make(chan bool, 1)

	fired := after(delay)

	go func() {
		select {
		case <-fired:
			s.Start(label)
			started <- true
		case <-done:
			started <- false
		}
	}()

	defer func() {
		close(done)
		if <-started {
			s.Stop()
		}
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	return fn()
}

func spinnerDisabledByEnv() bool {
	value, ok := os.LookupEnv(spinnerDisabledEnvVar)
	if !ok {
		return false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	switch strings.ToLower(trimmed) {
	case "0", "false", "no":
		return false
	default:
		// “invalid => disabled” is intentional to keep CI safe by default.
		return true
	}
}

// FetchFunc fetches the first page of a paginated resource.
type FetchFunc func(ctx context.Context) (asc.PaginatedResponse, error)

// PaginateWithSpinner fetches all pages with a spinner on d's stderr, shown
// only when d's stdout and stderr are terminals. It wraps both the initial
// fetch and the pagination loop so the spinner is visible even for single-page
// results.
func (d *Deps) PaginateWithSpinner(ctx context.Context, fetch FetchFunc, next asc.PaginateFunc) (asc.PaginatedResponse, error) {
	return paginateWithSpinner(ctx, d.WithSpinner, fetch, next)
}

func paginateWithSpinner(ctx context.Context, withSpinner func(string, func() error) error, fetch FetchFunc, next asc.PaginateFunc) (asc.PaginatedResponse, error) {
	var result asc.PaginatedResponse
	err := withSpinner("", func() error {
		firstPage, fetchErr := fetch(ctx)
		if fetchErr != nil {
			return fetchErr
		}
		var paginateErr error
		result, paginateErr = asc.PaginateAll(ctx, firstPage, next)
		return paginateErr
	})
	return result, err
}

func debugOrRetryLogsEnabled() bool {
	// Root-level flags should take effect immediately, even before shared.GetASCClient() applies
	// overrides into the asc package, so we need to resolve “effective” values here.

	debugEnabled := false
	if debug.IsSet() {
		debugEnabled = debug.Value()
	} else {
		debugEnabled = asc.ResolveDebugEnabled()
	}
	if debugEnabled {
		return true
	}

	// Treat --api-debug=true as “stderr will be noisy”, even if other debug flags conflict.
	if apiDebug.IsSet() && apiDebug.Value() {
		return true
	}

	retryEnabled := false
	if retryLog.IsSet() {
		retryEnabled = retryLog.Value()
	} else {
		retryEnabled = asc.ResolveRetryLogEnabled()
	}
	return retryEnabled
}

type spinner struct {
	w io.Writer

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}

	mu     sync.Mutex
	maxLen int // rune count of the longest line written (for clearing)
}

func newSpinner(w io.Writer) *spinner {
	if w == nil {
		w = io.Discard
	}
	return &spinner{
		w:      w,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (s *spinner) Start(label string) {
	label = strings.TrimSpace(label)

	// Render immediately (helps with short-running operations).
	s.renderLine(spinnerLine(spinnerFrames[0], label))

	go func() {
		ticker := time.NewTicker(spinnerTickRate)
		defer ticker.Stop()

		i := 1
		for {
			select {
			case <-s.stopCh:
				s.clearLine()
				close(s.doneCh)
				return
			case <-ticker.C:
				frame := spinnerFrames[i%len(spinnerFrames)]
				i++
				s.renderLine(spinnerLine(frame, label))
			}
		}
	}()
}

func (s *spinner) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
		<-s.doneCh
	})
}

func spinnerLine(frame, label string) string {
	if label == "" {
		return frame
	}
	return frame + " " + label
}

func (s *spinner) renderLine(line string) {
	// Use rune count for display width (frames are single-column braille runes).
	curLen := utf8.RuneCountInString(line)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear leftover characters from a longer previous line.
	if s.maxLen > curLen {
		line += strings.Repeat(" ", s.maxLen-curLen)
	} else {
		s.maxLen = curLen
	}

	_, _ = io.WriteString(s.w, "\r"+line)
}

func (s *spinner) clearLine() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.maxLen == 0 {
		return
	}
	_, _ = io.WriteString(s.w, "\r"+strings.Repeat(" ", s.maxLen)+"\r")
	s.maxLen = 0
}
