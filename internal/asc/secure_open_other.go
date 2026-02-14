//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package asc

import "os"

func openExistingNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
