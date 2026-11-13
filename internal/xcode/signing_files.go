package xcode

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func signingFileDigest(path string) (string, error) {
	data, err := readSigningRegularFile(path, signingPlanMaxBytes)
	if err != nil {
		return "", err
	}
	return signingFileDigestBytes(data), nil
}

func signingFileDigestBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func readSigningRegularFile(path string, limit int64) ([]byte, error) {
	absolute, err := canonicalSigningPath(path, "file")
	if err != nil {
		return nil, err
	}
	root, release, err := openSigningRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer release()
	return root.ReadFileLimited(filepath.Base(absolute), limit)
}

// signingPlanRoots lets one signing plan build reuse the Root selected for each
// directory instead of reselecting it for every xcconfig read or stat. Every
// rooted operation still reopens the selected directory without following
// symlinks and verifies its pinned identity, so a replaced directory fails
// closed rather than redirecting a read.
// signingPlanMaxCachedRoots bounds the directory descriptors one plan build
// keeps open; directories beyond it get a Root that closes after each use.
const signingPlanMaxCachedRoots = 64

var signingPlanRoots struct {
	sync.Mutex
	scopes   int
	borrowed int
	roots    map[string]rootfs.Root
}

func beginSigningPlanRootScope() func() {
	signingPlanRoots.Lock()
	signingPlanRoots.scopes++
	signingPlanRoots.Unlock()
	return func() {
		signingPlanRoots.Lock()
		defer signingPlanRoots.Unlock()
		signingPlanRoots.scopes--
		closeIdleSigningPlanRootsLocked()
	}
}

func releaseSigningPlanRoot() {
	signingPlanRoots.Lock()
	defer signingPlanRoots.Unlock()
	signingPlanRoots.borrowed--
	closeIdleSigningPlanRootsLocked()
}

func closeIdleSigningPlanRootsLocked() {
	if signingPlanRoots.scopes > 0 || signingPlanRoots.borrowed > 0 {
		return
	}
	for _, root := range signingPlanRoots.roots {
		_ = root.Close()
	}
	signingPlanRoots.roots = nil
}

func openSigningRoot(directory string) (rootfs.Root, func(), error) {
	signingPlanRoots.Lock()
	cached, ok := signingPlanRoots.roots[directory]
	if signingPlanRoots.scopes == 0 || (!ok && len(signingPlanRoots.roots) >= signingPlanMaxCachedRoots) {
		signingPlanRoots.Unlock()
		root, err := rootfs.New(directory)
		if err != nil {
			return rootfs.Root{}, nil, err
		}
		return root, func() { _ = root.Close() }, nil
	}
	defer signingPlanRoots.Unlock()
	root := cached
	if !ok {
		var err error
		root, err = rootfs.New(directory)
		if err != nil {
			return rootfs.Root{}, nil, err
		}
		if signingPlanRoots.roots == nil {
			signingPlanRoots.roots = make(map[string]rootfs.Root)
		}
		signingPlanRoots.roots[directory] = root
	}
	signingPlanRoots.borrowed++
	var once sync.Once
	return root, func() { once.Do(releaseSigningPlanRoot) }, nil
}

// signingXCConfigReadFileFn keeps configuration reads behind the same rooted
// reader while allowing tests to prove that authorization rejects a path
// before any read is attempted. Production always uses readSigningRegularFile.
var signingXCConfigReadFileFn = readSigningRegularFile

// signingXCConfigStatFileFn keeps later authorization-aware existence checks
// behind the same rooted reader while allowing tests to prove that an
// unauthorized path is rejected before stat/open is attempted.
var signingXCConfigStatFileFn = signingRegularFileInfo

// signingXCConfigIdentityFn is used only after signing authorization has
// accepted a path. On Windows, per-directory case sensitivity means a global
// lowercase lexical key cannot establish whether two spellings name one file
// or two; the rooted file identity supplies that distinction.
var signingXCConfigIdentityFn = signingRegularFileInfo

// signingRegularFileInfo obtains metadata through the same rooted no-follow
// path policy used for signing reads. Callers must establish authorization
// before invoking it for any path that came from project configuration.
func signingRegularFileInfo(path string) (os.FileInfo, error) {
	absolute, err := canonicalSigningPath(path, "file")
	if err != nil {
		return nil, err
	}
	root, release, err := openSigningRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer release()
	if err := root.CheckCreateNewFile(filepath.Base(absolute)); err == nil {
		return nil, os.ErrNotExist
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	file, err := root.OpenFile(filepath.Base(absolute))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return file.Stat()
}

// signingArtifactPathInfoFn is kept narrow so tests can inject a path
// inspection failure at the alias-validation boundary. Production alias
// checks always use signingRegularFileInfo's rooted, no-follow implementation.
var signingArtifactPathInfoFn = signingRegularFileInfo

// signingResolveProspectivePathFn keeps future-path alias resolution behind a
// narrow seam so tests can prove unauthorized external configuration paths are
// never inspected before opt-in. Production uses rootfs' parent-resolving,
// no-follow implementation.
var signingResolveProspectivePathFn = rootfs.ResolveProspectivePath
