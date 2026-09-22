//go:build windows

package screenshots

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"golang.org/x/sys/windows"
)

func TestRetainedDACLHandleRejectsPreopenedDeleteHandle(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "input.png")
	file, err := createMatrixOwnerOnlyFile(path)
	if err != nil {
		t.Fatalf("createMatrixOwnerOnlyFile() error: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close input: %v", err)
	}

	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString() error: %v", err)
	}
	deleteHandle, err := windows.CreateFile(
		name,
		windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile(DELETE) error: %v", err)
	}
	exactFile, err := os.Open(path)
	if err != nil {
		t.Fatalf("open exact input: %v", err)
	}
	retained, err := lockMatrixPrivateAttemptFileRetained(exactFile)
	if err == nil {
		_ = closeMatrixPrivateAttemptDACLHandle(retained)
		t.Fatal("lockMatrixPrivateAttemptFileRetained() accepted preopened DELETE handle")
	}
	if err := windows.CloseHandle(deleteHandle); err != nil {
		t.Fatalf("close DELETE handle: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original file after rejected lock: %v", err)
	}
}

func TestDirectoryDACLHandleRejectsPreopenedDeleteHandle(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "scratch")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create scratch directory: %v", err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString() error: %v", err)
	}
	deleteHandle, err := windows.CreateFile(
		name,
		windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile(DELETE) error: %v", err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		_ = windows.CloseHandle(deleteHandle)
		t.Fatalf("open scratch root: %v", err)
	}
	defer root.Close()
	retained, err := lockMatrixPrivateAttemptDirectoryRetained(root)
	if err == nil {
		_ = closeMatrixPrivateAttemptDACLHandle(retained)
		t.Fatal("lockMatrixPrivateAttemptDirectoryRetained() accepted preopened DELETE handle")
	}
	if err := windows.CloseHandle(deleteHandle); err != nil {
		t.Fatalf("close DELETE handle: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original directory after rejected lock: %v", err)
	}
}

func TestRetainedDACLHandleDoesNotFollowReparsePoint(t *testing.T) {
	if windowsProcessTokenBypassesDACLs(t) {
		t.Skip("current Windows token bypasses DACLs")
	}

	directory := t.TempDir()
	targetPath := filepath.Join(directory, "target.png")
	target, err := createMatrixOwnerOnlyFile(targetPath)
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := target.Close(); err != nil {
		t.Fatalf("close target: %v", err)
	}
	linkPath := filepath.Join(directory, "input.png")
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("create Windows symlink fixture: %v", err)
	}

	root, err := rootfs.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if file, err := root.OpenFile(filepath.Base(linkPath)); !errors.Is(err, rootfs.ErrSymlink) {
		if file != nil {
			_ = file.Close()
		}
		t.Fatalf("open reparse point error = %v, want %v", err, rootfs.ErrSymlink)
	}

	writable, err := os.OpenFile(targetPath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("reparse-point lock changed target DACL: %v", err)
	}
	if err := writable.Close(); err != nil {
		t.Fatalf("close target after reparse-point lock: %v", err)
	}
}

func TestVerifyMatrixDirectoryDACLHandleIdentityRejectsDifferentDirectory(t *testing.T) {
	directory := t.TempDir()
	expectedPath := filepath.Join(directory, "expected")
	actualPath := filepath.Join(directory, "actual")
	if err := os.Mkdir(expectedPath, 0o700); err != nil {
		t.Fatalf("mkdir expected: %v", err)
	}
	if err := os.Mkdir(actualPath, 0o700); err != nil {
		t.Fatalf("mkdir actual: %v", err)
	}
	expectedRoot, err := os.OpenRoot(expectedPath)
	if err != nil {
		t.Fatalf("open expected root: %v", err)
	}
	defer expectedRoot.Close()
	expected, err := expectedRoot.Open(".")
	if err != nil {
		t.Fatalf("open expected directory: %v", err)
	}
	defer expected.Close()

	actualName, err := windows.UTF16PtrFromString(actualPath)
	if err != nil {
		t.Fatalf("UTF16PtrFromString() error: %v", err)
	}
	actual, err := windows.CreateFile(
		actualName,
		windows.READ_CONTROL|windows.WRITE_DAC,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		t.Fatalf("open actual directory: %v", err)
	}
	defer windows.CloseHandle(actual)
	if err := verifyMatrixDirectoryDACLHandleIdentity(expected, actual); err == nil {
		t.Fatal("different directory handle passed identity verification")
	}
}
