package validate

import (
	"context"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const readinessConcurrencyLimit = 4

type readinessRequestGateKey struct{}

type readinessRequestGate struct {
	sem chan struct{}
}

type readinessTask func(context.Context) error

func withReadinessRequestGate(ctx context.Context) context.Context {
	if _, ok := ctx.Value(readinessRequestGateKey{}).(*readinessRequestGate); ok {
		return ctx
	}
	return context.WithValue(ctx, readinessRequestGateKey{}, &readinessRequestGate{
		sem: make(chan struct{}, readinessConcurrencyLimit),
	})
}

func doReadinessRequest[T any](ctx context.Context, request func(context.Context) (T, error)) (T, error) {
	var zero T
	gate, _ := ctx.Value(readinessRequestGateKey{}).(*readinessRequestGate)
	if gate != nil {
		select {
		case gate.sem <- struct{}{}:
			defer func() { <-gate.sem }()
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return request(requestCtx)
}

func runReadinessTasks(ctx context.Context, tasks ...readinessTask) error {
	if len(tasks) == 0 {
		return nil
	}

	groupCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	limit := min(readinessConcurrencyLimit, len(tasks))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for _, task := range tasks {
		if task == nil {
			continue
		}
		wg.Add(1)
		go func(task readinessTask) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-groupCtx.Done():
				return
			}

			if err := task(groupCtx); err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(task)
	}

	wg.Wait()
	if firstErr == nil {
		// Running tasks report a cancelled fetch as "could not verify", so any
		// parent cancellation leaves the report incomplete.
		return ctx.Err()
	}
	return firstErr
}

// runReadinessRequest runs a request that reports only an error through the
// shared readiness request gate.
func runReadinessRequest(ctx context.Context, request func(context.Context) error) error {
	_, err := doReadinessRequest(ctx, func(requestCtx context.Context) (struct{}, error) {
		return struct{}{}, request(requestCtx)
	})
	return err
}
