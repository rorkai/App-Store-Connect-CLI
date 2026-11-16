package distribute

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const distributionRunLockHelperEnvironment = "ASC_DISTRIBUTION_RUN_LOCK_HELPER"

func TestDistributionRunLockRejectsCanceledContextBeforeCreatingFile(t *testing.T) {
	stateDir, runID, runDir := distributionRunLockFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := acquireDistributionRunLock(ctx, newDistributeTestDeps(), stateDir, runID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireDistributionRunLock() error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Lstat(filepath.Join(runDir, distributionRunLockFilename)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("lock file should not be created for canceled context; Lstat() error = %v", statErr)
	}
}

func TestDistributionRunLockSameRunContendsAndHonorsCancellation(t *testing.T) {
	stateDir, runID, _ := distributionRunLockFixture(t)
	releaseFirst, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
	if err != nil {
		t.Fatalf("first acquireDistributionRunLock() error = %v", err)
	}
	defer func() { _ = releaseFirst() }()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, lockErr := acquireDistributionRunLock(ctx, newDistributeTestDeps(), stateDir, runID)
		errCh <- lockErr
	}()

	select {
	case err := <-errCh:
		t.Fatalf("contending lock returned before cancellation: %v", err)
	case <-time.After(3 * distributionRunLockPollInterval):
	}
	cancel()
	if err := <-errCh; !errors.Is(err, context.Canceled) {
		t.Fatalf("contending acquire error = %v, want context.Canceled", err)
	}
}

func TestDistributionRunLockRejectsPathReplacementWhileWaiterContends(t *testing.T) {
	stateDir, runID, runDir := distributionRunLockFixture(t)
	releaseFirst, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
	if err != nil {
		t.Fatalf("first acquireDistributionRunLock() error = %v", err)
	}
	firstReleased := false
	defer func() {
		if !firstReleased {
			_ = releaseFirst()
		}
	}()

	opened := make(chan struct{})
	continueAcquire := make(chan struct{})
	distributionRunLockAfterOpenForTest = func() {
		close(opened)
		<-continueAcquire
	}
	errCh := make(chan error, 1)
	go func() {
		_, lockErr := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
		errCh <- lockErr
	}()
	waiterReleased := false
	defer func() {
		if !waiterReleased {
			close(continueAcquire)
			<-errCh
		}
		distributionRunLockAfterOpenForTest = nil
	}()

	<-opened
	lockPath := filepath.Join(runDir, distributionRunLockFilename)
	oldLockPath := filepath.Join(runDir, "lock.replaced")
	if err := os.Rename(lockPath, oldLockPath); err != nil {
		t.Fatalf("rename held lock inode: %v", err)
	}
	if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
		t.Fatalf("create replacement lock inode: %v", err)
	}
	if err := os.Chmod(lockPath, 0o600); err != nil {
		t.Fatalf("chmod replacement lock inode: %v", err)
	}
	close(continueAcquire)
	waiterReleased = true
	if err := releaseFirst(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	firstReleased = true

	if err := <-errCh; err == nil || !strings.Contains(err.Error(), "path was replaced") {
		t.Fatalf("waiter error = %v, want lock path replacement rejection", err)
	}
}

func TestDistributionRunLockContendsAcrossProcesses(t *testing.T) {
	stateDir, runID, _ := distributionRunLockFixture(t)
	command := exec.Command(os.Args[0], "-test.run=^TestDistributionRunLockHelperProcess$")
	command.Env = append(
		os.Environ(),
		distributionRunLockHelperEnvironment+"=1",
		"ASC_DISTRIBUTION_RUN_LOCK_STATE_DIR="+stateDir,
		"ASC_DISTRIBUTION_RUN_LOCK_RUN_ID="+runID,
	)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("create helper stdin: %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("create helper stdout: %v", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start lock helper: %v", err)
	}
	helperFinished := false
	t.Cleanup(func() {
		if !helperFinished {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})

	// The helper either reports the held lock or exits, which ends this read
	// with EOF, so readiness needs no wall-clock bound.
	if line, err := bufio.NewReader(stdout).ReadString('\n'); err != nil || line != "locked\n" {
		t.Fatalf("wait for lock helper: line=%q err=%v", line, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	_, err = acquireDistributionRunLock(ctx, newDistributeTestDeps(), stateDir, runID)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cross-process acquire error = %v, want context.DeadlineExceeded", err)
	}
	if _, err := stdin.Write([]byte("release\n")); err != nil {
		t.Fatalf("signal helper release: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close helper stdin: %v", err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("wait for lock helper: %v (stderr: %s)", err, stderr.String())
	}
	helperFinished = true
}

func TestDistributionRunLockHelperProcess(t *testing.T) {
	if os.Getenv(distributionRunLockHelperEnvironment) != "1" {
		return
	}
	release, err := acquireDistributionRunLock(
		context.Background(),
		newDistributeTestDeps(),
		os.Getenv("ASC_DISTRIBUTION_RUN_LOCK_STATE_DIR"),
		os.Getenv("ASC_DISTRIBUTION_RUN_LOCK_RUN_ID"),
	)
	if err != nil {
		t.Fatalf("helper acquireDistributionRunLock() error = %v", err)
	}
	defer func() { _ = release() }()
	if _, err := fmt.Fprintln(os.Stdout, "locked"); err != nil {
		t.Fatalf("helper signal readiness: %v", err)
	}
	if _, err := bufio.NewReader(os.Stdin).ReadString('\n'); err != nil {
		t.Fatalf("helper wait for release: %v", err)
	}
}

func TestDistributionRunLockDifferentRunsDoNotContend(t *testing.T) {
	stateDir, firstRunID, _ := distributionRunLockFixture(t)
	secondRunID, err := newDistributionRunID()
	if err != nil {
		t.Fatalf("newDistributionRunID() error = %v", err)
	}
	if err := createDistributionRunDirectory(stateDir, secondRunID); err != nil {
		t.Fatalf("create second distribution run directory: %v", err)
	}

	releaseFirst, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, firstRunID)
	if err != nil {
		t.Fatalf("acquire first run lock: %v", err)
	}
	defer func() { _ = releaseFirst() }()

	releaseSecond, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, secondRunID)
	if err != nil {
		t.Fatalf("acquire different run lock: %v", err)
	}
	if err := releaseSecond(); err != nil {
		t.Fatalf("release different run lock: %v", err)
	}
}

func TestDistributionRunLockReleaseAllowsReacquire(t *testing.T) {
	stateDir, runID, runDir := distributionRunLockFixture(t)
	release, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
	if err != nil {
		t.Fatalf("first acquireDistributionRunLock() error = %v", err)
	}
	if err := release(); err != nil {
		t.Fatalf("first release error = %v", err)
	}
	// Release is deliberately idempotent so deferred cleanup is safe.
	if err := release(); err != nil {
		t.Fatalf("second release error = %v", err)
	}

	releaseAgain, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
	if err != nil {
		t.Fatalf("reacquireDistributionRunLock() error = %v", err)
	}
	defer func() { _ = releaseAgain() }()

	info, err := os.Lstat(filepath.Join(runDir, distributionRunLockFilename))
	if err != nil {
		t.Fatalf("Lstat(lock file): %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("lock mode = %v, want regular file", info.Mode())
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("lock permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestDistributionRunLockRejectsUnsafeLockFile(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, lockPath string)
		want  string
	}{
		{
			name: "symlink",
			setup: func(t *testing.T, lockPath string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(lockPath), "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatalf("write target: %v", err)
				}
				if err := os.Symlink(target, lockPath); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("symlink creation unavailable: %v", err)
					}
					t.Fatalf("create symlink: %v", err)
				}
			},
			want: "lock",
		},
		{
			name: "hardlink",
			setup: func(t *testing.T, lockPath string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(lockPath), "target")
				if err := os.WriteFile(target, nil, 0o600); err != nil {
					t.Fatalf("write target: %v", err)
				}
				if err := os.Link(target, lockPath); err != nil {
					t.Fatalf("create hard link: %v", err)
				}
			},
			want: "hard link",
		},
		{
			name: "permissions",
			setup: func(t *testing.T, lockPath string) {
				t.Helper()
				if err := os.WriteFile(lockPath, nil, 0o600); err != nil {
					t.Fatalf("write lock file: %v", err)
				}
				if err := os.Chmod(lockPath, 0o644); err != nil {
					t.Fatalf("chmod lock file: %v", err)
				}
			},
			want: "0600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateDir, runID, runDir := distributionRunLockFixture(t)
			lockPath := filepath.Join(runDir, distributionRunLockFilename)
			tt.setup(t, lockPath)

			_, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
			if err == nil {
				t.Fatal("acquireDistributionRunLock() error = nil, want unsafe lock rejection")
			}
			if runtime.GOOS != "windows" && !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("acquireDistributionRunLock() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestDistributionRunLockRejectsUnsafeRunDirectory(t *testing.T) {
	stateDir, runID, runDir := distributionRunLockFixture(t)
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permissions do not apply on Windows")
	}
	if err := os.Chmod(runDir, 0o755); err != nil {
		t.Fatalf("chmod run directory: %v", err)
	}
	defer func() { _ = os.Chmod(runDir, 0o700) }()

	_, err := acquireDistributionRunLock(context.Background(), newDistributeTestDeps(), stateDir, runID)
	if err == nil || !strings.Contains(err.Error(), "0700") {
		t.Fatalf("acquireDistributionRunLock() error = %v, want 0700 rejection", err)
	}
}

func distributionRunLockFixture(t *testing.T) (stateDir, runID, runDir string) {
	t.Helper()
	stateDir = filepath.Join(t.TempDir(), "state")
	var err error
	runID, err = newDistributionRunID()
	if err != nil {
		t.Fatalf("newDistributionRunID() error = %v", err)
	}
	if err := createDistributionRunDirectory(stateDir, runID); err != nil {
		t.Fatalf("createDistributionRunDirectory() error = %v", err)
	}
	return stateDir, runID, filepath.Join(stateDir, runID)
}
