package shared

import (
	"fmt"
	"os"
)

type (
	existingFileOpener func(path string) (*os.File, error)
	newFileCreator     func(path string, perm os.FileMode) (*os.File, error)
)

// openExistingNoFollowBestEffort provides a portable, best-effort "no-follow"
// implementation for platforms that do not expose O_NOFOLLOW.
//
// It validates that the path is not a symlink before open, then verifies that
// the opened file still matches the same path after open. This shrinks the
// TOCTOU window but cannot make the operation fully atomic.
func openExistingNoFollowBestEffort(path string, opener existingFileOpener) (*os.File, error) {
	before, err := lstatNoSymlink(path)
	if err != nil {
		return nil, err
	}

	file, err := opener(path)
	if err != nil {
		return nil, err
	}

	if err := verifyOpenedPath(path, file, before); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// openNewFileNoFollowBestEffort provides a portable, best-effort "no-follow"
// file-creation path for platforms that do not expose O_NOFOLLOW.
//
// It rejects symlink paths before creation and verifies the resulting file
// descriptor still maps to the destination path after creation. This reduces,
// but cannot eliminate, TOCTOU risk on platforms without atomic no-follow open.
func openNewFileNoFollowBestEffort(path string, perm os.FileMode, creator newFileCreator) (*os.File, error) {
	if _, err := lstatNoSymlink(path); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	file, err := creator(path, perm)
	if err != nil {
		return nil, err
	}

	if err := verifyOpenedPath(path, file, nil); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func lstatNoSymlink(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to follow symlink %q", path)
	}
	return info, nil
}

func verifyOpenedPath(path string, file *os.File, before os.FileInfo) error {
	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}

	after, err := lstatNoSymlink(path)
	if err != nil {
		return err
	}

	if before != nil && !os.SameFile(before, after) {
		return fmt.Errorf("file changed during open %q", path)
	}
	if !os.SameFile(after, openedInfo) {
		return fmt.Errorf("file changed during open %q", path)
	}
	return nil
}
