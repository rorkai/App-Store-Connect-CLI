package notarization

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	staplerInventoryMaxBytes   = int64(32 << 30)
	staplerInventoryMaxEntries = 250_000
	staplerInventoryMaxPath    = 4_096
	staplerInventoryVersion    = "asc-stapler-bundle-v1\x00"
	// Keep one directory-read allocation bounded. The aggregate names slice is
	// still capped by staplerInventoryMaxEntries before a batch is appended so a
	// hostile directory cannot make Readdir allocate without bound.
	staplerInventoryReadBatchSize = 256
)

var (
	errStaplerInventoryChanged               = errors.New("artifact directory contents changed during inspection")
	errStaplerRegularFileFingerprintTooLarge = errors.New("regular-file fingerprint exceeds supported size")

	// afterStaplerRegularFileFingerprintFn is a narrow test seam for replacing
	// the pathname after the retained descriptor has been hashed but before
	// the caller is allowed to invoke a child against that pathname.
	// Production leaves it nil.
	afterStaplerRegularFileFingerprintFn func()
)

// This narrow seam keeps the scanner testable without manufacturing hundreds
// of thousands of filesystem entries. Production reads are always bounded by
// staplerInventoryReadBatchSize.
var readdirStaplerInventoryNamesFn = func(file *os.File, count int) ([]string, error) {
	return file.Readdirnames(count)
}

// This narrow seam lets tests place a replacement after the final name
// snapshot, before the scanner rechecks each retained direct child. Production
// leaves it nil.
var afterStaplerInventoryNamesFn func()

// This narrow seam lets tests replace a retained entry after its identity and
// content checks but before the final name-set recapture. Production leaves it
// nil.
var afterStaplerInventoryEntriesFn func()

// This narrow seam lets tests remove an entry after the scanner's first-pass
// Lstat resolved it but before the entry is opened for recursion or hashing.
// Production leaves it nil.
var afterStaplerInventoryEntryLstatFn func(relative string)

// This narrow seam lets tests replace or remove the directory bundle after its
// inventory has been scanned and verified but before the final pathname rebind.
// Production leaves it nil.
var afterStaplerDirectoryInventoryScanFn func()

// staplerDirectoryInventory is deliberately private. It is comparison
// evidence for a single invocation, not a public artifact description.
type staplerDirectoryInventory struct {
	digest     [sha256.Size]byte
	sizeBytes  int64
	entryCount int
}

// staplerInventoryEntryEvidence is the private binding captured for every
// directory entry during the first inventory pass. The follow-up pass uses it
// to catch a same-name replacement that can otherwise evade name-set checks.
// It is comparison evidence only and is never serialized or exposed.
type staplerInventoryEntryEvidence struct {
	info          os.FileInfo
	kind          byte
	sizeBytes     int64
	digest        [sha256.Size]byte
	symlinkTarget string
}

func (evidence staplerInventoryEntryEvidence) equal(other staplerInventoryEntryEvidence) bool {
	return evidence.kind == other.kind &&
		evidence.sizeBytes == other.sizeBytes &&
		evidence.digest == other.digest &&
		evidence.symlinkTarget == other.symlinkTarget &&
		staplerInventoryInfoStable(evidence.info, other.info)
}

// staplerRegularFileFingerprint binds a regular-file target by both its exact
// byte content and its size. The outer target's inode identity alone is not
// sufficient: an in-place rewrite can preserve the same device/inode pair.
// This remains private comparison evidence and is never serialized or exposed
// in command output.
type staplerRegularFileFingerprint struct {
	digest    [sha256.Size]byte
	sizeBytes int64
}

func (fingerprint staplerRegularFileFingerprint) equal(other staplerRegularFileFingerprint) bool {
	return fingerprint.digest == other.digest && fingerprint.sizeBytes == other.sizeBytes
}

func (target *validatedStaplerTarget) captureRegularFileFingerprint(ctx context.Context) (staplerRegularFileFingerprint, error) {
	if target == nil || target.directory || target.identity == nil || target.handle == nil {
		return staplerRegularFileFingerprint{}, errors.New("regular-file target is missing")
	}
	if err := ctx.Err(); err != nil {
		return staplerRegularFileFingerprint{}, err
	}
	pinned, err := target.pinnedIdentity()
	if err != nil {
		return staplerRegularFileFingerprint{}, err
	}
	if !os.SameFile(target.identity, pinned) || !pinned.Mode().IsRegular() {
		return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
	}
	file, err := target.open()
	if err != nil {
		if staplerInventoryPathVanished(err) {
			// The retained descriptor still pins the validated inode and the
			// pathname was checked immediately before this open, so a pathname
			// that no longer resolves proves the artifact changed rather than an
			// operational filesystem failure.
			return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
		}
		return staplerRegularFileFingerprint{}, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return staplerRegularFileFingerprint{}, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(target.identity, openedInfo) {
		return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
	}
	if openedInfo.Size() < 0 || openedInfo.Size() > staplerInventoryMaxBytes {
		return staplerRegularFileFingerprint{}, errStaplerRegularFileFingerprintTooLarge
	}

	digest := sha256.New()
	reader := &staplerInventoryExactReader{
		ctx:       ctx,
		reader:    file,
		remaining: openedInfo.Size(),
	}
	written, err := io.Copy(digest, reader)
	if err != nil {
		return staplerRegularFileFingerprint{}, err
	}
	if written != openedInfo.Size() {
		return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
	}
	finalInfo, err := file.Stat()
	if err != nil {
		return staplerRegularFileFingerprint{}, err
	}
	if !finalInfo.Mode().IsRegular() || !os.SameFile(openedInfo, finalInfo) ||
		finalInfo.Size() != written || !staplerInventoryInfoStable(openedInfo, finalInfo) {
		return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
	}
	finalPinned, err := target.pinnedIdentity()
	if err != nil {
		return staplerRegularFileFingerprint{}, err
	}
	if !os.SameFile(target.identity, finalPinned) || !finalPinned.Mode().IsRegular() {
		return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
	}
	if afterStaplerRegularFileFingerprintFn != nil {
		afterStaplerRegularFileFingerprintFn()
	}
	// The retained descriptor proves that the bytes came from the originally
	// validated inode, but the child receives the pathname. Rebind that rooted,
	// no-follow pathname after hashing so a rename/replacement between open and
	// the child cannot redirect the operation to a different regular file. A
	// regular file behind search-only parents uses its retained openat
	// capability; directory bundles retain the existing rootfs path.
	var currentPathInfo os.FileInfo
	if target.regularAccess != nil {
		currentPathInfo, err = target.regularAccess.probe()
	} else {
		filesystemRoot, rootErr := target.root.OpenRoot()
		if rootErr != nil {
			return staplerRegularFileFingerprint{}, rootErr
		}
		currentPathInfo, err = filesystemRoot.Lstat(target.relative)
		closeErr := filesystemRoot.Close()
		if err == nil && closeErr != nil {
			err = closeErr
		}
	}
	if err != nil {
		if staplerInventoryPathVanished(err) {
			// The command retained and hashed this file, so a pathname that no
			// longer resolves proves the artifact changed rather than an
			// operational filesystem failure.
			return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
		}
		return staplerRegularFileFingerprint{}, err
	}
	if currentPathInfo == nil || currentPathInfo.Mode()&os.ModeSymlink != 0 ||
		!currentPathInfo.Mode().IsRegular() || !os.SameFile(target.identity, currentPathInfo) {
		return staplerRegularFileFingerprint{}, errStaplerInventoryChanged
	}
	var result staplerRegularFileFingerprint
	copy(result.digest[:], digest.Sum(nil))
	result.sizeBytes = written
	return result, nil
}

func (target *validatedStaplerTarget) captureRegularFileFingerprintAtStage(ctx context.Context, stage string) (staplerRegularFileFingerprint, error) {
	fingerprint, err := target.captureRegularFileFingerprint(ctx)
	if err == nil {
		return fingerprint, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return staplerRegularFileFingerprint{}, err
	}
	if errors.Is(err, errStaplerInventoryChanged) || errors.Is(err, errStaplerTargetRaced) {
		return staplerRegularFileFingerprint{}, &staplerTargetIdentityError{stage: stage}
	}
	return staplerRegularFileFingerprint{}, &staplerTargetVerifyError{stage: stage, err: err}
}

func (target *validatedStaplerTarget) verifyRegularFileFingerprint(ctx context.Context, expected staplerRegularFileFingerprint, stage string) error {
	actual, err := target.captureRegularFileFingerprint(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, errStaplerInventoryChanged) || errors.Is(err, errStaplerTargetRaced) {
			return &staplerTargetIdentityError{stage: stage}
		}
		return &staplerTargetVerifyError{stage: stage, err: err}
	}
	if !actual.equal(expected) {
		return &staplerTargetIdentityError{stage: stage}
	}
	return nil
}

func (inventory staplerDirectoryInventory) equal(other staplerDirectoryInventory) bool {
	return inventory.digest == other.digest &&
		inventory.sizeBytes == other.sizeBytes &&
		inventory.entryCount == other.entryCount
}

// captureDirectoryInventory opens the selected directory through the already
// pinned filesystem root and recursively inspects only rooted, no-follow
// entries. Any path-bearing scanner error is kept internal and must be wrapped
// by captureDirectoryInventoryAtStage before reaching command output.
func (target *validatedStaplerTarget) captureDirectoryInventory(ctx context.Context) (staplerDirectoryInventory, error) {
	if target == nil || !target.directory {
		return staplerDirectoryInventory{}, nil
	}
	if err := ctx.Err(); err != nil {
		return staplerDirectoryInventory{}, err
	}
	if target.identity == nil || target.handle == nil {
		return staplerDirectoryInventory{}, errors.New("artifact target descriptor is not retained")
	}
	pinned, err := target.pinnedIdentity()
	if err != nil {
		return staplerDirectoryInventory{}, err
	}
	if !os.SameFile(target.identity, pinned) || !pinned.IsDir() {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}

	filesystemRoot, err := target.root.OpenRoot()
	if err != nil {
		return staplerDirectoryInventory{}, fmt.Errorf("open inventory root: %w", err)
	}
	defer filesystemRoot.Close()

	selected, selectedInfo, err := openStaplerInventoryDirectory(filesystemRoot, target.relative, target.identity)
	if err != nil {
		if staplerInventoryPathVanished(err) {
			// The retained descriptor still pins the validated bundle and the
			// pathname was checked immediately before this open, so a pathname
			// that no longer resolves proves the artifact changed rather than an
			// operational filesystem failure.
			return staplerDirectoryInventory{}, errStaplerInventoryChanged
		}
		return staplerDirectoryInventory{}, err
	}
	defer selected.Close()

	hashTree := sha256.New()
	_, _ = io.WriteString(hashTree, staplerInventoryVersion)
	scanner := staplerInventoryScanner{
		ctx:             ctx,
		treeHash:        hashTree,
		capturedEntries: make(map[string]staplerInventoryEntryEvidence),
		runTestHooks:    true,
	}
	if err := scanner.recordDirectoryEntry(".", selectedInfo); err != nil {
		return staplerDirectoryInventory{}, err
	}
	if err := scanner.scanDirectory(selected, "", selectedInfo); err != nil {
		return staplerDirectoryInventory{}, err
	}
	initialInventory := staplerDirectoryInventoryFromScanner(hashTree, &scanner)

	// Re-scan the bounded tree and compare every captured entry's identity,
	// metadata, and content binding. This is intentionally a consistency
	// recheck, not an atomic filesystem snapshot: a replacement after the final
	// check can still race the caller's next operation.
	verificationSelected, err := selected.Stat(".")
	if err != nil {
		return staplerDirectoryInventory{}, fmt.Errorf("reinspect inventory root: %w", err)
	}
	if !staplerInventoryInfoStable(selectedInfo, verificationSelected) {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}
	verificationHash := sha256.New()
	_, _ = io.WriteString(verificationHash, staplerInventoryVersion)
	verificationScanner := staplerInventoryScanner{
		ctx:             ctx,
		treeHash:        verificationHash,
		expectedEntries: scanner.capturedEntries,
		capturedEntries: make(map[string]staplerInventoryEntryEvidence),
		runTestHooks:    true,
	}
	if err := verificationScanner.recordDirectoryEntry(".", verificationSelected); err != nil {
		return staplerDirectoryInventory{}, err
	}
	if err := verificationScanner.scanDirectory(selected, "", verificationSelected); err != nil {
		return staplerDirectoryInventory{}, err
	}
	verifiedInventory := staplerDirectoryInventoryFromScanner(verificationHash, &verificationScanner)
	if !initialInventory.equal(verifiedInventory) || len(scanner.capturedEntries) != len(verificationScanner.capturedEntries) {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}

	// A second complete scan can still miss a mutation made in an earlier
	// subtree after that subtree was visited and before the scan reached a
	// later sibling. Run one final bounded rooted pass against the evidence
	// captured by the completed verification scan so that window is checked
	// before the caller invokes a child operation. This remains a consistency
	// recheck rather than an atomic filesystem snapshot; a replacement after
	// this final pass is still reported by the caller's next identity check.
	finalHash := sha256.New()
	_, _ = io.WriteString(finalHash, staplerInventoryVersion)
	finalScanner := staplerInventoryScanner{
		ctx:             ctx,
		treeHash:        finalHash,
		expectedEntries: verificationScanner.capturedEntries,
		capturedEntries: make(map[string]staplerInventoryEntryEvidence),
	}
	if err := finalScanner.recordDirectoryEntry(".", verificationSelected); err != nil {
		return staplerDirectoryInventory{}, err
	}
	if err := finalScanner.scanDirectory(selected, "", verificationSelected); err != nil {
		return staplerDirectoryInventory{}, err
	}
	finalInventory := staplerDirectoryInventoryFromScanner(finalHash, &finalScanner)
	if !verifiedInventory.equal(finalInventory) || len(verificationScanner.capturedEntries) != len(finalScanner.capturedEntries) {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}

	finalSelected, err := selected.Stat(".")
	if err != nil {
		return staplerDirectoryInventory{}, fmt.Errorf("reinspect inventory root: %w", err)
	}
	if !staplerInventoryInfoStable(selectedInfo, finalSelected) {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}
	finalPinned, err := target.pinnedIdentity()
	if err != nil {
		return staplerDirectoryInventory{}, err
	}
	if !os.SameFile(target.identity, finalPinned) || !finalPinned.IsDir() {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}
	if afterStaplerDirectoryInventoryScanFn != nil {
		afterStaplerDirectoryInventoryScanFn()
	}
	finalPathInfo, err := filesystemRoot.Lstat(target.relative)
	if err != nil {
		if staplerInventoryPathVanished(err) {
			// The command retained and scanned this bundle, so a pathname that no
			// longer resolves proves the artifact changed rather than an
			// operational filesystem failure.
			return staplerDirectoryInventory{}, errStaplerInventoryChanged
		}
		return staplerDirectoryInventory{}, fmt.Errorf("reinspect inventory path: %w", err)
	}
	if finalPathInfo.Mode()&os.ModeSymlink != 0 || !staplerInventoryInfoStable(target.identity, finalPathInfo) {
		return staplerDirectoryInventory{}, errStaplerInventoryChanged
	}

	return initialInventory, nil
}

func staplerDirectoryInventoryFromScanner(treeHash hash.Hash, scanner *staplerInventoryScanner) staplerDirectoryInventory {
	var digest [sha256.Size]byte
	copy(digest[:], treeHash.Sum(nil))
	return staplerDirectoryInventory{
		digest:     digest,
		sizeBytes:  scanner.sizeBytes,
		entryCount: scanner.entryCount,
	}
}

// captureDirectoryInventoryAtStage maps scanner failures into the existing
// privacy-safe stage errors. The raw scanner cause remains available through
// Unwrap for internal classification, but Error never contains a path.
func (target *validatedStaplerTarget) captureDirectoryInventoryAtStage(ctx context.Context, stage string) (staplerDirectoryInventory, error) {
	inventory, err := target.captureDirectoryInventory(ctx)
	if err == nil {
		return inventory, nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return staplerDirectoryInventory{}, err
	}
	if errors.Is(err, errStaplerInventoryChanged) {
		return staplerDirectoryInventory{}, &staplerTargetIdentityError{stage: stage}
	}
	return staplerDirectoryInventory{}, &staplerTargetVerifyError{stage: stage, err: err}
}

func (target *validatedStaplerTarget) verifyDirectoryInventory(ctx context.Context, expected staplerDirectoryInventory, stage string) error {
	actual, err := target.captureDirectoryInventory(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, errStaplerInventoryChanged) {
			return &staplerTargetIdentityError{stage: stage}
		}
		return &staplerTargetVerifyError{stage: stage, err: err}
	}
	if !actual.equal(expected) {
		return &staplerTargetIdentityError{stage: stage}
	}
	return nil
}

func openStaplerInventoryDirectory(filesystemRoot *os.Root, relative string, expected os.FileInfo) (*os.Root, os.FileInfo, error) {
	if filesystemRoot == nil || expected == nil {
		return nil, nil, errors.New("inventory directory is missing")
	}
	cleaned := filepathCleanForStaplerInventory(relative)
	if cleaned == "." {
		selected, err := filesystemRoot.OpenRoot(".")
		if err != nil {
			return nil, nil, err
		}
		info, err := selected.Stat(".")
		if err != nil {
			_ = selected.Close()
			return nil, nil, err
		}
		if !info.IsDir() || !os.SameFile(expected, info) {
			_ = selected.Close()
			return nil, nil, errStaplerInventoryChanged
		}
		return selected, info, nil
	}
	if err := rootfs.ValidateRelative(cleaned); err != nil {
		return nil, nil, err
	}

	components := strings.Split(filepath.ToSlash(cleaned), "/")
	current := filesystemRoot
	owned := false
	defer func() {
		if owned {
			_ = current.Close()
		}
	}()
	for index, component := range components {
		if component == "" || component == "." || component == ".." {
			return nil, nil, errors.New("invalid inventory directory component")
		}
		before, err := current.Lstat(component)
		if err != nil {
			return nil, nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			return nil, nil, errStaplerInventoryChanged
		}
		child, err := current.OpenRoot(component)
		if err != nil {
			return nil, nil, err
		}
		childInfo, statErr := child.Stat(".")
		if statErr != nil {
			_ = child.Close()
			return nil, nil, statErr
		}
		after, lstatErr := current.Lstat(component)
		if lstatErr != nil {
			_ = child.Close()
			return nil, nil, lstatErr
		}
		if after.Mode()&os.ModeSymlink != 0 || !staplerInventoryInfoStable(before, childInfo) || !staplerInventoryInfoStable(before, after) {
			_ = child.Close()
			return nil, nil, errStaplerInventoryChanged
		}
		if owned {
			_ = current.Close()
		}
		current = child
		owned = true
		if index == len(components)-1 && !os.SameFile(expected, childInfo) {
			return nil, nil, errStaplerInventoryChanged
		}
	}
	info, err := current.Stat(".")
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() || !os.SameFile(expected, info) {
		return nil, nil, errStaplerInventoryChanged
	}
	owned = false
	return current, info, nil
}

// staplerInventoryPathVanished reports whether a failure to reopen a pathname
// that was already validated proves the selected artifact changed. A missing
// component or a parent that is no longer a directory can only mean the path
// was removed or replaced after that check, so the caller reports the
// identity-change signal instead of a generic operational failure.
func staplerInventoryPathVanished(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// staplerInventoryEntryVanished reports whether a filesystem failure observed
// on an entry name that directory enumeration already resolved proves the entry
// disappeared mid-scan. Observing a name and then failing to resolve it is
// evidence that the bundle changed during the scan, so callers report the
// identity-change signal instead of a generic operational scanner failure.
func staplerInventoryEntryVanished(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// staplerInventoryEntryChangedAtOpen reports whether an already-enumerated
// entry changed before its no-follow open. A direct symlink rejection is an
// identity change, not an operational scanner failure; the re-probe also
// covers portable no-follow implementations that return a descriptive error
// instead of ELOOP.
func staplerInventoryEntryChangedAtOpen(parent *os.Root, name string, openErr error) bool {
	if staplerInventoryEntryVanished(openErr) || errors.Is(openErr, syscall.ELOOP) {
		return true
	}
	if parent == nil {
		return false
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return staplerInventoryEntryVanished(err)
	}
	return current.Mode()&os.ModeSymlink != 0
}

type staplerInventoryScanner struct {
	ctx             context.Context
	treeHash        hash.Hash
	sizeBytes       int64
	entryCount      int
	capturedEntries map[string]staplerInventoryEntryEvidence
	expectedEntries map[string]staplerInventoryEntryEvidence
	runTestHooks    bool
	// initialDirectoryReads distinguishes the first bounded enumeration of each
	// directory from the later name-set rechecks in the same initial pass.
	initialDirectoryReads map[string]struct{}
}

func (scanner *staplerInventoryScanner) scanDirectory(directory *os.Root, relative string, initial os.FileInfo) error {
	if err := scanner.checkContext(); err != nil {
		return err
	}
	names, err := scanner.readDirectoryNames(directory, relative, staplerInventoryMaxEntries-scanner.entryCount)
	if err != nil {
		return err
	}
	initialEntries := make(map[string]os.FileInfo, len(names))
	initialSymlinkTargets := make(map[string]string)
	for _, name := range names {
		if err := scanner.checkContext(); err != nil {
			return err
		}
		if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '/') || (os.PathSeparator == '\\' && strings.ContainsRune(name, '\\')) {
			return fmt.Errorf("inventory contains invalid entry name %q", name)
		}
		entryRelative := name
		if relative != "" {
			entryRelative = path.Join(relative, name)
		}
		info, err := directory.Lstat(name)
		if err != nil {
			if staplerInventoryEntryVanished(err) {
				return errStaplerInventoryChanged
			}
			return fmt.Errorf("inspect inventory entry %q: %w", entryRelative, err)
		}
		initialEntries[name] = info
		if scanner.runTestHooks && afterStaplerInventoryEntryLstatFn != nil {
			afterStaplerInventoryEntryLstatFn(entryRelative)
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, err := directory.Readlink(name)
			if err != nil {
				if staplerInventoryEntryVanished(err) || errors.Is(err, syscall.EINVAL) {
					return errStaplerInventoryChanged
				}
				return fmt.Errorf("read inventory symlink %q: %w", entryRelative, err)
			}
			if !staplerContainedSymlinkTarget(entryRelative, target) {
				return fmt.Errorf("inventory symlink %q escapes the bundle", entryRelative)
			}
			initialSymlinkTargets[name] = target
			if err := scanner.recordSymlink(entryRelative, info, target); err != nil {
				return err
			}
		case info.IsDir():
			if err := scanner.recordDirectory(directory, name, entryRelative, info); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			if err := scanner.recordFile(directory, name, entryRelative, info); err != nil {
				return err
			}
		default:
			return fmt.Errorf("inventory entry %q is a special file", entryRelative)
		}
	}

	// Re-open the directory after every listed entry has been inspected. A
	// directory's size and mtime are intentionally ignored because unrelated
	// sibling churn can change them, so a second bounded name enumeration is
	// the content check for entries added or removed after the first
	// enumeration reached EOF. The retained first-pass entry metadata is
	// checked separately below to catch replacement under an unchanged name.
	finalNames, err := scanner.readDirectoryNames(directory, relative, len(names))
	if err != nil {
		return err
	}
	if !slices.Equal(names, finalNames) {
		return errStaplerInventoryChanged
	}
	if err := scanner.checkContext(); err != nil {
		return err
	}
	if scanner.runTestHooks && afterStaplerInventoryNamesFn != nil {
		afterStaplerInventoryNamesFn()
	}
	for _, name := range finalNames {
		if err := scanner.checkContext(); err != nil {
			return err
		}
		entryRelative := name
		if relative != "" {
			entryRelative = path.Join(relative, name)
		}
		initial := initialEntries[name]
		current, err := directory.Lstat(name)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errStaplerInventoryChanged
			}
			return fmt.Errorf("reinspect inventory entry %q: %w", entryRelative, err)
		}
		if initial == nil ||
			initial.IsDir() != current.IsDir() ||
			initial.Mode().IsRegular() != current.Mode().IsRegular() ||
			!staplerInventoryInfoStable(initial, current) {
			return errStaplerInventoryChanged
		}
		if current.Mode()&os.ModeSymlink != 0 {
			initialTarget, ok := initialSymlinkTargets[name]
			if !ok {
				return errStaplerInventoryChanged
			}
			currentTarget, readErr := directory.Readlink(name)
			if readErr != nil || currentTarget != initialTarget || !staplerContainedSymlinkTarget(entryRelative, currentTarget) {
				return errStaplerInventoryChanged
			}
		}
	}
	if scanner.runTestHooks && afterStaplerInventoryEntriesFn != nil {
		afterStaplerInventoryEntriesFn()
	}
	final, err := directory.Stat(".")
	if err != nil {
		return fmt.Errorf("reinspect inventory directory %q: %w", staplerInventoryDisplayPath(relative), err)
	}
	if !staplerInventoryInfoStable(initial, final) {
		return errStaplerInventoryChanged
	}
	// Re-read the direct-child name set after all retained entries and the
	// directory identity have been checked. This closes the window where a new
	// child appears after the earlier finalNames snapshot but before return;
	// directory size and mtime are intentionally not identity signals.
	latestNames, err := scanner.readDirectoryNames(directory, relative, len(finalNames))
	if err != nil {
		return err
	}
	if !slices.Equal(finalNames, latestNames) {
		return errStaplerInventoryChanged
	}
	return nil
}

func (scanner *staplerInventoryScanner) readDirectoryNames(directory *os.Root, relative string, maxNames int) ([]string, error) {
	if err := scanner.checkContext(); err != nil {
		return nil, err
	}
	if maxNames < 0 {
		return nil, errStaplerInventoryChanged
	}
	firstEnumeration := false
	if scanner.expectedEntries == nil {
		if scanner.initialDirectoryReads == nil {
			scanner.initialDirectoryReads = make(map[string]struct{})
		}
		if _, seen := scanner.initialDirectoryReads[relative]; !seen {
			firstEnumeration = true
			scanner.initialDirectoryReads[relative] = struct{}{}
		}
	}
	handle, err := directory.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open inventory directory %q: %w", staplerInventoryDisplayPath(relative), err)
	}
	var names []string
	for {
		if err := scanner.checkContext(); err != nil {
			_ = handle.Close()
			return nil, err
		}
		batch, readErr := readdirStaplerInventoryNamesFn(handle, staplerInventoryReadBatchSize)
		if len(batch) > maxNames-len(names) {
			_ = handle.Close()
			if firstEnumeration {
				return nil, fmt.Errorf("inventory contains more than %d entries", staplerInventoryMaxEntries)
			}
			return nil, errStaplerInventoryChanged
		}
		names = append(names, batch...)
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				_ = handle.Close()
				return nil, fmt.Errorf("read inventory directory %q: %w", staplerInventoryDisplayPath(relative), readErr)
			}
			break
		}
		if len(batch) == 0 {
			break
		}
	}
	if err := handle.Close(); err != nil {
		return nil, fmt.Errorf("close inventory directory %q: %w", staplerInventoryDisplayPath(relative), err)
	}
	sort.Strings(names)
	return names, nil
}

func (scanner *staplerInventoryScanner) recordDirectory(parent *os.Root, name, relative string, before os.FileInfo) error {
	if err := scanner.checkContext(); err != nil {
		return err
	}
	opened, err := parent.OpenRoot(name)
	if err != nil {
		if staplerInventoryDirectoryChangedAtOpen(parent, name, err) {
			return errStaplerInventoryChanged
		}
		return fmt.Errorf("open inventory directory %q: %w", relative, err)
	}
	defer opened.Close()
	openedInfo, err := opened.Stat(".")
	if err != nil {
		return fmt.Errorf("inspect inventory directory %q: %w", relative, err)
	}
	afterOpen, err := parent.Lstat(name)
	if err != nil {
		if staplerInventoryEntryVanished(err) {
			return errStaplerInventoryChanged
		}
		return fmt.Errorf("reinspect inventory directory %q: %w", relative, err)
	}
	if afterOpen.Mode()&os.ModeSymlink != 0 || !staplerInventoryInfoStable(before, openedInfo) || !staplerInventoryInfoStable(before, afterOpen) {
		return errStaplerInventoryChanged
	}
	if err := scanner.recordDirectoryEntry(relative, openedInfo); err != nil {
		return err
	}
	if err := scanner.scanDirectory(opened, relative, openedInfo); err != nil {
		return err
	}
	finalParent, err := parent.Lstat(name)
	if err != nil {
		if staplerInventoryEntryVanished(err) {
			return errStaplerInventoryChanged
		}
		return fmt.Errorf("reinspect inventory directory %q: %w", relative, err)
	}
	if finalParent.Mode()&os.ModeSymlink != 0 || !staplerInventoryInfoStable(openedInfo, finalParent) {
		return errStaplerInventoryChanged
	}
	return nil
}

// staplerInventoryDirectoryChangedAtOpen reports whether an already-enumerated
// directory entry was replaced by a different kind before OpenRoot completed.
// An operational error on an entry that is still the same directory remains an
// inspection failure; only a vanished, non-directory, or symlink replacement is
// inventory change evidence.
func staplerInventoryDirectoryChangedAtOpen(parent *os.Root, name string, openErr error) bool {
	if staplerInventoryPathVanished(openErr) || errors.Is(openErr, syscall.ELOOP) {
		return true
	}
	if parent == nil {
		return false
	}
	current, err := parent.Lstat(name)
	if err != nil {
		return staplerInventoryPathVanished(err)
	}
	return current.Mode()&os.ModeSymlink != 0 || !current.IsDir()
}

func (scanner *staplerInventoryScanner) recordDirectoryEntry(relative string, info os.FileInfo) error {
	if info == nil || !info.IsDir() {
		return errStaplerInventoryChanged
	}
	if err := scanner.noteEntry(relative); err != nil {
		return err
	}
	if err := scanner.recordEvidence(relative, staplerInventoryEntryEvidence{info: info, kind: 'd'}); err != nil {
		return err
	}
	writeStaplerInventoryEntry(scanner.treeHash, 'd', relative, info.Mode(), 0, nil)
	return nil
}

func (scanner *staplerInventoryScanner) recordSymlink(relative string, info os.FileInfo, target string) error {
	if info == nil || info.Mode()&os.ModeSymlink == 0 || target == "" {
		return errStaplerInventoryChanged
	}
	if err := scanner.noteEntry(relative); err != nil {
		return err
	}
	targetBytes := []byte(target)
	digest := sha256.Sum256(targetBytes)
	if err := scanner.recordEvidence(relative, staplerInventoryEntryEvidence{
		info:          info,
		kind:          'l',
		sizeBytes:     int64(len(targetBytes)),
		digest:        digest,
		symlinkTarget: target,
	}); err != nil {
		return err
	}
	writeStaplerInventoryEntry(scanner.treeHash, 'l', relative, info.Mode(), int64(len(targetBytes)), digest[:])
	return nil
}

func (scanner *staplerInventoryScanner) recordFile(parent *os.Root, name, relative string, before os.FileInfo) error {
	if err := scanner.checkContext(); err != nil {
		return err
	}
	if err := scanner.noteEntry(relative); err != nil {
		return err
	}
	if before.Size() < 0 || before.Size() > staplerInventoryMaxBytes-scanner.sizeBytes {
		return fmt.Errorf("inventory content exceeds %d bytes", staplerInventoryMaxBytes)
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(parent, name)
	if err != nil {
		if staplerInventoryEntryChangedAtOpen(parent, name, err) {
			return errStaplerInventoryChanged
		}
		return fmt.Errorf("open inventory file %q: %w", relative, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect inventory file %q: %w", relative, err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(before, openedInfo) ||
		!staplerInventoryInfoStable(before, openedInfo) {
		return errStaplerInventoryChanged
	}

	contentHash := sha256.New()
	reader := &staplerInventoryExactReader{
		ctx:       scanner.ctx,
		reader:    file,
		remaining: openedInfo.Size(),
	}
	written, err := io.Copy(contentHash, reader)
	if err != nil {
		return fmt.Errorf("read inventory file %q: %w", relative, err)
	}
	if written != openedInfo.Size() {
		return errStaplerInventoryChanged
	}
	finalInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("reinspect inventory file %q: %w", relative, err)
	}
	finalPathInfo, err := parent.Lstat(name)
	if err != nil {
		if staplerInventoryEntryVanished(err) {
			return errStaplerInventoryChanged
		}
		return fmt.Errorf("reinspect inventory file %q: %w", relative, err)
	}
	if finalPathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(finalInfo, finalPathInfo) ||
		!staplerInventoryInfoStable(openedInfo, finalInfo) {
		return errStaplerInventoryChanged
	}

	scanner.sizeBytes += written
	var digest [sha256.Size]byte
	copy(digest[:], contentHash.Sum(nil))
	if err := scanner.recordEvidence(relative, staplerInventoryEntryEvidence{
		info:      openedInfo,
		kind:      'f',
		sizeBytes: written,
		digest:    digest,
	}); err != nil {
		return err
	}
	writeStaplerInventoryEntry(scanner.treeHash, 'f', relative, openedInfo.Mode(), written, digest[:])
	return nil
}

func (scanner *staplerInventoryScanner) recordEvidence(relative string, evidence staplerInventoryEntryEvidence) error {
	if scanner.expectedEntries != nil {
		expected, ok := scanner.expectedEntries[relative]
		if !ok || !expected.equal(evidence) {
			return errStaplerInventoryChanged
		}
	}
	if scanner.capturedEntries != nil {
		scanner.capturedEntries[relative] = evidence
	}
	return nil
}

func (scanner *staplerInventoryScanner) noteEntry(relative string) error {
	if len([]byte(relative)) > staplerInventoryMaxPath {
		return fmt.Errorf("inventory path exceeds %d bytes", staplerInventoryMaxPath)
	}
	if scanner.entryCount >= staplerInventoryMaxEntries {
		return fmt.Errorf("inventory contains more than %d entries", staplerInventoryMaxEntries)
	}
	scanner.entryCount++
	return nil
}

// staplerContainedSymlinkTarget validates a symlink lexically without opening
// or following its target. A link may point at another entry inside the
// selected bundle, including a relative target from a nested directory, but
// absolute and escaping targets are rejected before they can affect later
// inspection.
func staplerContainedSymlinkTarget(relative, target string) bool {
	if target == "" {
		return false
	}
	targetSlash := filepath.ToSlash(target)
	if path.IsAbs(targetSlash) || filepath.IsAbs(target) || filepath.VolumeName(target) != "" {
		return false
	}
	resolved := path.Clean(path.Join(path.Dir(filepath.ToSlash(relative)), targetSlash))
	return resolved != ".." && !strings.HasPrefix(resolved, "../")
}

func (scanner *staplerInventoryScanner) checkContext() error {
	if scanner.ctx == nil {
		return nil
	}
	return scanner.ctx.Err()
}

type staplerInventoryExactReader struct {
	ctx       context.Context
	reader    io.Reader
	remaining int64
	verified  bool
}

func (reader *staplerInventoryExactReader) Read(buffer []byte) (int, error) {
	if reader.ctx != nil {
		if err := reader.ctx.Err(); err != nil {
			return 0, err
		}
	}
	if reader.remaining > 0 {
		if int64(len(buffer)) > reader.remaining {
			buffer = buffer[:reader.remaining]
		}
		n, err := reader.reader.Read(buffer)
		reader.remaining -= int64(n)
		if errors.Is(err, io.EOF) && reader.remaining > 0 {
			return n, io.ErrUnexpectedEOF
		}
		return n, err
	}
	if reader.verified {
		return 0, io.EOF
	}
	reader.verified = true
	var probe [1]byte
	n, err := reader.reader.Read(probe[:])
	if n != 0 || err == nil {
		return 0, errStaplerInventoryChanged
	}
	if !errors.Is(err, io.EOF) {
		return 0, err
	}
	return 0, io.EOF
}

func writeStaplerInventoryEntry(tree hash.Hash, kind byte, relative string, mode os.FileMode, size int64, contentDigest []byte) {
	_, _ = tree.Write([]byte{kind})
	var numeric [8]byte
	relativeBytes := []byte(relative)
	binary.BigEndian.PutUint32(numeric[:4], uint32(len(relativeBytes)))
	_, _ = tree.Write(numeric[:4])
	_, _ = tree.Write(relativeBytes)
	binary.BigEndian.PutUint32(numeric[:4], uint32(mode))
	_, _ = tree.Write(numeric[:4])
	binary.BigEndian.PutUint64(numeric[:], uint64(size))
	_, _ = tree.Write(numeric[:])
	if len(contentDigest) == sha256.Size {
		_, _ = tree.Write(contentDigest)
		return
	}
	var empty [sha256.Size]byte
	_, _ = tree.Write(empty[:])
}

func staplerInventoryInfoStable(before, after os.FileInfo) bool {
	if before == nil || after == nil || !os.SameFile(before, after) {
		return false
	}
	if before.Mode() != after.Mode() {
		return false
	}
	// Directory metadata (including size and modification time) can change
	// when unrelated children are created or removed. The recursive inventory
	// below binds directory contents; only regular-file metadata is used here.
	if before.IsDir() {
		return true
	}
	// Keep the size and timestamp check for regular files so a same-size
	// rewrite that restores identical bytes is still treated as changed during
	// one scan.
	return before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func filepathCleanForStaplerInventory(value string) string {
	if value == "" {
		return "."
	}
	cleaned := filepath.Clean(value)
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func staplerInventoryDisplayPath(relative string) string {
	if relative == "" {
		return "."
	}
	return relative
}
