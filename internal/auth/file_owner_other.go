//go:build !unix

package auth

import "io/fs"

// fileOwnedByOtherUser reports false where files have no POSIX owner, such as
// Windows, which does not apply the key file permission check.
func fileOwnedByOtherUser(fs.FileInfo, int) bool {
	return false
}
