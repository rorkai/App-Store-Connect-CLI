package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShotsFrame_RequiresInput(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "frame"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--input is required when --config is not set") {
		t.Fatalf("expected input required error, got %q", stderr)
	}
}

func TestShotsFrame_RejectsInputAndConfigTogether(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots",
			"frame",
			"--input", "/tmp/raw.png",
			"--config", "/tmp/frame.yaml",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "use either --input or --config, not both") {
		t.Fatalf("expected mutual exclusivity error, got %q", stderr)
	}
}

func TestShotsFrame_InvalidDevice(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots",
			"frame",
			"--input", "/tmp/raw.png",
			"--device", "iphone-se",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--device must be one of") {
		t.Fatalf("expected invalid device error, got %q", stderr)
	}
}

func TestShotsFrame_DefaultDeviceIsIPhoneAir(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(100, 220))
	kouFixturePath := filepath.Join(t.TempDir(), "kou-fixture.png")
	writeFramePNG(t, kouFixturePath, makeRawImage(1320, 2868))
	installMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	outputDir := filepath.Join(t.TempDir(), "framed")
	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--input", rawPath,
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
		Path         string `json:"path"`
		FramePath    string `json:"frame_path"`
		Device       string `json:"device"`
		DisplayType  string `json:"display_type"`
		UploadWidth  int    `json:"upload_width"`
		UploadHeight int    `json:"upload_height"`
		Normalized   bool   `json:"normalized"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}

	if result.Device != "iphone-air" {
		t.Fatalf("expected default device iphone-air, got %q", result.Device)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected output file to exist at %q: %v", result.Path, err)
	}
	if result.FramePath == "" {
		t.Fatalf("expected frame metadata, got empty frame_path")
	}
	if result.DisplayType != "APP_IPHONE_69" {
		t.Fatalf("expected display type APP_IPHONE_69, got %q", result.DisplayType)
	}
	if result.UploadWidth != 1320 || result.UploadHeight != 2868 {
		t.Fatalf("expected upload target 1320x2868, got %dx%d", result.UploadWidth, result.UploadHeight)
	}
	if result.Width != 1320 || result.Height != 2868 {
		t.Fatalf("expected normalized output 1320x2868, got %dx%d", result.Width, result.Height)
	}
	if !result.Normalized {
		t.Fatal("expected normalization to be applied")
	}
}

func TestShotsFrame_ExplicitDeviceIPhone17Pro(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(120, 240))
	kouFixturePath := filepath.Join(t.TempDir(), "kou-fixture.png")
	writeFramePNG(t, kouFixturePath, makeRawImage(1290, 2796))
	installMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", filepath.Join(t.TempDir(), "framed"),
		"--device", "iphone-17-pro",
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
		Device       string `json:"device"`
		DisplayType  string `json:"display_type"`
		UploadWidth  int    `json:"upload_width"`
		UploadHeight int    `json:"upload_height"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}
	if result.Device != "iphone-17-pro" {
		t.Fatalf("expected device iphone-17-pro, got %q", result.Device)
	}
	if result.DisplayType != "APP_IPHONE_67" {
		t.Fatalf("expected display type APP_IPHONE_67, got %q", result.DisplayType)
	}
	if result.UploadWidth != 1290 || result.UploadHeight != 2796 {
		t.Fatalf("expected upload target 1290x2796, got %dx%d", result.UploadWidth, result.UploadHeight)
	}
	if result.Width != 1290 || result.Height != 2796 {
		t.Fatalf("expected normalized output 1290x2796, got %dx%d", result.Width, result.Height)
	}
}

func TestShotsFrame_ConfigOnlyPath(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	kouFixturePath := filepath.Join(t.TempDir(), "kou-fixture.png")
	writeFramePNG(t, kouFixturePath, makeRawImage(1320, 2868))
	installMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	writeFile(t, configPath, `project:
  name: "Demo"
  output_dir: "./out"
  device: "iPhone Air - Light Gold - Portrait"
  output_size: "iPhone6_9"
screenshots:
  framed:
    content:
      - type: "image"
        asset: "screenshots/raw.png"
        frame: true
`)

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--config", configPath,
		"--output-dir", filepath.Join(t.TempDir(), "framed"),
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
		Path   string `json:"path"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected output file to exist at %q: %v", result.Path, err)
	}
	if result.Width != 1320 || result.Height != 2868 {
		t.Fatalf("expected output 1320x2868, got %dx%d", result.Width, result.Height)
	}
}

func TestShotsFrame_ConfigDefaultOutputUsesConfigDeviceInFilename(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	kouFixturePath := filepath.Join(t.TempDir(), "kou-fixture.png")
	writeFramePNG(t, kouFixturePath, makeRawImage(1290, 2796))
	installMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	writeFile(t, configPath, `project:
  name: "Demo"
  output_dir: "./out"
  device: "iPhone 17 Pro - Silver - Portrait"
  output_size: "iPhone6_7"
screenshots:
  framed:
    content:
      - type: "image"
        asset: "screenshots/raw.png"
        frame: true
`)

	outputDir := filepath.Join(t.TempDir(), "framed")
	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--config", configPath,
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
		Path   string `json:"path"`
		Device string `json:"device"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}

	wantPath := filepath.Join(outputDir, "screenshot-iphone-17-pro.png")
	if result.Path != wantPath {
		t.Fatalf("result.Path = %q, want %q", result.Path, wantPath)
	}
	if result.Device != "iphone-17-pro" {
		t.Fatalf("result.Device = %q, want %q", result.Device, "iphone-17-pro")
	}
}

func TestShotsFrame_WatchRequiresConfig(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots",
			"frame",
			"--input", "/tmp/raw.png",
			"--watch",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--watch requires --config") {
		t.Fatalf("expected watch-requires-config error, got %q", stderr)
	}
}

func TestShotsFrame_WatchWithoutInputOrConfig(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots",
			"frame",
			"--watch",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	// Should hit the --input required error first (since no --config means no watch gate).
	if !strings.Contains(stderr, "--input is required") {
		t.Fatalf("expected input required error, got %q", stderr)
	}
}

func installMockKou(t *testing.T, fixturePath, outputPath string) {
	t.Helper()

	binDir := t.TempDir()
	kouPath := filepath.Join(binDir, "kou")
	script := `#!/bin/sh
if [ "$1" = "generate" ]; then
  mkdir -p "$(dirname "$MOCK_KOU_OUTPUT")"
  cp "$MOCK_KOU_FIXTURE" "$MOCK_KOU_OUTPUT"
  printf '[{"name":"framed","path":"%s","success":true}]' "$MOCK_KOU_OUTPUT"
  exit 0
fi
echo "unsupported args" >&2
exit 1
`
	if err := os.WriteFile(kouPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write kou mock script: %v", err)
	}

	t.Setenv("MOCK_KOU_FIXTURE", fixturePath)
	t.Setenv("MOCK_KOU_OUTPUT", outputPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeFramePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error: %v", path, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode(%q) error: %v", path, err)
	}
}

func makeRawImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / max(width, 1)),
				G: uint8((y * 255) / max(height, 1)),
				B: 180,
				A: 255,
			})
		}
	}
	return img
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
