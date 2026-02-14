package asc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPollUntilReturnsOnFirstSuccessfulCheck(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := PollUntil(context.Background(), time.Millisecond, func(ctx context.Context) (int, bool, error) {
		calls++
		return 42, true, nil
	})
	if err != nil {
		t.Fatalf("PollUntil() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("PollUntil() = %d, want 42", got)
	}
	if calls != 1 {
		t.Fatalf("expected 1 poll call, got %d", calls)
	}
}

func TestPollUntilRetriesUntilDone(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := PollUntil(context.Background(), time.Millisecond, func(ctx context.Context) (string, bool, error) {
		calls++
		if calls < 3 {
			return "pending", false, nil
		}
		return "done", true, nil
	})
	if err != nil {
		t.Fatalf("PollUntil() error = %v", err)
	}
	if got != "done" {
		t.Fatalf("PollUntil() = %q, want %q", got, "done")
	}
	if calls != 3 {
		t.Fatalf("expected 3 poll calls, got %d", calls)
	}
}

func TestPollUntilReturnsPollError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("boom")
	_, err := PollUntil(context.Background(), time.Millisecond, func(ctx context.Context) (int, bool, error) {
		return 0, false, expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("PollUntil() error = %v, want %v", err, expectedErr)
	}
}

func TestPollUntilRespectsCanceledContextBeforePolling(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	_, err := PollUntil(ctx, time.Millisecond, func(ctx context.Context) (int, bool, error) {
		calls++
		return 1, true, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PollUntil() error = %v, want %v", err, context.Canceled)
	}
	if calls != 0 {
		t.Fatalf("expected 0 poll calls for canceled context, got %d", calls)
	}
}
