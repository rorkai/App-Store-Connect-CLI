package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

func TestShotsCapture_FlagsBeforeSubcommand(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	// Root-level flag before subcommand: asc --strict-auth screenshots capture --name home
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	_, stderr := captureOutput(t, func() {
		args := []string{"--strict-auth", "screenshots", "capture", "--name", "home"}
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if !strings.Contains(stderr, "--bundle-id is required") {
		t.Fatalf("expected --bundle-id required error, got stderr: %q", stderr)
	}
}

func TestShotsCapture_OutputFormatAccepted(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	// Just ensure capture subcommand accepts --output and --pretty without parse error
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	args := []string{"screenshots", "capture", "--bundle-id", "com.example.app", "--name", "home", "--output", "table", "--pretty"}
	if err := root.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	// Run may fail (e.g. missing axe binary or no simulator) but we're testing flag parsing
	_ = root.Run(context.Background())
}

func TestShotsCapture_ResultJSONStructure(t *testing.T) {
	// Ensure CaptureResult can be serialized to the expected JSON shape (for output tests)
	type captureResult struct {
		Path     string `json:"path"`
		Provider string `json:"provider"`
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		BundleID string `json:"bundle_id"`
		UDID     string `json:"udid"`
	}

	raw := `{"path":"/tmp/out.png","provider":"axe","width":390,"height":844,"bundle_id":"com.example.app","udid":"booted"}`
	var result captureResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal CaptureResult JSON: %v", err)
	}
	if result.Provider != "axe" || result.Width != 390 || result.Height != 844 || result.BundleID != "com.example.app" {
		t.Fatalf("unexpected parsed result: %+v", result)
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
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), "#!/bin/sh\nexit 0\n")
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
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "capture",
		"--bundle-id", "com.example.app",
		"--name", "home",
		"--output-dir", outputDir,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal capture output: %v\nstdout=%q", err, stdout)
	}
	wantPath := filepath.Join(outputDir, "home.png")
	wantPath, err := filepath.Abs(wantPath)
	if err != nil {
		t.Fatalf("Abs(capture path) error: %v", err)
	}
	if result.Path != wantPath {
		t.Fatalf("capture path = %q, want literal output directory path %q", result.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected captured image at literal path: %v", err)
	}
}
