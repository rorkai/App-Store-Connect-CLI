package shared

import (
	"context"
	"sync"
)

// RunIndexedTasks runs fn for each index in [0, count) using at most limit
// concurrent goroutines. The first task error cancels the context passed to
// the remaining tasks and is returned after every started task finishes.
// Tasks that never start because of cancellation are skipped silently, so
// callers must treat their per-index outputs as valid only when the returned
// error is nil. Callers preserve output ordering by writing results into
// index-keyed slices.
func RunIndexedTasks(ctx context.Context, count, limit int, fn func(ctx context.Context, index int) error) error {
	if count <= 0 || fn == nil {
		return nil
	}
	if limit < 1 {
		limit = 1
	}
	if limit > count {
		limit = count
	}

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once

	for index := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-taskCtx.Done():
				return
			}

			if err := fn(taskCtx, index); err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}()
	}

	wg.Wait()
	return firstErr
}
