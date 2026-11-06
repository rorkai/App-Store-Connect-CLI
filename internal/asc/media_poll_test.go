package asc

import (
	"context"
	"testing"
	"time"
)

// Each file still processing costs one status request per check. The default
// schedule must stay well under App Store Connect's 3600 requests per hour per
// key; a fixed 2s poll spent about 300 requests on a 10 minute wait.
func TestMediaPollIntervalDefaultScheduleBoundsRequestsPerWait(t *testing.T) {
	ctx := context.Background()
	checks, elapsed := 1, time.Duration(0)
	for elapsed < 10*time.Minute {
		elapsed += MediaPollInterval(ctx, checks)
		checks++
	}
	if checks > 60 {
		t.Fatalf("default schedule made %d checks in 10 minutes, want at most 60", checks)
	}
	if got := MediaPollInterval(ctx, 1); got != 2*time.Second {
		t.Fatalf("first wait = %s, want 2s so quick processing still returns promptly", got)
	}
	if got := MediaPollInterval(ctx, 1000); got != 15*time.Second {
		t.Fatalf("long wait interval = %s, want 15s cap", got)
	}
}

func TestMediaPollIntervalHonorsFixedOverride(t *testing.T) {
	ctx := WithMediaPollInterval(context.Background(), 30*time.Second)
	for _, checks := range []int{1, 16, 500} {
		if got := MediaPollInterval(ctx, checks); got != 30*time.Second {
			t.Fatalf("MediaPollInterval(check %d) = %s, want fixed 30s", checks, got)
		}
	}
}

func TestPollUntilTolerantUsesNextIntervalPerCheck(t *testing.T) {
	var asked []int
	calls := 0
	_, err := PollUntilTolerant(context.Background(), time.Hour, func(context.Context) (struct{}, bool, error) {
		calls++
		return struct{}{}, calls == 4, nil
	}, PollOptions{NextInterval: func(checks int) time.Duration {
		asked = append(asked, checks)
		return time.Nanosecond
	}})
	if err != nil {
		t.Fatalf("PollUntilTolerant() error = %v", err)
	}
	if want := []int{1, 2, 3}; len(asked) != len(want) || asked[0] != 1 || asked[2] != 3 {
		t.Fatalf("NextInterval called with %v, want %v (an hour-long fixed interval would hang)", asked, want)
	}
}
