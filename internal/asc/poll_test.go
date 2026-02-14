package asc

import (
	"context"
	"errors"
	"strings"
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

func TestPollUntilRejectsZeroInterval(t *testing.T) {
	t.Parallel()

	_, err := PollUntil(context.Background(), 0, func(ctx context.Context) (int, bool, error) {
		t.Fatal("check should not be called with zero interval")
		return 0, false, nil
	})
	if err == nil {
		t.Fatal("expected error for zero interval, got nil")
	}
	if !strings.Contains(err.Error(), "poll interval must be greater than zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPollUntilRejectsNegativeInterval(t *testing.T) {
	t.Parallel()

	_, err := PollUntil(context.Background(), -time.Second, func(ctx context.Context) (int, bool, error) {
		t.Fatal("check should not be called with negative interval")
		return 0, false, nil
	})
	if err == nil {
		t.Fatal("expected error for negative interval, got nil")
	}
	if !strings.Contains(err.Error(), "poll interval must be greater than zero") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPollUntilRespectsCanceledContextDuringPolling(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	_, err := PollUntil(ctx, time.Millisecond, func(ctx context.Context) (int, bool, error) {
		calls++
		if calls >= 2 {
			cancel()
		}
		return 0, false, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PollUntil() error = %v, want %v", err, context.Canceled)
	}
	if calls < 2 {
		t.Fatalf("expected at least 2 poll calls before cancel, got %d", calls)
	}
}
