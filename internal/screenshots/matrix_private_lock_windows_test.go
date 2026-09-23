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

func TestPrivateFileCreationBlocksDeleteAccessAcrossDACLLockHandoff(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "input.png")
	file, err := createMatrixOwnerOnlyFile(path)
	if err != nil {
		t.Fatalf("createMatrixOwnerOnlyFile() error: %v", err)
	}
	fileInfo, err := file.Stat()
	if err != nil {
		t.Fatalf("stat input handle: %v", err)
	}
	if _, err := os.ReadFile(path); err != nil {
		_ = file.Close()
		t.Fatalf("read through path while creation handle is live: %v", err)
	}
	assertMatrixTestDeleteOpenBlocked(t, path, false, true)

	retained, err := lockMatrixPrivateAttemptFileRetained(file)
	if err != nil {
		t.Fatalf("lockMatrixPrivateAttemptFileRetained() error: %v", err)
	}
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("read after retained DACL lock: %v", err)
	}
	assertMatrixTestDeleteOpenBlocked(t, path, false, false)
	if err := unlockMatrixPrivateAttemptFileRetained(retained); err != nil {
		t.Fatalf("unlockMatrixPrivateAttemptFileRetained() error: %v", err)
	}
	if err := closeMatrixPrivateAttemptDACLHandle(retained); err != nil {
		t.Fatalf("close retained file DACL handle: %v", err)
	}
	deleteHandle, err := openMatrixTestDeleteHandle(path, false)
	if err != nil {
		t.Fatalf("open DELETE after the retained lock is released: %v", err)
	}
	_ = windows.CloseHandle(deleteHandle)
	assertMatrixTestPathStillNamesFile(t, path, fileInfo)
}

func TestPrivateAttemptDirectoriesBlockDeleteAccessFromCreationThroughProvider(t *testing.T) {
	attempt, err := createMatrixPrivateAttemptRoot()
	if err != nil {
		t.Fatalf("createMatrixPrivateAttemptRoot() error: %v", err)
	}
	attemptOpen := true
	var outputRoot rootfs.Root
	outputOpen := false
	t.Cleanup(func() {
		if outputOpen {
			_ = outputRoot.Close()
		}
		if attemptOpen {
			_ = cleanupMatrixPrivateAttemptForExecution(&attempt)
			_ = closeMatrixPrivateAttemptForExecution(&attempt)
		}
	})
	for _, root := range []*os.Root{attempt.grandparent, attempt.parent, attempt.pinned} {
		file, err := root.Open(".")
		if err != nil {
			t.Fatalf("open rooted directory while its creation handle is live: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close rooted directory probe: %v", err)
		}
	}
	outputRoot, err = openMatrixPrivateAttemptOutputRoot(&attempt)
	if err != nil {
		t.Fatalf("openMatrixPrivateAttemptOutputRoot() error: %v", err)
	}
	outputOpen = true
	outputPath := filepath.Join(attempt.path, "output")
	assertMatrixTestDeleteOpenBlocked(t, filepath.Dir(attempt.path), true, false)
	assertMatrixTestDeleteOpenBlocked(t, attempt.namespacePath, true, false)
	assertMatrixTestDeleteOpenBlocked(t, attempt.path, true, true)
	assertMatrixTestDeleteOpenBlocked(t, outputPath, true, true)
	if err := lockMatrixPrivateAttemptChild(&attempt); err != nil {
		t.Fatalf("lockMatrixPrivateAttemptChild() error: %v", err)
	}
	assertMatrixTestDeleteOpenBlocked(t, attempt.path, true, false)
	assertMatrixTestDeleteOpenBlocked(t, outputPath, true, true)
	if err := outputRoot.WriteFile("provider.txt", []byte("provider output"), 0o600); err != nil {
		t.Fatalf("provider write through output root after directory locks: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(outputPath, "provider.txt")); err != nil || string(data) != "provider output" {
		t.Fatalf("read provider output through path = %q, %v", data, err)
	}
	if err := outputRoot.Close(); err != nil {
		t.Fatalf("close provider output root: %v", err)
	}
	outputOpen = false
	if err := cleanupMatrixPrivateAttemptForExecution(&attempt); err != nil {
		t.Fatalf("cleanupMatrixPrivateAttemptForExecution() error: %v", err)
	}
	if err := closeMatrixPrivateAttemptForExecution(&attempt); err != nil {
		t.Fatalf("closeMatrixPrivateAttemptForExecution() error: %v", err)
	}
	attemptOpen = false
	if _, err := os.Stat(attempt.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private attempt path after cleanup error = %v, want not-exist", err)
	}
	if _, err := os.Stat(attempt.namespacePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private namespace path after cleanup error = %v, want not-exist", err)
	}
}

func TestRemoveMatrixPrivateCreatedEntryReportsReplacement(t *testing.T) {
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatalf("os.OpenRoot() error: %v", err)
	}
	defer root.Close()

	original, err := createMatrixPrivateAttemptDirectoryInRootRetained(root, "entry", "entry")
	if err != nil {
		t.Fatalf("create original entry: %v", err)
	}
	originalInfo, err := original.Stat()
	if err != nil {
		_ = original.Close()
		t.Fatalf("stat original entry: %v", err)
	}
	if err := original.Close(); err != nil {
		t.Fatalf("close original entry: %v", err)
	}
	if err := root.Remove("entry"); err != nil {
		t.Fatalf("remove original entry: %v", err)
	}
	replacement, err := createMatrixPrivateAttemptDirectoryInRootRetained(root, "entry", "entry")
	if err != nil {
		t.Fatalf("create replacement entry: %v", err)
	}
	if err := replacement.Close(); err != nil {
		t.Fatalf("close replacement entry: %v", err)
	}

	err = removeMatrixPrivateCreatedEntry(root, "entry", originalInfo)
	if !errors.Is(err, errMatrixPrivateAttemptCleanupUncertain) {
		t.Fatalf("removeMatrixPrivateCreatedEntry() error = %v, want cleanup uncertainty", err)
	}
	current, err := root.Stat("entry")
	if err != nil {
		t.Fatalf("stat replacement entry: %v", err)
	}
	if os.SameFile(originalInfo, current) {
		t.Fatal("replacement entry has the original identity")
	}
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

func assertMatrixTestDeleteOpenBlocked(t *testing.T, path string, directory, requireSharingViolation bool) {
	t.Helper()
	handle, err := openMatrixTestDeleteHandle(path, directory)
	if err == nil {
		_ = windows.CloseHandle(handle)
		t.Fatalf("opened DELETE handle for protected path %q", path)
	}
	if errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		return
	}
	if !requireSharingViolation && errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return
	}
	t.Fatalf("open DELETE handle for %q error = %v, want sharing violation", path, err)
}

func openMatrixTestDeleteHandle(path string, directory bool) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	flags := uint32(windows.FILE_ATTRIBUTE_NORMAL)
	if directory {
		flags = windows.FILE_FLAG_BACKUP_SEMANTICS
	}
	return windows.CreateFile(
		name,
		windows.DELETE|windows.SYNCHRONIZE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		flags,
		0,
	)
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
