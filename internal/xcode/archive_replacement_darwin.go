package xcode

import (
	"os"

	"golang.org/x/sys/unix"
)

func swapArchiveDirectories(stage, destination *os.File, name string) error {
	return unix.RenameatxNp(int(stage.Fd()), name, int(destination.Fd()), name, unix.RENAME_SWAP|unix.RENAME_NOFOLLOW_ANY)
}
