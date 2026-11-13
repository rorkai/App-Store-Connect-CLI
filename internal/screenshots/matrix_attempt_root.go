package screenshots

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

var matrixTemporarySequence atomic.Uint64

// matrixPrivateAttemptRootBeforeChildRootForTest is a test-only seam placed
// after the selected attempt directory is pinned and before its parent is
// reopened. It models a directory-entry replacement in the construction
// window without changing production behavior.
var matrixPrivateAttemptRootBeforeChildRootForTest func(string)

// matrixPrivateAttemptBeforePinForTest is a test-only seam placed after the
// anchored child descriptor has been checked but before rootfs.New pins the
// selected pathname. It models a replacement in the initial pin window.
var matrixPrivateAttemptBeforePinForTest func(string)

// matrixPrivateAttemptBeforeParentLockForTest is a test-only seam placed after
// the child has been reopened/validated and immediately before the parent
// directory is locked. It models the final construction-window replacement.
var matrixPrivateAttemptBeforeParentLockForTest func(string)

// matrixPrivateAttemptBeforeChildLockForTest is a test-only seam placed just
// before the provider child is locked. The subsequent identity check must bind
// the held child to its current parent entry even if that entry was replaced
// in this last transition window.
var matrixPrivateAttemptBeforeChildLockForTest func(string)

// matrixPrivateAttemptBeforeCleanupRemoveForTest is a test-only seam placed
// after cleanup has validated an entry identity and before it quarantines the
// entry. It models a replacement in the final publication/cleanup window.
var matrixPrivateAttemptBeforeCleanupRemoveForTest func(string)

// These seams inject cleanup/close failures after the real resource handling
// has run, so tests can verify that a successful attempt never hides resource
// uncertainty without leaking temporary directories.
var (
	matrixPrivateAttemptCleanupForTest func(string) error
	matrixPrivateAttemptCloseForTest   func(string) error
)

// matrixPrivateAttemptOperationForTest is a test-only seam that can inject a
// construction failure immediately before a rooted filesystem operation. It
// deliberately does not replace the production operation or allow tests to
// select a different path.
var matrixPrivateAttemptOperationForTest func(stage, path string) error

// matrixPrivateAttemptParentCreatedForTest observes the freshly-created
// private parent so tests can replace its directory entry while an early
// construction failure is being injected.
var matrixPrivateAttemptParentCreatedForTest func(string)

var errMatrixPrivateAttemptCleanupUncertain = errors.New("private matrix attempt cleanup uncertain")

// matrixPrivateAttemptRoot retains both the rooted API used by the provider
// and an independently held parent/child descriptor. The latter lets cleanup
// find and remove the original directory after a provider renames it, while
// never recursively deleting a replacement that now occupies the old path.
type matrixPrivateAttemptRoot struct {
	root              rootfs.Root
	path              string
	grandparent       *os.Root
	parent            *os.Root
	pinned            *os.Root
	child             *os.Root
	childCreator      *os.File
	outputCreator     *os.File
	fileDACLs         []*matrixPrivateAttemptDACLHandle
	parentID          os.FileInfo
	identity          os.FileInfo
	namespaceID       os.FileInfo
	namespacePath     string
	output            *rootfs.Root
	grandparentDACL   *matrixPrivateAttemptDACLHandle
	parentDACL        *matrixPrivateAttemptDACLHandle
	childDACL         *matrixPrivateAttemptDACLHandle
	parentLocked      bool
	childLocked       bool
	grandparentLocked bool
}

func matrixPrivateAttemptOperation(stage, path string) error {
	if matrixPrivateAttemptOperationForTest != nil {
		return matrixPrivateAttemptOperationForTest(stage, path)
	}
	return nil
}

func closeMatrixPrivateAttemptRoots(roots ...*os.Root) error {
	var closeErr error
	for _, root := range roots {
		if root != nil {
			closeErr = errors.Join(closeErr, root.Close())
		}
	}
	return closeErr
}

func matrixPrivateAttemptConstructionFailure(primary, cleanupErr error, uncertain bool, roots ...*os.Root) error {
	// Before a rooted identity has been captured, the freshly-created parent
	// cannot be removed by pathname: that entry may already name a replacement.
	// In that fail-closed case the private temporary tree can remain for the
	// host's temporary-directory cleanup, and the closed uncertainty sentinel
	// tells callers not to report construction cleanup as complete.
	closeErr := closeMatrixPrivateAttemptRoots(roots...)
	if cleanupErr != nil || closeErr != nil {
		uncertain = true
	}
	var errs []error
	if primary != nil {
		errs = append(errs, primary)
	}
	if uncertain {
		errs = append(errs, errMatrixPrivateAttemptCleanupUncertain)
	}
	if cleanupErr != nil {
		errs = append(errs, cleanupErr)
	}
	if closeErr != nil {
		errs = append(errs, closeErr)
	}
	return errors.Join(errs...)
}

// lockMatrixPrivateAttemptParentRetained removes the directory-entry mutation rights
// needed to rename or replace the provider's destination. The child remains
// writable, so legacy path-based adapters can create their files, but a same
// user rename+symlink swap cannot redirect those writes outside the pinned
// child root. The platform helper applies a real protected DACL on Windows;
// mode bits alone are not a confidentiality or identity boundary there.
func lockMatrixPrivateAttemptParentRetained(parent *os.Root) (*matrixPrivateAttemptDACLHandle, error) {
	if parent == nil {
		return nil, errors.New("private matrix attempt parent is unavailable")
	}
	return lockMatrixPrivateAttemptDirectoryRetained(parent)
}

func restoreMatrixPrivateAttemptDirectory(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	if err := unlockMatrixPrivateAttemptDirectoryRetained(handle, root); err != nil {
		// Keep the retained WRITE_DAC handle open so the final close path can
		// retry a transient restoration failure.
		return err
	}
	return closeMatrixPrivateAttemptDACLHandle(handle)
}

type matrixPrivateAttemptDirectoryUnlockFunc func(*matrixPrivateAttemptDACLHandle, *os.Root) error

type matrixPrivateAttemptDACLHandleCloseFunc func(*matrixPrivateAttemptDACLHandle) error

func finalizeMatrixPrivateAttemptDACLHandle(handle *matrixPrivateAttemptDACLHandle) error {
	return finalizeMatrixPrivateAttemptDACLHandleWith(handle, closeMatrixPrivateAttemptDACLHandle)
}

func finalizeMatrixPrivateAttemptDACLHandleWith(handle *matrixPrivateAttemptDACLHandle, closeHandle matrixPrivateAttemptDACLHandleCloseFunc) error {
	firstErr := closeHandle(handle)
	if firstErr == nil {
		return nil
	}
	return errors.Join(firstErr, closeHandle(handle))
}

func retryMatrixPrivateAttemptDirectoryRestore(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	return retryMatrixPrivateAttemptDirectoryRestoreWith(handle, root, unlockMatrixPrivateAttemptDirectoryRetained)
}

func retryMatrixPrivateAttemptDirectoryRestoreWith(handle *matrixPrivateAttemptDACLHandle, root *os.Root, unlock matrixPrivateAttemptDirectoryUnlockFunc) error {
	firstErr := unlock(handle, root)
	if firstErr == nil {
		return finalizeMatrixPrivateAttemptDACLHandle(handle)
	}
	if retryErr := unlock(handle, root); retryErr != nil {
		return errors.Join(firstErr, retryErr)
	}
	return finalizeMatrixPrivateAttemptDACLHandle(handle)
}

// finalizeMatrixPrivateAttemptDirectory is for construction failures that do
// not return an attempt with a later close phase. Retry a transient restore
// once, then close the retained handle even if the directory is still locked.
func finalizeMatrixPrivateAttemptDirectory(handle *matrixPrivateAttemptDACLHandle, root *os.Root) error {
	restoreErr := retryMatrixPrivateAttemptDirectoryRestore(handle, root)
	if restoreErr == nil {
		return nil
	}
	return errors.Join(restoreErr, finalizeMatrixPrivateAttemptDACLHandle(handle))
}

func lockMatrixPrivateAttemptChild(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil || attempt.pinned == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	if matrixPrivateAttemptBeforeChildLockForTest != nil {
		matrixPrivateAttemptBeforeChildLockForTest(attempt.path)
	}
	creator := attempt.childCreator
	attempt.childCreator = nil
	handle, err := lockMatrixPrivateAttemptDirectoryCreated(creator, attempt.pinned)
	if err != nil {
		return err
	}
	attempt.childDACL = handle
	attempt.childLocked = true
	if err := verifyMatrixPrivateAttemptChildIdentity(attempt); err != nil {
		unlockErr := unlockMatrixPrivateAttemptChild(attempt)
		return errors.Join(err, unlockErr)
	}
	return nil
}

func verifyMatrixPrivateAttemptChildIdentity(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil || attempt.grandparent == nil || attempt.parent == nil || attempt.pinned == nil || attempt.child == nil || attempt.identity == nil || attempt.parentID == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	childInfo, err := attempt.pinned.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, childInfo) {
		return errors.New("private matrix attempt root changed while locking")
	}
	heldChildInfo, err := attempt.child.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, heldChildInfo) {
		return errors.New("private matrix attempt root changed while locking")
	}
	parentInfo, err := attempt.parent.Stat(".")
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.parentID, parentInfo) {
		return errors.New("private matrix attempt parent changed while locking")
	}
	parentName := filepath.Base(filepath.Dir(attempt.path))
	currentParent, err := attempt.grandparent.Lstat(parentName)
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.parentID, currentParent) {
		return errors.New("private matrix attempt parent changed while locking")
	}
	childName := filepath.Base(attempt.path)
	currentChild, err := attempt.parent.Lstat(childName)
	if err != nil {
		return err
	}
	if !os.SameFile(attempt.identity, currentChild) {
		return errors.New("private matrix attempt root changed while locking")
	}
	reopened, err := attempt.parent.OpenRoot(childName)
	if err != nil {
		return err
	}
	reopenedInfo, statErr := reopened.Stat(".")
	closeErr := reopened.Close()
	if statErr != nil || closeErr != nil {
		return errors.Join(statErr, closeErr)
	}
	if !os.SameFile(attempt.identity, reopenedInfo) {
		return errors.New("private matrix attempt root changed while locking")
	}
	if attempt.output != nil {
		outputRoot := *attempt.output
		if err := verifyMatrixRetainedOutputRoot(outputRoot); err != nil {
			return err
		}
		openedOutput, err := outputRoot.OpenRoot()
		if err != nil {
			return err
		}
		_, statErr := openedOutput.Stat(".")
		closeErr := openedOutput.Close()
		if statErr != nil || closeErr != nil {
			return errors.Join(statErr, closeErr)
		}
	}
	return nil
}

func unlockMatrixPrivateAttemptChild(attempt *matrixPrivateAttemptRoot) error {
	if attempt == nil || !attempt.childLocked {
		return nil
	}
	if attempt.pinned == nil {
		return errors.New("private matrix attempt root is unavailable")
	}
	if err := restoreMatrixPrivateAttemptDirectory(attempt.childDACL, attempt.pinned); err != nil {
		return err
	}
	attempt.childDACL = nil
	attempt.childLocked = false
	return nil
}

// createMatrixPrivateAttemptRoot creates an owner-only staging directory for
// capture or framing adapters. Provider paths are still required by the
// existing adapter contracts, but they no longer sit below user-selected
// artifact roots where another process could replace an attempt directory and
// redirect pathname-based writes outside the selected root.
func createMatrixPrivateAttemptRoot() (result matrixPrivateAttemptRoot, returnErr error) {
	parentPath, grandparentCreator, parentCreator, err := createMatrixPrivateAttemptParentWithHandles()
	if err != nil {
		return matrixPrivateAttemptRoot{}, err
	}
	var childCreator *os.File
	defer func() {
		for _, file := range []*os.File{childCreator, parentCreator, grandparentCreator} {
			if file != nil {
				returnErr = errors.Join(returnErr, file.Close())
			}
		}
	}()
	cleanupMatrixPrivateAttemptConstruction := func(parent, grandparent, pinned *os.Root, identity, parentID os.FileInfo) error {
		var creatorErr error
		for _, file := range []*os.File{childCreator, parentCreator, grandparentCreator} {
			if file != nil {
				creatorErr = errors.Join(creatorErr, file.Close())
			}
		}
		childCreator = nil
		parentCreator = nil
		grandparentCreator = nil
		return errors.Join(creatorErr, cleanupMatrixPrivateAttemptConstructionAfterHandles(parent, grandparent, pinned, identity, parentID))
	}
	createdParentID, err := os.Lstat(parentPath)
	if err != nil || !createdParentID.IsDir() {
		if err == nil {
			err = errors.New("private matrix attempt parent is not a directory")
		}
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true)
	}
	if matrixPrivateAttemptParentCreatedForTest != nil {
		matrixPrivateAttemptParentCreatedForTest(parentPath)
	}
	grandparentPath := filepath.Dir(parentPath)
	if err := matrixPrivateAttemptOperation("grandparent_open", grandparentPath); err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true)
	}
	grandparent, err := os.OpenRoot(grandparentPath)
	if err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true)
	}
	parentName := filepath.Base(parentPath)
	if err := matrixPrivateAttemptOperation("parent_open", parentPath); err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, grandparent)
	}
	parent, err := grandparent.OpenRoot(parentName)
	if err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, grandparent)
	}
	if err := matrixPrivateAttemptOperation("parent_stat", parentPath); err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, parent, grandparent)
	}
	parentID, err := parent.Stat(".")
	if err != nil {
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, nil, true, parent, grandparent)
	}
	if !os.SameFile(createdParentID, parentID) {
		identityErr := errors.New("private matrix attempt parent changed while opening")
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(identityErr, nil, true, parent, grandparent)
	}
	name := fmt.Sprintf(".asc-matrix-attempt-%d", matrixTemporarySequence.Add(1))
	path := filepath.Join(parentPath, name)
	if err := matrixPrivateAttemptOperation("mkdir", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	childCreator, err = createMatrixPrivateAttemptChildRetained(parent, parentPath, name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	if err := matrixPrivateAttemptOperation("child_lstat", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, true, parent, grandparent)
	}
	createdID, err := parent.Lstat(name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, nil, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, true, parent, grandparent)
	}
	if err := matrixPrivateAttemptOperation("child_open", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	anchoredChild, err := parent.OpenRoot(name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, parent, grandparent)
	}
	if err := matrixPrivateAttemptOperation("child_stat", path); err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, anchoredChild, parent, grandparent)
	}
	anchoredID, err := anchoredChild.Stat(".")
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, nil, createdID, parentID)
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, anchoredChild, parent, grandparent)
	}
	if !os.SameFile(createdID, anchoredID) {
		identityErr := errors.New("private matrix attempt root changed before child open")
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(identityErr, nil, true, anchoredChild, parent, grandparent)
	}
	if matrixPrivateAttemptBeforePinForTest != nil {
		matrixPrivateAttemptBeforePinForTest(path)
	}
	root, err := rootfs.New(path)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	pinned, err := root.OpenRoot()
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	pinnedID, err := pinned.Stat(".")
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	if !os.SameFile(anchoredID, pinnedID) {
		identityErr := errors.New("private matrix attempt root changed while opening")
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, anchoredChild, anchoredID, parentID)
		closeErr := errors.Join(pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(identityErr, cleanupErr, closeErr)
	}
	if matrixPrivateAttemptRootBeforeChildRootForTest != nil {
		matrixPrivateAttemptRootBeforeChildRootForTest(path)
	}
	reopenedChild, err := parent.OpenRoot(name)
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	identity, err := reopenedChild.Stat(".")
	if err != nil {
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(reopenedChild.Close(), pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(err, cleanupErr, closeErr)
	}
	if !os.SameFile(pinnedID, identity) {
		identityErr := errors.New("private matrix attempt root changed while opening")
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(reopenedChild.Close(), pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, errors.Join(identityErr, cleanupErr, closeErr)
	}
	if matrixPrivateAttemptBeforeParentLockForTest != nil {
		matrixPrivateAttemptBeforeParentLockForTest(path)
	}
	if err := matrixPrivateAttemptOperation("parent_lock", parentPath); err != nil {
		closeErr := reopenedChild.Close()
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		constructionErr := matrixPrivateAttemptConstructionFailure(err, cleanupErr, false, pinned, anchoredChild, parent, grandparent)
		return matrixPrivateAttemptRoot{}, errors.Join(constructionErr, closeErr, root.Close())
	}
	parentCreationHandle := parentCreator
	parentCreator = nil
	parentDACL, err := lockMatrixPrivateAttemptDirectoryCreated(parentCreationHandle, parent)
	if err != nil {
		childCloseErr := reopenedChild.Close()
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(childCloseErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, errors.Join(cleanupErr, closeErr), false)
	}
	grandparentCreationHandle := grandparentCreator
	grandparentCreator = nil
	grandparentDACL, err := lockMatrixPrivateAttemptDirectoryCreated(grandparentCreationHandle, grandparent)
	if err != nil {
		unlockErr := finalizeMatrixPrivateAttemptDirectory(parentDACL, parent)
		childCloseErr := reopenedChild.Close()
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(childCloseErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(err, errors.Join(unlockErr, cleanupErr, closeErr), false)
	}
	currentParentEntry, parentEntryErr := grandparent.Lstat(parentName)
	currentParentID, parentStatErr := parent.Stat(".")
	currentChildEntry, childEntryErr := parent.Lstat(name)
	validatedChild, childOpenErr := parent.OpenRoot(name)
	var validatedChildID os.FileInfo
	var validatedChildCloseErr error
	if childOpenErr == nil {
		validatedChildID, childOpenErr = validatedChild.Stat(".")
		validatedChildCloseErr = validatedChild.Close()
	}
	if parentEntryErr != nil || parentStatErr != nil || childEntryErr != nil || childOpenErr != nil || validatedChildCloseErr != nil || !os.SameFile(createdParentID, currentParentEntry) || !os.SameFile(createdParentID, currentParentID) || !os.SameFile(pinnedID, currentChildEntry) || !os.SameFile(pinnedID, validatedChildID) {
		identityErr := errors.Join(
			errors.New("private matrix attempt root changed before parent lock"),
			parentEntryErr,
			parentStatErr,
			childEntryErr,
			childOpenErr,
			validatedChildCloseErr,
		)
		unlockErr := errors.Join(
			finalizeMatrixPrivateAttemptDirectory(grandparentDACL, grandparent),
			finalizeMatrixPrivateAttemptDirectory(parentDACL, parent),
		)
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		closeErr := errors.Join(reopenedChild.Close(), pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(identityErr, errors.Join(unlockErr, cleanupErr, closeErr), false)
	}
	closeErr := reopenedChild.Close()
	if closeErr != nil {
		unlockErr := errors.Join(
			finalizeMatrixPrivateAttemptDirectory(grandparentDACL, grandparent),
			finalizeMatrixPrivateAttemptDirectory(parentDACL, parent),
		)
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		cleanupFailure := errors.Join(unlockErr, cleanupErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(closeErr, cleanupFailure, false)
	}
	namespaceID, namespaceErr := grandparent.Stat(".")
	if namespaceErr != nil {
		unlockErr := errors.Join(
			finalizeMatrixPrivateAttemptDirectory(grandparentDACL, grandparent),
			finalizeMatrixPrivateAttemptDirectory(parentDACL, parent),
		)
		cleanupErr := cleanupMatrixPrivateAttemptConstruction(parent, grandparent, pinned, pinnedID, parentID)
		cleanupFailure := errors.Join(unlockErr, cleanupErr, pinned.Close(), root.Close(), anchoredChild.Close(), parent.Close(), grandparent.Close())
		return matrixPrivateAttemptRoot{}, matrixPrivateAttemptConstructionFailure(namespaceErr, cleanupFailure, false)
	}
	result = matrixPrivateAttemptRoot{
		root: root, path: path, grandparent: grandparent,
		parent: parent, pinned: pinned, child: anchoredChild, childCreator: childCreator, parentID: parentID, identity: identity,
		namespaceID: namespaceID, namespacePath: grandparent.Name(),
		grandparentDACL: grandparentDACL, parentDACL: parentDACL,
		parentLocked: true, grandparentLocked: true,
	}
	childCreator = nil
	return result, nil
}

// openMatrixPrivateAttemptOutputRoot creates and pins the writable directory
// handed to a rooted plan/frame adapter. The attempt child itself is locked
// before the returned root is used by an external path-based process, so the
// child/output path pair cannot be redirected by replacing the nested output
// entry. The identity comparison also closes the construction race before the
// child lock is acquired.
func openMatrixPrivateAttemptOutputRoot(attempt *matrixPrivateAttemptRoot) (rootfs.Root, error) {
	if attempt == nil || attempt.pinned == nil {
		return rootfs.Root{}, errors.New("private matrix attempt root is unavailable")
	}
	outputCreator, err := createMatrixPrivateAttemptOutputDirInRootRetained(attempt.pinned)
	if err != nil {
		return rootfs.Root{}, err
	}
	attempt.outputCreator = outputCreator
	outputPath := filepath.Join(attempt.path, "output")
	anchored, err := attempt.pinned.OpenRoot("output")
	if err != nil {
		return rootfs.Root{}, err
	}
	anchoredInfo, anchoredErr := anchored.Stat(".")
	if anchoredErr != nil {
		_ = anchored.Close()
		return rootfs.Root{}, anchoredErr
	}
	output, err := rootfs.New(outputPath)
	if err != nil {
		_ = anchored.Close()
		return rootfs.Root{}, err
	}
	opened, err := output.OpenRoot()
	if err != nil {
		_ = output.Close()
		_ = anchored.Close()
		return rootfs.Root{}, err
	}
	openedInfo, openedErr := opened.Stat(".")
	closeErr := opened.Close()
	anchoredCloseErr := anchored.Close()
	if anchoredCloseErr != nil {
		closeErr = errors.Join(closeErr, anchoredCloseErr)
	}
	if openedErr != nil || closeErr != nil || !os.SameFile(anchoredInfo, openedInfo) {
		_ = output.Close()
		if openedErr != nil || closeErr != nil {
			return rootfs.Root{}, errors.Join(openedErr, closeErr)
		}
		return rootfs.Root{}, errors.New("private matrix attempt output changed while opening")
	}
	attempt.output = &output
	return output, nil
}
