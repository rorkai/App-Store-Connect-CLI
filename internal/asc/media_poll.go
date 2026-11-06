package asc

import (
	"context"
	"time"
)

// Media processing waits (screenshots, app previews, Asset Library images and
// videos) poll one status endpoint per file. App Store Connect allows 3600
// requests per rolling hour per API key, so a fixed 2s poll costs about 1800
// requests per hour for every file still processing. The default schedule
// checks every mediaPollInitialInterval while processing is usually quick,
// then doubles the wait up to mediaPollMaxInterval.
var (
	mediaPollInitialInterval = 2 * time.Second
	mediaPollMaxInterval     = 15 * time.Second
)

// mediaPollFastChecks is how many checks run at the initial interval (about
// 30 seconds at 2s) before the wait starts to grow.
const mediaPollFastChecks = 15

type mediaPollIntervalKey struct{}

// WithMediaPollInterval returns a context whose media processing waits check
// status every interval instead of using the default growing schedule.
// Non-positive intervals leave the default schedule in place.
func WithMediaPollInterval(ctx context.Context, interval time.Duration) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		return ctx
	}
	return context.WithValue(ctx, mediaPollIntervalKey{}, interval)
}

// MediaPollInterval returns the wait before the next media processing check,
// given how many checks have already run (1-based).
func MediaPollInterval(ctx context.Context, checks int) time.Duration {
	if ctx != nil {
		if fixed, ok := ctx.Value(mediaPollIntervalKey{}).(time.Duration); ok && fixed > 0 {
			return fixed
		}
	}
	initial, maxInterval := mediaPollInitialInterval, mediaPollMaxInterval
	if checks <= mediaPollFastChecks {
		return initial
	}
	wait := initial
	for step := mediaPollFastChecks; step < checks && wait < maxInterval; step++ {
		wait *= 2
	}
	return min(wait, maxInterval)
}

// PollMediaUntil polls a media processing state using the context's media
// poll schedule. See PollUntilTolerant for opts; NextInterval is replaced.
func PollMediaUntil[T any](ctx context.Context, check func(context.Context) (T, bool, error), opts PollOptions) (T, error) {
	opts.NextInterval = func(checks int) time.Duration { return MediaPollInterval(ctx, checks) }
	return PollUntilTolerant(ctx, MediaPollInterval(ctx, 1), check, opts)
}
