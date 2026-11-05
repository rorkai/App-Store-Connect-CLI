//go:build !windows

package xcode

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

func TestRunXcodeCommandWithProcessGroupCleanupPreservesWaitAndCleanupFailures(t *testing.T) {
	wantCleanupErr := errors.New("forced process-group cleanup failure")
	originalTerminate := terminateXcodeProcessGroupFn
	cleanupPID := 0
	terminateXcodeProcessGroupFn = func(pid int) error {
		if pid <= 0 {
			t.Fatalf("cleanup PID = %d, want positive child PID", pid)
		}
		if err := syscall.Kill(pid, 0); err != nil {
			t.Fatalf("process-group anchor was not live during cleanup: %v", err)
		}
		cleanupPID = pid
		return wantCleanupErr
	}
	t.Cleanup(func() { terminateXcodeProcessGroupFn = originalTerminate })

	cmd := exec.CommandContext(context.Background(), os.Args[0], "-test.run=TestXcodeProcessGroupExitHelper")
	cmd.Env = append(os.Environ(), "GO_WANT_XCODE_PROCESS_GROUP_EXIT_HELPER=1")
	err := runXcodeCommandWithProcessGroupCleanup(cmd)
	if !errors.Is(err, wantCleanupErr) {
		t.Fatalf("error = %v, want cleanup failure", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error = %v, want child exit failure", err)
	}
	if got := exitErr.ExitCode(); got != 23 {
		t.Fatalf("exit code = %d, want 23", got)
	}
	if cleanupPID == cmd.Process.Pid {
		t.Fatalf("cleanup reused reaped command PID %d instead of durable anchor", cleanupPID)
	}
}

func TestRunXcodeCommandWithProcessGroupCleanupDoesNotCleanupAfterStartFailure(t *testing.T) {
	originalTerminate := terminateXcodeProcessGroupFn
	called := false
	terminateXcodeProcessGroupFn = func(int) error {
		called = true
		return nil
	}
	t.Cleanup(func() { terminateXcodeProcessGroupFn = originalTerminate })

	err := runXcodeCommandWithProcessGroupCleanup(exec.CommandContext(context.Background(), "/asc-test-command-that-does-not-exist"))
	if err == nil {
		t.Fatal("expected start failure, got nil")
	}
	if called {
		t.Fatal("process-group cleanup ran without a successfully started child PID")
	}
}

func TestXcodeProcessGroupExitHelper(t *testing.T) {
	if os.Getenv("GO_WANT_XCODE_PROCESS_GROUP_EXIT_HELPER") != "1" {
		return
	}
	os.Exit(23)
}
