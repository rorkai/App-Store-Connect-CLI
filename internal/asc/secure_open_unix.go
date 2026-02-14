//go:build darwin || linux || freebsd || netbsd || openbsd || dragonfly

package asc

import (
	"os"

	"golang.org/x/sys/unix"
)

func openExistingNoFollow(path string) (*os.File, error) {
	flags := os.O_RDONLY | unix.O_NOFOLLOW
	return os.OpenFile(path, flags, 0)
}
