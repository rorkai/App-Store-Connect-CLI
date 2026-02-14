package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestShotsRun_MissingPlanFile(t *testing.T) {
	root := RootCommand("1.2.3")
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")

	if err := root.Parse([]string{"screenshots", "run", "--plan", missing}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for missing plan file")
		}
		if !errors.Is(err, screenshots.ErrPlanRead) {
			t.Fatalf("expected ErrPlanRead, got: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr for runtime error, got %q", stderr)
	}
}

func TestShotsRun_InvalidPlanJSON(t *testing.T) {
	root := RootCommand("1.2.3")
	path := filepath.Join(t.TempDir(), "invalid.json")
	writeFile(t, path, "{invalid")

	if err := root.Parse([]string{"screenshots", "run", "--plan", path}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for invalid plan")
		}
		if !errors.Is(err, screenshots.ErrPlanParseJSON) {
			t.Fatalf("expected ErrPlanParseJSON, got: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr for runtime error, got %q", stderr)
	}
}

func TestShotsRun_ValidWaitPlan(t *testing.T) {
	root := RootCommand("1.2.3")
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

	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
	root := RootCommand("1.2.3")
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

	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

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
