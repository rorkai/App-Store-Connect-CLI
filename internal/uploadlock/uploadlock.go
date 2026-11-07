// Package uploadlock serializes App Store Connect asset uploads across asc
// processes on one machine.
//
// Several asc processes uploading at once multiply their requests against one
// App Store Connect account, which answers with 429s, and share one network
// link, which pushes slow uploads past their deadline. The lock makes a second
// upload wait for the first instead.
package uploadlock

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

// EnvVar disables the lock when set to a false value such as 0.
const EnvVar = "ASC_UPLOAD_LOCK"

const (
	lockFileName        = "upload.lock"
	defaultWaitLimit    = 30 * time.Minute
	defaultPollInterval = 250 * time.Millisecond
)

var (
	lockDir      = defaultLockDir
	waitLimit    = defaultWaitLimit
	pollInterval = defaultPollInterval
)

func defaultLockDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".asc", "cache"), nil
}

// Acquire takes the machine-wide asc upload lock and returns the function that
// releases it. The operating system also releases the lock when the process
// exits, including after a crash.
//
// While another asc process holds the lock, Acquire tells stderr once and
// waits up to 30 minutes. Callers should take the lock before they start any
// upload deadline, so the wait does not count against it. If the lock file
// cannot be opened, Acquire warns and returns without locking, so an unusable
// cache directory never blocks an upload. ASC_UPLOAD_LOCK=0 skips the lock.
func Acquire(ctx context.Context, stderr io.Writer) (func(), error) {
	noop := func() {}
	enabled, err := lockEnabled()
	if err != nil {
		return nil, err
	}
	if !enabled {
		return noop, nil
	}

	file, path, err := openLockFile()
	if err != nil {
		fmt.Fprintf(stderr, "Warning: could not open the asc upload lock (%v); uploading without waiting for other asc uploads\n", err)
		return noop, nil
	}

	deadline := time.Now().Add(waitLimit)
	announced := false
	for {
		locked, err := tryLock(file)
		if err != nil {
			_ = file.Close()
			fmt.Fprintf(stderr, "Warning: could not take the asc upload lock %s (%v); uploading without waiting for other asc uploads\n", path, err)
			return noop, nil
		}
		if locked {
			var once sync.Once
			return func() {
				once.Do(func() {
					_ = unlock(file)
					_ = file.Close()
				})
			}, nil
		}
		if !announced {
			fmt.Fprintf(stderr, "Waiting for another asc upload to finish (lock %s; set %s=0 to skip)...\n", path, EnvVar)
			announced = true
		}
		if !time.Now().Before(deadline) {
			_ = file.Close()
			return nil, fmt.Errorf("another asc upload still held %s after %s; wait for it to finish, or set %s=0 to upload anyway", path, waitLimit, EnvVar)
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			_ = file.Close()
			return nil, fmt.Errorf("wait for asc upload lock: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func lockEnabled() (bool, error) {
	value := strings.TrimSpace(os.Getenv(EnvVar))
	if value == "" {
		return true, nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be 0 or 1, got %q", EnvVar, value)
	}
	return enabled, nil
}

func openLockFile() (*os.File, string, error) {
	dir, err := lockDir()
	if err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, lockFileName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, path, fmt.Errorf("create %s: %w", dir, err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, path, err
	}
	defer root.Close()
	file, err := secureopen.OpenLockFileNoFollowInRoot(root, lockFileName, 0o600)
	if err != nil {
		return nil, path, err
	}
	return file, path, nil
}
