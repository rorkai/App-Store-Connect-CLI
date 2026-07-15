package assets

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// assetTransferWorkerLimit bounds concurrent asset transfer operations
// (uploads, downloads, deletes, and detail fetches). The API client also
// throttles mutating requests globally, so this mainly limits local file
// handles and network fan-out.
const assetTransferWorkerLimit = 4

// errAssetTaskNotAttempted marks tasks that never started because an earlier
// task already failed and cancelled the remaining work.
var errAssetTaskNotAttempted = errors.New("not attempted after earlier failure")

// forEachAssetTask runs task for every index in [0, count) on a bounded
// worker pool. Tasks must write their outputs into index-keyed slots owned by
// the caller so input ordering is preserved. When cancelOnFirstError is true,
// the first failure cancels the shared context so in-flight work stops early,
// and tasks skipped because of that sibling failure are recorded as
// errAssetTaskNotAttempted. Caller cancellation is preserved separately.
// The returned slice holds one error slot per index (nil on success).
func forEachAssetTask(ctx context.Context, count int, cancelOnFirstError bool, task func(ctx context.Context, idx int) error) []error {
	taskErrs := make([]error, count)
	if count == 0 {
		return taskErrs
	}

	taskCtx := ctx
	cancel := context.CancelFunc(func() {})
	if cancelOnFirstError {
		taskCtx, cancel = context.WithCancel(ctx)
	}
	defer cancel()

	workers := count
	if workers > assetTransferWorkerLimit {
		workers = assetTransferWorkerLimit
	}
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup
	for idx := range taskErrs {
		idx := idx
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-taskCtx.Done():
				taskErrs[idx] = assetTaskCancellationError(ctx)
				return
			}
			defer func() { <-sem }()

			if taskCtx.Err() != nil {
				taskErrs[idx] = assetTaskCancellationError(ctx)
				return
			}
			if err := task(taskCtx, idx); err != nil {
				taskErrs[idx] = err
				cancel()
			}
		}()
	}
	wg.Wait()
	return taskErrs
}

func assetTaskCancellationError(parent context.Context) error {
	if err := parent.Err(); err != nil {
		return err
	}
	return errAssetTaskNotAttempted
}

// firstAssetTaskError returns the lowest-index error that represents a real
// failure, preferring genuine task errors over cancellation fallout from a
// sibling failure. It returns (-1, nil) when every task succeeded.
func firstAssetTaskError(taskErrs []error) (int, error) {
	canceledIdx := -1
	skippedIdx := -1
	for idx, err := range taskErrs {
		if err == nil {
			continue
		}
		if errors.Is(err, errAssetTaskNotAttempted) {
			if skippedIdx == -1 {
				skippedIdx = idx
			}
			continue
		}
		if errors.Is(err, context.Canceled) {
			if canceledIdx == -1 {
				canceledIdx = idx
			}
			continue
		}
		return idx, err
	}
	if canceledIdx != -1 {
		return canceledIdx, taskErrs[canceledIdx]
	}
	if skippedIdx != -1 {
		return skippedIdx, taskErrs[skippedIdx]
	}
	return -1, nil
}

// aggregateAssetTaskErrors reduces per-index task errors to a single error.
// A single failure is returned unchanged; multiple failures wrap the first
// one and report how many more occurred.
func aggregateAssetTaskErrors(taskErrs []error) error {
	_, firstErr := firstAssetTaskError(taskErrs)
	if firstErr == nil {
		return nil
	}

	failed := 0
	for _, err := range taskErrs {
		if err == nil || errors.Is(err, errAssetTaskNotAttempted) || errors.Is(err, context.Canceled) {
			continue
		}
		failed++
	}
	if failed > 1 {
		return fmt.Errorf("%w (and %d more failure(s))", firstErr, failed-1)
	}
	return firstErr
}
