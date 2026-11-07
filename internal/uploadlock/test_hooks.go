package uploadlock

import "time"

// SetDirForTest replaces the directory that holds the lock file. The returned
// function restores the previous value.
func SetDirForTest(dir string) func() {
	previous := lockDir
	lockDir = func() (string, error) { return dir, nil }
	return func() { lockDir = previous }
}

// SetWaitLimitForTest replaces how long Acquire waits for another upload. The
// returned function restores the previous value.
func SetWaitLimitForTest(limit time.Duration) func() {
	previous := waitLimit
	waitLimit = limit
	return func() { waitLimit = previous }
}
