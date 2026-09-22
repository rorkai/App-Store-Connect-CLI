//go:build !windows

package screenshots

import (
	"errors"
	"os"
	"path/filepath"
)

func createMatrixPrivateScratchDir(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

func createMatrixPrivateAttemptParent() (string, error) {
	namespace, err := createMatrixPrivateScratchDir(".asc-matrix-attempt-ns-")
	if err != nil {
		return "", err
	}
	parent := filepath.Join(namespace, "parent")
	if err := os.Mkdir(parent, 0o700); err != nil {
		_ = os.RemoveAll(namespace)
		return "", err
	}
	return parent, nil
}

func createMatrixPrivateAttemptChild(parent *os.Root, _ string, name string) error {
	return parent.Mkdir(name, 0o700)
}

func createMatrixPrivateAttemptOutputDirInRoot(parent *os.Root) error {
	return parent.Mkdir("output", 0o700)
}

func createMatrixPrivateAttemptOutputDir(workDir string) error {
	return os.MkdirAll(filepath.Join(workDir, "output"), 0o755)
}

func createMatrixPrivateAttemptFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

func createMatrixPrivateAttemptFileInRoot(parent *os.Root, name, _ string) (*os.File, error) {
	return parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

// matrixPrivateAttemptDACLHandle retains the exact file descriptor whose mode
// was narrowed. Restoring through the descriptor prevents a replacement path
// or symlink from redirecting chmod during cleanup.
type matrixPrivateAttemptDACLHandle struct {
	file   *os.File
	locked bool
}

func lockMatrixPrivateAttemptFileRetained(file *os.File) (*matrixPrivateAttemptDACLHandle, error) {
	if file == nil {
		return nil, os.ErrInvalid
	}
	if err := file.Chmod(0o400); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return &matrixPrivateAttemptDACLHandle{file: file, locked: true}, nil
}

func lockMatrixPrivateAttemptDirectoryRetained(root *os.Root) (*matrixPrivateAttemptDACLHandle, error) {
	if root == nil {
		return nil, os.ErrInvalid
	}
	if err := root.Chmod(".", 0o500); err != nil {
		return nil, err
	}
	return &matrixPrivateAttemptDACLHandle{locked: true}, nil
}

func unlockMatrixPrivateAttemptFileRetained(handle *matrixPrivateAttemptDACLHandle) error {
	if handle == nil || !handle.locked {
		return nil
	}
	if handle.file == nil {
		return os.ErrInvalid
	}
	if err := handle.file.Chmod(0o600); err != nil {
		return err
	}
	handle.locked = false
	return nil
}

func unlockMatrixPrivateAttemptDirectoryRetained(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	if handle == nil || !handle.locked {
		return nil
	}
	if root == nil {
		return os.ErrInvalid
	}
	if err := root.Chmod(".", 0o700); err != nil {
		return err
	}
	handle.locked = false
	return nil
}

func closeMatrixPrivateAttemptDACLHandle(handle *matrixPrivateAttemptDACLHandle) error {
	if handle == nil {
		return nil
	}
	handle.locked = false
	if handle.file == nil {
		return nil
	}
	err := handle.file.Close()
	handle.file = nil
	return err
}

func lockMatrixPrivateAttemptFileHandle(file *os.File) error {
	if file == nil {
		return os.ErrInvalid
	}
	return file.Chmod(0o400)
}
