//go:build darwin

package signing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// keychainSecurityOutputLimit bounds security output. A login keychain dump of
// every certificate easily exceeds the 64 KiB diagnostic limit, so the keychain
// commands use a larger bound and fail instead of silently truncating.
const keychainSecurityOutputLimit = 16 << 20

func defaultRunKeychainSecurity(ctx context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
	return runKeychainUtility(ctx, "/usr/bin/security", stdin, keychainSecurityOutputLimit, args...)
}

func runKeychainUtility(ctx context.Context, utility string, stdin []byte, limit int, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, utility, args...)
	cmd.Env = SanitizedChildEnvironment(os.Environ())
	cmd.Stdin = bytes.NewReader(stdin)
	// security prompts on a controlling terminal and ignores stdin if it has one.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	stdout := &boundedOutputBuffer{limit: limit}
	stderr := &boundedOutputBuffer{limit: signingRunDiagnosticLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil && stdout.overflow {
		err = fmt.Errorf("output exceeded the %d-byte limit", limit)
	}
	return stdout.data, stderr.data, err
}
