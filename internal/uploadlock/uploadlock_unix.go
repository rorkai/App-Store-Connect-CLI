//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package uploadlock

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// accountLockDir reads the account database instead of HOME, as the web
// session lock does.
func accountLockDir() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", err
	}
	if current.HomeDir == "" || !filepath.IsAbs(current.HomeDir) {
		return "", fmt.Errorf("account home %q is not an absolute path", current.HomeDir)
	}
	home, err := filepath.EvalSymlinks(current.HomeDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".asc", "cache"), nil
}

func tryLock(file *os.File) (bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
		return false, nil
	}
	return false, err
}

func unlock(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
