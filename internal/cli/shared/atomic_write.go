package shared

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func createTempFileNoFollowWithPerm(dir string, pattern string, perm os.FileMode) (*os.File, error) {
	// Mirror os.CreateTemp pattern semantics: replace the last "*" with random text,
	// or append random text if no "*" is present.
	prefix := pattern
	suffix := ""
	if idx := strings.LastIndex(pattern, "*"); idx != -1 {
		prefix = pattern[:idx]
		suffix = pattern[idx+1:]
	}

	const maxAttempts = 10_000
	var randBytes [12]byte
	for i := 0; i < maxAttempts; i++ {
		if _, err := rand.Read(randBytes[:]); err != nil {
			return nil, err
		}
		name := prefix + hex.EncodeToString(randBytes[:]) + suffix
		f, err := OpenNewFileNoFollow(filepath.Join(dir, name), perm)
		if err == nil {
			return f, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return nil, err
	}

	return nil, fmt.Errorf("failed to create temporary file in %q", dir)
}

// WriteFileNoSymlinkOverwrite writes reader to path via temp+rename.
// It refuses to overwrite symlinks and uses a Windows-safe replace when needed.
func WriteFileNoSymlinkOverwrite(path string, reader io.Reader, perm os.FileMode, tempPattern string, backupPattern string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	// Do not remove/replace a symlink.
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

	tempFile, err := createTempFileNoFollowWithPerm(filepath.Dir(path), tempPattern, perm)
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

	written, err := io.Copy(tempFile, reader)
	if err != nil {
		return 0, err
	}
	if err := tempFile.Sync(); err != nil {
		return 0, err
	}
	if err := tempFile.Close(); err != nil {
		return 0, err
	}

	// On Unix, rename replaces the destination atomically. On Windows, rename fails if the
	// destination exists, so we fall back to a safe replace that preserves the original
	// file if the final move fails.
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
