package versions

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// configurePhasedReleaseCheckProcess stops the whole check process tree on
// timeout or cancellation, falling back to killing only the check.
func configurePhasedReleaseCheckProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	command.Cancel = func() error {
		ctx, cancel := context.WithTimeout(context.Background(), phasedReleaseCheckWaitDelay)
		defer cancel()
		killer := exec.CommandContext(ctx, "taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid))
		killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if killer.Run() == nil {
			return nil
		}
		if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return nil
	}
}
