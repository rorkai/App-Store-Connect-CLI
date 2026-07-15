//go:build !windows && !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package web

import "errors"

// lockSessionFileKeyCreation disables encrypted persistence on platforms where
// the first key write cannot be made safe across processes.
func lockSessionFileKeyCreation() (func() error, error) {
	return nil, errors.New("cross-process session file key locking is unavailable on this platform")
}
