package asc

import "time"

// ResetConfigCacheForTest clears cached config state for tests.
func ResetConfigCacheForTest() {
	resetConfigCacheForTest()
}

// SetMediaPollIntervalsForTest replaces the default media processing poll
// schedule bounds. The returned function restores the previous values.
func SetMediaPollIntervalsForTest(initial, maxInterval time.Duration) func() {
	previousInitial, previousMax := mediaPollInitialInterval, mediaPollMaxInterval
	mediaPollInitialInterval, mediaPollMaxInterval = initial, maxInterval
	return func() { mediaPollInitialInterval, mediaPollMaxInterval = previousInitial, previousMax }
}
