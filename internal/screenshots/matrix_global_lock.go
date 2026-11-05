package screenshots

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const matrixGlobalLockDirectory = ".asc-matrix-locks"

// acquireMatrixGlobalLock uses a stable, privacy-safe lock file outside the
// output tree. Keeping the name independent of the operator path avoids
// leaking paths into filesystem entries while still coordinating independent
// processes that selected the same destination or simulator.
func acquireMatrixGlobalLock(ctx context.Context, key string) (func() error, error) {
	if ctx == nil {
		return nil, errors.New("matrix lock context is required")
	}
	root, err := openMatrixGlobalLockRoot()
	if err != nil {
		return nil, errors.New("matrix global lock is unavailable")
	}
	digest := sha256.Sum256([]byte(key))
	name := ".asc-matrix-lock-" + hex.EncodeToString(digest[:])
	release, err := acquireMatrixNamedLock(ctx, root, name)
	if err != nil {
		_ = root.Close()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, errors.New("matrix global lock is unavailable")
	}
	return func() error {
		return errors.Join(release(), root.Close())
	}, nil
}

// matrixGlobalLockBaseDirForTest is intentionally a narrow test seam. The
// production namespace is rooted in a stable per-user system location, while
// tests use a disposable equivalent without changing the host's lock state.
var matrixGlobalLockBaseDirForTest string

// matrixGlobalLockSystemBaseDirForTest keeps the production identity resolver
// testable without writing into the real system temporary directory.
var matrixGlobalLockSystemBaseDirForTest string

func openMatrixGlobalLockRoot() (rootfs.Root, error) {
	baseDir := matrixGlobalLockBaseDirForTest
	if baseDir == "" {
		current, err := user.Current()
		if err != nil {
			return rootfs.Root{}, err
		}
		// user.Current reads the OS account database rather than the mutable
		// HOME environment variable, so separate invocations cannot bypass
		// the same-user lock namespace by selecting different homes.
		identity := strings.TrimSpace(current.Uid)
		if identity == "" {
			identity = strings.TrimSpace(current.Username)
		}
		if identity == "" {
			return rootfs.Root{}, errors.New("stable OS user identity is empty")
		}
		digest := sha256.Sum256([]byte(identity))
		systemBase := matrixGlobalLockSystemBaseDirForTest
		if systemBase == "" {
			// HomeDir comes from the OS account record, not the mutable HOME
			// environment variable. It keeps the namespace user-private while
			// remaining stable across independent invocations.
			systemBase = current.HomeDir
		}
		if strings.TrimSpace(systemBase) == "" {
			return rootfs.Root{}, errors.New("stable OS user home directory is empty")
		}
		baseDir = filepath.Join(systemBase, ".asc-matrix-users-"+hex.EncodeToString(digest[:8]))
	}
	if strings.TrimSpace(baseDir) == "" {
		return rootfs.Root{}, errors.New("matrix lock base directory is empty")
	}
	lockRootPath := filepath.Join(baseDir, ".asc", matrixGlobalLockDirectory)
	lockRoot, err := rootfs.New(lockRootPath)
	if err != nil {
		return rootfs.Root{}, err
	}
	if err := lockRoot.MkdirAll(".", 0o700); err != nil {
		_ = lockRoot.Close()
		return rootfs.Root{}, err
	}
	anchor, err := lockRoot.OpenDir(".")
	if err != nil {
		_ = lockRoot.Close()
		return rootfs.Root{}, err
	}
	if err := anchor.Chmod(0o700); err != nil {
		_ = anchor.Close()
		_ = lockRoot.Close()
		return rootfs.Root{}, err
	}
	if err := anchor.Close(); err != nil {
		_ = lockRoot.Close()
		return rootfs.Root{}, err
	}
	return lockRoot, nil
}

func acquireMatrixSimulatorLock(ctx context.Context, udid string) (func() error, error) {
	return acquireMatrixGlobalLock(ctx, "simulator:"+normalizeMatrixUDID(udid))
}

// matrixOutputLockTarget carries the already-open destination root whose
// filesystem identity must determine the lock key. A path string is not
// sufficient: bind mounts and other filesystem aliases can name the same
// directory through different lexical and resolved paths.
type matrixOutputLockTarget struct {
	root   rootfs.Root
	opened *os.Root
}

// acquireMatrixOutputLocks orders all destination locks before acquisition so
// two concurrent runs with the same paths in a different order cannot deadlock.
// The compatibility wrapper opens each destination before deriving its key;
// RunMatrixWithDependencies uses acquireMatrixOutputLocksForRoots so the roots
// used for execution can be the same identities that were locked.
func acquireMatrixOutputLocks(ctx context.Context, paths []string) (func() error, error) {
	targets := make([]matrixOutputLockTarget, 0, len(paths))
	roots := make([]rootfs.Root, 0, len(paths))
	openedRoots := make([]*os.Root, 0, len(paths))
	for _, path := range paths {
		root, err := openMatrixOutputRoot(path)
		if err != nil {
			for _, opened := range openedRoots {
				_ = opened.Close()
			}
			for _, opened := range roots {
				_ = opened.Close()
			}
			return nil, err
		}
		opened, err := root.OpenRoot()
		if err != nil {
			_ = root.Close()
			for _, opened := range openedRoots {
				_ = opened.Close()
			}
			for _, opened := range roots {
				_ = opened.Close()
			}
			return nil, err
		}
		roots = append(roots, root)
		openedRoots = append(openedRoots, opened)
		if matrixOutputLockRootOpenedForTest != nil {
			matrixOutputLockRootOpenedForTest(opened)
		}
		targets = append(targets, matrixOutputLockTarget{root: root, opened: opened})
	}
	release, acquireErr := acquireMatrixOutputLocksForRoots(ctx, targets)
	var closeErr error
	for _, opened := range openedRoots {
		closeErr = errors.Join(closeErr, opened.Close())
	}
	for _, root := range roots {
		closeErr = errors.Join(closeErr, root.Close())
	}
	if closeErr != nil {
		if release != nil {
			closeErr = errors.Join(closeErr, release())
			release = nil
		}
	}
	return release, errors.Join(acquireErr, closeErr)
}

// acquireMatrixOutputLocksForRoots derives each lock key from the opened root
// identity, not from a canonicalized pathname. The identity representation is
// immediately hashed by acquireMatrixGlobalLock and never appears in output.
func acquireMatrixOutputLocksForRoots(ctx context.Context, targets []matrixOutputLockTarget) (func() error, error) {
	identities := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		opened := target.opened
		var err error
		if opened == nil {
			opened, err = target.root.OpenRoot()
		}
		if err != nil {
			return nil, errors.New("matrix output filesystem identity is unavailable")
		}
		identity, identityErr := matrixStableFilesystemIdentity(opened)
		if target.opened == nil {
			err = opened.Close()
		}
		if identityErr != nil {
			err = errors.Join(err, identityErr)
		}
		if err != nil {
			return nil, errors.New("matrix output filesystem identity is unavailable")
		}
		identities[identity] = struct{}{}
	}
	ordered := make([]string, 0, len(identities))
	for identity := range identities {
		ordered = append(ordered, identity)
	}
	sort.Strings(ordered)

	releases := make([]func() error, 0, len(ordered))
	for _, identity := range ordered {
		release, err := acquireMatrixGlobalLock(ctx, "output:"+identity)
		if err != nil {
			var releaseErr error
			for i := len(releases) - 1; i >= 0; i-- {
				releaseErr = errors.Join(releaseErr, releases[i]())
			}
			return nil, errors.Join(err, releaseErr)
		}
		releases = append(releases, release)
	}
	return func() error {
		var releaseErr error
		for i := len(releases) - 1; i >= 0; i-- {
			releaseErr = errors.Join(releaseErr, releases[i]())
		}
		return releaseErr
	}, nil
}

// matrixFilesystemCaseInsensitive reports whether the filesystem holding path
// aliases case variants, probing an existing path component without creating
// anything.
func matrixFilesystemCaseInsensitive(path string) bool {
	if runtime.GOOS == "windows" {
		return true
	}
	current := filepath.Clean(path)
	for {
		info, err := os.Lstat(current)
		if err == nil {
			name := filepath.Base(current)
			variant := matrixCaseVariant(name)
			if variant != "" {
				variantInfo, variantErr := os.Lstat(filepath.Join(filepath.Dir(current), variant))
				return variantErr == nil && os.SameFile(info, variantInfo)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false
		}
		current = parent
	}
}

func matrixCaseVariant(name string) string {
	variant := []byte(name)
	for i, char := range variant {
		switch {
		case char >= 'a' && char <= 'z':
			variant[i] = char - ('a' - 'A')
			return string(variant)
		case char >= 'A' && char <= 'Z':
			variant[i] = char + ('a' - 'A')
			return string(variant)
		}
	}
	return ""
}

func matrixLockError(kind string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("matrix %s lock failed: %w", kind, err)
}
