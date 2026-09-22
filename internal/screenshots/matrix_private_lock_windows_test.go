//go:build windows

package screenshots

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"golang.org/x/sys/windows"
)

func TestRetainedDACLHandleBlocksRenameThroughPreopenedDeleteHandle(t *testing.T) {
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
	exactHandle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile(read) error: %v", err)
	}
	exactFile := os.NewFile(uintptr(exactHandle), path)
	fileInfo, err := exactFile.Stat()
	if err != nil {
		_ = exactFile.Close()
		t.Fatalf("stat input handle: %v", err)
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
		_ = exactFile.Close()
		t.Fatalf("CreateFile(DELETE) error: %v", err)
	}
	defer windows.CloseHandle(deleteHandle)
	parentHandle := openMatrixTestRenameParent(t, directory)
	defer windows.CloseHandle(parentHandle)
	if err := renameWindowsHandle(deleteHandle, parentHandle, "before-lock.png"); err != nil {
		t.Fatalf("pre-lock rename through DELETE handle: %v", err)
	}
	if err := renameWindowsHandle(deleteHandle, parentHandle, "input.png"); err != nil {
		t.Fatalf("restore pre-lock file name: %v", err)
	}
	retained, err := lockMatrixPrivateAttemptFileRetained(exactFile)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Fatalf("lockMatrixPrivateAttemptFileRetained() error = %v, want sharing violation or locked handle", err)
		}
		assertMatrixTestPathStillNamesFile(t, path, fileInfo)
		return
	}
	defer func() {
		if err := unlockMatrixPrivateAttemptFileRetained(retained); err != nil {
			t.Errorf("unlockMatrixPrivateAttemptFileRetained() error: %v", err)
		}
		if err := closeMatrixPrivateAttemptDACLHandle(retained); err != nil {
			t.Errorf("close retained file DACL handle: %v", err)
		}
	}()
	if err := renameWindowsHandle(deleteHandle, parentHandle, "renamed.png"); err == nil {
		t.Fatal("preopened DELETE handle renamed file while retained DACL lock was active")
	}
	assertMatrixTestPathStillNamesFile(t, path, fileInfo)
}

func TestDirectoryDACLHandleBlocksRenameThroughPreopenedDeleteHandle(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "scratch")
	if err := createMatrixOwnerOnlyDirectory(path); err != nil {
		t.Fatalf("createMatrixOwnerOnlyDirectory() error: %v", err)
	}
	directoryInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat scratch directory: %v", err)
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString() error: %v", err)
	}
	exactHandle, err := windows.CreateFile(
		name,
		windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile(read directory) error: %v", err)
	}
	exactFile := os.NewFile(uintptr(exactHandle), path)
	defer exactFile.Close()
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
	defer windows.CloseHandle(deleteHandle)
	parentHandle := openMatrixTestRenameParent(t, directory)
	defer windows.CloseHandle(parentHandle)
	if err := renameWindowsHandle(deleteHandle, parentHandle, "before-lock"); err != nil {
		t.Fatalf("pre-lock rename through DELETE handle: %v", err)
	}
	if err := renameWindowsHandle(deleteHandle, parentHandle, "scratch"); err != nil {
		t.Fatalf("restore pre-lock directory name: %v", err)
	}
	daclHandle, err := openMatrixDirectoryForDACL(exactFile)
	if err != nil {
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Fatalf("openMatrixDirectoryForDACL() error = %v, want sharing violation or locked handle", err)
		}
		assertMatrixTestPathStillNamesFile(t, path, directoryInfo)
		return
	}
	retained := &matrixPrivateAttemptDACLHandle{handle: daclHandle, open: true}
	if err := retained.set("D:P(A;;GRGX;;;OW)", "set private matrix attempt directory access control"); err != nil {
		_ = closeMatrixPrivateAttemptDACLHandle(retained)
		t.Fatalf("set private matrix attempt directory access control: %v", err)
	}
	defer func() {
		if err := restoreMatrixPrivateAttemptDirectory(retained, nil); err != nil {
			t.Errorf("restore retained directory DACL: %v", err)
		}
	}()
	if err := renameWindowsHandle(deleteHandle, parentHandle, "replacement"); err == nil {
		t.Fatal("preopened DELETE handle renamed directory while retained DACL lock was active")
	}
	assertMatrixTestPathStillNamesFile(t, path, directoryInfo)
}

func assertMatrixTestPathStillNamesFile(t *testing.T, path string, expected os.FileInfo) {
	t.Helper()
	actual, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat path after rejected rename: %v", err)
	}
	if !os.SameFile(expected, actual) {
		t.Fatal("path no longer names the original locked object")
	}
}

type matrixTestFileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func openMatrixTestRenameParent(t *testing.T, path string) windows.Handle {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString(rename parent) error: %v", err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_WRITE_DATA|windows.FILE_APPEND_DATA|windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		t.Fatalf("open rename parent: %v", err)
	}
	return handle
}

func renameWindowsHandle(handle, parent windows.Handle, name string) error {
	newName, err := windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	fileNameLen := len(newName)*2 - 2
	var renameInfo matrixTestFileRenameInformation
	bufferSize := int(unsafe.Offsetof(renameInfo.FileName)) + fileNameLen
	buffer := make([]byte, bufferSize)
	info := (*matrixTestFileRenameInformation)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = parent
	info.FileNameLength = uint32(fileNameLen)
	copy((*[windows.MAX_LONG_PATH]uint16)(unsafe.Pointer(&info.FileName[0]))[:fileNameLen/2:fileNameLen/2], newName)
	var ioStatus windows.IO_STATUS_BLOCK
	return windows.NtSetInformationFile(handle, &ioStatus, &buffer[0], uint32(bufferSize), windows.FileRenameInformation)
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
