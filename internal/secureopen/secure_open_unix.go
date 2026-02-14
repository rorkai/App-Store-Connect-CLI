//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package secureopen

import (
	"os"

	"golang.org/x/sys/unix"
)

// OpenNewFileNoFollow creates a new file without following symlinks.
// Uses O_EXCL to prevent overwriting existing files and O_NOFOLLOW to prevent symlink attacks.
func OpenNewFileNoFollow(path string, perm os.FileMode) (*os.File, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL | unix.O_NOFOLLOW
	return os.OpenFile(path, flags, perm)
}

// OpenExistingNoFollow opens an existing file without following symlinks.
func OpenExistingNoFollow(path string) (*os.File, error) {
	flags := os.O_RDONLY | unix.O_NOFOLLOW
	return os.OpenFile(path, flags, 0)
}
