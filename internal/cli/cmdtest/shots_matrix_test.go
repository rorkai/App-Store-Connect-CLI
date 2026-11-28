package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestShotsMatrix_Registered(t *testing.T) {
	root, _, rootStderr := shotsRootWithDeps(t)
	missing := filepath.Join(t.TempDir(), "matrix.json")
	if err := root.Parse([]string{"screenshots", "matrix", "--plan", missing}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); !errors.Is(err, screenshots.ErrMatrixPlanRead) {
		t.Fatalf("expected missing matrix plan error, got %v", err)
	}
	stderr := rootStderr.String()
	if !strings.Contains(stderr, "Error:") {
		t.Fatalf("expected a usage diagnostic, got %q", stderr)
	}
}

func TestShotsMatrix_InvalidConcurrencyIsUsageError(t *testing.T) {
	root, _, rootStderr := shotsRootWithDeps(t)
	if err := root.Parse([]string{"screenshots", "matrix", "--plan", filepath.Join(t.TempDir(), "matrix.json"), "--max-concurrency", "9"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if !isUsageClassError(err) {
		t.Fatalf("expected usage error, got %v", err)
	}
	stderr := rootStderr.String()
	if !strings.Contains(stderr, "--max-concurrency must be between 1 and 8") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestShotsMatrix_LocalFakeRunProducesResultAndReview(t *testing.T) {
	binDir := t.TempDir()
	templatePNG := filepath.Join(t.TempDir(), "template.png")
	writeShotsMatrixPNG(t, templatePNG)
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
test -s "$MATRIX_PROGRESS_PATH"
if [ "$1" = "simctl" ] && [ "$2" = "list" ]; then
  printf '%s\n' '{"devices":{"runtime":[{"udid":"SIM-UDID","state":"Booted","isAvailable":true}]}}'
  exit 0
fi
if [ "$1" = "simctl" ] && [ "$2" = "ui" ] && [ "$4" = "appearance" ]; then
  printf '%s\n' light
  exit 0
fi
exit 0
`)
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "axe"), `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; break; fi
  shift
done
cp "$AXE_TEMPLATE_PNG" "$out"
`)
	t.Setenv("AXE_TEMPLATE_PNG", templatePNG)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	progressPath := filepath.Join(dir, "progress.jsonl")
	t.Setenv("MATRIX_PROGRESS_PATH", progressPath)
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	rawDir := filepath.Join(dir, "raw")
	reviewDir := filepath.Join(dir, "review")
	writeShotsMatrixFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"launch"},{"action":"screenshot","name":"home"}]}`)
	writeShotsMatrixFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"`+rawDir+`","review_dir":"`+reviewDir+`"}}`)

	root, rootStdout, rootStderr := shotsRootWithDeps(t)
	if err := root.Parse([]string{"screenshots", "matrix", "--plan", matrixPath, "--output", "json", "--progress-file", progressPath}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := root.Run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
	stdout, stderr := rootStdout.String(), rootStderr.String()
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	var result struct {
		Status     string                        `json:"status"`
		TotalCells int                           `json:"totalCells"`
		Cells      []struct{ ID, Status string } `json:"cells"`
		Review     struct {
			ManifestPath string `json:"manifestPath"`
			HTMLPath     string `json:"htmlPath"`
		} `json:"review"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v\nstdout=%s", err, stdout)
	}
	if result.Status != "success" || result.TotalCells != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Contains(stdout, "total_cells") || strings.Contains(stdout, "manifest_path") {
		t.Fatalf("matrix output contains legacy snake_case fields: %s", stdout)
	}
	for _, path := range []string{result.Review.ManifestPath, result.Review.HTMLPath, filepath.Join(rawDir, "en-US", "phone", "light", "default", "home.png")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %q: %v", path, err)
		}
	}
	data, err := os.ReadFile(progressPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	wantEvents := []string{"run_started", "attempt_started", "attempt_finished", "cell_finished", "run_finished"}
	if len(lines) != len(wantEvents) {
		t.Fatalf("progress events = %s", data)
	}
	for i, line := range lines {
		var event struct {
			Event, CellID, Status string
			Attempt               int
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event != wantEvents[i] {
			t.Fatalf("event %d = %+v", i, event)
		}
		if i > 0 && i < len(lines)-1 && event.CellID != result.Cells[0].ID {
			t.Fatalf("event ID = %q, cell = %q", event.CellID, result.Cells[0].ID)
		}
		if i == len(lines)-1 && event.Status != result.Status {
			t.Fatalf("final event = %+v, result = %+v", event, result)
		}
	}
	if strings.Contains(string(data), "SIM-UDID") || strings.Contains(string(data), "AppleLocale") {
		t.Fatalf("progress leaked execution details: %s", data)
	}
	manifest, err := os.ReadFile(result.Review.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if strings.Contains(string(manifest), "SIM-UDID") || strings.Contains(string(manifest), "AppleLocale") {
		t.Fatalf("manifest leaked execution details: %s", manifest)
	}
}

func TestShotsMatrix_OutputRootFailureStillPrintsReviewResult(t *testing.T) {
	binDir := t.TempDir()
	writeShotsMatrixExecutable(t, filepath.Join(binDir, "xcrun"), `#!/bin/sh
set -eu
printf '%s\n' '{"devices":{}}'
`)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	dir := t.TempDir()
	basePath := filepath.Join(dir, "base.json")
	matrixPath := filepath.Join(dir, "matrix.json")
	rawPath := filepath.Join(dir, "raw-file")
	reviewDir := filepath.Join(dir, "review")
	writeShotsMatrixFile(t, basePath, `{"version":1,"app":{"bundle_id":"com.example.app"},"steps":[{"action":"screenshot","name":"home"}]}`)
	writeShotsMatrixFile(t, rawPath, "not a directory")
	writeShotsMatrixFile(t, matrixPath, `{"version":1,"base_plan":"base.json","devices":[{"id":"phone","udid":"SIM-UDID"}],"locales":["en-US"],"appearances":["light"],"content_variants":[{"id":"default"}],"output":{"raw_dir":"`+rawPath+`","review_dir":"`+reviewDir+`"}}`)

	root, rootStdout, _ := shotsRootWithDeps(t)
	if err := root.Parse([]string{"screenshots", "matrix", "--plan", matrixPath, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	stdout := rootStdout.String()
	if runErr == nil {
		t.Fatal("root.Run() error = nil, want output-root failure")
	}
	var result struct {
		Status string `json:"status"`
		Review *struct {
			ManifestPath string `json:"manifestPath"`
			HTMLPath     string `json:"htmlPath"`
		} `json:"review"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal structured result: %v\nstdout=%s", err, stdout)
	}
	if result.Status != "failed" || result.Review == nil {
		t.Fatalf("structured result = %+v, want failed result with review", result)
	}
	for _, path := range []string{result.Review.ManifestPath, result.Review.HTMLPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("review artifact %q missing after output-root failure: %v", path, err)
		}
	}
}

func writeShotsMatrixFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeShotsMatrixExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func writeShotsMatrixPNG(t *testing.T, path string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create PNG: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
}
