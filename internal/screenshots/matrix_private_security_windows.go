//go:build windows

package screenshots

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const matrixOwnerOnlyRandomNameAttempts = 64

// matrixOwnerOnlySecurityAttributes returns a protected owner-only DACL for
// objects that must never be visible through a broad inherited Windows ACL.
// The descriptor is attached to CreateDirectory/CreateFile so the object is
// protected from its first externally visible instant.
func matrixOwnerOnlySecurityAttributes() (*windows.SecurityAttributes, error) {
	descriptor, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;OW)")
	if err != nil {
		return nil, fmt.Errorf("create owner-only security descriptor: %w", err)
	}
	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: descriptor,
	}, nil
}

func createMatrixOwnerOnlyTempDir(prefix string) (string, error) {
	security, err := matrixOwnerOnlySecurityAttributes()
	if err != nil {
		return "", err
	}
	for attempt := 0; attempt < matrixOwnerOnlyRandomNameAttempts; attempt++ {
		var suffix [16]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", fmt.Errorf("generate owner-only temporary name: %w", err)
		}
		path := filepath.Join(os.TempDir(), prefix+hex.EncodeToString(suffix[:]))
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return "", fmt.Errorf("encode owner-only temporary path: %w", err)
		}
		if err := windows.CreateDirectory(name, security); err == nil {
			return path, nil
		} else if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
			continue
		} else {
			return "", fmt.Errorf("create owner-only temporary directory: %w", err)
		}
	}
	return "", errors.New("create owner-only temporary directory: random-name collision limit exceeded")
}

func createMatrixOwnerOnlyDirectory(path string) error {
	security, err := matrixOwnerOnlySecurityAttributes()
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.CreateDirectory(name, security)
}

// createMatrixOwnerOnlyObjectInRoot creates a new object relative to a held
// directory handle. The security descriptor is supplied to NtCreateFile,
// rather than applied after creation, so the object is owner-only from its
// first externally visible instant. Keeping the parent handle in the call
// also prevents a same-user rename of the parent path from redirecting the
// create operation.
func createMatrixOwnerOnlyObjectInRoot(parent *os.Root, name, displayPath string, directory bool) (*os.File, error) {
	if parent == nil {
		return nil, errors.New("private matrix parent is unavailable")
	}
	parentFile, err := parent.Open(".")
	if err != nil {
		return nil, err
	}
	defer parentFile.Close()

	security, err := matrixOwnerOnlySecurityAttributes()
	if err != nil {
		return nil, err
	}
	objectName, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	objectAttributes := &windows.OBJECT_ATTRIBUTES{
		Length:             uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory:      windows.Handle(parentFile.Fd()),
		ObjectName:         objectName,
		Attributes:         windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
		SecurityDescriptor: security.SecurityDescriptor,
	}
	options := uint32(windows.FILE_NON_DIRECTORY_FILE)
	if directory {
		options = windows.FILE_DIRECTORY_FILE
	}
	options |= windows.FILE_SYNCHRONOUS_IO_NONALERT | windows.FILE_OPEN_REPARSE_POINT
	var handle windows.Handle
	if err := windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.READ_CONTROL|windows.WRITE_DAC|windows.SYNCHRONIZE,
		objectAttributes,
		&windows.IO_STATUS_BLOCK{},
		nil,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		windows.FILE_CREATE,
		options,
		0,
		0,
	); err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), displayPath), nil
}

func createMatrixOwnerOnlyDirectoryInRoot(parent *os.Root, name string) error {
	file, err := createMatrixOwnerOnlyDirectoryInRootRetained(parent, name)
	if err != nil {
		return err
	}
	return file.Close()
}

func createMatrixOwnerOnlyDirectoryInRootRetained(parent *os.Root, name string) (*os.File, error) {
	return createMatrixOwnerOnlyObjectInRoot(parent, name, name, true)
}

func createMatrixOwnerOnlyFileInRoot(parent *os.Root, name, displayPath string) (*os.File, error) {
	return createMatrixOwnerOnlyObjectInRoot(parent, name, displayPath, false)
}

func createMatrixOwnerOnlyFile(path string) (*os.File, error) {
	security, err := matrixOwnerOnlySecurityAttributes()
	if err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_WRITE|windows.READ_CONTROL|windows.WRITE_DAC,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		security,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func createMatrixPrivateScratchDir(prefix string) (string, error) {
	return createMatrixOwnerOnlyTempDir(prefix)
}

func createMatrixPrivateAttemptParent() (string, error) {
	parentPath, namespace, parent, err := createMatrixPrivateAttemptParentWithHandles()
	if err != nil {
		return "", err
	}
	return parentPath, errors.Join(namespace.Close(), parent.Close())
}

func createMatrixPrivateAttemptParentWithHandles() (string, *os.File, *os.File, error) {
	tempRoot, err := os.OpenRoot(os.TempDir())
	if err != nil {
		return "", nil, nil, err
	}
	defer tempRoot.Close()
	for attempt := 0; attempt < matrixOwnerOnlyRandomNameAttempts; attempt++ {
		var suffix [16]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", nil, nil, fmt.Errorf("generate private matrix namespace: %w", err)
		}
		name := ".asc-matrix-attempt-ns-" + hex.EncodeToString(suffix[:])
		namespacePath := filepath.Join(os.TempDir(), name)
		namespace, err := createMatrixOwnerOnlyObjectInRoot(tempRoot, name, namespacePath, true)
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
			continue
		}
		if err != nil {
			return "", nil, nil, fmt.Errorf("create private matrix namespace: %w", err)
		}
		namespaceRoot, err := os.OpenRoot(namespacePath)
		if err != nil {
			namespaceID, identityErr := namespace.Stat()
			closeErr := namespace.Close()
			cleanupErr := removeMatrixPrivateCreatedEntry(tempRoot, name, namespaceID)
			return "", nil, nil, errors.Join(err, identityErr, closeErr, cleanupErr)
		}
		parent, err := createMatrixPrivateAttemptDirectoryInRootRetained(namespaceRoot, "parent", filepath.Join(namespacePath, "parent"))
		var parentID os.FileInfo
		var parentStatErr error
		if parent != nil {
			parentID, parentStatErr = parent.Stat()
		}
		if err != nil || parentStatErr != nil {
			var parentCloseErr error
			if parent != nil {
				parentCloseErr = parent.Close()
			}
			parentCleanupErr := removeMatrixPrivateCreatedEntry(namespaceRoot, "parent", parentID)
			namespaceCloseErr := namespaceRoot.Close()
			namespaceID, identityErr := namespace.Stat()
			namespaceCreatorCloseErr := namespace.Close()
			namespaceCleanupErr := removeMatrixPrivateCreatedEntry(tempRoot, name, namespaceID)
			return "", nil, nil, errors.Join(err, parentStatErr, parentCloseErr, parentCleanupErr, namespaceCloseErr, identityErr, namespaceCreatorCloseErr, namespaceCleanupErr)
		}
		if closeErr := namespaceRoot.Close(); closeErr != nil {
			parentCloseErr := parent.Close()
			cleanupRoot, openErr := os.OpenRoot(namespacePath)
			var parentCleanupErr, cleanupRootCloseErr error
			if openErr == nil {
				parentCleanupErr = removeMatrixPrivateCreatedEntry(cleanupRoot, "parent", parentID)
				cleanupRootCloseErr = cleanupRoot.Close()
			}
			namespaceID, identityErr := namespace.Stat()
			namespaceCreatorCloseErr := namespace.Close()
			namespaceCleanupErr := removeMatrixPrivateCreatedEntry(tempRoot, name, namespaceID)
			return "", nil, nil, errors.Join(closeErr, parentCloseErr, openErr, parentCleanupErr, cleanupRootCloseErr, identityErr, namespaceCreatorCloseErr, namespaceCleanupErr)
		}
		return filepath.Join(namespacePath, "parent"), namespace, parent, nil
	}
	return "", nil, nil, errors.New("create private matrix namespace: random-name collision limit exceeded")
}

func removeMatrixPrivateCreatedEntry(parent *os.Root, name string, identity os.FileInfo) error {
	if parent == nil || identity == nil {
		return nil
	}
	current, err := parent.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !os.SameFile(identity, current) {
		return errors.Join(errMatrixPrivateAttemptCleanupUncertain, errors.New("private matrix created entry identity changed before cleanup"))
	}
	return parent.Remove(name)
}

func createMatrixPrivateAttemptDirectoryInRootRetained(parent *os.Root, name, displayPath string) (*os.File, error) {
	return createMatrixOwnerOnlyObjectInRoot(parent, name, displayPath, true)
}

func createMatrixPrivateAttemptChildRetained(parent *os.Root, parentPath, name string) (*os.File, error) {
	return createMatrixPrivateAttemptDirectoryInRootRetained(parent, name, filepath.Join(parentPath, name))
}

func createMatrixPrivateAttemptOutputDir(workDir string) error {
	return createMatrixOwnerOnlyDirectory(filepath.Join(workDir, "output"))
}

func createMatrixPrivateAttemptOutputDirInRootRetained(parent *os.Root) (*os.File, error) {
	return createMatrixPrivateAttemptDirectoryInRootRetained(parent, "output", "output")
}

func createMatrixPrivateAttemptFile(path string) (*os.File, error) {
	return createMatrixOwnerOnlyFile(path)
}

func createMatrixPrivateAttemptFileInRoot(parent *os.Root, name, displayPath string) (*os.File, error) {
	return createMatrixOwnerOnlyFileInRoot(parent, name, displayPath)
}

func matrixOwnerOnlyProtectedDACL(file *os.File) bool {
	if file == nil {
		return false
	}
	descriptor, err := windows.GetSecurityInfo(
		windows.Handle(file.Fd()),
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return false
	}
	control, _, err := descriptor.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return false
	}
	dacl, _, err := descriptor.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 1 {
		return false
	}
	return true
}
