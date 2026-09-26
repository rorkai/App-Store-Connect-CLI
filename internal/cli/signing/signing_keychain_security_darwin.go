//go:build darwin

package signing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// keychainSecurityOutputLimit bounds security output. A login keychain dump of
// every certificate easily exceeds the 64 KiB diagnostic limit, so the keychain
// commands use a larger bound and fail instead of silently truncating.
const keychainSecurityOutputLimit = 16 << 20

func defaultRunKeychainSecurity(ctx context.Context, stdin []byte, args ...string) ([]byte, []byte, error) {
	return runKeychainUtility(ctx, "/usr/bin/security", stdin, keychainSecurityOutputLimit, args...)
}

func runKeychainUtility(ctx context.Context, utility string, stdin []byte, limit int, args ...string) ([]byte, []byte, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("run keychain utility: context is required")
	}
	cmd := exec.CommandContext(ctx, utility, args...)
	cmd.Env = SanitizedChildEnvironment(os.Environ())
	cmd.Stdin = bytes.NewReader(stdin)
	stdout := &keychainOutputBuffer{limit: limit}
	stderr := &keychainOutputBuffer{limit: signingRunDiagnosticLimit}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err == nil && stdout.overflow {
		err = fmt.Errorf("output exceeded the %d-byte limit", limit)
	}
	return stdout.data, stderr.data, err
}

type keychainOutputBuffer struct {
	data     []byte
	limit    int
	overflow bool
}

func (b *keychainOutputBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := b.limit - len(b.data)
	if written > remaining {
		b.overflow = true
		data = data[:max(remaining, 0)]
	}
	b.data = append(b.data, data...)
	return written, nil
}
