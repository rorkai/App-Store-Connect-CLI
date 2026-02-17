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
	// Reuse the existing “safe to emit progress” gate (stderr TTY + tests can suppress).
	if !ProgressEnabled() {
		return false
	}
	// If stdout is piped, keep stderr quiet to preserve clean stdout contracts (often JSON).
	if !isTerminal(int(os.Stdout.Fd())) {
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

// WithSpinner runs fn while rendering a gh-style indeterminate spinner on stderr.
// It is a no-op when SpinnerEnabled() is false.
func WithSpinner(label string, fn func() error) (err error) {
	if fn == nil {
		return nil
	}
	if !SpinnerEnabled() {
		return fn()
	}

	s := newSpinner(os.Stderr)
	s.Start(label)
	defer func() {
		s.Stop()
		if r := recover(); r != nil {
			panic(r)
		}
	}()
	return fn()
}

// PaginateAllWithSpinner wraps asc.PaginateAll with the standard CLI spinner pattern.
func PaginateAllWithSpinner(ctx context.Context, firstPage asc.PaginatedResponse, fetchNext asc.PaginateFunc) (asc.PaginatedResponse, error) {
	var resp asc.PaginatedResponse
	err := WithSpinner("", func() error {
		var paginateErr error
		resp, paginateErr = asc.PaginateAll(ctx, firstPage, fetchNext)
		return paginateErr
	})
	return resp, err
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
