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
// When overwrite is false, the destination must not already exist.
// When overwrite is true, we refuse to overwrite symlinks and we use temp+rename; if rename fails
// because the destination exists (notably on Windows), we fall back to a safe replace that uses a
// backup file to preserve the original if the final move fails.
func SafeWriteFileNoSymlink(path string, perm os.FileMode, overwrite bool, tempPattern string, backupPattern string, write func(*os.File) (int64, error)) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	if !overwrite {
		file, err := OpenNewFileNoFollow(path, perm)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return 0, fmt.Errorf("output file already exists: %w", err)
			}
			return 0, err
		}
		defer func() { _ = file.Close() }()

		written, err := write(file)
		if err != nil {
			return 0, err
		}
		return written, file.Sync()
	}

	// Overwrite mode: do not remove the destination until the new file is fully written.
	hadExisting := false
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return 0, fmt.Errorf("refusing to overwrite symlink %q", path)
		}
		if info.IsDir() {
			return 0, fmt.Errorf("output path %q is a directory", path)
		}
		hadExisting = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), tempPattern)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tempFile.Close() }()

	tempPath := tempFile.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tempPath)
		}
	}()

	if err := tempFile.Chmod(perm); err != nil {
		return 0, err
	}

	written, err := write(tempFile)
	if err != nil {
		return 0, err
	}
	if err := tempFile.Sync(); err != nil {
		return 0, err
	}
	if err := tempFile.Close(); err != nil {
		return 0, err
	}

	if err := os.Rename(tempPath, path); err != nil {
		if !hadExisting {
			return 0, err
		}

		backupFile, backupErr := os.CreateTemp(filepath.Dir(path), backupPattern)
		if backupErr != nil {
			return 0, err
		}
		backupPath := backupFile.Name()
		if closeErr := backupFile.Close(); closeErr != nil {
			return 0, closeErr
		}
		if removeErr := os.Remove(backupPath); removeErr != nil {
			return 0, removeErr
		}

		if moveErr := os.Rename(path, backupPath); moveErr != nil {
			return 0, moveErr
		}
		if moveErr := os.Rename(tempPath, path); moveErr != nil {
			_ = os.Rename(backupPath, path)
			return 0, moveErr
		}
		_ = os.Remove(backupPath)
	}

	success = true
	return written, nil
}
