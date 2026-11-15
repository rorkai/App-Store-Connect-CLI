package shared

import (
	"context"
	"errors"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// RetryReadWithFreshTimeout retries child request deadlines with a fresh
// timeout budget while the operation's parent context remains healthy.
func RetryReadWithFreshTimeout[T any](ctx context.Context, read func(context.Context) (T, error)) (T, error) {
	return retryReadWithFreshTimeout(ctx, ContextWithTimeout, read)
}

// RetryReadWithFreshTimeoutWith is RetryReadWithFreshTimeout with each
// attempt's deadline from d.Timeout.
func RetryReadWithFreshTimeoutWith[T any](ctx context.Context, d *Deps, read func(context.Context) (T, error)) (T, error) {
	return retryReadWithFreshTimeout(ctx, d.Timeout, read)
}

func retryReadWithFreshTimeout[T any](ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), read func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}

	retryOpts := asc.ResolveRetryOptions()
	for attempt := 0; ; attempt++ {
		requestCtx, cancel := timeout(ctx)
		value, err := read(requestCtx)
		childTimedOut := err != nil && errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
		cancel()

		if err == nil {
			return value, nil
		}
		if !childTimedOut || attempt >= retryOpts.MaxRetries {
			return zero, err
		}
		if err := sleepForReadRetry(ctx, readRetryDelay(retryOpts, attempt)); err != nil {
			return zero, err
		}
	}
}

func readRetryDelay(opts asc.RetryOptions, attempt int) time.Duration {
	delay := opts.BaseDelay
	if attempt > 0 && attempt < 31 {
		delay *= time.Duration(1 << attempt)
	}
	if opts.MaxDelay > 0 && (delay > opts.MaxDelay || delay <= 0) {
		return opts.MaxDelay
	}
	return delay
}

func sleepForReadRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
