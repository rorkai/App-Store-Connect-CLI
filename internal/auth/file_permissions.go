package auth

import (
	"io/fs"
	"runtime"
)

func filePermissionsTooPermissive(mode fs.FileMode) bool {
	return filePermissionsTooPermissiveForOS(mode, runtime.GOOS)
}

func filePermissionsTooPermissiveForOS(mode fs.FileMode, goos string) bool {
	return goos != "windows" && mode.Perm()&0o077 != 0
}
