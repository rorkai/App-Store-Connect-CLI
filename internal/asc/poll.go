package asc

import (
	"context"
	"fmt"
	"time"
)

// PollUntil repeatedly executes check until it returns done=true, an error,
// or the context is canceled. It executes check immediately before waiting.
func PollUntil[T any](ctx context.Context, interval time.Duration, check func(context.Context) (T, bool, error)) (T, error) {
	var zero T

	if interval <= 0 {
		return zero, fmt.Errorf("poll interval must be greater than zero")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-ctx.Done():
		return zero, ctx.Err()
	default:
	}

	value, done, err := check(ctx)
	if err != nil {
		return zero, err
	}
	if done {
		return value, nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-ticker.C:
			value, done, err = check(ctx)
			if err != nil {
				return zero, err
			}
			if done {
				return value, nil
			}
		}
	}
}
