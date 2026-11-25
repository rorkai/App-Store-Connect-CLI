package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// ReconciledMutationStatus describes whether a mutation response was observed
// directly or inferred from a matching readback after an ambiguous failure.
type ReconciledMutationStatus string

const (
	ReconciledMutationApplied   ReconciledMutationStatus = "applied"
	ReconciledMutationRecovered ReconciledMutationStatus = "reconciled"
)

// RunReconciledMutation executes a mutation with a fresh request deadline. If
// the mutation fails ambiguously, it reads state immediately and once more
// after backoff before replaying the mutation. Readback callbacks may span
// pagination and are responsible for fresh per-request deadlines.
func RunReconciledMutation[T any](
	ctx context.Context,
	mutate func(context.Context) (T, error),
	readback func(context.Context) (T, bool, error),
) (T, ReconciledMutationStatus, error) {
	return runReconciledMutation(ctx, ContextWithTimeout, time.After, mutate, readback, true)
}

// RunReconciledMutationWith is RunReconciledMutation with each mutation's
// deadline from d.Timeout and the backoff on d.After.
func RunReconciledMutationWith[T any](
	ctx context.Context,
	d *Deps,
	mutate func(context.Context) (T, error),
	readback func(context.Context) (T, bool, error),
) (T, ReconciledMutationStatus, error) {
	return runReconciledMutation(ctx, d.Timeout, d.After, mutate, readback, true)
}

// RunReconciledMutationNoReplayWith executes an ambiguous mutation once and
// uses immediate and one delayed readback to determine whether that attempt
// applied. It never replays a mutation after a negative readback.
// Deterministic failures are returned directly so strict API errors keep their
// existing classification. The mutation's deadline comes from d.Timeout and the
// backoff runs on d.After.
func RunReconciledMutationNoReplayWith[T any](
	ctx context.Context,
	d *Deps,
	mutate func(context.Context) (T, error),
	readback func(context.Context) (T, bool, error),
) (T, ReconciledMutationStatus, error) {
	return runReconciledMutation(ctx, d.Timeout, d.After, mutate, readback, false)
}

// RunReconciledMultiRequestMutationWith reconciles a mutation whose callback
// sets deadlines for its own requests. Its eligibility reads, backoff and write
// share the caller's context. replay allows replay after an ambiguous failure.
func RunReconciledMultiRequestMutationWith[T any](
	ctx context.Context,
	d *Deps,
	mutate func(context.Context) (T, error),
	readback func(context.Context) (T, bool, error),
	replay bool,
) (T, ReconciledMutationStatus, error) {
	return runReconciledMutation(ctx, context.WithCancel, d.After, mutate, readback, replay)
}

func runReconciledMutation[T any](
	ctx context.Context,
	timeout func(context.Context) (context.Context, context.CancelFunc),
	after func(time.Duration) <-chan time.Time,
	mutate func(context.Context) (T, error),
	readback func(context.Context) (T, bool, error),
	replay bool,
) (T, ReconciledMutationStatus, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, "", err
	}

	retryOpts := asc.ResolveRetryOptions()
	for retry := 0; ; retry++ {
		value, mutationErr := runMutationWithFreshTimeout(ctx, timeout, mutate)
		if mutationErr == nil {
			return value, ReconciledMutationApplied, nil
		}
		if err := ctx.Err(); err != nil {
			return zero, "", err
		}
		if !replay && !isAmbiguousMutationError(ctx, mutationErr) {
			return zero, "", mutationErr
		}

		value, matches, readErr := runMutationReadback(ctx, readback)
		if readErr != nil {
			return zero, "", fmt.Errorf("mutation and readback failed: %w", errors.Join(mutationErr, readErr))
		}
		if matches {
			return value, ReconciledMutationRecovered, nil
		}
		if !replay {
			if retry >= retryOpts.MaxRetries {
				return zero, "", mutationErr
			}
			if err := sleepForMutationRetry(ctx, after, mutationReadbackDelay(retryOpts, retry, mutationErr)); err != nil {
				return zero, "", err
			}

			value, matches, readErr = runMutationReadback(ctx, readback)
			if readErr != nil {
				return zero, "", fmt.Errorf("mutation and pre-retry readback failed: %w", errors.Join(mutationErr, readErr))
			}
			if matches {
				return value, ReconciledMutationRecovered, nil
			}
			return zero, "", mutationErr
		}
		if isRateLimitRejection(mutationErr) || !IsTransientMutationError(ctx, mutationErr) || retry >= retryOpts.MaxRetries {
			return zero, "", mutationErr
		}

		if err := sleepForMutationRetry(ctx, after, mutationRetryDelay(retryOpts, retry, mutationErr)); err != nil {
			return zero, "", err
		}

		value, matches, readErr = runMutationReadback(ctx, readback)
		if readErr != nil {
			return zero, "", fmt.Errorf("mutation and pre-retry readback failed: %w", errors.Join(mutationErr, readErr))
		}
		if matches {
			return value, ReconciledMutationRecovered, nil
		}
	}
}

func isAmbiguousMutationError(parent context.Context, err error) bool {
	return !isRateLimitRejection(err) && IsTransientMutationError(parent, err)
}

func isRateLimitRejection(err error) bool {
	if !asc.IsRetryable(err) {
		return false
	}
	var statusErr interface{ HTTPStatusCode() int }
	return errors.As(err, &statusErr) && statusErr.HTTPStatusCode() == http.StatusTooManyRequests
}

func runMutationWithFreshTimeout[T any](ctx context.Context, timeout func(context.Context) (context.Context, context.CancelFunc), mutate func(context.Context) (T, error)) (T, error) {
	requestCtx, cancel := timeout(ctx)
	defer cancel()
	return mutate(requestCtx)
}

func runMutationReadback[T any](ctx context.Context, readback func(context.Context) (T, bool, error)) (T, bool, error) {
	return readback(ctx)
}

// IsTransientMutationError reports whether a mutation can be retried after
// state readback while the parent operation remains healthy.
func IsTransientMutationError(parent context.Context, err error) bool {
	if asc.IsRetryBudgetExhausted(err) {
		return false
	}
	if asc.IsRetryDelayExceeded(err) {
		return false
	}
	if asc.IsRetryable(err) {
		return true
	}
	return parent.Err() == nil && errors.Is(err, context.DeadlineExceeded)
}

func mutationRetryDelay(opts asc.RetryOptions, retry int, err error) time.Duration {
	if delay := asc.GetRetryAfter(err); delay > 0 {
		return delay
	}
	delay := opts.BaseDelay
	if retry > 0 && retry < 31 {
		delay *= time.Duration(1 << retry)
	}
	if opts.MaxDelay > 0 && (delay > opts.MaxDelay || delay <= 0) {
		return opts.MaxDelay
	}
	return delay
}

func mutationReadbackDelay(opts asc.RetryOptions, retry int, err error) time.Duration {
	delay := mutationRetryDelay(opts, retry, err)
	if opts.MaxDelay > 0 && delay > opts.MaxDelay {
		return opts.MaxDelay
	}
	return delay
}

func sleepForMutationRetry(ctx context.Context, after func(time.Duration) <-chan time.Time, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-after(delay):
		return nil
	}
}
