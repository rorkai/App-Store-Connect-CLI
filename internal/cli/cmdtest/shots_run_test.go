package cmdtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

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

type screenshotLocationReceipt struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Status    string   `json:"status"`
}

type screenshotLocationOutput struct {
	Location *screenshotLocationReceipt `json:"location"`
}

func TestShotsRunLocationSetup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell executable fixtures require a Unix PATH")
	}
	for _, test := range []struct {
		name       string
		location   string
		coordinate string
		latitude   float64
		longitude  float64
		failSet    bool
		wantError  string
	}{
		{name: "zero coordinates", location: `{"latitude":0,"longitude":0}`, coordinate: "0,0"},
		{name: "signed coordinates", location: `{"latitude":-33.8688,"longitude":151.2093}`, coordinate: "-33.8688,151.2093", latitude: -33.8688, longitude: 151.2093},
		{name: "latitude out of range", location: `{"latitude":91,"longitude":0}`, wantError: "defaults.location.latitude"},
		{name: "missing longitude", location: `{"latitude":0}`, wantError: "defaults.location.longitude"},
		{name: "setup rejected", location: `{"latitude":0,"longitude":0}`, coordinate: "0,0", failSet: true, wantError: "apply simulated location: xcrun: exit status 7 (output: location fixture rejected)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			binDir := t.TempDir()
			callLog := filepath.Join(t.TempDir(), "calls")
			writeFile(t, callLog, "")
			writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
if [ "$1" = "simctl" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","state":"Booted","isAvailable":true}]}}'
  exit 0
fi
printf '%s\n' "$*" >> "$LOCATION_CALL_LOG"
if [ "$2" = "location" ] && [ "$LOCATION_FAIL_SET" = "1" ]; then
  echo 'location fixture rejected' >&2
  exit 7
fi
`)
			t.Setenv("PATH", binDir)
			t.Setenv("LOCATION_CALL_LOG", callLog)
			failSet := "0"
			if test.failSet {
				failSet = "1"
			}
			t.Setenv("LOCATION_FAIL_SET", failSet)
			planPath := filepath.Join(t.TempDir(), "plan.json")
			writeFile(t, planPath, `{"version":1,"app":{"bundle_id":"com.example.app","udid":"SIM-UDID"},"defaults":{"location":`+test.location+`},"steps":[{"action":"launch"}]}`)
			stdout, stderr, code := runWithDeps(t, nil, "screenshots", "run", "--plan", planPath,
				"--output-dir", filepath.Join(t.TempDir(), "output"), "--output", "json")
			wantCalls := ""
			if test.coordinate != "" {
				wantCalls = "simctl location SIM-UDID set " + test.coordinate + "\n"
			}
			if test.wantError != "" {
				if code != rootcmd.ExitError || stdout != "" || !strings.Contains(stderr, test.wantError) {
					t.Fatalf("exit = %d, stdout = %q, stderr = %q; want setup rejection with %q", code, stdout, stderr, test.wantError)
				}
			} else {
				if code != rootcmd.ExitSuccess || stderr != "" {
					t.Fatalf("exit = %d, stderr = %q; want successful setup", code, stderr)
				}
				wantCalls += "simctl launch SIM-UDID com.example.app\n"
				var output screenshotLocationOutput
				if err := json.Unmarshal([]byte(stdout), &output); err != nil {
					t.Fatalf("decode receipt: %v; stdout = %q", err, stdout)
				}
				location := output.Location
				if location == nil || location.Latitude == nil || location.Longitude == nil || location.Status != "applied" || *location.Latitude != test.latitude || *location.Longitude != test.longitude {
					t.Fatalf("location receipt = %+v; want acknowledged coordinates %s", location, test.coordinate)
				}
			}
			calls, err := os.ReadFile(callLog)
			if err != nil || string(calls) != wantCalls {
				t.Fatalf("calls = %q, error = %v; want %q with no implicit location clear", calls, err, wantCalls)
			}
		})
	}
}
