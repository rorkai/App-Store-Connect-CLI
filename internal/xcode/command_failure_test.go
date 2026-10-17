package xcode

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

type fixedCommandOutput string

func (o fixedCommandOutput) String() string                { return string(o) }
func (o fixedCommandOutput) Truncated() bool               { return false }
func (o fixedCommandOutput) TruncationDescription() string { return "" }

func TestFormatCommandOutputErrorMarksOnlyToolExitFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh to produce a real exit status")
	}
	exitErr := exec.Command("/bin/sh", "-c", "exit 65").Run()
	var processExit *exec.ExitError
	if !errors.As(exitErr, &processExit) {
		t.Fatalf("expected an *exec.ExitError, got %T", exitErr)
	}
	tests := []struct {
		name     string
		err      error
		wantTool bool
	}{
		{name: "tool exited non-zero", err: exitErr, wantTool: true},
		{name: "tool could not start", err: &exec.Error{Name: "xcodebuild", Err: exec.ErrNotFound}},
		{name: "exit joined with cleanup failure", err: errors.Join(exitErr, errors.New("kill process group: operation not permitted"))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, preserve := range []bool{false, true} {
				got := formatCommandOutputError(context.Background(), test.err, fixedCommandOutput("error: build failed"), "archive", "xcodebuild", preserve)
				var toolFailure *CommandFailureError
				if errors.As(got, &toolFailure) != test.wantTool {
					t.Fatalf("preserve=%t: CommandFailureError = %t, want %t (%v)", preserve, !test.wantTool, test.wantTool, got)
				}
				if preserve && !errors.Is(got, test.err) {
					t.Fatalf("preserve=%t: %v no longer wraps the process error", preserve, got)
				}
			}
		})
	}
}
