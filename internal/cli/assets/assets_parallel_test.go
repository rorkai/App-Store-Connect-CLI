package assets

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestForEachAssetTaskBoundsConcurrencyAndCoversEveryIndex(t *testing.T) {
	const count = 12

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64
	done := make([]atomic.Bool, count)

	taskErrs := forEachAssetTask(context.Background(), count, true, func(_ context.Context, idx int) error {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			observed := maxInFlight.Load()
			if current <= observed || maxInFlight.CompareAndSwap(observed, current) {
				break
			}
		}
		done[idx].Store(true)
		return nil
	})

	if len(taskErrs) != count {
		t.Fatalf("expected %d error slots, got %d", count, len(taskErrs))
	}
	for idx, err := range taskErrs {
		if err != nil {
			t.Fatalf("expected nil error at index %d, got %v", idx, err)
		}
		if !done[idx].Load() {
			t.Fatalf("expected task %d to run", idx)
		}
	}
	if got := maxInFlight.Load(); got > assetTransferWorkerLimit {
		t.Fatalf("expected at most %d concurrent tasks, got %d", assetTransferWorkerLimit, got)
	}
}

func TestForEachAssetTaskFirstFailureCancelsInFlightWork(t *testing.T) {
	errBoom := errors.New("boom")
	// Exactly one worker slot per task, so every task is guaranteed to be
	// in flight when the failure happens and no sibling can starve the
	// failing task of a slot.
	const count = assetTransferWorkerLimit

	// The failing task waits until every sibling is in flight, so the
	// outcome is deterministic: all siblings observe the cancellation.
	started := make(chan struct{}, count-1)
	taskErrs := forEachAssetTask(context.Background(), count, true, func(taskCtx context.Context, idx int) error {
		if idx == 1 {
			for i := 0; i < count-1; i++ {
				<-started
			}
			return errBoom
		}
		started <- struct{}{}
		<-taskCtx.Done()
		return taskCtx.Err()
	})

	if !errors.Is(taskErrs[1], errBoom) {
		t.Fatalf("expected failure at index 1, got %v", taskErrs[1])
	}
	for idx, err := range taskErrs {
		if idx == 1 {
			continue
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected canceled in-flight task at index %d, got %v", idx, err)
		}
	}

	failedIdx, failedErr := firstAssetTaskError(taskErrs)
	if failedIdx != 1 || !errors.Is(failedErr, errBoom) {
		t.Fatalf("firstAssetTaskError() = (%d, %v), want (1, %v)", failedIdx, failedErr, errBoom)
	}
	if err := aggregateAssetTaskErrors(taskErrs); !errors.Is(err, errBoom) || err.Error() != errBoom.Error() {
		t.Fatalf("expected single real failure to be returned unchanged, got %v", err)
	}
}

func TestForEachAssetTaskSkipsTasksWhenContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var ran atomic.Int64
	taskErrs := forEachAssetTask(ctx, 6, true, func(_ context.Context, _ int) error {
		ran.Add(1)
		return nil
	})

	if got := ran.Load(); got != 0 {
		t.Fatalf("expected no task to run on a cancelled context, got %d", got)
	}
	for idx, err := range taskErrs {
		if !errors.Is(err, errAssetTaskNotAttempted) {
			t.Fatalf("expected skipped task at index %d, got %v", idx, err)
		}
	}
	if _, err := firstAssetTaskError(taskErrs); !errors.Is(err, errAssetTaskNotAttempted) {
		t.Fatalf("expected skip marker as fallback error, got %v", err)
	}
}

func TestForEachAssetTaskWithoutCancelOnFirstErrorRunsEveryTask(t *testing.T) {
	errBoom := errors.New("boom")
	var ran atomic.Int64

	taskErrs := forEachAssetTask(context.Background(), 6, false, func(_ context.Context, idx int) error {
		ran.Add(1)
		if idx == 0 {
			return errBoom
		}
		return nil
	})

	if got := ran.Load(); got != 6 {
		t.Fatalf("expected all 6 tasks to run, got %d", got)
	}
	if !errors.Is(taskErrs[0], errBoom) {
		t.Fatalf("expected failure at index 0, got %v", taskErrs[0])
	}
	for idx := 1; idx < 6; idx++ {
		if taskErrs[idx] != nil {
			t.Fatalf("expected nil error at index %d, got %v", idx, taskErrs[idx])
		}
	}
}

func TestAggregateAssetTaskErrorsReportsAdditionalFailureCount(t *testing.T) {
	errA := errors.New("first failure")
	errB := errors.New("second failure")

	err := aggregateAssetTaskErrors([]error{nil, errA, errAssetTaskNotAttempted, errB})
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if !errors.Is(err, errA) {
		t.Fatalf("expected aggregated error to wrap the first failure, got %v", err)
	}
	if !strings.Contains(err.Error(), "1 more failure") {
		t.Fatalf("expected aggregated error to count additional failures, got %v", err)
	}

	if err := aggregateAssetTaskErrors([]error{nil, nil}); err != nil {
		t.Fatalf("expected nil aggregate for all-success, got %v", err)
	}
}
