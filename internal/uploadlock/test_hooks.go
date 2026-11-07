package uploadlock

import "time"

// SetWaitLimitForTest replaces how long Acquire waits for another upload. The
// returned function restores the previous value.
func SetWaitLimitForTest(limit time.Duration) func() {
	previous := waitLimit
	waitLimit = limit
	return func() { waitLimit = previous }
}
