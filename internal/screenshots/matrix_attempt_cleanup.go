package screenshots

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func cleanupMatrixPrivateAttemptConstructionAfterHandles(parent, grandparent, pinned *os.Root, identity, parentID os.FileInfo) error {
	var cleanupErr error
	if pinned != nil {
		entries, err := fs.ReadDir(pinned.FS(), ".")
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		} else {
			for _, entry := range entries {
				cleanupErr = errors.Join(cleanupErr, pinned.RemoveAll(entry.Name()))
			}
		}
	}
	if parent == nil {
		return cleanupErr
	}
	cleanupErr = errors.Join(cleanupErr, removeMatrixExpectedEntry(parent, identity, nil))
	if empty, err := matrixPrivateAttemptDirectoryEmpty(parent); err != nil {
		cleanupErr = errors.Join(cleanupErr, err)
	} else if empty && grandparent != nil && parentID != nil {
		cleanupErr = errors.Join(cleanupErr, removeMatrixExpectedEntry(grandparent, parentID, nil))
	}
	return cleanupErr
}

func (attempt matrixPrivateAttemptRoot) matrixPrivateAttemptRootIsStable() error {
	if attempt.pinned == nil || attempt.child == nil || attempt.identity == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	pinnedInfo, err := attempt.pinned.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, pinnedInfo) {
		return errors.New("private matrix attempt root identity changed")
	}
	current, err := attempt.child.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, current) {
		return errors.New("private matrix attempt root identity changed")
	}
	opened, err := attempt.root.OpenRoot()
	if err != nil {
		return err
	}
	return opened.Close()
}

func (attempt *matrixPrivateAttemptRoot) cleanup() error {
	if attempt == nil {
		return nil
	}
	if attempt.grandparent == nil || attempt.parent == nil || attempt.child == nil || attempt.identity == nil {
		return nil
	}
	var restoreErr error
	if attempt.childLocked {
		restoreErr = errors.Join(restoreErr, retryMatrixPrivateAttemptDirectoryRestore(attempt.childDACL, attempt.pinned))
	}
	if attempt.parentLocked {
		restoreErr = errors.Join(restoreErr, retryMatrixPrivateAttemptDirectoryRestore(attempt.parentDACL, attempt.parent))
	}
	if attempt.grandparentLocked {
		restoreErr = errors.Join(restoreErr, retryMatrixPrivateAttemptDirectoryRestore(attempt.grandparentDACL, attempt.grandparent))
	}
	if restoreErr != nil {
		return restoreErr
	}
	for index, handle := range attempt.fileDACLs {
		restoreErr = errors.Join(restoreErr, finalizeMatrixPrivateAttemptFile(handle))
		attempt.fileDACLs[index] = nil
	}
	if attempt.outputCreator != nil {
		restoreErr = errors.Join(restoreErr, attempt.outputCreator.Close())
		attempt.outputCreator = nil
	}
	if attempt.childCreator != nil {
		restoreErr = errors.Join(restoreErr, attempt.childCreator.Close())
		attempt.childCreator = nil
	}
	if restoreErr != nil {
		return restoreErr
	}
	// Remove entries through the held child descriptor, so a rename cannot
	// redirect recursive cleanup to an unrelated directory.
	entries, err := fs.ReadDir(attempt.child.FS(), ".")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if err := attempt.child.RemoveAll(entry.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := removeMatrixExpectedEntry(attempt.parent, attempt.identity, matrixPrivateAttemptBeforeCleanupRemoveForTest); err != nil {
		return err
	}
	empty, err := matrixPrivateAttemptDirectoryEmpty(attempt.parent)
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}
	return removeMatrixExpectedEntry(attempt.grandparent, attempt.parentID, nil)
}

func (attempt *matrixPrivateAttemptRoot) close() error {
	if attempt == nil {
		return nil
	}
	var closeErr error
	closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptDirectory(attempt.childDACL, attempt.pinned))
	closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptDirectory(attempt.parentDACL, attempt.parent))
	closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptDirectory(attempt.grandparentDACL, attempt.grandparent))
	for index, handle := range attempt.fileDACLs {
		closeErr = errors.Join(closeErr, finalizeMatrixPrivateAttemptFile(handle))
		attempt.fileDACLs[index] = nil
	}
	for _, file := range []*os.File{attempt.outputCreator, attempt.childCreator} {
		if file != nil {
			closeErr = errors.Join(closeErr, file.Close())
		}
	}
	attempt.outputCreator = nil
	attempt.childCreator = nil
	if attempt.child != nil {
		closeErr = errors.Join(closeErr, attempt.child.Close())
	}
	if attempt.pinned != nil {
		closeErr = errors.Join(closeErr, attempt.pinned.Close())
	}
	closeErr = errors.Join(closeErr, attempt.root.Close())
	if attempt.parent != nil {
		closeErr = errors.Join(closeErr, attempt.parent.Close())
	}
	if attempt.grandparent != nil {
		closeErr = errors.Join(closeErr, attempt.grandparent.Close())
	}
	if nsErr := removeEmptyMatrixPrivateAttemptNamespace(attempt.namespacePath, attempt.namespaceID); nsErr != nil && !errors.Is(nsErr, os.ErrPermission) {
		// An empty leftover namespace is host-temp cleanup, not a failed
		// matrix cell. Windows can still see a sharing violation immediately
		// after the last handle is closed.
		if !isMatrixPrivateNamespaceBusy(nsErr) {
			closeErr = errors.Join(closeErr, nsErr)
		}
	}
	return closeErr
}

func isMatrixPrivateNamespaceBusy(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "being used by another process") || strings.Contains(message, "resource busy")
}

func removeEmptyMatrixPrivateAttemptNamespace(path string, identity os.FileInfo) error {
	if strings.TrimSpace(path) == "" || identity == nil {
		return nil
	}
	current, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !os.SameFile(identity, current) {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	var removeErr error
	for pass := 0; pass < 5; pass++ {
		removeErr = os.Remove(path)
		if removeErr == nil || errors.Is(removeErr, os.ErrNotExist) {
			return nil
		}
		if !isMatrixPrivateNamespaceBusy(removeErr) {
			return removeErr
		}
		time.Sleep(10 * time.Millisecond)
	}
	return removeErr
}

func cleanupMatrixPrivateAttemptForExecution(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil {
		return nil
	}
	err := attempt.cleanup()
	if matrixPrivateAttemptCleanupForTest != nil {
		err = errors.Join(err, matrixPrivateAttemptCleanupForTest(attempt.path))
	}
	return err
}

func closeMatrixPrivateAttemptForExecution(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil {
		return nil
	}
	err := attempt.close()
	if matrixPrivateAttemptCloseForTest != nil {
		err = errors.Join(err, matrixPrivateAttemptCloseForTest(attempt.path))
	}
	return err
}

func joinMatrixAttemptResourceErrors(attempt *matrixAttemptResult, returnErr *error, resourceErr error) {
	if resourceErr == nil {
		return
	}
	attempt.CleanupFailed = true
	*returnErr = errors.Join(*returnErr, resourceErr)
	if attempt.FailureCode == "" {
		attempt.FailureStage = "cleanup"
		attempt.FailureCode = "temporary_output_cleanup_failed"
		attempt.Error = "temporary screenshot output cleanup failed"
		return
	}
	if attempt.Error == "" {
		attempt.Error = "screenshot attempt failed; temporary output cleanup is uncertain"
		return
	}
	attempt.Error += "; temporary output cleanup is uncertain"
}

func removeMatrixExpectedEntry(parent *os.Root, expected os.FileInfo, before func(string)) error {
	if parent == nil || expected == nil {
		return nil
	}
	for pass := 0; pass < 4; pass++ {
		entries, err := fs.ReadDir(parent.FS(), ".")
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		found := false
		for _, entry := range entries {
			current, statErr := parent.Lstat(entry.Name())
			if statErr != nil || !os.SameFile(expected, current) {
				continue
			}
			current, statErr = parent.Lstat(entry.Name())
			if statErr != nil || !os.SameFile(expected, current) {
				continue
			}
			found = true
			if before != nil {
				before(filepath.Join(parent.Name(), entry.Name()))
			}
			if err := quarantineMatrixExpectedEntry(parent, entry.Name(), expected); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return err
			}
		}
		if !found {
			return nil
		}
	}
	return errors.New("matrix private attempt cleanup did not converge")
}

func quarantineMatrixExpectedEntry(parent *os.Root, name string, expected os.FileInfo) error {
	// Quarantining first keeps a replacement at the original name out of the
	// cleanup path and the repeated identity checks narrow the race window. The
	// final Remove is still path-based; rootfs has no compare-and-unlink
	// primitive, so a replacement after the last Lstat can race with removal.
	// Callers must treat any resulting cleanup error/uncertainty as a failed
	// cleanup, not as proof of atomic deletion.
	for attempt := 0; attempt < 100; attempt++ {
		quarantine := fmt.Sprintf(".asc-matrix-cleanup-%d", matrixTemporarySequence.Add(1))
		if _, err := parent.Lstat(quarantine); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := parent.Rename(name, quarantine); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return err
		}
		quarantined, err := parent.Lstat(quarantine)
		if err != nil {
			return err
		}
		if !os.SameFile(expected, quarantined) {
			restoreErr := secureopen.RenameNoReplaceInRoot(parent, quarantine, name)
			if restoreErr != nil {
				return errors.Join(errors.New("matrix private attempt entry changed during cleanup"), restoreErr)
			}
			return nil
		}
		latest, err := parent.Lstat(quarantine)
		if err != nil {
			return err
		}
		if !os.SameFile(expected, latest) {
			restoreErr := secureopen.RenameNoReplaceInRoot(parent, quarantine, name)
			if restoreErr != nil {
				return errors.Join(errors.New("matrix private attempt entry changed during cleanup"), restoreErr)
			}
			return nil
		}
		if err := parent.Remove(quarantine); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	}
	return errors.New("matrix private attempt cleanup quarantine unavailable")
}

func matrixPrivateAttemptDirectoryEmpty(root *os.Root) (bool, error) {
	if root == nil {
		return true, nil
	}
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

func matrixPrivateAttemptRootIsStable(attempt matrixPrivateAttemptRoot) error {
	return attempt.matrixPrivateAttemptRootIsStable()
}
