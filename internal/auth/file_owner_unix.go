//go:build unix

package auth

import (
	"io/fs"
	"syscall"
)

func fileOwnedByOtherUser(info fs.FileInfo, uid int) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && uid >= 0 && int64(stat.Uid) != int64(uid)
}
