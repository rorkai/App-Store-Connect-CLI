package asc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

type toleratedPollFailure struct {
	failures int
	max      int
}

func TestPollUntilTolerantToleratesTransientFailuresUntilSuccess(t *testing.T) {
	t.Parallel()

	transient := &RetryableError{Err: errors.New("service unavailable")}
	calls := 0
	var tolerated []toleratedPollFailure

	got, err := PollUntilTolerant(context.Background(), time.Millisecond, func(ctx context.Context) (string, bool, error) {
		calls++
		if calls <= 2 {
			return "", false, transient
		}
		return "done", true, nil
	}, PollOptions{
		Tolerate: IsTransientWaitError,
		OnToleratedFailure: func(err error, failures, max int) {
			tolerated = append(tolerated, toleratedPollFailure{failures: failures, max: max})
		},
	})
	if err != nil {
		t.Fatalf("PollUntilTolerant() error = %v", err)
	}
	if got != "done" {
		t.Fatalf("PollUntilTolerant() = %q, want %q", got, "done")
	}
	want := []toleratedPollFailure{
		{failures: 1, max: DefaultMaxConsecutivePollFailures},
		{failures: 2, max: DefaultMaxConsecutivePollFailures},
	}
	if len(tolerated) != len(want) {
		t.Fatalf("tolerated failures = %+v, want %+v", tolerated, want)
	}
	for i := range want {
		if tolerated[i] != want[i] {
			t.Fatalf("tolerated failure %d = %+v, want %+v", i, tolerated[i], want[i])
		}
	}
}

func TestPollUntilTolerantDoesNotReplayRetryAfterBeyondCap(t *testing.T) {
	t.Parallel()

	checks := 0
	_, err := PollUntilTolerant(context.Background(), time.Millisecond, func(context.Context) (string, bool, error) {
		checks++
		_, retryErr := WithRetry(context.Background(), func() (struct{}, error) {
			return struct{}{}, rateLimitedError(time.Hour)
		}, RetryOptions{MaxRetries: 1, BaseDelay: time.Millisecond, MaxDelay: time.Second})
		return "", false, retryErr
	}, PollOptions{
		Tolerate: IsTransientWaitError,
		OnToleratedFailure: func(err error, failures, max int) {
			t.Fatalf("did not expect bounded Retry-After to be tolerated (%d/%d): %v", failures, max, err)
		},
	})
	if err == nil {
		t.Fatal("expected bounded Retry-After error, got nil")
	}
	if checks != 1 {
		t.Fatalf("expected one poll check for bounded Retry-After, got %d", checks)
	}
	if !strings.Contains(err.Error(), "retry cap") {
		t.Fatalf("expected retry-cap error, got %v", err)
	}
	if IsTransientWaitError(context.Background(), err) {
		t.Fatalf("expected bounded Retry-After to be terminal to poll recovery, got %v", err)
	}
	if !IsRetryable(err) || GetRetryAfter(err) != time.Hour {
		t.Fatalf("expected direct retry classification and Retry-After to survive, got %v", err)
	}
}

func TestPollUntilTolerantFailsAfterConsecutiveFailureCeiling(t *testing.T) {
	t.Parallel()

	transient := &RetryableError{Err: errors.New("service unavailable")}
	calls := 0
	toleratedCount := 0

	_, err := PollUntilTolerant(context.Background(), time.Millisecond, func(ctx context.Context) (string, bool, error) {
		calls++
		return "", false, transient
	}, PollOptions{
		Tolerate:               IsTransientWaitError,
		MaxConsecutiveFailures: 2,
		OnToleratedFailure: func(err error, failures, max int) {
			toleratedCount++
		},
	})
	if err == nil {
		t.Fatal("expected error once the consecutive failure ceiling is exceeded, got nil")
	}
	if !strings.Contains(err.Error(), "giving up after 3 consecutive transient App Store Connect errors") {
		t.Fatalf("expected consecutive failure message, got %v", err)
	}
	if !errors.Is(err, transient) {
		t.Fatalf("expected wrapped transient cause, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 check calls, got %d", calls)
	}
	if toleratedCount != 2 {
		t.Fatalf("expected 2 tolerated failures before giving up, got %d", toleratedCount)
	}
}

func TestPollUntilTolerantResetsFailureStreakAfterSuccessfulCheck(t *testing.T) {
	t.Parallel()

	transient := &RetryableError{Err: errors.New("service unavailable")}
	calls := 0

	got, err := PollUntilTolerant(context.Background(), time.Millisecond, func(ctx context.Context) (string, bool, error) {
		calls++
		switch calls {
		case 1, 2, 4, 5:
			return "", false, transient
		case 3:
			return "", false, nil
		default:
			return "done", true, nil
		}
	}, PollOptions{
		Tolerate:               IsTransientWaitError,
		MaxConsecutiveFailures: 2,
		OnToleratedFailure:     func(err error, failures, max int) {},
	})
	if err != nil {
		t.Fatalf("PollUntilTolerant() error = %v", err)
	}
	if got != "done" {
		t.Fatalf("PollUntilTolerant() = %q, want %q", got, "done")
	}
	if calls != 6 {
		t.Fatalf("expected 6 check calls, got %d", calls)
	}
}

func TestPollUntilTolerantPropagatesNonTransientErrors(t *testing.T) {
	t.Parallel()

	permanent := errors.New("build upload failed")
	calls := 0

	_, err := PollUntilTolerant(context.Background(), time.Millisecond, func(ctx context.Context) (string, bool, error) {
		calls++
		return "", false, permanent
	}, PollOptions{
		Tolerate: IsTransientWaitError,
		OnToleratedFailure: func(err error, failures, max int) {
			t.Fatalf("did not expect tolerated failure for %v", err)
		},
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("PollUntilTolerant() error = %v, want %v", err, permanent)
	}
	if calls != 1 {
		t.Fatalf("expected 1 check call, got %d", calls)
	}
}

func TestPollUntilTolerantPrefersContextErrorOverTolerance(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transient := &RetryableError{Err: errors.New("service unavailable")}
	calls := 0

	_, err := PollUntilTolerant(ctx, time.Millisecond, func(ctx context.Context) (string, bool, error) {
		calls++
		cancel()
		return "", false, transient
	}, PollOptions{
		Tolerate: IsTransientWaitError,
		OnToleratedFailure: func(err error, failures, max int) {
			t.Fatalf("did not expect tolerated failure after context cancellation: %v", err)
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("PollUntilTolerant() error = %v, want %v", err, context.Canceled)
	}
	if calls != 1 {
		t.Fatalf("expected 1 check call, got %d", calls)
	}
}

func TestIsTransientWaitError(t *testing.T) {
	t.Parallel()

	expiredCtx, cancelExpired := context.WithDeadline(context.Background(), time.Now().Add(-time.Minute))
	defer cancelExpired()

	tests := []struct {
		name string
		ctx  context.Context
		err  error
		want bool
	}{
		{name: "nil error", ctx: context.Background(), err: nil, want: false},
		{name: "retryable", ctx: context.Background(), err: &RetryableError{Err: errors.New("429")}, want: true},
		{
			name: "retry limit exceeded wraps retryable",
			ctx:  context.Background(),
			err:  fmt.Errorf("retry limit exceeded after 3 retries: %w", &RetryableError{Err: errors.New("503")}),
			want: true,
		},
		{name: "server error", ctx: context.Background(), err: &APIError{StatusCode: 503, Title: "unavailable"}, want: true},
		{name: "client error", ctx: context.Background(), err: &APIError{StatusCode: 401, Title: "unauthorized"}, want: false},
		{name: "not found", ctx: context.Background(), err: ErrNotFound, want: false},
		{name: "per-request deadline", ctx: context.Background(), err: context.DeadlineExceeded, want: true},
		{name: "wait deadline", ctx: expiredCtx, err: context.DeadlineExceeded, want: false},
		{name: "canceled", ctx: context.Background(), err: context.Canceled, want: false},
		{name: "permanent", ctx: context.Background(), err: errors.New("boom"), want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := IsTransientWaitError(test.ctx, test.err); got != test.want {
				t.Fatalf("IsTransientWaitError(%v) = %v, want %v", test.err, got, test.want)
			}
		})
	}
}

func TestPollUntilTolerantReportsToleratedFailuresOnStderrOption(t *testing.T) {
	transient := &RetryableError{Err: errors.New("service unavailable")}
	poll := func(stderr io.Writer) error {
		calls := 0
		_, err := PollUntilTolerant(context.Background(), time.Millisecond, func(context.Context) (string, bool, error) {
			calls++
			if calls <= 2 {
				return "", false, transient
			}
			return "done", true, nil
		}, PollOptions{Tolerate: IsTransientWaitError, Stderr: stderr})
		return err
	}
	want := fmt.Sprintf("transient App Store Connect error while waiting (1/%[1]d): %[2]s\ntransient App Store Connect error while waiting (2/%[1]d): %[2]s\n",
		DefaultMaxConsecutivePollFailures, transient.Error())

	tests := []struct {
		name        string
		stderr      *bytes.Buffer
		wantProcess string
	}{
		{name: "nil Stderr reports on the process stderr", wantProcess: want},
		{name: "Stderr receives the report", stderr: &bytes.Buffer{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var writer io.Writer
			if test.stderr != nil {
				writer = test.stderr
			}
			var err error
			process := captureWaitStderr(t, func() { err = poll(writer) })
			if err != nil {
				t.Fatalf("PollUntilTolerant() error = %v", err)
			}
			if process != test.wantProcess {
				t.Fatalf("process stderr = %q, want %q", process, test.wantProcess)
			}
			if test.stderr != nil && test.stderr.String() != want {
				t.Fatalf("Stderr = %q, want %q", test.stderr.String(), want)
			}
		})
	}
}
