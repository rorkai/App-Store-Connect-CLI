package shared

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunIndexedTasksPreservesIndexKeyedResults(t *testing.T) {
	results := make([]int, 8)
	err := RunIndexedTasks(context.Background(), len(results), 3, func(_ context.Context, index int) error {
		results[index] = index * 10
		return nil
	})
	if err != nil {
		t.Fatalf("RunIndexedTasks() error = %v", err)
	}
	for index, got := range results {
		if got != index*10 {
			t.Fatalf("results[%d] = %d, want %d", index, got, index*10)
		}
	}
}

func TestRunIndexedTasksReturnsLowestIndexedError(t *testing.T) {
	lowIndexErr := errors.New("lower index failed")
	highIndexErr := errors.New("higher index failed first")
	highFinished := make(chan struct{})

	err := RunIndexedTasks(context.Background(), 4, 2, func(_ context.Context, index int) error {
		switch index {
		case 0:
			<-highFinished
			return lowIndexErr
		case 1:
			close(highFinished)
			return highIndexErr
		default:
			return nil
		}
	})
	if !errors.Is(err, lowIndexErr) {
		t.Fatalf("RunIndexedTasks() error = %v, want %v", err, lowIndexErr)
	}
}

func TestRunIndexedTasksReturnsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var calls atomic.Int32
	err := RunIndexedTasks(ctx, 6, 2, func(context.Context, int) error {
		calls.Add(1)
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunIndexedTasks() error = %v, want context.Canceled", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("expected canceled context to skip all tasks, got %d calls", calls.Load())
	}
}

func TestRunIndexedTasksIgnoresCancellationAfterEveryTaskCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	completed := false
	err := RunIndexedTasks(ctx, 1, 1, func(context.Context, int) error {
		completed = true
		cancel()
		return nil
	})
	if err != nil {
		t.Fatalf("RunIndexedTasks() error = %v, want nil after completed work", err)
	}
	if !completed {
		t.Fatal("expected indexed task to complete")
	}
}

func TestRunIndexedTasksHonorsWorkerLimit(t *testing.T) {
	const limit = 2
	var mu sync.Mutex
	var once sync.Once
	block := make(chan struct{})
	running := 0
	peak := 0

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := RunIndexedTasks(ctx, 10, limit, func(ctx context.Context, _ int) error {
		mu.Lock()
		running++
		if running > peak {
			peak = running
		}
		reachedLimit := running == limit
		mu.Unlock()

		if reachedLimit {
			once.Do(func() { close(block) })
		}
		select {
		case <-block:
		case <-ctx.Done():
			return ctx.Err()
		}

		mu.Lock()
		running--
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("RunIndexedTasks() error = %v", err)
	}
	if peak != limit {
		t.Fatalf("peak concurrency = %d, want %d", peak, limit)
	}
}

func TestRunIndexedTasksZeroCountIsNoOp(t *testing.T) {
	called := false
	err := RunIndexedTasks(context.Background(), 0, 4, func(_ context.Context, _ int) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("RunIndexedTasks() error = %v", err)
	}
	if called {
		t.Fatal("task should not run for zero count")
	}
}
