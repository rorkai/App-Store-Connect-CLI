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

func TestRetainedDACLHandleRestoresAfterPathRename(t *testing.T) {
	if windowsProcessTokenBypassesDACLs(t) {
		t.Skip("current Windows token bypasses DACLs")
	}

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
	defer windows.CloseHandle(deleteHandle)

	exactFile, err := os.Open(path)
	if err != nil {
		t.Fatalf("open exact input: %v", err)
	}
	retained, err := lockMatrixPrivateAttemptFileRetained(exactFile)
	if err != nil {
		t.Fatalf("lockMatrixPrivateAttemptFileRetained() error: %v", err)
	}
	defer closeMatrixPrivateAttemptDACLHandle(retained)

	renamedPath := filepath.Join(directory, "renamed.png")
	if err := renameWindowsHandle(deleteHandle, filepath.Base(renamedPath)); err != nil {
		t.Fatalf("rename through pre-lock handle: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old path stat error = %v, want not-exist", err)
	}
	if file, err := os.OpenFile(renamedPath, os.O_WRONLY, 0); err == nil {
		_ = file.Close()
		t.Fatal("locked file remained writable after its pathname moved")
	}

	if err := unlockMatrixPrivateAttemptFileRetained(retained); err != nil {
		t.Fatalf("unlockMatrixPrivateAttemptFileRetained() error: %v", err)
	}
	if file, err := os.OpenFile(renamedPath, os.O_WRONLY, 0); err != nil {
		t.Fatalf("renamed file was not restored through retained handle: %v", err)
	} else if err := file.Close(); err != nil {
		t.Fatalf("close restored file: %v", err)
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

type matrixTestFileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func renameWindowsHandle(handle windows.Handle, name string) error {
	newName, err := windows.UTF16FromString(name)
	if err != nil {
		return err
	}
	fileNameLen := len(newName)*2 - 2
	var renameInfo matrixTestFileRenameInformation
	bufferSize := int(unsafe.Offsetof(renameInfo.FileName)) + fileNameLen
	buffer := make([]byte, bufferSize)
	info := (*matrixTestFileRenameInformation)(unsafe.Pointer(&buffer[0]))
	info.ReplaceIfExists = windows.FILE_RENAME_REPLACE_IF_EXISTS | windows.FILE_RENAME_POSIX_SEMANTICS
	info.FileNameLength = uint32(fileNameLen)
	copy((*[windows.MAX_LONG_PATH]uint16)(unsafe.Pointer(&info.FileName[0]))[:fileNameLen/2:fileNameLen/2], newName)
	var ioStatus windows.IO_STATUS_BLOCK
	return windows.NtSetInformationFile(handle, &ioStatus, &buffer[0], uint32(bufferSize), windows.FileRenameInformation)
}
