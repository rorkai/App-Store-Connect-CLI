package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestShotsCapture_RequiredFlagErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing bundle-id",
			args:    []string{"screenshots", "capture", "--name", "home"},
			wantErr: "--bundle-id is required",
		},
		{
			name:    "missing name",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app"},
			wantErr: "--name is required",
		},
		{
			name:    "invalid provider",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "home", "--provider", "invalid"},
			wantErr: "--provider must be",
		},
		{
			name:    "simctl is not a valid provider",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "home", "--provider", "simctl"},
			wantErr: "--provider must be",
		},
		{
			name:    "name cannot contain path separators",
			args:    []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "../home"},
			wantErr: "--name must be a file name without path separators",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestShotsCapturePreservesWhitespaceOutputDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows trims trailing spaces from path components")
	}
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	baseDir := t.TempDir()
	fixturePath := filepath.Join(baseDir, "fixture.png")
	writeFramePNG(t, fixturePath, makeRawImage(390, 844))
	outputDir := filepath.Join(baseDir, "raw ")
	binDir := t.TempDir()
	xcrunLog := filepath.Join(t.TempDir(), "xcrun.log")
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
printf '%s\n' "$*" >> "$XCRUN_LOG"
if [ "$1" = "simctl" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","name":"iPhone 17","state":"Booted","isAvailable":true}]}}'
fi
`)
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; shift 2; continue; fi
  shift
done
cp "$CAPTURE_FIXTURE" "$out"
`)
	t.Setenv("CAPTURE_FIXTURE", fixturePath)
	t.Setenv("XCRUN_LOG", xcrunLog)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	stdout, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "capture",
		"--bundle-id", "com.example.app",
		"--name", "home",
		"--output-dir", outputDir,
		"--output", "json",
	}...)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Path string `json:"path"`
		UDID string `json:"udid"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal capture output: %v\nstdout=%q", err, stdout)
	}
	if result.UDID != "SIM-UDID" {
		t.Fatalf("capture udid = %q, want the booted simulator's UDID", result.UDID)
	}
	wantPath := filepath.Join(outputDir, "home.png")
	wantPath, err = filepath.Abs(wantPath)
	if err != nil {
		t.Fatalf("Abs(capture path) error: %v", err)
	}
	if result.Path != wantPath {
		t.Fatalf("capture path = %q, want literal output directory path %q", result.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected captured image at literal path: %v", err)
	}
	calls, err := os.ReadFile(xcrunLog)
	if err != nil || strings.Count(string(calls), "simctl list devices --json") != 1 {
		t.Fatalf("xcrun calls = %q, error = %v; want one inventory lookup per capture", calls, err)
	}
}

func TestShotsCaptureRejectsUnknownSimulatorBeforeLaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
	binDir := t.TempDir()
	callLog := filepath.Join(t.TempDir(), "calls")
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
printf '%s\n' "xcrun $*" >> "$SIMULATOR_CALL_LOG"
printf '%s\n' '{"devices":{}}'
`)
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
printf '%s\n' "axe $*" >> "$SIMULATOR_CALL_LOG"
exit 1
`)
	t.Setenv("PATH", binDir)
	t.Setenv("SIMULATOR_CALL_LOG", callLog)
	stdout, stderr, code := runWithDeps(t, nil, "screenshots", "capture", "--bundle-id", "com.example.app",
		"--udid", "MISSING", "--name", "home", "--output-dir", t.TempDir(), "--output", "json")
	if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, `simulator "MISSING" was not found`) || !strings.Contains(stderr, "xcrun simctl list devices") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q; want an actionable missing simulator error", code, stdout, stderr)
	}
	calls, err := os.ReadFile(callLog)
	if err != nil || string(calls) != "xcrun simctl list devices --json\n" {
		t.Fatalf("calls = %q, error = %v; want inventory only before rejection", calls, err)
	}
}
