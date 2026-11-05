//go:build !windows

package xcode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

var terminateXcodeProcessGroupFn = terminateExactXcodeProcessGroup

type xcodeProcessGroupAnchor struct {
	command *exec.Cmd
	release *os.File
	pid     int
}

// runXcodeCommandWithProcessGroupCleanup owns a fresh process group whose
// anchor remains live until post-Wait cleanup completes. The anchor prevents
// the numeric group ID from being reused after the command leader is reaped.
func runXcodeCommandWithProcessGroupCleanup(cmd *exec.Cmd) (result error) {
	anchor, err := startXcodeProcessGroupAnchor()
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, anchor.Close())
	}()

	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: anchor.pid}
	cmd.Cancel = func() error {
		return killExactXcodeProcessGroup(anchor.pid)
	}
	cmd.WaitDelay = xcodeCommandPipeWaitDelay
	if err := cmd.Start(); err != nil {
		return err
	}
	waitErr := normalizeXcodeCommandWaitError(cmd, cmd.Wait())
	cleanupErr := terminateXcodeProcessGroupFn(anchor.pid)
	return errors.Join(waitErr, cleanupErr)
}

func startXcodeProcessGroupAnchor() (*xcodeProcessGroupAnchor, error) {
	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create xcodebuild process-group anchor: %w", err)
	}
	command := exec.Command("/bin/sh", "-c", "read _ || :")
	command.Stdin = reader
	command.Env = []string{}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return nil, fmt.Errorf("start xcodebuild process-group anchor: %w", err)
	}
	if err := reader.Close(); err != nil {
		_ = writer.Close()
		_ = command.Process.Kill()
		_ = command.Wait()
		return nil, fmt.Errorf("close xcodebuild process-group anchor reader: %w", err)
	}
	return &xcodeProcessGroupAnchor{command: command, release: writer, pid: command.Process.Pid}, nil
}

func (anchor *xcodeProcessGroupAnchor) Close() error {
	if anchor == nil {
		return nil
	}
	closeErr := anchor.release.Close()
	waitErr := anchor.command.Wait()
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		waitErr = nil
	}
	return errors.Join(closeErr, waitErr)
}

func killExactXcodeProcessGroup(pid int) error {
	// The group ID belongs to the live anchor process. Check it immediately
	// before signalling so an already-finished group remains benign.
	if err := syscall.Kill(-pid, 0); errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	} else if err != nil {
		return err
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	} else {
		return err
	}
}

func terminateExactXcodeProcessGroup(pid int) error {
	err := killExactXcodeProcessGroup(pid)
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("terminate xcodebuild process group: %w", err)
	}
	// A successful SIGKILL syscall has synchronously delivered an uncatchable
	// signal to every current member. Do not poll the numeric PGID afterward:
	// once the group disappears, that number can be reused by an unrelated
	// process group before a subsequent probe.
	return nil
}
