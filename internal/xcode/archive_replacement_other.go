//go:build !darwin

package xcode

import (
	"errors"
	"os"
)

func swapArchiveDirectories(_, _ *os.File, _ string) error {
	return errors.New("atomic archive replacement requires macOS")
}
