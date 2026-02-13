package screenshots

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAXeProvider_MissingBinary(t *testing.T) {
	t.Setenv("PATH", "")

	provider := &AXeProvider{}
	_, err := provider.Capture(context.Background(), CaptureRequest{
		UDID:      "booted",
		Name:      "home",
		OutputDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error when axe binary is missing")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("expected exec.ErrNotFound, got %v", err)
	}
}

func TestAXeProvider_CaptureLaunchesRequestedBundleID(t *testing.T) {
	binDir := t.TempDir()
	logDir := t.TempDir()
	xcrunLog := filepath.Join(logDir, "xcrun.log")
	axeLog := filepath.Join(logDir, "axe.log")

	writeExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$XCRUN_LOG"
`)
	writeExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
printf '%s\n' "$*" >> "$AXE_LOG"
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then
    out="$2"
    break
  fi
  shift
done
: > "$out"
`)

	t.Setenv("PATH", binDir)
	t.Setenv("XCRUN_LOG", xcrunLog)
	t.Setenv("AXE_LOG", axeLog)

	provider := &AXeProvider{}
	_, err := provider.Capture(context.Background(), CaptureRequest{
		BundleID:  "com.example.app",
		UDID:      "SIM-UDID-123",
		Name:      "home",
		OutputDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected successful capture, got %v", err)
	}

	xcrunArgs, err := os.ReadFile(xcrunLog)
	if err != nil {
		t.Fatalf("read xcrun log: %v", err)
	}
	if !strings.Contains(string(xcrunArgs), "simctl launch SIM-UDID-123 com.example.app") {
		t.Fatalf("expected xcrun launch args with bundle id, got %q", string(xcrunArgs))
	}

	axeArgs, err := os.ReadFile(axeLog)
	if err != nil {
		t.Fatalf("read axe log: %v", err)
	}
	if !strings.Contains(string(axeArgs), "screenshot") {
		t.Fatalf("expected axe screenshot invocation, got %q", string(axeArgs))
	}
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("write executable %q: %v", path, err)
	}
}
