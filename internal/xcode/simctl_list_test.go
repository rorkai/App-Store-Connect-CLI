package xcode

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestSimctlListJSONUsesOnlyList(t *testing.T) {
	previousGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	useLookPathAsTrustedResolver(t)
	t.Cleanup(func() {
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
	})
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcrun", nil }
	var commands [][]string
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		commands = append(commands, append([]string{name}, args...))
		return exec.CommandContext(ctx, "printf", "%s", `{"devices":{}}`)
	}

	if _, err := SimctlListJSON(context.Background()); err != nil {
		t.Fatalf("SimctlListJSON() error = %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("commands = %#v", commands)
	}
	got := strings.Join(commands[0], " ")
	if !strings.Contains(got, "simctl list -j") {
		t.Fatalf("argv = %q", got)
	}
	for _, forbidden := range []string{"boot", "create", "delete", "erase", "shutdown", "spawn", "install", "uninstall"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("argv %q contains mutating verb %s", got, forbidden)
		}
	}
}

func TestSimctlListJSONRejectsOversizedStdout(t *testing.T) {
	previousGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	useTrustedTestCommandNames(t)
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 16777217 /dev/zero")
	}

	if _, err := SimctlListJSON(context.Background()); err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatalf("SimctlListJSON() error = %v, want bounded-output error", err)
	}
}

func TestSimctlListJSONStopsAnUnboundedStdoutProcess(t *testing.T) {
	previousGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	useTrustedTestCommandNames(t)
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "yes")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := SimctlListJSON(ctx)
		done <- err
	}()

	waitForResult := func(timeout time.Duration) (error, bool) {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case err := <-done:
			return err, true
		case <-timer.C:
			return nil, false
		}
	}
	err, completed := waitForResult(2 * time.Second)
	if !completed {
		cancel()
		if _, cleanedUp := waitForResult(time.Second); !cleanedUp {
			t.Fatal("SimctlListJSON() did not terminate after cancellation")
		}
		t.Fatal("SimctlListJSON() did not stop promptly after output overflow")
	}
	if err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatalf("SimctlListJSON() error = %v, want bounded-output error", err)
	}
}

func TestSimctlListJSONPreservesStderrDiagnostics(t *testing.T) {
	previousGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	useTrustedTestCommandNames(t)
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "printf '%s\\n' SIMCTL_FAILURE >&2; exit 65")
	}

	if _, err := SimctlListJSON(context.Background()); err == nil || !strings.Contains(err.Error(), "SIMCTL_FAILURE") {
		t.Fatalf("SimctlListJSON() error = %v, want preserved stderr diagnostics", err)
	}
}

func TestSimctlListJSONBoundsStderrDiagnostics(t *testing.T) {
	previousGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	useTrustedTestCommandNames(t)
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", "head -c 65536 /dev/zero >&2; exit 65")
	}

	_, err := SimctlListJSON(context.Background())
	if err == nil || !strings.Contains(err.Error(), "[truncated]") {
		t.Fatalf("SimctlListJSON() error = %v, want bounded stderr diagnostics", err)
	}
	if len(err.Error()) > maxSimctlListDiagnosticBytes+256 {
		t.Fatalf("error length = %d, want bounded diagnostics", len(err.Error()))
	}
}

func TestSimctlListJSONRejectsOversizedStderrBeforeReturningJSON(t *testing.T) {
	previousGOOS := runtimeGOOS
	runtimeGOOS = "darwin"
	t.Cleanup(func() { runtimeGOOS = previousGOOS })
	originalCommandContext := commandContextFn
	t.Cleanup(func() { commandContextFn = originalCommandContext })
	useTrustedTestCommandNames(t)
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sh", "-c", `printf '%s' '{"devices":{}}'; head -c 65536 /dev/zero >&2`)
	}

	body, err := SimctlListJSON(context.Background())
	if err == nil || !strings.Contains(err.Error(), "diagnostics exceed") {
		t.Fatalf("SimctlListJSON() error = %v, want oversized-diagnostics error", err)
	}
	if body != nil {
		t.Fatalf("SimctlListJSON() body = %d bytes, want no successful JSON result", len(body))
	}
}

func TestSimctlListJSONRefusesOtherPlatforms(t *testing.T) {
	previous := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = previous })
	if _, err := SimctlListJSON(context.Background()); err == nil || !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("error = %v", err)
	}
}
