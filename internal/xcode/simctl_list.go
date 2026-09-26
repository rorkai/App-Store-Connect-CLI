package xcode

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	maxSimctlListBytes           = 16 << 20
	maxSimctlListDiagnosticBytes = 8 << 10
)

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
	stopOnOverflow := func() {
		if cmd.Cancel != nil {
			_ = cmd.Cancel()
			return
		}
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	stdout := newSimctlListOutput(maxSimctlListBytes, stopOnOverflow)
	stderr := newSimctlListOutput(maxSimctlListDiagnosticBytes, stopOnOverflow)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	commandErr := runXcodeCommand(cmd)
	var overflowErrors []error
	if stdout.exceeded {
		overflowErrors = append(overflowErrors, fmt.Errorf("simctl list output exceeds %d bytes", maxSimctlListBytes))
	}
	if stderr.exceeded {
		overflowErrors = append(overflowErrors, fmt.Errorf("simctl list diagnostics exceed %d bytes", maxSimctlListDiagnosticBytes))
	}
	if len(overflowErrors) > 0 {
		commandErr = errors.Join(commandErr, errors.Join(overflowErrors...))
	}
	if commandErr != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			if stderr.exceeded {
				detail += " [truncated]"
			}
			return nil, fmt.Errorf("simctl list: %w: diagnostics: %s", commandErr, detail)
		}
		return nil, fmt.Errorf("simctl list: %w", commandErr)
	}
	return append([]byte(nil), stdout.data...), nil
}

// simctlListOutput bounds the captured output and terminates the subprocess as
// soon as either stream exceeds its limit. Returning a truncated prefix while
// allowing simctl to continue would keep a hostile or broken tool alive
// indefinitely, even though its output is no longer useful to the caller.
type simctlListOutput struct {
	data          []byte
	limit         int
	exceeded      bool
	stopOnLimit   func()
	stopRequested bool
}

func newSimctlListOutput(limit int, stopOnLimit func()) *simctlListOutput {
	return &simctlListOutput{limit: limit, stopOnLimit: stopOnLimit}
}

func (output *simctlListOutput) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	remaining := output.limit - len(output.data)
	if remaining <= 0 {
		output.markExceeded()
		return len(data), nil
	}
	if len(data) > remaining {
		output.data = append(output.data, data[:remaining]...)
		output.markExceeded()
		return len(data), nil
	}
	output.data = append(output.data, data...)
	return len(data), nil
}

func (output *simctlListOutput) markExceeded() {
	if output.exceeded {
		return
	}
	output.exceeded = true
	if output.stopRequested {
		return
	}
	output.stopRequested = true
	if output.stopOnLimit != nil {
		output.stopOnLimit()
	}
}

func (output *simctlListOutput) String() string {
	return string(output.data)
}

// ReadTestResultSummary loads the structured summary of an existing .xcresult bundle.
func ReadTestResultSummary(ctx context.Context, resultBundlePath string) (*TestSummary, error) {
	return readTestResultSummary(ctx, resultBundlePath)
}
