//go:build !darwin

package xcode

import "os"

func swapArchiveDirectories(_, _ *os.File, _ string) error {
	return errArchiveSwapUnsupported
}
