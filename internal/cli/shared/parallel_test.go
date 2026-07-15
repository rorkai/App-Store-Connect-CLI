package shared

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
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

func TestRunIndexedTasksReturnsFirstErrorAndCancels(t *testing.T) {
	wantErr := errors.New("boom")
	var canceled atomic.Int32
	release := make(chan struct{})

	err := RunIndexedTasks(context.Background(), 6, 2, func(ctx context.Context, index int) error {
		if index == 0 {
			return wantErr
		}
		select {
		case <-ctx.Done():
			canceled.Add(1)
			return ctx.Err()
		case <-release:
			return nil
		}
	})
	close(release)
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunIndexedTasks() error = %v, want %v", err, wantErr)
	}
	if canceled.Load() == 0 {
		t.Fatal("expected remaining tasks to observe cancellation")
	}
}

func TestRunIndexedTasksHonorsWorkerLimit(t *testing.T) {
	const limit = 2
	var mu sync.Mutex
	var once sync.Once
	block := make(chan struct{})
	running := 0
	peak := 0

	err := RunIndexedTasks(context.Background(), 10, limit, func(_ context.Context, _ int) error {
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
		<-block

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
