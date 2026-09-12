package xcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	trustedXcodeSelectPath = "/usr/bin/xcode-select"
	trustedStaplerPath     = "/usr/bin/stapler"
)

var (
	trustedXcrunPathFn             = resolveTrustedXcrunPath
	trustedXcodeToolPathFn         = resolveTrustedXcodeToolPath
	errTrustedXcodeToolUnavailable = errors.New("trusted Xcode tool is unavailable")
)

// OverrideTrustedXcrunPathForTesting replaces the trusted xcrun resolver for
// repository integration tests that need to exercise real subprocess control.
// The returned cleanup restores the previous resolver. Callers must not use
// this process-global test seam from parallel tests.
func OverrideTrustedXcrunPathForTesting(pathValue string) func() {
	previous := trustedXcrunPathFn
	trustedXcrunPathFn = func() (string, error) { return pathValue, nil }
	return func() { trustedXcrunPathFn = previous }
}

// resolveTrustedXcrunPath returns the system xcrun rather than consulting
// PATH. xcrun is the trust anchor for selecting the rest of the Xcode tools.
func resolveTrustedXcrunPath() (string, error) {
	info, err := statPathFn(trustedXcrunPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("%w: trusted xcrun: %w", errTrustedXcodeToolUnavailable, err)
		}
		return "", fmt.Errorf("trusted xcrun is unavailable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%w: trusted xcrun is not an executable regular file", errTrustedXcodeToolUnavailable)
	}
	return trustedXcrunPath, nil
}

func trustedXcodeCommand(ctx context.Context, tool string, args, environment []string) (*exec.Cmd, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	pathValue, err := trustedXcodeToolPathFn(ctx, tool, environment)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", tool, err)
	}
	return commandContextFn(ctx, pathValue, args...), nil
}

func resolveTrustedXcodeToolPath(ctx context.Context, tool string, environment []string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := contextError(ctx); err != nil {
		return "", err
	}
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return "", errors.New("xcode tool name is empty")
	}
	if tool == "xcrun" {
		return trustedXcrunPathFn()
	}
	switch tool {
	case "xcodebuild", "agvtool", "stapler":
	default:
		return "", fmt.Errorf("unsupported Xcode tool %q", tool)
	}

	xcrunPath, err := trustedXcrunPathFn()
	if err != nil {
		return "", err
	}
	developerDir, err := trustedDeveloperDirectory(ctx, environment)
	if err != nil {
		return "", err
	}
	developerDir, _, _, err = normalizeToolchainDeveloperDir(developerDir)
	if err != nil {
		return "", err
	}

	stdout, stderr, exceeded, err := runTrustedXcrunFind(ctx, xcrunPath, tool, developerDir, environment)
	if err != nil {
		if detail := strings.TrimSpace(stderr); detail != "" {
			return "", fmt.Errorf("xcrun --find %s failed: %s: %w", tool, detail, err)
		}
		return "", fmt.Errorf("xcrun --find %s failed: %w", tool, err)
	}
	if exceeded {
		return "", fmt.Errorf("xcrun --find %s output exceeds %d bytes", tool, toolchainProbeDiagnosticLimit)
	}
	pathValue, err := parseTrustedToolPath(stdout, tool)
	if err != nil {
		return "", fmt.Errorf("xcrun --find %s: %w", tool, err)
	}

	switch tool {
	case "xcodebuild":
		return validateResolvedXcodebuildPath(pathValue, developerDir)
	case "stapler":
		return validateResolvedStaplerPath(pathValue, developerDir)
	default:
		return validateResolvedDeveloperToolPath(pathValue, tool, developerDir)
	}
}

func trustedDeveloperDirectory(ctx context.Context, environment []string) (string, error) {
	if value, ok := environmentValue(environment, "DEVELOPER_DIR"); ok {
		if strings.TrimSpace(value) != "" {
			return filepath.Clean(strings.TrimSpace(value)), nil
		}
	}
	if environment == nil {
		value, _, err := resolveToolchainDeveloperDir(ctx, "")
		return value, err
	}
	// A caller-supplied environment is exact, so do not accidentally select a
	// DEVELOPER_DIR inherited by the current process when it is absent there.
	return activeDeveloperDirFromTrustedSelect(ctx, environment)
}

func activeDeveloperDirFromTrustedSelect(ctx context.Context, environment []string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := commandContextFn(ctx, trustedXcodeSelectPath, "-p")
	if environment != nil {
		// The caller supplied an exact environment. Preserve that boundary for
		// xcode-select as well, so an ambient DEVELOPER_DIR cannot override it.
		cmd.Env = append([]string(nil), environment...)
	}
	output, err := outputXcodeCommand(cmd)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(output))
	if value == "" {
		return "", errors.New("xcode-select returned an empty developer directory")
	}
	return filepath.Clean(value), nil
}

func environmentValue(environment []string, name string) (string, bool) {
	if environment == nil {
		value, ok := os.LookupEnv(name)
		return value, ok
	}
	prefix := name + "="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix), true
		}
	}
	return "", false
}

func runTrustedXcrunFind(ctx context.Context, xcrunPath, tool, developerDir string, environment []string) (stdout, stderr string, exceeded bool, err error) {
	cmd := commandContextFn(ctx, xcrunPath, "--find", tool)
	cmd.Env = toolchainProbeEnvironmentForCommand(environment, cmd.Env, developerDir)
	stdoutBuffer := newBoundedToolOutput(toolchainProbeDiagnosticLimit)
	stderrBuffer := newBoundedToolOutput(toolchainProbeDiagnosticLimit)
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stderrBuffer
	err = runXcodeCommand(cmd)
	if ctxErr := ctx.Err(); err != nil && ctxErr != nil {
		err = ctxErr
	}
	return stdoutBuffer.String(), stderrBuffer.String(), stdoutBuffer.exceeded || stderrBuffer.exceeded, err
}

func toolchainProbeEnvironmentForCommand(environment, inherited []string, developerDir string) []string {
	base := environment
	if base == nil {
		base = inherited
	}
	if base == nil {
		base = os.Environ()
	}
	result := make([]string, 0, len(base)+1)
	for _, entry := range base {
		if strings.HasPrefix(entry, "DEVELOPER_DIR=") {
			continue
		}
		result = append(result, entry)
	}
	return append(result, "DEVELOPER_DIR="+developerDir)
}

func parseTrustedToolPath(output, tool string) (string, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return "", errors.New("resolver returned an empty path")
	}
	if strings.ContainsAny(trimmed, "\x00\r\n") {
		return "", errors.New("resolver returned multiple or invalid paths")
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("resolver returned a non-absolute %s path %q", tool, trimmed)
	}
	return filepath.Clean(trimmed), nil
}

func validateResolvedDeveloperToolPath(pathValue, tool, developerDir string) (string, error) {
	if !strings.EqualFold(filepath.Base(pathValue), tool) {
		return "", fmt.Errorf("xcrun returned an unexpected executable path %q", pathValue)
	}
	canonicalDeveloperDir, err := filepath.EvalSymlinks(developerDir)
	if err != nil {
		return "", fmt.Errorf("resolve selected developer directory symlinks: %w", err)
	}
	canonicalPath, err := filepath.EvalSymlinks(pathValue)
	if err != nil {
		if !pathWithinDirectoryFold(canonicalDeveloperDir, pathValue) {
			return "", fmt.Errorf("xcrun resolved %s outside selected developer directory %q", tool, canonicalDeveloperDir)
		}
		return "", fmt.Errorf("resolved %s path %q is unavailable: %w", tool, pathValue, err)
	}
	contained, err := pathIdentityWithinDirectory(filepath.Clean(canonicalDeveloperDir), filepath.Clean(canonicalPath))
	if err != nil {
		return "", fmt.Errorf("compare %s path with developer directory: %w", tool, err)
	}
	if !contained {
		return "", fmt.Errorf("xcrun resolved %s outside selected developer directory %q", tool, canonicalDeveloperDir)
	}
	info, err := statPathFn(filepath.Clean(canonicalPath))
	if err != nil {
		return "", fmt.Errorf("resolved %s path %q is unavailable: %w", tool, canonicalPath, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("resolved %s path %q is not a regular file", tool, canonicalPath)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("resolved %s path %q is not executable", tool, canonicalPath)
	}
	return pathValue, nil
}

func validateResolvedStaplerPath(pathValue, developerDir string) (string, error) {
	if filepath.Clean(pathValue) == trustedStaplerPath {
		return trustedStaplerPath, nil
	}
	return validateResolvedDeveloperToolPath(pathValue, "stapler", developerDir)
}

func isTrustedXcodeToolUnavailable(err error, _ string) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, exec.ErrNotFound) || errors.Is(err, errTrustedXcodeToolUnavailable)
}

type boundedToolOutput struct {
	data     []byte
	limit    int
	exceeded bool
}

func newBoundedToolOutput(limit int) *boundedToolOutput {
	return &boundedToolOutput{limit: limit}
}

func (b *boundedToolOutput) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	remaining := b.limit - len(b.data)
	if remaining <= 0 {
		b.exceeded = true
		return len(data), nil
	}
	if len(data) > remaining {
		b.data = append(b.data, data[:remaining]...)
		b.exceeded = true
		return len(data), nil
	}
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *boundedToolOutput) String() string {
	return string(b.data)
}
