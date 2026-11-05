package signing

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunSigningResignToolNamesFallbackTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test relies on a POSIX sleep executable")
	}
	output, err := runSigningResignToolWithFallback(context.Background(), 50*time.Millisecond, "/bin/sleep", "1")
	if err == nil {
		t.Fatal("runSigningResignToolWithFallback() error = nil, want fallback timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runSigningResignToolWithFallback() error = %v, want context.DeadlineExceeded", err)
	}
	if !strings.Contains(err.Error(), "sleep timed out after 50ms") {
		t.Fatalf("runSigningResignToolWithFallback() error = %v, want the tool and timeout named", err)
	}
	if len(output.Stdout) != 0 {
		t.Fatalf("runSigningResignToolWithFallback() stdout = %q, want empty", output.Stdout)
	}
}

func TestRunSigningResignToolKeepsCallerCancellationBare(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test relies on a POSIX sleep executable")
	}
	callerContext, cancel := context.WithCancel(context.Background())
	timer := time.AfterFunc(50*time.Millisecond, cancel)
	defer timer.Stop()
	defer cancel()
	_, err := runSigningResignToolWithFallback(callerContext, time.Minute, "/bin/sleep", "1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runSigningResignToolWithFallback() error = %v, want context.Canceled", err)
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("runSigningResignToolWithFallback() error = %v, want caller cancellation without a timeout label", err)
	}
}

func TestRunSigningResignToolRetainsChildExitCause(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test relies on a POSIX shell executable")
	}
	_, err := runSigningResignToolWithFallback(context.Background(), time.Minute, "/bin/sh", "-c", "exit 7")
	if err == nil {
		t.Fatal("runSigningResignToolWithFallback() error = nil, want child failure")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("runSigningResignToolWithFallback() error = %v, want *exec.ExitError cause retained", err)
	}
}

func TestBoundedOutputBufferReportsTruncation(t *testing.T) {
	buffer := &boundedOutputBuffer{limit: 4}
	if _, err := buffer.Write([]byte("12345")); err != nil {
		t.Fatal(err)
	}
	if got := string(buffer.Bytes()); got != "1234" {
		t.Fatalf("buffer.Bytes() = %q, want capped capture", got)
	}
	if !buffer.Truncated() {
		t.Fatal("buffer.Truncated() = false, want overflow reported")
	}
}

func TestReadSigningResignEntitlementsFailsClosedOnTruncatedToolOutput(t *testing.T) {
	original := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = original })
	for _, test := range []struct {
		name   string
		output signingResignToolOutput
		err    error
	}{
		{name: "truncated stdout", output: signingResignToolOutput{StdoutTruncated: true}},
		{name: "truncated stderr", output: signingResignToolOutput{StderrTruncated: true}},
		{
			name: "truncated stderr cannot prove an unsigned object",
			output: signingResignToolOutput{
				Stderr:          []byte("code object is not signed at all"),
				StderrTruncated: true,
			},
			err: errors.New("codesign failed"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
				return test.output, test.err
			}
			entitlements, err := readSigningResignEntitlements(context.Background(), "/staged/App")
			if err == nil {
				t.Fatalf("readSigningResignEntitlements() = %v, want truncation failure instead of empty claims", entitlements)
			}
		})
	}
}

func TestReadSigningResignEntitlementsDoesNotTrustPathTextAsUnsignedDiagnostic(t *testing.T) {
	original := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = original })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{
			Stderr: []byte("codesign failed while inspecting /tmp/name: code object is not signed"),
		}, errors.New("codesign failed")
	}
	if _, err := readSigningResignEntitlements(context.Background(), "/tmp/name: code object is not signed"); err == nil {
		t.Fatal("readSigningResignEntitlements() treated attacker-controlled path text as an unsigned diagnostic")
	}
}

func TestReadSigningResignEntitlementsAcceptsExactUnsignedDiagnostic(t *testing.T) {
	original := runSigningResignToolFn
	t.Cleanup(func() { runSigningResignToolFn = original })
	runSigningResignToolFn = func(context.Context, string, ...string) (signingResignToolOutput, error) {
		return signingResignToolOutput{
			Stderr: []byte("/tmp/App: code object is not signed at all"),
		}, errors.New("codesign failed")
	}
	entitlements, err := readSigningResignEntitlements(context.Background(), "/tmp/App")
	if err != nil {
		t.Fatalf("readSigningResignEntitlements() error = %v, want unsigned object accepted", err)
	}
	if len(entitlements) != 0 {
		t.Fatalf("readSigningResignEntitlements() = %#v, want empty claims", entitlements)
	}
}
