package shared

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SafeWriteFileNoSymlink writes a file to path without following symlinks and with an optional
// overwrite mode that preserves the original destination until the new file is fully written.
//
// When overwrite is false, the destination must not already exist, and a failed
// write removes the newly created destination if it still names the same inode.
// When overwrite is true, we refuse to overwrite symlinks and we use temp+rename; if rename fails
// because the destination exists (notably on Windows), we fall back to a safe replace that uses a
// backup file to preserve the original if the final move fails.
func SafeWriteFileNoSymlink(path string, perm os.FileMode, overwrite bool, tempPattern string, backupPattern string, write func(*os.File) (int64, error)) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	if !overwrite {
		return writeNewFileNoSymlink(path, perm, write, func(file *os.File) error {
			return file.Sync()
		})
	}

	return writeFileNoSymlinkOverwrite(path, perm, tempPattern, backupPattern, write)
}

func writeNewFileNoSymlink(
	path string,
	perm os.FileMode,
	write func(*os.File) (int64, error),
	syncFile func(*os.File) error,
) (written int64, err error) {
	file, err := OpenNewFileNoFollow(path, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, fmt.Errorf("output file already exists: %w", err)
		}
		return 0, err
	}
	createdInfo, err := file.Stat()
	if err != nil {
		return 0, errors.Join(err, file.Close(), os.Remove(path))
	}
	closed := false
	success := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("close new output: %w", closeErr))
			}
		}
		if !success {
			if cleanupErr := removeCreatedOutput(path, createdInfo); cleanupErr != nil {
				err = errors.Join(err, cleanupErr)
			}
		}
	}()

	written, err = write(file)
	if err != nil {
		return 0, err
	}
	if err = syncFile(file); err != nil {
		return 0, err
	}
	closed = true
	if err = file.Close(); err != nil {
		return 0, err
	}
	success = true
	return written, nil
}

func removeCreatedOutput(path string, createdInfo os.FileInfo) error {
	currentInfo, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect failed output %q: %w", path, err)
	}
	if !os.SameFile(createdInfo, currentInfo) {
		return fmt.Errorf("refusing to remove changed output %q after failed write", path)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove failed output %q: %w", path, err)
	}
	return nil
}
