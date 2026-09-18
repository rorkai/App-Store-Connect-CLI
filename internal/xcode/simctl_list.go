package xcode

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

const maxSimctlListBytes = 16 << 20

// SimctlListJSON runs `xcrun simctl list -j` and returns the JSON body.
// The argument list is fixed so callers cannot turn this into a mutating simctl invocation.
func SimctlListJSON(ctx context.Context) ([]byte, error) {
	if runtimeGOOS != "darwin" {
		return nil, fmt.Errorf("supported on macOS only; current platform is %s", runtimeGOOS)
	}
	cmd, err := trustedXcodeCommand(ctx, "xcrun", []string{"simctl", "list", "-j"}, nil)
	if err != nil {
		return nil, fmt.Errorf("locate xcrun: %w", err)
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := runXcodeCommand(cmd); err != nil {
		return nil, fmt.Errorf("simctl list: %w", err)
	}
	if stdout.Len() > maxSimctlListBytes {
		return nil, fmt.Errorf("simctl list output exceeds %d bytes", maxSimctlListBytes)
	}
	return stdout.Bytes(), nil
}

// ReadTestResultSummary loads the structured summary of an existing .xcresult bundle.
func ReadTestResultSummary(ctx context.Context, resultBundlePath string) (*TestSummary, error) {
	return readTestResultSummary(ctx, resultBundlePath)
}
