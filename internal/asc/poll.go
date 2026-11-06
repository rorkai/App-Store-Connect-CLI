package asc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// DefaultMaxConsecutivePollFailures bounds how many consecutive transient
// failures a tolerant poll absorbs before giving up. It keeps a long wait alive
// across a multi-minute App Store Connect outage while still terminating when an
// endpoint stays broken.
const DefaultMaxConsecutivePollFailures = 5

// PollOptions configures transient-failure tolerance for PollUntilTolerant.
type PollOptions struct {
	// Tolerate reports whether a check error is transient and should not end
	// the poll loop. A nil predicate tolerates nothing, which makes
	// PollUntilTolerant behave exactly like PollUntil.
	Tolerate func(context.Context, error) bool

	// MaxConsecutiveFailures caps how many consecutive tolerated failures may
	// occur before the poll fails. Values <= 0 use
	// DefaultMaxConsecutivePollFailures. A successful check resets the streak.
	MaxConsecutiveFailures int

	// OnToleratedFailure observes every tolerated failure with the 1-based
	// position in the current streak and the configured ceiling. A nil handler
	// reports tolerated failures on stderr.
	OnToleratedFailure func(err error, failures, max int)

	// NextInterval, when set, returns the wait before the next check given how
	// many checks have run so far (1-based). It replaces the fixed interval so a
	// long wait can slow down instead of polling at one rate throughout.
	NextInterval func(checks int) time.Duration
}

// PollUntil repeatedly executes check until it returns done=true, an error,
// or the context is canceled. It executes check immediately before waiting.
func PollUntil[T any](ctx context.Context, interval time.Duration, check func(context.Context) (T, bool, error)) (T, error) {
	return PollUntilTolerant(ctx, interval, check, PollOptions{})
}

// PollUntilTolerant polls like PollUntil but absorbs a bounded number of
// consecutive transient check failures instead of aborting the whole wait on
// the first one. Long waits (build processing, build discovery) survive a short
// App Store Connect outage this way, while a permanently broken endpoint still
// terminates once the consecutive-failure ceiling is exceeded.
func PollUntilTolerant[T any](ctx context.Context, interval time.Duration, check func(context.Context) (T, bool, error), opts PollOptions) (T, error) {
	var zero T

	if interval <= 0 {
		return zero, fmt.Errorf("poll interval must be greater than zero")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	maxFailures := opts.MaxConsecutiveFailures
	if maxFailures <= 0 {
		maxFailures = DefaultMaxConsecutivePollFailures
	}
	onToleratedFailure := opts.OnToleratedFailure
	if onToleratedFailure == nil {
		onToleratedFailure = logToleratedPollFailure
	}

	consecutiveFailures := 0
	runCheck := func() (T, bool, error) {
		value, done, err := check(ctx)
		if err == nil {
			consecutiveFailures = 0
			return value, done, nil
		}
		if opts.Tolerate == nil {
			return zero, false, err
		}
		// A canceled or expired wait context is authoritative: never spend the
		// transient-failure budget on errors caused by the caller's own deadline.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return zero, false, ctxErr
		}
		if !opts.Tolerate(ctx, err) {
			return zero, false, err
		}
		consecutiveFailures++
		if consecutiveFailures > maxFailures {
			return zero, false, fmt.Errorf(
				"giving up after %d consecutive transient App Store Connect errors: %w",
				consecutiveFailures,
				err,
			)
		}
		onToleratedFailure(err, consecutiveFailures, maxFailures)
		return zero, false, nil
	}

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
	}

	value, done, err := runCheck()
	if err != nil {
		return zero, err
	}
	if done {
		return value, nil
	}

	nextInterval := opts.NextInterval
	if nextInterval == nil {
		nextInterval = func(int) time.Duration { return interval }
	}

	timer := time.NewTimer(pollWait(nextInterval(1), interval))
	defer timer.Stop()

	for checks := 2; ; checks++ {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-timer.C:
			value, done, err = runCheck()
			if err != nil {
				return zero, err
			}
			if done {
				return value, nil
			}
			timer.Reset(pollWait(nextInterval(checks), interval))
		}
	}
}

// pollWait falls back to the fixed interval when a schedule yields a
// non-positive wait, so a schedule bug can never turn into a busy loop.
func pollWait(next, fallback time.Duration) time.Duration {
	if next <= 0 {
		return fallback
	}
	return next
}

// IsTransientWaitError reports whether err is a transient App Store Connect or
// network failure that a long-running wait should absorb instead of aborting.
// Errors caused by the wait's own context are never transient.
func IsTransientWaitError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	// context.DeadlineExceeded also satisfies net.Error, so an expired or
	// canceled wait context has to short-circuit before any timeout check.
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if IsRetryDelayExceeded(err) {
		return false
	}
	if IsRetryable(err) {
		return true
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 500 {
		return true
	}

	if errors.Is(err, context.DeadlineExceeded) && ctx != nil && ctx.Err() == nil {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || isTemporaryNetError(netErr)) {
		return true
	}

	return false
}

type temporaryNetError interface {
	Temporary() bool
}

func isTemporaryNetError(err net.Error) bool {
	tempErr, ok := err.(temporaryNetError)
	return ok && tempErr.Temporary()
}

func logToleratedPollFailure(err error, failures, max int) {
	fmt.Fprintf(
		os.Stderr,
		"transient App Store Connect error while waiting (%d/%d): %s\n",
		failures,
		max,
		SanitizeTerminalText(err.Error()),
	)
}
