package xcode

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func swapArchiveDirectories(stage, destination *os.File, name string) error {
	err := unix.RenameatxNp(int(stage.Fd()), name, int(destination.Fd()), name, unix.RENAME_SWAP)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
		return errArchiveSwapUnsupported
	}
	return err
}
