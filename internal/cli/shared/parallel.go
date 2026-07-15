package shared

import (
	"context"
	"sync"
)

// RunIndexedTasks runs fn for each index in [0, count) using at most limit
// concurrent goroutines. Errors are returned in index order, independent of
// completion order. Callers preserve output ordering by writing results into
// index-keyed slices.
func RunIndexedTasks(ctx context.Context, count, limit int, fn func(ctx context.Context, index int) error) error {
	if count <= 0 || fn == nil {
		return ctx.Err()
	}
	if limit < 1 {
		limit = 1
	}
	if limit > count {
		limit = count
	}

	var wg sync.WaitGroup
	var nextMu sync.Mutex
	nextIndex := 0
	errs := make([]error, count)

	for range limit {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				nextMu.Lock()
				if nextIndex >= count {
					nextMu.Unlock()
					return
				}
				index := nextIndex
				nextIndex++
				nextMu.Unlock()
				errs[index] = fn(ctx, index)
			}
		}()
	}

	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	nextMu.Lock()
	allTasksRan := nextIndex == count
	nextMu.Unlock()
	if allTasksRan {
		return nil
	}
	return ctx.Err()
}
