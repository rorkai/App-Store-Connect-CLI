package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShotsReviewOpen_DryRun(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	outputDir := filepath.Join(t.TempDir(), "review")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	htmlPath := filepath.Join(outputDir, "index.html")
	if err := os.WriteFile(htmlPath, []byte("<html><body>ok</body></html>"), 0o644); err != nil {
		t.Fatalf("WriteFile(index.html) error: %v", err)
	}

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "review-open",
		"--output-dir", outputDir,
		"--dry-run",
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
		HTMLPath string `json:"html_path"`
		Opened   bool   `json:"opened"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if result.HTMLPath != htmlPath {
		t.Fatalf("html_path=%q, want %q", result.HTMLPath, htmlPath)
	}
	if result.Opened {
		t.Fatal("expected opened=false in dry-run mode")
	}
}

func TestShotsReviewApprove_AllReady(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	outputDir := filepath.Join(t.TempDir(), "review")
	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	manifest := `{
  "generated_at": "2026-01-01T00:00:00Z",
  "framed_dir": "/tmp/framed",
  "output_dir": "/tmp/review",
  "summary": { "total": 2, "ready": 1, "missing_raw": 0, "invalid_size": 1, "approved": 0, "pending_approval": 2 },
  "entries": [
    { "key": "en|iPhone_Air|home", "screenshot_id": "home", "locale": "en", "device": "iPhone_Air", "status": "ready" },
    { "key": "en|iPhone_Air|details", "screenshot_id": "details", "locale": "en", "device": "iPhone_Air", "status": "invalid_size" }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error: %v", err)
	}

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "review-approve",
		"--output-dir", outputDir,
		"--all-ready",
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
		Matched       int      `json:"matched"`
		Added         int      `json:"added"`
		TotalApproved int      `json:"total_approved"`
		Keys          []string `json:"keys"`
		ApprovalPath  string   `json:"approval_path"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if result.Matched != 1 || result.Added != 1 || result.TotalApproved != 1 {
		t.Fatalf("unexpected approve summary: %+v", result)
	}
	if len(result.Keys) != 1 || result.Keys[0] != "en|iPhone_Air|home" {
		t.Fatalf("unexpected approved keys: %+v", result.Keys)
	}

	approvalData, err := os.ReadFile(result.ApprovalPath)
	if err != nil {
		t.Fatalf("ReadFile(approval) error: %v", err)
	}
	if !strings.Contains(string(approvalData), "en|iPhone_Air|home") {
		t.Fatalf("expected key in approvals file, got %q", string(approvalData))
	}
}

func TestShotsReviewApprove_LocaleDeviceSelectors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	outputDir := filepath.Join(t.TempDir(), "review")
	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	manifest := `{
  "generated_at": "2026-01-01T00:00:00Z",
  "framed_dir": "/tmp/framed",
  "output_dir": "/tmp/review",
  "summary": { "total": 2, "ready": 1, "missing_raw": 0, "invalid_size": 1, "approved": 0, "pending_approval": 2 },
  "entries": [
    { "key": "en|iPhone_Air|home", "screenshot_id": "home", "locale": "en", "device": "iPhone_Air", "status": "invalid_size" },
    { "key": "fr|iPhone_Air|home", "screenshot_id": "home", "locale": "fr", "device": "iPhone_Air", "status": "ready" }
  ]
}`
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error: %v", err)
	}

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "review-approve",
		"--output-dir", outputDir,
		"--locale", "en",
		"--device", "iPhone_Air",
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
		Matched int      `json:"matched"`
		Added   int      `json:"added"`
		Keys    []string `json:"keys"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%q", err, stdout)
	}
	if result.Matched != 1 || result.Added != 1 {
		t.Fatalf("unexpected approve summary: %+v", result)
	}
	if len(result.Keys) != 1 || result.Keys[0] != "en|iPhone_Air|home" {
		t.Fatalf("unexpected approved keys: %+v", result.Keys)
	}
}

func TestShotsReviewApprove_RequiresSelector(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"screenshots", "review-approve",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "provide at least one selector") {
		t.Fatalf("expected selector error in stderr, got %q", stderr)
	}
}
