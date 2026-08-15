// Package rootfs provides rooted filesystem operations for paths that are not
// fully trusted, such as filenames, directory components, or manifest entries
// that come from a repository checkout or a remote API response.
//
// Every operation is anchored to a trusted root chosen by the operator (for
// example a --out-dir flag, a manifest directory, or the resolved .asc
// directory). Paths are validated lexically so absolute paths, volume or
// UNC-style changes, and parent traversal are rejected, and filesystem access
// refuses to follow symlinks for any component below the root. Writes stage
// through unpredictable, exclusive, no-follow temporary files so a
// pre-created symlink cannot redirect them.
//
// Roots created with AllowingInternalSymlinks relax only the parent-component
// rule, accepting a symlinked directory whose target stays inside the root; a
// symlinked final component is always refused.
package rootfs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

var (
	// ErrEscapesRoot reports a path that does not stay beneath the trusted root.
	ErrEscapesRoot = errors.New("path escapes trusted root")
	// ErrSymlink reports a path component that is a symlink below the trusted root.
	ErrSymlink = errors.New("refusing to follow symlink")
)

const (
	temporaryFilePattern = ".asc-tmp-*"
	backupFilePattern    = ".asc-tmp-backup-*"
)

// Root is a trusted directory anchor for rooted filesystem operations.
type Root struct {
	path             string
	openPath         string
	selectedIdentity *rootIdentity
	pendingCreation  *rootCreation
	// internalSymlinks tolerates symlinked components below the root when they
	// resolve back inside the root.
	internalSymlinks bool
	// afterValidationForTest makes path-swap regressions deterministic. It is
	// intentionally unexported and unset outside package tests.
	afterValidationForTest func()
	// beforeOpenRootForTest makes trusted-root path-swap regressions
	// deterministic. It is intentionally unexported and unset outside tests.
	beforeOpenRootForTest func()
	// beforeCreateRootForTest makes missing-root ancestor replacement races
	// deterministic. It is intentionally unexported and unset outside tests.
	beforeCreateRootForTest func()
	// renameNoReplaceForTest makes unsupported-filesystem regressions
	// deterministic. It is intentionally unexported and unset outside tests.
	renameNoReplaceForTest func(root *os.Root, oldName, newName string) error
	// requireNativeNoReplace preserves CreateNewFileAtomic's strict contract
	// while CreateNewFrom may use the atomic hard-link fallback.
	requireNativeNoReplace bool
}

type rootCreation struct {
	mu           sync.Mutex
	lexicalBase  string
	physicalBase string
	suffix       []string
	baseIdentity *rootIdentity
}

type rootIdentity struct {
	mu         sync.RWMutex
	pinned     *os.Root
	cleanup    runtime.Cleanup
	hasCleanup bool
	closed     bool
}

func (identity *rootIdentity) isPinned() bool {
	if identity == nil {
		return false
	}
	identity.mu.RLock()
	defer identity.mu.RUnlock()
	return identity.pinned != nil
}

// pin retains one descriptor for the selected directory. Keeping that
// descriptor open prevents the original inode or file ID from being recycled
// while Root values still refer to it. The cleanup is attached to the shared
// identity rather than a Root copy so the descriptor is closed exactly once.
func (identity *rootIdentity) pin(candidate *os.Root) bool {
	if candidate == nil {
		return false
	}
	if identity == nil {
		_ = candidate.Close()
		return false
	}
	identity.mu.Lock()
	defer identity.mu.Unlock()
	if identity.closed {
		_ = candidate.Close()
		return false
	}
	if identity.pinned == nil {
		identity.pinned = candidate
		identity.cleanup = runtime.AddCleanup(identity, closePinnedRoot, candidate)
		identity.hasCleanup = true
		return true
	}
	selectedInfo, selectedErr := identity.pinned.Stat(".")
	candidateInfo, candidateErr := candidate.Stat(".")
	_ = candidate.Close()
	return selectedErr == nil && candidateErr == nil && os.SameFile(selectedInfo, candidateInfo)
}

func (identity *rootIdentity) matches(candidate os.FileInfo) bool {
	if identity == nil || candidate == nil {
		return false
	}
	identity.mu.RLock()
	defer identity.mu.RUnlock()
	if identity.pinned == nil {
		return false
	}
	selected, err := identity.pinned.Stat(".")
	return err == nil && os.SameFile(selected, candidate)
}

func closePinnedRoot(root *os.Root) {
	_ = root.Close()
}

func (identity *rootIdentity) close() error {
	if identity == nil {
		return nil
	}
	identity.mu.Lock()
	if identity.closed {
		identity.mu.Unlock()
		return nil
	}
	identity.closed = true
	pinned := identity.pinned
	identity.pinned = nil
	cleanup := identity.cleanup
	hasCleanup := identity.hasCleanup
	identity.hasCleanup = false
	identity.mu.Unlock()
	if hasCleanup {
		cleanup.Stop()
	}
	if pinned != nil {
		return pinned.Close()
	}
	return nil
}

// New returns a Root anchored at path. The root itself is operator-selected and
// may live outside the current repository; only paths below it are constrained.
func New(path string) (Root, error) {
	if path == "" {
		return Root{}, fmt.Errorf("%w: trusted root path is empty", ErrEscapesRoot)
	}
	if strings.ContainsRune(path, 0) {
		return Root{}, fmt.Errorf("%w: trusted root path contains a NUL byte", ErrEscapesRoot)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return Root{}, fmt.Errorf("resolve trusted root %q: %w", path, err)
	}
	absolute = filepath.Clean(absolute)
	lexicalBase, physicalBase, suffix, err := resolveRootSelection(absolute)
	if err != nil {
		return Root{}, fmt.Errorf("resolve trusted root %q: %w", path, err)
	}
	openPath := filepath.Join(append([]string{physicalBase}, suffix...)...)
	selectedExists := len(suffix) == 0
	root := Root{path: absolute, openPath: openPath, selectedIdentity: &rootIdentity{}}
	if !selectedExists {
		base, err := openAbsoluteRootNoFollow(physicalBase)
		if err != nil {
			return Root{}, fmt.Errorf("open trusted root ancestor %q: %w", lexicalBase, err)
		}
		baseInfo, statErr := base.Stat(".")
		if statErr != nil {
			_ = base.Close()
			return Root{}, fmt.Errorf("stat trusted root ancestor %q: %w", lexicalBase, statErr)
		}
		selectedAtPath, statErr := os.Stat(lexicalBase)
		if statErr != nil {
			_ = base.Close()
			return Root{}, fmt.Errorf("stat selected root ancestor %q: %w", lexicalBase, statErr)
		}
		if !os.SameFile(baseInfo, selectedAtPath) {
			_ = base.Close()
			return Root{}, symlinkError(lexicalBase)
		}
		baseIdentity := &rootIdentity{}
		if !baseIdentity.pin(base) {
			return Root{}, symlinkError(lexicalBase)
		}
		root.pendingCreation = &rootCreation{
			lexicalBase:  lexicalBase,
			physicalBase: physicalBase,
			suffix:       append([]string(nil), suffix...),
			baseIdentity: baseIdentity,
		}
		return root, nil
	}
	selected, err := openAbsoluteRootNoFollow(openPath)
	if err != nil {
		return Root{}, fmt.Errorf("open trusted root %q: %w", path, err)
	}
	identity, statErr := selected.Stat(".")
	if statErr != nil {
		_ = selected.Close()
		return Root{}, fmt.Errorf("stat trusted root %q: %w", path, statErr)
	}
	selectedAtPath, err := os.Stat(absolute)
	if err != nil {
		_ = selected.Close()
		return Root{}, fmt.Errorf("stat selected root %q: %w", path, err)
	}
	if !os.SameFile(identity, selectedAtPath) {
		_ = selected.Close()
		return Root{}, symlinkError(absolute)
	}
	if !root.selectedIdentity.pin(selected) {
		return Root{}, symlinkError(absolute)
	}
	return root, nil
}

func resolveRootSelection(absolute string) (string, string, []string, error) {
	candidate := absolute
	reversedSuffix := make([]string, 0)
	for {
		_, err := os.Lstat(candidate)
		if err == nil {
			physical, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", "", nil, fmt.Errorf("resolve existing ancestor %q: %w", candidate, err)
			}
			resolvedInfo, err := os.Stat(physical)
			if err != nil {
				return "", "", nil, fmt.Errorf("stat existing ancestor %q: %w", candidate, err)
			}
			if !resolvedInfo.IsDir() {
				return "", "", nil, fmt.Errorf("trusted root ancestor %q is not a directory", candidate)
			}
			suffix := make([]string, len(reversedSuffix))
			for index := range reversedSuffix {
				suffix[len(reversedSuffix)-1-index] = reversedSuffix[index]
			}
			return candidate, physical, suffix, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", "", nil, fmt.Errorf("inspect trusted root ancestor %q: %w", candidate, err)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", "", nil, fmt.Errorf("no existing ancestor for trusted root %q", absolute)
		}
		component := filepath.Base(candidate)
		if err := validateMissingRootComponent(component); err != nil {
			return "", "", nil, err
		}
		reversedSuffix = append(reversedSuffix, component)
		candidate = parent
	}
}

func validateMissingRootComponent(component string) error {
	if component == "" || component == "." || component == ".." ||
		filepath.Clean(component) != component || filepath.IsAbs(component) ||
		filepath.VolumeName(component) != "" || strings.ContainsRune(component, 0) {
		return fmt.Errorf("%w: unsafe missing trusted-root component %q", ErrEscapesRoot, component)
	}
	return nil
}

// OpenFile opens an existing regular file through a rooted traversal. Paths
// below the current working directory or OS temporary directory use that
// trusted anchor; other paths use their filesystem root. Unlike a
// final-component O_NOFOLLOW open, this rejects symlinks in parent components
// below the selected root.
func OpenFile(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: path is empty", ErrEscapesRoot)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %q: %w", path, err)
	}
	volumeRoot := filepath.VolumeName(absolute) + string(filepath.Separator)
	rootPath := volumeRoot
	for _, candidate := range []string{workingDirectory(), os.TempDir()} {
		candidate, err = filepath.Abs(candidate)
		if err != nil {
			continue
		}
		candidate = filepath.Clean(candidate)
		if _, err := relativeWithinRoot(candidate, absolute); err == nil && len(candidate) > len(rootPath) {
			rootPath = candidate
		}
	}
	root, err := New(rootPath)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root.Path(), absolute)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve %q below %q: %w", ErrEscapesRoot, path, root.Path(), err)
	}
	return root.OpenFile(relative)
}

func workingDirectory() string {
	path, err := os.Getwd()
	if err != nil {
		return ""
	}
	return path
}

// Path returns the absolute trusted root path.
func (r Root) Path() string {
	return r.path
}

// Close releases the selected directory descriptor shared by this Root and all
// of its copies. Close is idempotent; no copied Root may be used afterward.
func (r Root) Close() error {
	var pendingErr error
	if r.pendingCreation != nil {
		r.pendingCreation.mu.Lock()
		pendingErr = r.pendingCreation.baseIdentity.close()
		r.pendingCreation.mu.Unlock()
	}
	return errors.Join(r.selectedIdentity.close(), pendingErr)
}

// OpenRoot opens the trusted root without following symlinks introduced after
// New selected it. New records the physical target of a pre-existing trusted
// symlink layout, while later path substitutions cannot change the selected
// directory identity. Every physical component and the final root are reopened
// from parent directory handles.
func (r Root) OpenRoot() (*os.Root, error) {
	if r.beforeOpenRootForTest != nil {
		r.beforeOpenRootForTest()
	}
	if !r.selectedIdentity.isPinned() {
		return nil, symlinkError(r.path)
	}
	opened, err := openAbsoluteRootNoFollow(r.openPath)
	if err != nil {
		return nil, err
	}
	identity, err := opened.Stat(".")
	if err != nil || !r.selectedIdentity.matches(identity) {
		_ = opened.Close()
		if err != nil {
			return nil, err
		}
		return nil, symlinkError(r.path)
	}
	return opened, nil
}

// ContainsPath reports whether path resolves within the directory identity
// selected by New. It verifies that the retained root is still reachable at
// its selected physical path before comparing prospective paths, so replacing
// the root after selection fails closed.
func (r Root) ContainsPath(path string) (bool, error) {
	if path == "" {
		return false, fmt.Errorf("%w: path is empty", ErrEscapesRoot)
	}
	if strings.ContainsRune(path, 0) {
		return false, fmt.Errorf("%w: path contains a NUL byte", ErrEscapesRoot)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("resolve path %q: %w", path, err)
	}
	opened, err := r.OpenRoot()
	if err != nil {
		return false, fmt.Errorf("verify selected root %q: %w", r.path, err)
	}
	if err := opened.Close(); err != nil {
		return false, err
	}
	physical, err := resolveProspectivePhysicalPath(filepath.Clean(absolute))
	if err != nil {
		return false, err
	}
	return pathWithinRootIdentity(r.selectedIdentity, physical)
}

// ContainsAnchoredPath reports whether an already-open directory is within this
// root. The lexical path must still resolve to the supplied directory identity;
// replacements between anchoring and comparison fail closed.
func (r Root) ContainsAnchoredPath(path string, anchored *os.Root) (bool, error) {
	if anchored == nil {
		return false, fmt.Errorf("%w: anchored path is nil", ErrEscapesRoot)
	}
	anchoredInfo, err := anchored.Stat(".")
	if err != nil {
		return false, fmt.Errorf("stat anchored path %q: %w", path, err)
	}
	if path == "" {
		return false, fmt.Errorf("%w: path is empty", ErrEscapesRoot)
	}
	if strings.ContainsRune(path, 0) {
		return false, fmt.Errorf("%w: path contains a NUL byte", ErrEscapesRoot)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("resolve path %q: %w", path, err)
	}
	selected, err := r.OpenRoot()
	if err != nil {
		return false, fmt.Errorf("verify selected root %q: %w", r.path, err)
	}
	if err := selected.Close(); err != nil {
		return false, err
	}
	physical, err := resolveProspectivePhysicalPath(filepath.Clean(absolute))
	if err != nil {
		return false, err
	}
	current, err := openAbsoluteRootNoFollow(physical)
	if err != nil {
		return false, fmt.Errorf("open anchored path %q: %w", path, err)
	}
	currentInfo, statErr := current.Stat(".")
	closeErr := current.Close()
	if statErr != nil {
		return false, statErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	if !os.SameFile(anchoredInfo, currentInfo) {
		return false, symlinkError(path)
	}
	return pathWithinRootIdentity(r.selectedIdentity, physical)
}

func pathWithinRootIdentity(identity *rootIdentity, physical string) (bool, error) {
	current := filepath.Clean(physical)
	for {
		info, err := os.Stat(current)
		if err == nil {
			if identity.matches(info) {
				return true, nil
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
		current = parent
	}
}

func resolveProspectivePhysicalPath(absolute string) (string, error) {
	candidate := absolute
	reversedSuffix := make([]string, 0)
	for {
		if _, err := os.Lstat(candidate); err == nil {
			physical, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", fmt.Errorf("resolve existing path %q: %w", candidate, err)
			}
			suffix := make([]string, len(reversedSuffix))
			for index := range reversedSuffix {
				suffix[len(reversedSuffix)-1-index] = reversedSuffix[index]
			}
			return filepath.Join(append([]string{physical}, suffix...)...), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect path %q: %w", candidate, err)
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", fmt.Errorf("no existing ancestor for path %q", absolute)
		}
		reversedSuffix = append(reversedSuffix, filepath.Base(candidate))
		candidate = parent
	}
}

func openAbsoluteRootNoFollow(absolute string) (*os.Root, error) {
	workingDir := workingDirectory()
	if workingDir != "" {
		physicalWorkingDir, err := filepath.EvalSymlinks(workingDir)
		if err == nil {
			workingDir = filepath.Clean(physicalWorkingDir)
		} else {
			workingDir = ""
		}
	}
	return openAbsoluteRootNoFollowFrom(
		absolute,
		workingDir,
		func() (*os.Root, error) { return os.OpenRoot(".") },
		os.OpenRoot,
	)
}

func openAbsoluteRootNoFollowFrom(
	absolute string,
	workingDir string,
	openWorkingDir func() (*os.Root, error),
	openVolumeRoot func(string) (*os.Root, error),
) (*os.Root, error) {
	absolute = filepath.Clean(absolute)
	volume := filepath.VolumeName(absolute)
	anchor := volume + string(filepath.Separator)
	relative := strings.TrimPrefix(absolute, anchor)

	var current *os.Root
	if workingDir != "" {
		workingDir = filepath.Clean(workingDir)
		if workingRelative, err := relativeWithinRoot(workingDir, absolute); err == nil {
			current, err = openWorkingDir()
			if err != nil {
				return nil, err
			}
			openedInfo, openedErr := current.Stat(".")
			selectedInfo, selectedErr := os.Stat(workingDir)
			if openedErr != nil || selectedErr != nil || !os.SameFile(openedInfo, selectedInfo) {
				_ = current.Close()
				if openedErr != nil {
					return nil, openedErr
				}
				if selectedErr != nil {
					return nil, selectedErr
				}
				return nil, symlinkError(workingDir)
			}
			anchor = workingDir
			relative = workingRelative
		}
	}
	if current == nil {
		var err error
		current, err = openVolumeRoot(anchor)
		if err != nil {
			return nil, err
		}
	}
	if relative == "" || relative == "." {
		return current, nil
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		before, err := current.Lstat(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 {
			_ = current.Close()
			return nil, symlinkError(absolute)
		}
		if !before.IsDir() {
			_ = current.Close()
			return nil, fmt.Errorf("%q is not a directory", absolute)
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			_ = current.Close()
			return nil, err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			_ = next.Close()
			_ = current.Close()
			if err != nil {
				return nil, err
			}
			return nil, symlinkError(absolute)
		}
		_ = current.Close()
		current = next
	}
	return current, nil
}

// AllowingInternalSymlinks returns a copy of the root that accepts a symlinked
// directory component below the root when that component resolves back inside
// the root, and still rejects one that escapes.
//
// Use it only where symlinked directories inside the root are an established,
// supported layout. A symlinked final component is still refused.
func (r Root) AllowingInternalSymlinks() Root {
	r.internalSymlinks = true
	return r
}

// containsResolvedComponent reports whether a symlinked component below the root
// resolves back inside the root.
func (r Root) containsResolvedComponent(path string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	root := r.path
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// checkSymlinkComponent decides whether a symlinked component below the root is
// acceptable for this root's policy.
func (r Root) checkSymlinkComponent(path string) error {
	if r.internalSymlinks && r.containsResolvedComponent(path) {
		return nil
	}
	return symlinkError(path)
}

// ValidateRelative reports whether name is safe to join onto a trusted root.
// Both Unix and Windows separator conventions are considered so a repository
// can not smuggle a drive-relative, UNC-style, or backslash-traversing path
// past validation on a different host platform.
func ValidateRelative(name string) error {
	if name == "" {
		return fmt.Errorf("%w: path is empty", ErrEscapesRoot)
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("%w: %q contains a NUL byte", ErrEscapesRoot, name)
	}
	if isAbsoluteLike(name) {
		return fmt.Errorf("%w: %q must be relative to the trusted root", ErrEscapesRoot, name)
	}
	for _, component := range strings.FieldsFunc(name, isPathSeparator) {
		if component == ".." {
			return fmt.Errorf("%w: %q traverses above the trusted root", ErrEscapesRoot, name)
		}
	}
	return nil
}

// ValidateRelativeAllowingTraversal rejects absolute, drive-relative and
// UNC-style paths but permits ".." segments, for callers that resolve a path
// against a base directory below the root and then confirm containment of the
// joined result with Resolve.
func ValidateRelativeAllowingTraversal(name string) error {
	if name == "" {
		return fmt.Errorf("%w: path is empty", ErrEscapesRoot)
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("%w: %q contains a NUL byte", ErrEscapesRoot, name)
	}
	if isAbsoluteLike(name) {
		return fmt.Errorf("%w: %q must be relative to the trusted root", ErrEscapesRoot, name)
	}
	return nil
}

// Resolve validates name and returns its absolute path beneath the root. name
// may be relative to the root or an absolute path that is already inside it.
func (r Root) Resolve(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: path is empty", ErrEscapesRoot)
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: %q contains a NUL byte", ErrEscapesRoot, name)
	}

	if isAbsoluteLike(name) {
		if !filepath.IsAbs(name) {
			return "", fmt.Errorf("%w: %q is not an absolute path below %q", ErrEscapesRoot, name, r.path)
		}
		cleaned := filepath.Clean(name)
		if err := r.checkWithin(cleaned, name); err != nil {
			return "", err
		}
		return cleaned, nil
	}

	if err := ValidateRelative(name); err != nil {
		return "", err
	}
	joined := filepath.Join(r.path, name)
	if err := r.checkWithin(joined, name); err != nil {
		return "", err
	}
	return joined, nil
}

// ResolveContainedFinalSymlink resolves a final symlink only when its physical
// target remains beneath this root. The returned name is relative to the root
// and contains no symlink components, so callers can perform the actual I/O
// through rooted no-follow operations without reopening the link.
func (r Root) ResolveContainedFinalSymlink(name string) (string, error) {
	absolute, err := r.Resolve(name)
	if err != nil {
		return "", err
	}
	if err := r.checkParentComponents(absolute); err != nil {
		return "", err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("%w: %q is not a symlink", ErrSymlink, absolute)
	}

	physicalRoot, err := filepath.EvalSymlinks(r.path)
	if err != nil {
		return "", err
	}
	physicalTarget, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	return relativeWithinRoot(physicalRoot, physicalTarget)
}

// CheckContained verifies that name stays beneath the root and that neither its
// parent components nor its final component is a symlink below the root.
func (r Root) CheckContained(name string) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	if err := r.checkParentComponents(resolved); err != nil {
		return err
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return symlinkError(resolved)
	}
	return nil
}

// CheckParents verifies that name stays beneath the root and that every
// component below the root leading to it is acceptable under the root's symlink
// policy. The final component is not inspected.
func (r Root) CheckParents(name string) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	return r.checkParentComponents(resolved)
}

// OpenFile opens an existing regular file beneath the root without following
// symlinks in the final component or in any component below the root.
func (r Root) OpenFile(name string) (*os.File, error) {
	resolved, err := r.Resolve(name)
	if err != nil {
		return nil, err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err := r.checkParentComponents(resolved); err != nil {
		return nil, err
	}
	info, err := parent.Lstat(base)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, symlinkError(resolved)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", resolved)
	}
	if r.afterValidationForTest != nil {
		r.afterValidationForTest()
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(parent, base)
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("%q is not a regular file", resolved)
	}
	return file, nil
}

// OpenDir opens an existing directory beneath the root without following
// symlinks in the final component or in any component below the root.
func (r Root) OpenDir(name string) (*os.File, error) {
	resolved, err := r.Resolve(name)
	if err != nil {
		return nil, err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err := r.checkParentComponents(resolved); err != nil {
		return nil, err
	}
	info, err := parent.Lstat(base)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, symlinkError(resolved)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", resolved)
	}
	file, err := secureopen.OpenExistingNoFollowInRoot(parent, base)
	if err != nil {
		return nil, err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if !openedInfo.IsDir() {
		_ = file.Close()
		return nil, fmt.Errorf("%q is not a directory", resolved)
	}
	return file, nil
}

// ReadFile reads a regular file beneath the root without following symlinks.
func (r Root) ReadFile(name string) ([]byte, error) {
	file, err := r.OpenFile(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

// ReadFileLimited reads at most limit bytes from a regular file beneath the
// root. It rejects, rather than truncates, files that exceed the limit.
func (r Root) ReadFileLimited(name string, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, fmt.Errorf("read limit must not be negative")
	}
	file, err := r.OpenFile(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if limit == math.MaxInt64 {
		return io.ReadAll(file)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%q exceeds the %d-byte size limit", name, limit)
	}
	return data, nil
}

// ReadFileOptional reads a regular file beneath the root and reports whether it
// exists. A missing file is not an error; a symlinked path still is.
func (r Root) ReadFileOptional(name string) ([]byte, bool, error) {
	data, err := r.ReadFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// MkdirAll creates name and any missing parents beneath the root, rejecting any
// existing component that is a symlink or not a directory.
func (r Root) MkdirAll(name string, perm os.FileMode) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	if err := r.ensureRootDir(perm); err != nil {
		return err
	}
	rooted, relative, err := r.openRooted(resolved, true)
	if err != nil {
		return err
	}
	defer rooted.Close()
	if err := r.validateDirectoryComponents(resolved); err != nil {
		return err
	}
	if err := rooted.MkdirAll(relative, perm); err != nil {
		return err
	}
	return r.validateDirectoryComponents(resolved)
}

// WriteFile atomically creates or replaces a file beneath the root.
func (r Root) WriteFile(name string, data []byte, perm os.FileMode) error {
	_, err := r.WriteFrom(name, bytes.NewReader(data), perm)
	return err
}

// WriteFrom atomically creates or replaces a file beneath the root with the
// contents of reader and returns the number of bytes written.
func (r Root) WriteFrom(name string, reader io.Reader, perm os.FileMode) (int64, error) {
	return r.writeFrom(name, reader, perm, true)
}

func (r Root) writeFrom(name string, reader io.Reader, perm os.FileMode, exactModeForNew bool) (int64, error) {
	return r.writeFromPreservingMetadata(name, reader, perm, exactModeForNew, nil, nil)
}

func (r Root) writeFromPreservingMetadata(
	name string,
	reader io.Reader,
	perm os.FileMode,
	exactModeForNew bool,
	metadataSource *os.File,
	metadataInfo os.FileInfo,
) (int64, error) {
	resolved, err := r.prepareWrite(name)
	if err != nil {
		return 0, err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return 0, err
	}
	defer parent.Close()

	hadExisting, err := checkReplaceableFileInRoot(parent, base, resolved)
	if err != nil {
		return 0, err
	}
	if r.afterValidationForTest != nil {
		r.afterValidationForTest()
	}

	temporary, temporaryName, err := secureopen.CreateTempNoFollowInRoot(parent, ".", temporaryFilePattern, perm)
	if err != nil {
		return 0, err
	}
	success := false
	defer func() {
		_ = temporary.Close()
		if !success {
			_ = parent.Remove(temporaryName)
		}
	}()

	// Preserve supported filesystem metadata from the already-open original.
	// Otherwise keep the exact mode of an ordinary replacement. For a new file,
	// retain the process umask unless the caller explicitly requested an exact
	// mode.
	if metadataSource != nil {
		if err := copyReplacementMetadata(temporary, metadataSource, metadataInfo); err != nil {
			return 0, err
		}
	} else if hadExisting || exactModeForNew {
		if err := temporary.Chmod(perm); err != nil {
			return 0, err
		}
	}
	written, err := io.Copy(temporary, reader)
	if err != nil {
		return 0, err
	}
	if err := temporary.Sync(); err != nil {
		return 0, err
	}
	if err := temporary.Close(); err != nil {
		return 0, err
	}

	if err := replaceFileInRoot(parent, temporaryName, base, hadExisting); err != nil {
		return 0, err
	}
	success = true
	return written, nil
}

// WriteFilePreservingMode atomically creates or replaces a regular file beneath
// the root. Existing files retain supported ownership, permission, ACL, and
// extended-attribute metadata without mutating aliases outside the rooted path.
// Where the platform exposes link counts, multiply linked files are refused
// rather than silently changing hard-link semantics. New files use perm subject
// to the process umask.
func (r Root) WriteFilePreservingMode(name string, data []byte, perm os.FileMode) error {
	resolved, err := r.prepareWrite(name)
	if err != nil {
		return err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return err
	}
	defer parent.Close()

	hadExisting, err := checkReplaceableFileInRoot(parent, base, resolved)
	if err != nil {
		return err
	}
	if !hadExisting {
		_, err := r.writeFrom(name, bytes.NewReader(data), perm, false)
		return err
	}

	file, err := secureopen.OpenExistingWritableNoFollowInRoot(parent, base)
	if err != nil {
		return fmt.Errorf("open existing file %q for replacement: %w", resolved, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if multiple, err := hasMultipleHardLinks(file, openedInfo); err != nil {
		return fmt.Errorf("inspect existing file %q: %w", resolved, err)
	} else if multiple {
		return fmt.Errorf("refusing to rewrite multiply linked file %q", resolved)
	}
	_, err = r.writeFromPreservingMetadata(name, bytes.NewReader(data), perm, false, file, openedInfo)
	return err
}

// CheckWriteFilePreservingMode performs the non-mutating checks required before
// WriteFilePreservingMode replaces an existing file. Missing destinations are
// accepted; callers can use this to preflight a multi-file plan before its first
// write.
func (r Root) CheckWriteFilePreservingMode(name string) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	if resolved == r.path {
		return fmt.Errorf("%w: %q is the trusted root itself", ErrEscapesRoot, name)
	}
	if err := r.checkParentComponents(resolved); err != nil {
		return err
	}

	info, err := os.Lstat(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return symlinkError(resolved)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", resolved)
	}

	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return err
	}
	defer parent.Close()
	file, err := secureopen.OpenExistingWritableNoFollowInRoot(parent, base)
	if err != nil {
		return fmt.Errorf("open existing file %q for replacement: %w", resolved, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}
	if multiple, err := hasMultipleHardLinks(file, openedInfo); err != nil {
		return fmt.Errorf("inspect existing file %q: %w", resolved, err)
	} else if multiple {
		return fmt.Errorf("refusing to rewrite multiply linked file %q", resolved)
	}
	return nil
}

// CheckCreateNewFile performs the non-mutating checks required before
// CreateNewFile publishes a destination. Missing parents are accepted because
// the eventual rooted write creates them; existing files and symlinks are not.
func (r Root) CheckCreateNewFile(name string) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	if resolved == r.path {
		return fmt.Errorf("%w: %q is the trusted root itself", ErrEscapesRoot, name)
	}
	if err := r.checkParentComponents(resolved); err != nil {
		return err
	}
	info, err := os.Lstat(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return symlinkError(resolved)
	}
	return fmt.Errorf("%q already exists: %w", resolved, os.ErrExist)
}

// CheckFileParent validates a future file path and all existing parent
// components without requiring the final destination name to be absent.
func (r Root) CheckFileParent(name string) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	if resolved == r.path {
		return fmt.Errorf("%w: %q is the trusted root itself", ErrEscapesRoot, name)
	}
	return r.checkParentComponents(resolved)
}

// CheckDirectoryWritable verifies that a temporary regular file can be created
// and removed within an existing directory beneath the root.
func (r Root) CheckDirectoryWritable(name string, perm os.FileMode) error {
	resolved, err := r.Resolve(name)
	if err != nil {
		return err
	}
	if err := r.validateExistingDir(resolved); err != nil {
		return err
	}
	rooted, relative, err := r.openRooted(resolved, false)
	if err != nil {
		return err
	}
	directory, err := rooted.OpenRoot(relative)
	_ = rooted.Close()
	if err != nil {
		return err
	}
	defer directory.Close()

	probe, probeName, err := secureopen.CreateTempNoFollowInRoot(directory, ".", ".asc-write-probe-*", perm)
	if err != nil {
		return err
	}
	defer func() {
		_ = probe.Close()
		_ = directory.Remove(probeName)
	}()
	if err := probe.Chmod(perm); err != nil {
		return err
	}
	if err := probe.Close(); err != nil {
		return err
	}
	if err := directory.Remove(probeName); err != nil {
		return err
	}
	return nil
}

// CreateNewFile writes data to a new file beneath the root and fails when the
// destination already exists. It prefers atomic no-replace publication, then
// falls back to rooted, no-follow O_EXCL creation when the filesystem does not
// support atomic no-replace rename.
func (r Root) CreateNewFile(name string, data []byte, perm os.FileMode) error {
	_, err := r.CreateNewFrom(name, bytes.NewReader(data), perm)
	if !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		return err
	}
	return r.createNewFileExclusive(name, data, perm)
}

// CreateNewFileAtomic atomically publishes complete data as a new file beneath
// the root. It returns ErrRenameNoReplaceUnsupported instead of falling back
// when the filesystem cannot provide atomic no-replace rename semantics.
func (r Root) CreateNewFileAtomic(name string, data []byte, perm os.FileMode) error {
	r.requireNativeNoReplace = true
	_, err := r.CreateNewFrom(name, bytes.NewReader(data), perm)
	return err
}

func (r Root) createNewFileExclusive(name string, data []byte, perm os.FileMode) error {
	resolved, err := r.prepareWrite(name)
	if err != nil {
		return err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return err
	}
	defer parent.Close()

	if r.afterValidationForTest != nil {
		r.afterValidationForTest()
	}
	file, err := secureopen.OpenNewFileNoFollowInRoot(parent, base, perm)
	if err != nil {
		return err
	}
	createdInfo, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return statErr
	}
	complete := false
	defer func() {
		_ = file.Close()
		if complete {
			return
		}
		currentInfo, currentErr := parent.Lstat(base)
		if currentErr == nil && os.SameFile(createdInfo, currentInfo) {
			_ = parent.Remove(base)
		}
	}()
	if err := file.Chmod(perm); err != nil {
		return err
	}
	written, err := file.Write(data)
	if err != nil {
		return err
	}
	if written != len(data) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}

// CreateNewFrom atomically publishes reader's complete contents as a new file
// beneath the root. It stages an unpredictable no-follow file in the same
// directory, syncs and closes it, then uses an atomic no-replace rename. A read,
// write, sync, close, or publish failure leaves an existing destination intact.
func (r Root) CreateNewFrom(name string, reader io.Reader, perm os.FileMode) (int64, error) {
	resolved, err := r.prepareWrite(name)
	if err != nil {
		return 0, err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return 0, err
	}
	defer parent.Close()

	info, err := parent.Lstat(base)
	switch {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return 0, symlinkError(resolved)
		}
		return 0, fmt.Errorf("%q already exists: %w", resolved, os.ErrExist)
	case !errors.Is(err, os.ErrNotExist):
		return 0, err
	}
	if r.afterValidationForTest != nil {
		r.afterValidationForTest()
	}

	file, temporaryName, err := secureopen.CreateTempNoFollowInRoot(parent, ".", temporaryFilePattern, perm)
	if err != nil {
		return 0, err
	}
	published := false
	defer func() {
		_ = file.Close()
		if !published {
			_ = parent.Remove(temporaryName)
		}
	}()
	if err := file.Chmod(perm); err != nil {
		return 0, err
	}
	written, err := io.Copy(file, reader)
	if err != nil {
		return written, err
	}
	if err := file.Sync(); err != nil {
		return written, err
	}
	if err := file.Close(); err != nil {
		return written, err
	}
	renameNoReplace := secureopen.RenameNoReplaceInRoot
	if r.renameNoReplaceForTest != nil {
		renameNoReplace = r.renameNoReplaceForTest
	}
	if err := renameNoReplace(parent, temporaryName, base); err != nil {
		if !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
			return written, err
		}
		if r.requireNativeNoReplace {
			return written, err
		}
		// A hard link atomically publishes the complete staged inode without
		// replacing an existing destination.
		if linkErr := parent.Link(temporaryName, base); linkErr != nil {
			return written, linkErr
		}
		if removeErr := parent.Remove(temporaryName); removeErr != nil {
			return written, fmt.Errorf("publish succeeded but remove staged file: %w", removeErr)
		}
	}
	published = true
	directory, err := parent.Open(".")
	if err != nil {
		return written, fmt.Errorf("open parent directory for durability sync: %w", err)
	}
	if err := directory.Sync(); err != nil && !unsupportedDirectorySyncError(err) {
		_ = directory.Close()
		return written, fmt.Errorf("sync parent directory after publish: %w", err)
	}
	if err := directory.Close(); err != nil {
		return written, fmt.Errorf("close parent directory after durability sync: %w", err)
	}
	return written, nil
}

// AppendFile appends data to a file beneath the root, creating it when missing,
// without following a final or parent symlink.
func (r Root) AppendFile(name string, data []byte, perm os.FileMode) error {
	resolved, err := r.prepareWrite(name)
	if err != nil {
		return err
	}
	parent, base, err := r.openParentRooted(resolved)
	if err != nil {
		return err
	}
	defer parent.Close()

	hadExisting := false
	if info, err := parent.Lstat(base); err == nil {
		hadExisting = true
		if info.Mode()&os.ModeSymlink != 0 {
			return symlinkError(resolved)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%q is not a regular file", resolved)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if r.afterValidationForTest != nil {
		r.afterValidationForTest()
	}

	var file *os.File
	if hadExisting {
		file, err = secureopen.OpenExistingAppendNoFollowInRoot(parent, base)
	} else {
		file, err = secureopen.OpenNewFileNoFollowInRoot(parent, base, perm)
	}
	if err != nil {
		return err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return err
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return fmt.Errorf("%q is not a regular file", resolved)
	}
	if hadExisting {
		if multiple, err := hasMultipleHardLinks(file, openedInfo); err != nil {
			_ = file.Close()
			return fmt.Errorf("inspect existing file %q: %w", resolved, err)
		} else if multiple {
			_ = file.Close()
			return fmt.Errorf("refusing to append to multiply linked file %q", resolved)
		}
		// Security logs and similar callers use perm to tighten an existing
		// file. New files retain the process umask from their exclusive create.
		if err := file.Chmod(perm); err != nil {
			_ = file.Close()
			return err
		}
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func (r Root) prepareWrite(name string) (string, error) {
	resolved, err := r.Resolve(name)
	if err != nil {
		return "", err
	}
	if resolved == r.path {
		return "", fmt.Errorf("%w: %q is the trusted root itself", ErrEscapesRoot, name)
	}
	parent, err := r.relativeToRoot(filepath.Dir(resolved))
	if err != nil {
		return "", err
	}
	if err := r.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	return resolved, nil
}

func (r Root) ensureRootDir(perm os.FileMode) error {
	info, err := os.Stat(r.path)
	switch {
	case err == nil:
		if !info.IsDir() {
			return fmt.Errorf("trusted root %q is not a directory", r.path)
		}
		if r.selectedIdentity.isPinned() {
			return nil
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	if r.selectedIdentity.isPinned() {
		return symlinkError(r.path)
	}
	if r.pendingCreation == nil {
		return symlinkError(r.path)
	}
	r.pendingCreation.mu.Lock()
	defer r.pendingCreation.mu.Unlock()
	if r.selectedIdentity.isPinned() {
		return nil
	}
	if r.beforeCreateRootForTest != nil {
		r.beforeCreateRootForTest()
	}
	baseAtPath, err := os.Stat(r.pendingCreation.lexicalBase)
	if err != nil {
		return err
	}
	if !r.pendingCreation.baseIdentity.matches(baseAtPath) {
		return symlinkError(r.pendingCreation.lexicalBase)
	}
	base, err := openAbsoluteRootNoFollow(r.pendingCreation.physicalBase)
	if err != nil {
		return err
	}
	baseInfo, err := base.Stat(".")
	if err != nil || !r.pendingCreation.baseIdentity.matches(baseInfo) {
		_ = base.Close()
		if err != nil {
			return err
		}
		return symlinkError(r.pendingCreation.physicalBase)
	}
	created, err := createMissingRoot(base, r.pendingCreation.suffix, perm, r.pendingCreation.physicalBase)
	if err != nil {
		return err
	}
	selectedAtPath, err := os.Stat(r.path)
	if err != nil {
		created.rollback()
		return err
	}
	openedInfo, err := created.final.Stat(".")
	if err != nil || !os.SameFile(openedInfo, selectedAtPath) {
		created.rollback()
		if err != nil {
			return err
		}
		return symlinkError(r.path)
	}
	if err := r.pendingCreation.baseIdentity.close(); err != nil {
		created.rollback()
		return err
	}
	opened := created.release()
	if !r.selectedIdentity.pin(opened) {
		return symlinkError(r.path)
	}
	return nil
}

type missingRootCreation struct {
	roots      []*os.Root
	suffix     []string
	created    []bool
	final      *os.Root
	terminated bool
}

func (creation *missingRootCreation) rollback() {
	if creation == nil || creation.terminated {
		return
	}
	creation.terminated = true
	for index := len(creation.suffix) - 1; index >= 0; index-- {
		if creation.created[index] {
			_ = creation.roots[index].Remove(creation.suffix[index])
		}
	}
	for _, root := range creation.roots {
		_ = root.Close()
	}
}

func (creation *missingRootCreation) release() *os.Root {
	if creation == nil || creation.terminated {
		return nil
	}
	creation.terminated = true
	for index := 0; index < len(creation.roots)-1; index++ {
		_ = creation.roots[index].Close()
	}
	return creation.final
}

func createMissingRoot(base *os.Root, suffix []string, perm os.FileMode, basePath string) (_ *missingRootCreation, resultErr error) {
	creation := &missingRootCreation{
		roots:   []*os.Root{base},
		suffix:  append([]string(nil), suffix...),
		created: make([]bool, len(suffix)),
	}
	defer func() {
		if resultErr != nil {
			creation.rollback()
		}
	}()
	current := base
	for index, component := range suffix {
		componentPath := filepath.Join(append([]string{basePath}, suffix[:index+1]...)...)
		if err := validateMissingRootComponent(component); err != nil {
			return nil, err
		}
		if _, err := current.Lstat(component); err == nil {
			return nil, symlinkError(componentPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := current.Mkdir(component, perm); err != nil {
			return nil, err
		}
		creation.created[index] = true
		before, err := current.Lstat(component)
		if err != nil {
			return nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			return nil, symlinkError(componentPath)
		}
		next, err := current.OpenRoot(component)
		if err != nil {
			return nil, err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			_ = next.Close()
			if err != nil {
				return nil, err
			}
			return nil, symlinkError(componentPath)
		}
		creation.roots = append(creation.roots, next)
		current = next
	}
	creation.final = current
	return creation, nil
}

func (r Root) openRooted(absolute string, resolveFinal bool) (*os.Root, string, error) {
	rooted, err := r.OpenRoot()
	if err != nil {
		return nil, "", err
	}
	relative, err := r.rootedRelative(absolute, resolveFinal)
	if err != nil {
		_ = rooted.Close()
		return nil, "", err
	}
	return rooted, relative, nil
}

func (r Root) openParentRooted(absolute string) (*os.Root, string, error) {
	rooted, relative, err := r.openRooted(absolute, false)
	if err != nil {
		return nil, "", err
	}
	parent, err := rooted.OpenRoot(filepath.Dir(relative))
	_ = rooted.Close()
	if err != nil {
		return nil, "", err
	}
	return parent, filepath.Base(relative), nil
}

// rootedRelative converts an already-contained absolute path into a name for
// os.Root. Existing internal directory symlinks are resolved to their physical
// path so AllowingInternalSymlinks remains compatible with absolute in-root
// links, while the final file component remains unresolved for no-follow open.
func (r Root) rootedRelative(absolute string, resolveFinal bool) (string, error) {
	components, err := r.componentsBelowRoot(absolute)
	if err != nil {
		return "", err
	}
	physicalRoot, err := filepath.EvalSymlinks(r.path)
	if err != nil {
		return "", err
	}
	current := physicalRoot
	resolveCount := len(components)
	if !resolveFinal && resolveCount > 0 {
		resolveCount--
	}

	for index := 0; index < resolveCount; index++ {
		candidate := filepath.Join(current, components[index])
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			for _, remaining := range components[index:] {
				current = filepath.Join(current, remaining)
			}
			return relativeWithinRoot(physicalRoot, current)
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if !r.internalSymlinks {
				return "", symlinkError(candidate)
			}
			resolved, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				return "", err
			}
			if _, err := relativeWithinRoot(physicalRoot, resolved); err != nil {
				return "", symlinkError(candidate)
			}
			resolvedInfo, err := os.Stat(candidate)
			if err != nil {
				return "", err
			}
			if !resolvedInfo.IsDir() {
				return "", fmt.Errorf("%q is not a directory", candidate)
			}
			current = resolved
			continue
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%q is not a directory", candidate)
		}
		current = candidate
	}
	for _, remaining := range components[resolveCount:] {
		current = filepath.Join(current, remaining)
	}
	return relativeWithinRoot(physicalRoot, current)
}

func relativeWithinRoot(root string, path string) (string, error) {
	if !strings.EqualFold(filepath.VolumeName(path), filepath.VolumeName(root)) {
		return "", fmt.Errorf("%w: %q changes volume from %q", ErrEscapesRoot, path, root)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("%w: %q is not below %q", ErrEscapesRoot, path, root)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q is not below %q", ErrEscapesRoot, path, root)
	}
	return relative, nil
}

func (r Root) componentsBelowRoot(absolute string) ([]string, error) {
	relative, err := r.relativeToRoot(absolute)
	if err != nil {
		return nil, err
	}
	if relative == "." {
		return nil, nil
	}
	parts := strings.Split(relative, string(filepath.Separator))
	components := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		components = append(components, part)
	}
	return components, nil
}

func (r Root) relativeToRoot(absolute string) (string, error) {
	relative, err := filepath.Rel(r.path, absolute)
	if err != nil {
		return "", fmt.Errorf("%w: %q is not below %q", ErrEscapesRoot, absolute, r.path)
	}
	return relative, nil
}

func (r Root) checkWithin(absolute string, original string) error {
	if !strings.EqualFold(filepath.VolumeName(absolute), filepath.VolumeName(r.path)) {
		return fmt.Errorf("%w: %q changes volume from %q", ErrEscapesRoot, original, r.path)
	}
	relative, err := filepath.Rel(r.path, absolute)
	if err != nil {
		return fmt.Errorf("%w: %q is not below %q", ErrEscapesRoot, original, r.path)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: %q is not below %q", ErrEscapesRoot, original, r.path)
	}
	return nil
}

func (r Root) checkParentComponents(absolute string) error {
	components, err := r.componentsBelowRoot(absolute)
	if err != nil {
		return err
	}
	if len(components) == 0 {
		return nil
	}
	current := r.path
	for _, component := range components[:len(components)-1] {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			if err := r.checkSymlinkComponent(current); err != nil {
				return err
			}
			if resolved, err := os.Stat(current); err != nil {
				return err
			} else if !resolved.IsDir() {
				return fmt.Errorf("%q is not a directory", current)
			}
			continue
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", current)
		}
	}
	return nil
}

func (r Root) validateDirectoryComponents(absolute string) error {
	components, err := r.componentsBelowRoot(absolute)
	if err != nil {
		return err
	}
	current := r.path
	for _, component := range components {
		current = filepath.Join(current, component)
		if err := r.validateExistingDir(current); errors.Is(err, os.ErrNotExist) {
			return nil
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (r Root) validateExistingDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if err := r.checkSymlinkComponent(path); err != nil {
			return err
		}
		resolved, err := os.Stat(path)
		if err != nil {
			return err
		}
		if !resolved.IsDir() {
			return fmt.Errorf("%q is not a directory", path)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", path)
	}
	return nil
}

func checkReplaceableFileInRoot(rooted *os.Root, name string, displayPath string) (bool, error) {
	info, err := rooted.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, symlinkError(displayPath)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%q is not a regular file", displayPath)
	}
	return true, nil
}

// replaceFileInRoot moves the staged temporary file onto path. Unix renames replace
// the destination atomically; Windows renames fail when the destination exists,
// so the original is moved aside first and restored if the final move fails.
func replaceFileInRoot(parent *os.Root, temporaryName string, name string, hadExisting bool) error {
	if err := parent.Rename(temporaryName, name); err == nil {
		return nil
	} else if !hadExisting || runtime.GOOS != "windows" {
		return err
	}

	backup, backupName, err := secureopen.CreateTempNoFollowInRoot(parent, ".", backupFilePattern, 0o600)
	if err != nil {
		return err
	}
	if err := backup.Close(); err != nil {
		return errors.Join(err, parent.Remove(backupName))
	}
	if err := parent.Remove(backupName); err != nil {
		return err
	}
	if err := parent.Rename(name, backupName); err != nil {
		return err
	}
	if err := parent.Rename(temporaryName, name); err != nil {
		restoreErr := parent.Rename(backupName, name)
		if restoreErr != nil {
			return errors.Join(
				err,
				fmt.Errorf("restore original from backup %q: %w", backupName, restoreErr),
			)
		}
		return err
	}
	if err := parent.Remove(backupName); err != nil {
		return fmt.Errorf("replacement succeeded but remove backup %q: %w", backupName, err)
	}
	return nil
}

func symlinkError(path string) error {
	return fmt.Errorf("%w: %q", ErrSymlink, path)
}

func isAbsoluteLike(path string) bool {
	if path == "" {
		return false
	}
	if isPathSeparator(rune(path[0])) {
		return true
	}
	if filepath.IsAbs(path) {
		return true
	}
	return len(path) >= 2 && path[1] == ':' && isASCIILetter(path[0])
}

func isPathSeparator(r rune) bool {
	return r == '/' || r == '\\'
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
