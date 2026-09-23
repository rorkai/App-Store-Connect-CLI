//go:build windows

package screenshots

import (
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

var matrixReOpenFile = windows.NewLazySystemDLL("kernel32.dll").NewProc("ReOpenFile")

// matrixPrivateAttemptDACLHandle retains the native handle that was granted
// WRITE_DAC before a staging object is made read-only. Once the DACL has been
// narrowed, reopening the pathname for cleanup is not reliable: the caller
// may only have the read-only access that the lock intentionally grants.
type matrixPrivateAttemptDACLHandle struct {
	handle windows.Handle
	open   bool
}

func (handle *matrixPrivateAttemptDACLHandle) close() error {
	if handle == nil {
		return nil
	}
	var closeErr error
	if handle.open {
		if err := windows.CloseHandle(handle.handle); err != nil {
			closeErr = errors.Join(closeErr, err)
		} else {
			handle.handle = windows.InvalidHandle
			handle.open = false
		}
	}
	return closeErr
}

func closeMatrixPrivateAttemptDACLHandle(handle *matrixPrivateAttemptDACLHandle) error {
	return handle.close()
}

func (handle *matrixPrivateAttemptDACLHandle) set(sddl, operation string) error {
	if handle == nil || !handle.open || handle.handle == windows.InvalidHandle {
		return errors.New("private matrix DACL handle is unavailable")
	}
	descriptor, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	if err := windows.SetSecurityInfo(
		handle.handle,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func lockMatrixPrivateAttemptDirectoryRetained(root *os.Root) (*matrixPrivateAttemptDACLHandle, error) {
	if root == nil {
		return nil, errors.New("private matrix attempt root is unavailable")
	}
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	handle, err := openMatrixDirectoryForDACL(file)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	retained := &matrixPrivateAttemptDACLHandle{handle: handle, open: true}
	if err := file.Close(); err != nil {
		return nil, errors.Join(err, finalizeMatrixPrivateAttemptDACLHandle(retained))
	}
	if err := retained.set("D:P(A;;GRGX;;;OW)", "set private matrix attempt directory access control"); err != nil {
		return nil, errors.Join(err, finalizeMatrixPrivateAttemptDACLHandle(retained))
	}
	return retained, nil
}

func lockMatrixPrivateAttemptDirectoryCreated(file *os.File, root *os.Root) (*matrixPrivateAttemptDACLHandle, error) {
	if file == nil {
		return lockMatrixPrivateAttemptDirectoryRetained(root)
	}
	retained, err := lockMatrixPrivateAttemptDirectoryRetained(root)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return nil, errors.Join(err, finalizeMatrixPrivateAttemptDirectory(retained, root))
	}
	return retained, nil
}

func unlockMatrixPrivateAttemptDirectoryRetained(handle *matrixPrivateAttemptDACLHandle, _ *os.Root) error {
	if handle == nil || !handle.open || handle.handle == windows.InvalidHandle {
		return nil
	}
	return handle.set("D:P(A;;GA;;;OW)", "restore private matrix attempt directory access control")
}

func lockMatrixPrivateAttemptFileHandle(file *os.File) error {
	return setMatrixPrivateAttemptFileACLHandle(file, "D:P(A;;GR;;;OW)")
}

func lockMatrixPrivateAttemptFileRetained(file *os.File) (*matrixPrivateAttemptDACLHandle, error) {
	if file == nil {
		return nil, errors.New("private matrix file is unavailable")
	}
	handle, err := reopenMatrixFileForDACL(file)
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	retained := &matrixPrivateAttemptDACLHandle{handle: handle, open: true}
	if err := retained.set("D:P(A;;GR;;;OW)", "set private matrix attempt file access control"); err != nil {
		return nil, errors.Join(err, unlockMatrixPrivateAttemptFileRetained(retained), file.Close(), finalizeMatrixPrivateAttemptDACLHandle(retained))
	}
	// The original creation handle can deny path-based readers on Windows.
	// The reopened WRITE_DAC handle keeps the exact file pinned for restoration.
	if err := file.Close(); err != nil {
		return nil, errors.Join(err, unlockMatrixPrivateAttemptFileRetained(retained), finalizeMatrixPrivateAttemptDACLHandle(retained))
	}
	return retained, nil
}

func unlockMatrixPrivateAttemptFileRetained(handle *matrixPrivateAttemptDACLHandle) error {
	if handle == nil || !handle.open || handle.handle == windows.InvalidHandle {
		return nil
	}
	return handle.set("D:P(A;;GA;;;OW)", "restore private matrix attempt file access control")
}

func setMatrixPrivateAttemptFileACLHandle(file *os.File, sddl string) error {
	if file == nil {
		return errors.New("private matrix file is unavailable")
	}
	handle, err := reopenMatrixFileForDACL(file)
	if err != nil {
		return err
	}
	retained := &matrixPrivateAttemptDACLHandle{handle: handle, open: true}
	setErr := setMatrixPrivateAttemptFileACLHandleValue(handle, sddl)
	closeErr := finalizeMatrixPrivateAttemptDACLHandle(retained)
	return errors.Join(setErr, closeErr)
}

func setMatrixPrivateAttemptFileACLHandleValue(handle windows.Handle, sddl string) error {
	return (&matrixPrivateAttemptDACLHandle{handle: handle, open: true}).set(sddl, "set private matrix attempt file access control")
}

func reopenMatrixFileForDACL(file *os.File) (windows.Handle, error) {
	handle, _, callErr := matrixReOpenFile.Call(
		file.Fd(),
		uintptr(windows.READ_CONTROL|windows.WRITE_DAC),
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE),
		0,
	)
	if reopened := windows.Handle(handle); reopened != windows.InvalidHandle {
		return reopened, nil
	}
	if callErr != nil && callErr != syscall.Errno(0) {
		return windows.InvalidHandle, callErr
	}
	return windows.InvalidHandle, syscall.EINVAL
}

func openMatrixDirectoryForDACL(file *os.File) (windows.Handle, error) {
	if file == nil {
		return windows.InvalidHandle, errors.New("private matrix directory is unavailable")
	}
	name, err := windows.UTF16PtrFromString(file.Name())
	if err != nil {
		return windows.InvalidHandle, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.READ_CONTROL|windows.WRITE_DAC,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return windows.InvalidHandle, err
	}
	if err := verifyMatrixDirectoryDACLHandleIdentity(file, handle); err != nil {
		return windows.InvalidHandle, errors.Join(err, windows.CloseHandle(handle))
	}
	return handle, nil
}

func verifyMatrixDirectoryDACLHandleIdentity(file *os.File, handle windows.Handle) error {
	if file == nil || handle == windows.InvalidHandle {
		return errors.New("private matrix directory handle is unavailable")
	}
	var expected windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &expected); err != nil {
		return fmt.Errorf("inspect private matrix directory handle: %w", err)
	}
	var actual windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &actual); err != nil {
		return fmt.Errorf("inspect private matrix DACL handle: %w", err)
	}
	if expected.VolumeSerialNumber != actual.VolumeSerialNumber ||
		expected.FileIndexHigh != actual.FileIndexHigh ||
		expected.FileIndexLow != actual.FileIndexLow {
		return errors.New("private matrix directory changed before access control")
	}
	return nil
}
