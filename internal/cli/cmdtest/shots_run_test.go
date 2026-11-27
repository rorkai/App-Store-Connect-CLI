package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestShotsRun_FailureReceipt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
	binDir := t.TempDir()
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
printf '%s\n' '{"devices":{"runtime":[{"udid":"fixture-udid","state":"Booted","isAvailable":true}]}}'
`)
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
printf '%s\n' "$1" >> "$AXE_CALL_LOG"
if [ "$1" = "type" ]; then
  echo 'fixture typing failure' >&2
  exit 7
fi
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	const failingSteps = `[{"action":"tap","label":"Ready"},{"action":"type","text":"hello"},{"action":"tap","label":"Done"}]`
	for _, test := range []struct {
		name          string
		steps         string
		wantReceipt   bool
		outputFailure bool
		wantError     string
	}{
		{"execution", failingSteps, true, false, "step 2 (type): axe: exit status 7 (output: fixture typing failure)"},
		{"output failure", failingSteps, true, true, "step 2 (type): axe: exit status 7 (output: fixture typing failure)"},
		{"validation", `[{"action":"tap","label":"Ready"},{"action":"type"}]`, false, false, "steps[2] type requires text"},
	} {
		t.Run(test.name, func(t *testing.T) {
			callLog := filepath.Join(t.TempDir(), "calls")
			t.Setenv("AXE_CALL_LOG", callLog)
			planPath := filepath.Join(t.TempDir(), "plan.json")
			writeFile(t, planPath, `{"version":1,"app":{"bundle_id":"com.example.app","udid":"fixture-udid"},"steps":`+test.steps+`}`)
			d, stdoutBuf, stderrBuf := newTestDeps(t, nil)
			if test.outputFailure {
				reader, writer := io.Pipe()
				_ = reader.Close()
				t.Cleanup(func() { _ = writer.Close() })
				d.IO.Out = writer
			}
			code := rootcmd.RunWithDeps([]string{
				"screenshots", "run", "--plan", planPath,
				"--output-dir", filepath.Join(t.TempDir(), "output"), "--output", "json",
			}, "1.2.3", d)
			stdout, stderr := stdoutBuf.String(), stderrBuf.String()
			if code != rootcmd.ExitError || !strings.Contains(stderr, "Error: screenshots run: "+test.wantError) {
				t.Fatalf("exit = %d, stderr = %q; want failure with %q", code, stderr, test.wantError)
			}
			calls, err := os.ReadFile(callLog)
			if !test.wantReceipt {
				if stdout != "" || !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("validation stdout = %q, calls = %q, log error = %v; want no receipt or device calls", stdout, calls, err)
				}
				return
			}
			if err != nil || string(calls) != "tap\ntype\n" {
				t.Fatalf("calls = %q, error = %v; want execution to stop at the failed step", calls, err)
			}
			if test.outputFailure {
				if !strings.Contains(stderr, io.ErrClosedPipe.Error()) {
					t.Fatalf("stderr = %q, want both execution and output errors", stderr)
				}
				return
			}
			var result screenshots.RunResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("decode partial receipt: %v; stdout = %q", err, stdout)
			}
			if len(result.Steps) != 2 || result.Steps[0].Index != 1 || result.Steps[0].Action != "tap" || result.Steps[0].Status != "ok" ||
				result.Steps[1].Index != 2 || result.Steps[1].Action != "type" || result.Steps[1].Status != "error" ||
				result.Steps[1].Error != "axe: exit status 7 (output: fixture typing failure)" {
				t.Fatalf("partial receipt steps = %+v, want completed tap and failed type with original cause", result.Steps)
			}
		})
	}
}

func TestShotsRun_MissingPlanFile(t *testing.T) {
	root, _, rootStderr := shotsRootWithDeps(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	if err := root.Parse([]string{"screenshots", "run", "--plan", missing}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for missing plan file")
	}
	if !errors.Is(err, screenshots.ErrPlanRead) {
		t.Fatalf("expected ErrPlanRead, got: %v", err)
	}
	stderr := rootStderr.String()

	if stderr != "" {
		t.Fatalf("expected empty stderr for runtime error, got %q", stderr)
	}
}

func TestShotsRun_InvalidPlanJSON(t *testing.T) {
	root, _, rootStderr := shotsRootWithDeps(t)
	path := filepath.Join(t.TempDir(), "invalid.json")
	writeFile(t, path, "{invalid")

	if err := root.Parse([]string{"screenshots", "run", "--plan", path}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid plan")
	}
	if !errors.Is(err, screenshots.ErrPlanParseJSON) {
		t.Fatalf("expected ErrPlanParseJSON, got: %v", err)
	}
	stderr := rootStderr.String()

	if stderr != "" {
		t.Fatalf("expected empty stderr for runtime error, got %q", stderr)
	}
}

func TestShotsRunPreservesWhitespacePlanPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows trims trailing spaces from path components")
	}
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
	planPath := filepath.Join(t.TempDir(), "screenshots.json ")
	writeFile(t, planPath, `{
  "version": 1,
  "app": {"bundle_id": "com.example.app", "output_dir": "`+filepath.Join(t.TempDir(), "raw")+`"},
  "steps": [{"action": "wait", "duration_ms": 1}]
}`)

	root, rootStdout, rootStderr := shotsRootWithDeps(t)
	if err := root.Parse([]string{"screenshots", "run", "--plan", planPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	stdout, stderr := rootStdout.String(), rootStderr.String()
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"bundle_id":"com.example.app"`) {
		t.Fatalf("expected structured run output from literal plan path, got %q", stdout)
	}
}

func TestShotsRun_ValidWaitPlan_WithJSONCComments(t *testing.T) {
	root, rootStdout, rootStderr := shotsRootWithDeps(t)
	outDir := filepath.Join(t.TempDir(), "shots")
	planPath := filepath.Join(t.TempDir(), "screenshots.json")
	writeFile(t, planPath, `{
  // JSONC comments are allowed in screenshot plans.
  "version": 1,
  "app": {
    "bundle_id": "com.example.app",
    "output_dir": "`+outDir+`" // where PNGs go
  },
  "steps": [
    { "action": "wait", "duration_ms": 1 } // no external dependencies
  ]
}`)

	if err := root.Parse([]string{"screenshots", "run", "--plan", planPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	stdout, stderr := rootStdout.String(), rootStderr.String()

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		BundleID string `json:"bundle_id"`
		UDID     string `json:"udid"`
		Steps    []struct {
			Action string `json:"action"`
			Status string `json:"status"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout JSON: %v\nstdout=%q", err, stdout)
	}
	if result.BundleID != "com.example.app" {
		t.Fatalf("unexpected bundle_id: %q", result.BundleID)
	}
	if result.UDID != "booted" {
		t.Fatalf("expected default udid booted, got %q", result.UDID)
	}
	if len(result.Steps) != 1 || result.Steps[0].Action != "wait" || result.Steps[0].Status != "ok" {
		t.Fatalf("unexpected step results: %+v", result.Steps)
	}
}

func TestShotsRun_ValidWaitPlan(t *testing.T) {
	root, rootStdout, rootStderr := shotsRootWithDeps(t)
	outDir := filepath.Join(t.TempDir(), "shots")
	planPath := filepath.Join(t.TempDir(), "screenshots.json")
	writeFile(t, planPath, `{
  "version": 1,
  "app": {
    "bundle_id": "com.example.app",
    "output_dir": "`+outDir+`"
  },
  "steps": [
    { "action": "wait", "duration_ms": 1 }
  ]
}`)

	if err := root.Parse([]string{"screenshots", "run", "--plan", planPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	stdout, stderr := rootStdout.String(), rootStderr.String()

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		BundleID string `json:"bundle_id"`
		UDID     string `json:"udid"`
		Steps    []struct {
			Action string `json:"action"`
			Status string `json:"status"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout JSON: %v\nstdout=%q", err, stdout)
	}
	if result.BundleID != "com.example.app" {
		t.Fatalf("unexpected bundle_id: %q", result.BundleID)
	}
	if result.UDID != "booted" {
		t.Fatalf("expected default udid booted, got %q", result.UDID)
	}
	if len(result.Steps) != 1 || result.Steps[0].Action != "wait" || result.Steps[0].Status != "ok" {
		t.Fatalf("unexpected step results: %+v", result.Steps)
	}
}

func TestShotsRun_BundleIDOverrideAppliedForPlanMissingBundleID(t *testing.T) {
	root, rootStdout, rootStderr := shotsRootWithDeps(t)
	outDir := filepath.Join(t.TempDir(), "shots")
	planPath := filepath.Join(t.TempDir(), "screenshots.json")
	writeFile(t, planPath, `{
  "version": 1,
  "app": {
    "output_dir": "`+outDir+`"
  },
  "steps": [
    { "action": "wait", "duration_ms": 1 }
  ]
}`)

	if err := root.Parse([]string{"screenshots", "run", "--plan", planPath, "--bundle-id", "com.override.app", "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	stdout, stderr := rootStdout.String(), rootStderr.String()

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result struct {
		BundleID string `json:"bundle_id"`
		Steps    []struct {
			Action string `json:"action"`
			Status string `json:"status"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout JSON: %v\nstdout=%q", err, stdout)
	}
	if result.BundleID != "com.override.app" {
		t.Fatalf("expected overridden bundle_id, got %q", result.BundleID)
	}
	if len(result.Steps) != 1 || result.Steps[0].Action != "wait" || result.Steps[0].Status != "ok" {
		t.Fatalf("unexpected step results: %+v", result.Steps)
	}
}

// shotsRootWithDeps returns a root command built on test deps without an App
// Store Connect client, and the deps' stdout and stderr buffers.
func shotsRootWithDeps(t *testing.T) (*ffcli.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()

	d, stdout, stderr := newTestDeps(t, nil)
	return rootcmd.RootCommandWithDeps("1.2.3", d), stdout, stderr
}
