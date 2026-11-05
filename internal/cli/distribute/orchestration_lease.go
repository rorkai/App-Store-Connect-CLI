package distribute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

type distributionRunLeaseContextKey struct{}

func verifyDistributionRunPathLease(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	verify, _ := ctx.Value(distributionRunLeaseContextKey{}).(func() error)
	if verify == nil {
		return nil
	}
	return verify()
}

// distributionRunPathLease retains the exact state and run directory inodes
// selected after the run lock is acquired. Path-based lower seams remain
// compatible, while Verify makes any rename/replacement a terminal fail-stop.
type distributionRunPathLease struct {
	stateDir  string
	runID     string
	stateRoot *os.Root
	runRoot   *os.Root
	subtrees  []distributionPinnedSubtree
}

// distributionVerifyPathLease is a read-only lease used by `distribute
// verify`. In addition to retaining the exact state and run roots, it retains
// every artifact subtree consumed by verification. Verify therefore fails
// closed if a same-user process renames and replaces any selected directory.
// It deliberately does not acquire the advisory run lock: verification must
// not create or modify local run artifacts.
type distributionVerifyPathLease struct {
	run *distributionRunPathLease
}

type distributionPinnedSubtree struct {
	relative string
	root     *os.Root
}

type distributionVerifyLease interface {
	PinSubtrees(...string) error
	Verify() error
	Close() error
}

func openDistributionRunPathLease(stateDir, runID string) (*distributionRunPathLease, error) {
	if !distributionRunIDPattern.MatchString(runID) {
		return nil, fmt.Errorf("invalid distribution run identifier")
	}
	stateRoot, err := openExistingDistributionDirectory(stateDir, true)
	if err != nil {
		return nil, err
	}
	runRoot, err := openPinnedDistributionChild(stateRoot, runID)
	if err != nil {
		_ = stateRoot.Close()
		return nil, err
	}
	lease := &distributionRunPathLease{stateDir: stateDir, runID: runID, stateRoot: stateRoot, runRoot: runRoot}
	if err := lease.verify(); err != nil {
		_ = lease.close()
		return nil, err
	}
	return lease, nil
}

func acquireDistributionRunLease(ctx context.Context, stateDir, runID string) (func() error, func() error, error) {
	lease, err := openDistributionRunPathLease(stateDir, runID)
	if err != nil {
		return nil, nil, err
	}
	verifyLock, releaseLock, err := acquireDistributionRunLockLease(ctx, stateDir, runID)
	if err != nil {
		_ = lease.close()
		return nil, nil, err
	}
	verify := func() error {
		return errors.Join(lease.verify(), verifyLock())
	}
	if err := verify(); err != nil {
		_ = releaseLock()
		_ = lease.close()
		return nil, nil, err
	}
	if err := lease.pinSubtrees(distributionRunScaffoldDirectories()...); err != nil {
		_ = releaseLock()
		_ = lease.close()
		return nil, nil, err
	}
	return verify, func() error { return errors.Join(releaseLock(), lease.close()) }, nil
}

func (lease *distributionRunPathLease) pinSubtrees(relatives ...string) error {
	if lease == nil || lease.runRoot == nil {
		return fmt.Errorf("distribution run path lease is unavailable")
	}
	for _, relative := range relatives {
		clean := path.Clean(strings.TrimSpace(relative))
		if clean == "." || clean == "" || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, `\`) {
			return fmt.Errorf("invalid distribution run subtree")
		}
		alreadyPinned := false
		for _, pinned := range lease.subtrees {
			if pinned.relative == clean {
				alreadyPinned = true
				break
			}
		}
		if alreadyPinned {
			continue
		}
		rooted, err := openPinnedDistributionRelativeDirectory(lease.runRoot, clean)
		if err != nil {
			return fmt.Errorf("pin distribution run subtree %s: %w", clean, err)
		}
		lease.subtrees = append(lease.subtrees, distributionPinnedSubtree{relative: clean, root: rooted})
	}
	return lease.verify()
}

func (lease *distributionRunPathLease) verify() error {
	if lease == nil || lease.stateRoot == nil || lease.runRoot == nil {
		return fmt.Errorf("distribution run path lease is unavailable")
	}
	openedState, err := lease.stateRoot.Stat(".")
	if err != nil {
		return err
	}
	currentState, err := os.Lstat(lease.stateDir)
	if err != nil || !os.SameFile(openedState, currentState) {
		return fmt.Errorf("distribution state directory path changed while the run was active")
	}
	openedRun, err := lease.runRoot.Stat(".")
	if err != nil {
		return err
	}
	currentRun, err := lease.stateRoot.Lstat(lease.runID)
	if err != nil || !os.SameFile(openedRun, currentRun) {
		return fmt.Errorf("distribution run directory path changed while the run was active")
	}
	for _, pinned := range lease.subtrees {
		opened, err := openPinnedDistributionRelativeDirectory(lease.runRoot, pinned.relative)
		if err != nil {
			return fmt.Errorf("distribution run subtree changed: %w", err)
		}
		want, wantErr := pinned.root.Stat(".")
		got, gotErr := opened.Stat(".")
		closeErr := opened.Close()
		if wantErr != nil || gotErr != nil || closeErr != nil || !os.SameFile(want, got) {
			return fmt.Errorf("distribution run subtree path changed while the run was active")
		}
	}
	return nil
}

func (lease *distributionRunPathLease) close() error {
	if lease == nil {
		return nil
	}
	var err error
	for index := len(lease.subtrees) - 1; index >= 0; index-- {
		err = errors.Join(err, lease.subtrees[index].root.Close())
	}
	return errors.Join(err, lease.runRoot.Close(), lease.stateRoot.Close())
}

func acquireDistributionVerifyPathLease(stateDir, runID string) (distributionVerifyLease, error) {
	run, err := openDistributionRunPathLease(stateDir, runID)
	if err != nil {
		return nil, err
	}
	return &distributionVerifyPathLease{run: run}, nil
}

func (lease *distributionVerifyPathLease) PinSubtrees(relatives ...string) error {
	if lease == nil || lease.run == nil {
		return fmt.Errorf("distribution verification path lease is unavailable")
	}
	return lease.run.pinSubtrees(relatives...)
}

func (lease *distributionVerifyPathLease) Verify() error {
	if lease == nil || lease.run == nil {
		return fmt.Errorf("distribution verification path lease is unavailable")
	}
	return lease.run.verify()
}

func (lease *distributionVerifyPathLease) Close() error {
	if lease == nil || lease.run == nil {
		return nil
	}
	return lease.run.close()
}

func openPinnedDistributionRelativeDirectory(runRoot *os.Root, relative string) (*os.Root, error) {
	current := runRoot
	owned := false
	for _, component := range strings.Split(relative, "/") {
		next, err := openPinnedDistributionChild(current, component)
		if err != nil {
			if owned {
				_ = current.Close()
			}
			return nil, err
		}
		if owned {
			_ = current.Close()
		}
		current, owned = next, true
	}
	if !owned {
		return nil, fmt.Errorf("distribution verification subtree is empty")
	}
	return current, nil
}
