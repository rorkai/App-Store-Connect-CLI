//go:build !windows

package versions

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configurePhasedReleaseCheckProcess runs the check in its own process group so
// a timeout or cancellation also stops processes the check started.
func configurePhasedReleaseCheckProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
