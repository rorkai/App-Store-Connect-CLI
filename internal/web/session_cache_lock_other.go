//go:build !windows && !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package web

// lockSessionFileKeyCreation has no cross-process file lock on platforms
// without flock or LockFileEx support. Key creation is still serialized
// within the process by sessionFileKeyMu, and the keychain re-check after
// acquiring the lock keeps the remaining cross-process window narrow.
func lockSessionFileKeyCreation() (func() error, error) {
	return func() error { return nil }, nil
}
