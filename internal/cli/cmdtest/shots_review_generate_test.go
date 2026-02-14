package cmdtest

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShotsReviewGenerate_JSON(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	baseDir := t.TempDir()
	rawDir := filepath.Join(baseDir, "raw")
	framedDir := filepath.Join(baseDir, "framed")
	outputDir := filepath.Join(baseDir, "review")

	writeReviewPNG(t, filepath.Join(rawDir, "home.png"), 1320, 2868)
	writeReviewPNG(t, filepath.Join(framedDir, "en", "iPhone_Air", "home.png"), 1320, 2868)
	writeReviewPNG(t, filepath.Join(framedDir, "en", "iPhone_Air", "details.png"), 1000, 1000)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "approved.json"), []byte(`["en|iPhone_Air|home"]`), 0o644); err != nil {
		t.Fatalf("WriteFile(approved.json) error: %v", err)
	}

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "review-generate",
		"--raw-dir", rawDir,
		"--framed-dir", framedDir,
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
		ManifestPath string `json:"manifest_path"`
		HTMLPath     string `json:"html_path"`
		Total        int    `json:"total"`
		Ready        int    `json:"ready"`
		MissingRaw   int    `json:"missing_raw"`
		InvalidSize  int    `json:"invalid_size"`
		Approved     int    `json:"approved"`
		Pending      int    `json:"pending"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal review output: %v\nstdout=%q", err, stdout)
	}

	if _, err := os.Stat(result.ManifestPath); err != nil {
		t.Fatalf("expected manifest at %q: %v", result.ManifestPath, err)
	}
	if _, err := os.Stat(result.HTMLPath); err != nil {
		t.Fatalf("expected html at %q: %v", result.HTMLPath, err)
	}
	if result.Total != 2 {
		t.Fatalf("total=%d, want 2", result.Total)
	}
	if result.Ready != 1 {
		t.Fatalf("ready=%d, want 1", result.Ready)
	}
	if result.MissingRaw != 1 {
		t.Fatalf("missing_raw=%d, want 1", result.MissingRaw)
	}
	if result.InvalidSize != 1 {
		t.Fatalf("invalid_size=%d, want 1", result.InvalidSize)
	}
	if result.Approved != 1 {
		t.Fatalf("approved=%d, want 1", result.Approved)
	}
	if result.Pending != 1 {
		t.Fatalf("pending=%d, want 1", result.Pending)
	}
}

func TestShotsReviewGenerate_MissingFramedDir(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	missingDir := filepath.Join(t.TempDir(), "missing-framed")
	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "review-generate",
		"--framed-dir", missingDir,
		"--output", "json",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error for missing framed directory")
		}
		if !strings.Contains(err.Error(), "read framed directory") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func writeReviewPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error: %v", path, err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / maxInt(width, 1)),
				G: uint8((y * 255) / maxInt(height, 1)),
				B: 180,
				A: 255,
			})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode(%q) error: %v", path, err)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
