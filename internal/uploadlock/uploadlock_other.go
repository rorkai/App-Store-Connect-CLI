//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package uploadlock

import (
	"errors"
	"os"
)

func accountLockDir() (string, error) {
	return "", errors.New("no account lock directory on this platform")
}

// Other platforms have no file lock here, so uploads are not serialized.
func tryLock(*os.File) (bool, error) { return true, nil }

func unlock(*os.File) error { return nil }
