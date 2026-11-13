package shared

import (
	"context"
	"os/exec"
	"runtime"
)

// ShellCommand runs a user-supplied command line through the platform shell,
// without login-shell startup files whose output would precede the command's.
func ShellCommand(ctx context.Context, command string) *exec.Cmd {
	return exec.CommandContext(ctx, shellCommandName(), shellCommandArgs(command)...)
}

func shellCommandName() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "/bin/sh"
}

func shellCommandArgs(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/d", "/s", "/c", command}
	}
	return []string{"-c", command}
}
