//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package web

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func lockSessionFileKeyCreation() (func() error, error) {
	dir, err := sessionFileKeyLockDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create session cache dir: %w", err)
	}
	path := filepath.Join(dir, ".web-session-file-key.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := flockRetryingEINTR(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() error {
		unlockErr := flockRetryingEINTR(int(file.Fd()), unix.LOCK_UN)
		closeErr := file.Close()
		if unlockErr != nil {
			return unlockErr
		}
		return closeErr
	}, nil
}

// flockRetryingEINTR retries flock when a signal interrupts the blocking
// wait. The Go runtime's asynchronous preemption signal (SIGURG) routinely
// interrupts syscalls, so a contended LOCK_EX would otherwise fail spuriously
// with EINTR in exactly the concurrent-creation scenario the lock exists for.
func flockRetryingEINTR(fd int, how int) error {
	for {
		err := unix.Flock(fd, how)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
