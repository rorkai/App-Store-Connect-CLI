package xcode

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestSimctlListJSONUsesOnlyList(t *testing.T) {
	if runtimeGOOS != "darwin" {
		t.Skip("simctl list is macOS only")
	}
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

func TestSimctlListJSONRefusesOtherPlatforms(t *testing.T) {
	previous := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = previous })
	if _, err := SimctlListJSON(context.Background()); err == nil || !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("error = %v", err)
	}
}
