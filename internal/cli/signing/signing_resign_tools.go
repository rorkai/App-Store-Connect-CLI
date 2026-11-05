package signing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/infoplist"
	"howett.net/plist"
)

// codesign may process large app bundles and many nested code objects. Keep a
// bounded fallback for callers without a deadline, but never replace a
// caller-supplied operation deadline with a shorter per-object timeout.
const signingResignToolTimeout = 5 * time.Minute

type signingResignToolOutput struct {
	Stdout          []byte
	Stderr          []byte
	StdoutTruncated bool
	StderrTruncated bool
}

var runSigningResignToolFn = runSigningResignTool

func runSigningResignTool(ctx context.Context, executable string, args ...string) (signingResignToolOutput, error) {
	return runSigningResignToolWithFallback(ctx, signingResignToolTimeout, executable, args...)
}

func runSigningResignToolWithFallback(ctx context.Context, fallbackTimeout time.Duration, executable string, args ...string) (signingResignToolOutput, error) {
	if err := ctx.Err(); err != nil {
		return signingResignToolOutput{}, err
	}
	toolContext, cancel := signingResignToolContext(ctx, fallbackTimeout)
	defer cancel()
	command := exec.CommandContext(toolContext, executable, args...)
	command.Env = SanitizedChildEnvironment(os.Environ())
	stdout := &boundedOutputBuffer{limit: signingResignToolOutputLimit}
	stderr := &boundedOutputBuffer{limit: signingResignToolOutputLimit}
	command.Stdout = stdout
	command.Stderr = stderr
	err := command.Run()
	result := signingResignToolOutput{
		Stdout:          stdout.Bytes(),
		Stderr:          stderr.Bytes(),
		StdoutTruncated: stdout.Truncated(),
		StderrTruncated: stderr.Truncated(),
	}
	if err == nil {
		return result, nil
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return result, contextErr
	}
	if contextErr := toolContext.Err(); contextErr != nil {
		// Only the internal fallback deadline can end here: a caller deadline
		// reuses ctx itself and is reported through the branch above. Name the
		// tool and budget so internal callers can distinguish a slow tool from
		// an operator cancellation; the closed operational error boundary still
		// hides this text from public CLI output.
		return result, fmt.Errorf("%s timed out after %s: %w", filepath.Base(executable), fallbackTimeout, contextErr)
	}
	return result, fmt.Errorf("%s failed: %w", filepath.Base(executable), err)
}

func signingResignToolContext(ctx context.Context, fallbackTimeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, fallbackTimeout)
}

const signingResignToolOutputLimit = infoplist.MaxBytes + 64*1024

type boundedOutputBuffer struct {
	data     []byte
	overflow bool
	limit    int
}

func (buffer *boundedOutputBuffer) Write(data []byte) (int, error) {
	if len(buffer.data) >= buffer.limit {
		buffer.overflow = true
		return len(data), nil
	}
	remaining := buffer.limit - len(buffer.data)
	if len(data) > remaining {
		buffer.data = append(buffer.data, data[:remaining]...)
		buffer.overflow = true
		return len(data), nil
	}
	buffer.data = append(buffer.data, data...)
	return len(data), nil
}

func (buffer *boundedOutputBuffer) Bytes() []byte {
	return append([]byte(nil), buffer.data...)
}

func (buffer *boundedOutputBuffer) Truncated() bool {
	return buffer.overflow
}

func readSigningResignEntitlements(ctx context.Context, executablePath string) (map[string]any, error) {
	if strings.TrimSpace(executablePath) == "" {
		return nil, fmt.Errorf("signed executable path is missing")
	}
	result, err := runSigningResignToolFn(ctx, "/usr/bin/codesign", "-d", "--entitlements", ":-", executablePath)
	if err != nil {
		// An unsigned nested code object has no claims to preserve. Any other
		// inspection failure is terminal so that a damaged signature is never
		// silently replaced. A truncated capture cannot prove the unsigned
		// diagnostic, so it stays terminal too.
		if !result.StderrTruncated && isUnsignedSigningResignCodeObject(result.Stderr) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("read signed entitlements: %w", err)
	}
	// A truncated capture could hold a partial entitlement document that then
	// decodes as empty, which would silently drop the target's existing
	// capabilities. Fail instead of interpreting incomplete output.
	if result.StdoutTruncated || result.StderrTruncated ||
		len(result.Stdout) > infoplist.MaxBytes || len(result.Stderr) > signingResignToolOutputLimit {
		return nil, fmt.Errorf("signed entitlements output exceeds the size limit")
	}
	data := bytes.TrimSpace(result.Stdout)
	if len(data) == 0 {
		data = extractPlistDocument(result.Stderr)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	if len(data) > infoplist.MaxBytes {
		return nil, fmt.Errorf("signed entitlements exceed %d bytes", infoplist.MaxBytes)
	}
	if err := infoplist.ValidateStructure(data); err != nil {
		return nil, fmt.Errorf("validate signed entitlements: %w", err)
	}
	var entitlements map[string]any
	if _, err := plist.Unmarshal(data, &entitlements); err != nil {
		return nil, fmt.Errorf("decode signed entitlements: %w", err)
	}
	if entitlements == nil {
		entitlements = map[string]any{}
	}
	return entitlements, nil
}

func isUnsignedSigningResignCodeObject(stderr []byte) bool {
	for _, rawLine := range strings.Split(strings.ToLower(string(stderr)), "\n") {
		line := strings.TrimSpace(rawLine)
		const diagnostic = "code object is not signed at all"
		if line == diagnostic || strings.HasSuffix(line, ": "+diagnostic) {
			return true
		}
	}
	return false
}

func signSigningResignObject(ctx context.Context, pathValue, identitySHA1, keychainPath, entitlementsPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := []string{"--force", "--sign", identitySHA1, "--keychain", keychainPath, "--timestamp=none"}
	if entitlementsPath != "" {
		args = append(args, "--entitlements", entitlementsPath)
	}
	args = append(args, pathValue)
	if _, err := runSigningResignToolFn(ctx, "/usr/bin/codesign", args...); err != nil {
		return fmt.Errorf("sign code object: %w", err)
	}
	return nil
}

func verifySigningResignObject(ctx context.Context, pathValue, teamID string, deep bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	args := []string{"--verify", "--strict", "--all-architectures"}
	if deep {
		args = append(args, "--deep")
	}
	teamRequirement := `anchor apple generic and certificate leaf[subject.OU] = "` + teamID + `"`
	args = append(args, "-R="+teamRequirement, pathValue)
	if _, err := runSigningResignToolFn(ctx, "/usr/bin/codesign", args...); err != nil {
		return fmt.Errorf("verify code signature: %w", err)
	}
	return nil
}
