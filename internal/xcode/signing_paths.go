package xcode

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func canonicalSigningPath(path, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s path is empty", label)
	}
	if strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("%s path contains NUL", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s path: %w", label, err)
	}
	return filepath.Clean(absolute), nil
}

// signingXCConfigPhysicalKey groups authorized xcconfig paths by the
// filesystem identity captured during collection. It is used only for
// consumer/conflict decisions: a hard link is one physical source even when
// the project names it through two paths.
func signingXCConfigPhysicalKey(path string, fileIdentities map[string]string) string {
	if identity, ok := signingFileIdentity(path, fileIdentities); ok && identity != "" {
		return "identity:" + identity
	}
	return "path:" + signingLexicalPathKey(path)
}

// signingXCConfigOperationKey is the path-preserving key used for plan files,
// prepared writes, and receipts. A physical identity alone is insufficient:
// hard-linked paths are distinct directory entries, and an atomic rename of
// one must not silently satisfy or discard the other. Parent-symlink aliases
// such as Configs/Shared.xcconfig and AliasDir/Shared.xcconfig resolve to the
// same directory entry; those must coalesce or the first rename changes the
// inode and the second write fails with "source changed before commit".
func signingXCConfigOperationKey(path string, fileIdentities map[string]string) string {
	entryKey := signingXCConfigDirectoryEntryKey(path)
	if identity, ok := signingFileIdentity(path, fileIdentities); ok && identity != "" {
		return "identity:" + identity + "\x00entry:" + entryKey
	}
	return "path:" + entryKey
}

// signingXCConfigDirectoryEntryKey identifies the directory entry that a
// requested xcconfig path names. Intermediate directory symlinks are resolved
// so in-project aliases of one parent share a key, while the final component
// stays lexical so two hard links in the same directory remain distinct.
func signingXCConfigDirectoryEntryKey(path string) string {
	normalized := normalizeSigningLexicalPath(path)
	parent := filepath.Dir(normalized)
	base := filepath.Base(normalized)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		parent = resolved
	}
	return signingXCConfigPathIntentKey(filepath.Join(parent, base))
}

func signingXCConfigPathIntentKey(path string) string {
	normalized := normalizeSigningLexicalPath(path)
	caseInsensitive, known := signingCaseInsensitiveVolumeFn(filepath.Dir(normalized))
	if known && caseInsensitive {
		return strings.ToLower(normalized)
	}
	return normalized
}

// signingFileIdentity first uses the exact normalized spelling emitted by the
// collector. The lexical-key fallback keeps compatibility with older callers
// and hand-built test maps, but an exact entry always wins so two proven
// case-distinct files on a Windows case-sensitive directory cannot collide.
func signingFileIdentity(path string, fileIdentities map[string]string) (string, bool) {
	if fileIdentities == nil {
		return "", false
	}
	if identity, ok := fileIdentities[normalizeSigningLexicalPath(path)]; ok {
		return identity, identity != ""
	}
	legacyKey := signingLexicalPathKey(path)
	if identity, ok := fileIdentities[legacyKey]; ok {
		return identity, identity != ""
	}
	return "", false
}

func signingPathContained(project *structuredVersionProject, path string) bool {
	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return false
	}
	defer root.Close()
	return root.AllowingInternalSymlinks().CheckContained(path) == nil
}

// normalizeSigningLexicalPath returns the absolute, cleaned spelling used for
// lexical path decisions. It intentionally does not resolve symlinks: lexical
// protection must also cover a path that does not exist yet.
func normalizeSigningLexicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absolute)
}

// signingLexicalPathKey applies the host filesystem's equality semantics to a
// normalized path. Windows paths are case-insensitive even when the protected
// or artifact path is still missing, so SameFile cannot be relied on there.
func signingLexicalPathKey(path string) string {
	path = normalizeSigningLexicalPath(path)
	if runtimeGOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func signingLexicalPathEqual(left, right string) bool {
	return signingLexicalPathKey(left) == signingLexicalPathKey(right)
}

// signingArtifactLexicalPathEqual applies filesystem case semantics only at
// the artifact-alias boundary. General signing path keys retain their
// platform-independent spelling rules; this helper additionally protects
// missing paths on case-insensitive Darwin volumes where SameFile cannot
// inspect an inode yet. Unknown volume metadata is treated conservatively as
// case-insensitive so an uncertain alias cannot reach an artifact write.
func signingArtifactLexicalPathEqual(left, right string) bool {
	left = normalizeSigningLexicalPath(left)
	right = normalizeSigningLexicalPath(right)
	if signingLexicalPathEqual(left, right) {
		return true
	}
	if !strings.EqualFold(left, right) {
		return false
	}
	caseInsensitive, known := signingCaseInsensitiveVolumeFn(left)
	return !known || caseInsensitive
}

func signingPathLexicallyContained(project *structuredVersionProject, path string) bool {
	root := normalizeSigningLexicalPath(project.rootDir)
	absolute := normalizeSigningLexicalPath(path)
	if signingNormalizedPathContained(root, absolute) {
		return true
	}
	if runtimeGOOS != "windows" && runtimeGOOS != "darwin" && runtimeGOOS != "linux" {
		return false
	}
	// A path outside the exact normalized root and outside its case-folded
	// component prefix is definitely external. Reject it without consulting
	// per-volume case metadata; that metadata is reserved for the narrow case
	// variant that could genuinely name the selected root on an insensitive
	// filesystem.
	if !signingPathCaseFoldedPrefixContained(root, absolute) {
		return false
	}
	// A case-folded containment result is safe only when both the project
	// root and the candidate's containing directory are proven
	// case-insensitive. Windows supports case-sensitive directories on an
	// otherwise case-insensitive volume, so a global lowercase comparison can
	// incorrectly authorize C:\\project when the root is C:\\Project.
	rootInsensitive, rootKnown := signingCaseInsensitiveVolumeFn(root)
	pathInsensitive, pathKnown := signingCaseInsensitiveVolumeFn(filepath.Dir(absolute))
	if !rootKnown || !pathKnown || !rootInsensitive || !pathInsensitive {
		return false
	}
	return signingNormalizedPathContained(strings.ToLower(root), strings.ToLower(absolute))
}

func signingPathCaseFoldedPrefixContained(root, absolute string) bool {
	root = normalizeSigningLexicalPath(root)
	absolute = normalizeSigningLexicalPath(absolute)
	if strings.EqualFold(root, absolute) {
		return true
	}
	if len(absolute) <= len(root) || !strings.EqualFold(root, absolute[:len(root)]) {
		return false
	}
	return absolute[len(root)] == filepath.Separator
}

// signingPathDefinitelyExternal answers the lexical portion of authorization
// without consulting filesystem metadata. A case-folded root prefix remains
// unresolved until signingPathLexicallyContained can inspect the relevant
// volume semantics; every other non-contained path is safely external.
func signingHasIncompleteInternalXCConfig(project *structuredVersionProject, paths []string, allowExternal bool) bool {
	if len(paths) == 0 {
		return false
	}
	if allowExternal {
		// An opted-in external graph that still failed collection cannot be
		// inventoried. A blocked plan is unsafe to serialize over an unread
		// entitlement assignment.
		return true
	}
	for _, path := range paths {
		if !signingPathDefinitelyExternal(project, path) {
			return true
		}
	}
	return false
}

func signingPathDefinitelyExternal(project *structuredVersionProject, path string) bool {
	root := normalizeSigningLexicalPath(project.rootDir)
	absolute := normalizeSigningLexicalPath(path)
	if signingNormalizedPathContained(root, absolute) {
		return false
	}
	if runtimeGOOS != "windows" && runtimeGOOS != "darwin" && runtimeGOOS != "linux" {
		return true
	}
	return !signingPathCaseFoldedPrefixContained(root, absolute)
}

func signingNormalizedPathContained(root, absolute string) bool {
	root = filepath.Clean(root)
	absolute = filepath.Clean(absolute)
	if runtimeGOOS == "windows" {
		// filepath.Rel uses case-folded component comparisons on Windows. That
		// is unsafe for a case-sensitive directory, so containment must first
		// use the exact normalized spelling and separator boundaries.
		return signingPathComponentsContained(root, absolute, string(filepath.Separator))
	}
	relative, err := signingPathRelFn(root, absolute)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// signingPathComponentsContained compares already-normalized absolute paths
// without filepath.Rel's platform-specific case folding. The separator is
// explicit so Windows component semantics can be tested on other hosts.
func signingPathComponentsContained(root, absolute, separator string) bool {
	if root == absolute {
		return true
	}
	if root == separator || strings.HasSuffix(root, separator) {
		return strings.HasPrefix(absolute, root)
	}
	return strings.HasPrefix(absolute, root+separator)
}

func signingXCConfigCollectionBlocker(project *structuredVersionProject, path string, allowExternal bool) string {
	if signingPathDefinitelyExternal(project, path) {
		if !allowExternal {
			return fmt.Sprintf("xcconfig %s is external and could not be read without --allow-external-xcconfig", path)
		}
		return fmt.Sprintf("xcconfig %s is external and could not be safely collected", path)

	}
	if !signingPathLexicallyContained(project, path) {
		if !allowExternal {
			return fmt.Sprintf("xcconfig %s is external and could not be read without --allow-external-xcconfig", path)
		}
		return fmt.Sprintf("xcconfig %s is external and could not be safely collected", path)
	}
	return fmt.Sprintf("xcconfig %s could not be safely collected; signing scope is uncertain", path)
}

func appendUniqueSigningPaths(paths []string, additions ...string) []string {
	for _, path := range additions {
		path = normalizeSigningLexicalPath(path)
		if path == "" {
			continue
		}
		found := false
		for _, existing := range paths {
			// This list is also populated from collector onPath callbacks before
			// authorization and filesystem identity checks. Keep deduplication
			// purely lexical so an untrusted external path cannot trigger a case
			// volume probe merely because it resembles an earlier spelling.
			if normalizeSigningLexicalPath(existing) == path {
				found = true
				break
			}
		}
		if !found {
			paths = append(paths, path)
		}
	}
	return paths
}

type signingAuthorizedPathIndex struct {
	exact  map[string]struct{}
	folded map[string][]string
}

// newSigningAuthorizedPathIndex builds a per-validation lexical membership
// index. Exact normalized paths are the common case. Folded buckets only
// narrow candidates; signingPathCaseEquivalentNormalized remains the authority
// for case-sensitive versus case-insensitive directory semantics.
func newSigningAuthorizedPathIndex(paths []string) signingAuthorizedPathIndex {
	index := signingAuthorizedPathIndex{
		exact:  make(map[string]struct{}, len(paths)),
		folded: make(map[string][]string, len(paths)),
	}
	for _, path := range paths {
		normalized := normalizeSigningLexicalPath(path)
		index.exact[normalized] = struct{}{}
		key := signingPathCaseFoldKey(normalized)
		index.folded[key] = append(index.folded[key], normalized)
	}
	return index
}

func (index signingAuthorizedPathIndex) contains(path string) bool {
	normalized := normalizeSigningLexicalPath(path)
	if _, ok := index.exact[normalized]; ok {
		return true
	}
	for _, authorized := range index.folded[signingPathCaseFoldKey(normalized)] {
		if signingPathCaseEquivalentNormalized(normalized, authorized) {
			return true
		}
	}
	return false
}

// signingPathCaseFoldKey is an EqualFold-compatible bucket key. It is only an
// index accelerator: authorization still requires the platform-aware
// comparator above. Simple-folding avoids false negatives for Unicode path
// spellings that strings.ToLower does not coalesce.
func signingPathCaseFoldKey(path string) string {
	return strings.Map(func(r rune) rune {
		folded := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < folded {
				folded = next
			}
		}
		return folded
	}, path)
}

func validateSigningXCConfigPath(project *structuredVersionProject, path string, allowExternal bool) error {
	// Do the lexical/platform check before opening any rooted handle. On
	// Windows, a case-sensitive directory can make a case-variant sibling look
	// internal to a global lowercase comparison; unknown case metadata is
	// intentionally treated as external and therefore requires explicit opt-in.
	if !signingPathLexicallyContained(project, path) && !allowExternal {
		return fmt.Errorf("xcconfig path %s is outside the project directory: %w", path, rootfs.ErrEscapesRoot)
	}
	root, release, err := openSigningRoot(project.rootDir)
	if err != nil {
		return err
	}
	defer release()
	if err := root.AllowingInternalSymlinks().CheckContained(path); err == nil {
		return nil
	} else if !allowExternal {
		return err
	}
	externalRoot, releaseExternal, err := openSigningRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer releaseExternal()
	return externalRoot.CheckContained(filepath.Base(path))
}
