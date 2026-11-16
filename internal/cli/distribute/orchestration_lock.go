package distribute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	distributionRunLockFilename     = "lock"
	distributionRunLockPollInterval = 25 * time.Millisecond
)

// distributionRunLockAfterOpenForTest lets a focused test replace the lock
// pathname after a waiter has opened the original inode. Tests must not mutate
// it until the acquire call using it has returned.
var distributionRunLockAfterOpenForTest func()

// acquireDistributionRunLock serializes mutation of one persisted run. The
// lock inode remains in the protected run directory permanently: unlinking it
// would allow a second process to lock a replacement inode while the first
// process still owns the original lock.
func acquireDistributionRunLock(ctx context.Context, d *shared.Deps, stateDir, runID string) (func() error, error) {
	_, release, err := acquireDistributionRunLockLease(ctx, d, stateDir, runID)
	return release, err
}

// acquireDistributionRunLockLease returns both the release operation and a
// verifier for the exact lock inode retained by the holder. Mutating callers
// compose the verifier into every path-lease guard so replacing the permanent
// lock pathname can never be accepted by a later checkpoint.
func acquireDistributionRunLockLease(ctx context.Context, d *shared.Deps, stateDir, runID string) (func() error, func() error, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("distribution run lock context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	runRoot, err := protectedDistributionRunRoot(stateDir, runID)
	if err != nil {
		return nil, nil, fmt.Errorf("validate distribution run directory: %w", err)
	}
	closeRootOnFailure := true
	defer func() {
		if closeRootOnFailure {
			_ = runRoot.Close()
		}
	}()
	file, err := openDistributionRunLockFile(runRoot, distributionRunLockFilename)
	if err != nil {
		return nil, nil, fmt.Errorf("open distribution run lock: %w", err)
	}
	closeOnFailure := true
	defer func() {
		if closeOnFailure {
			_ = file.Close()
		}
	}()
	if err := validateDistributionRunLockFile(file); err != nil {
		return nil, nil, fmt.Errorf("validate distribution run lock: %w", err)
	}
	if distributionRunLockAfterOpenForTest != nil {
		distributionRunLockAfterOpenForTest()
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		locked, lockErr := tryDistributionRunFileLock(file)
		if lockErr != nil {
			return nil, nil, fmt.Errorf("lock distribution run: %w", lockErr)
		}
		if locked {
			// Recheck the opened inode after claiming it so a hard-link change
			// made while this caller waited cannot silently weaken the lock.
			if err := validateDistributionRunLockPath(runRoot, file); err != nil {
				return nil, nil, errors.Join(
					fmt.Errorf("validate locked distribution run: %w", err),
					unlockDistributionRunFile(file),
				)
			}
			closeOnFailure = false
			closeRootOnFailure = false
			var once sync.Once
			var releaseErr error
			verify := func() error {
				if err := validateDistributionRunLockPath(runRoot, file); err != nil {
					return fmt.Errorf("distribution run lock path changed while the run was active: %w", err)
				}
				return nil
			}
			release := func() error {
				once.Do(func() {
					releaseErr = errors.Join(unlockDistributionRunFile(file), file.Close(), runRoot.Close())
				})
				return releaseErr
			}
			return verify, release, nil
		}

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-d.After(distributionRunLockPollInterval):
		}
	}
}

func validateDistributionRunLockFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := validatePrivateDistributionFileInfo(info); err != nil {
		return err
	}
	return validateDistributionRunLockPlatform(file)
}

func validateDistributionRunLockPath(runRoot *os.Root, file *os.File) error {
	if err := validateDistributionRunLockFile(file); err != nil {
		return err
	}
	openedInfo, err := file.Stat()
	if err != nil {
		return err
	}
	pathInfo, err := runRoot.Lstat(distributionRunLockFilename)
	if err != nil {
		return fmt.Errorf("inspect lock path after acquisition: %w", err)
	}
	if err := validatePrivateDistributionFileInfo(pathInfo); err != nil {
		return fmt.Errorf("validate lock path after acquisition: %w", err)
	}
	if !os.SameFile(openedInfo, pathInfo) {
		return fmt.Errorf("distribution run lock path was replaced while waiting")
	}
	return nil
}
