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
		if err := root.Parse([]string{"shots", "frame"}); err != nil {
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
	if !strings.Contains(stderr, "--input is required") {
		t.Fatalf("expected input required error, got %q", stderr)
	}
}

func TestShotsFrame_InvalidDevice(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"shots",
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
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(100, 220))

	framePath := filepath.Join(
		homeDir,
		".asc", "frames", "apple", "iphone-air", "png",
		"iPhone Air - Light Gold - Portrait.png",
	)
	writeFramePNG(t, framePath, makeFrameImage(180, 360))

	outputDir := filepath.Join(t.TempDir(), "framed")
	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"shots", "frame",
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
		Path      string `json:"path"`
		FramePath string `json:"frame_path"`
		Device    string `json:"device"`
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
	if !strings.Contains(result.FramePath, "iphone-air") {
		t.Fatalf("expected air frame path, got %q", result.FramePath)
	}
}

func TestShotsFrame_ExplicitDeviceIPhone17Pro(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(120, 240))

	framePath := filepath.Join(
		homeDir,
		".asc", "frames", "apple", "iphone-17-pro", "png",
		"iPhone 17 Pro - Silver - Portrait.png",
	)
	writeFramePNG(t, framePath, makeFrameImage(200, 400))

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"shots", "frame",
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
		Device string `json:"device"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}
	if result.Device != "iphone-17-pro" {
		t.Fatalf("expected device iphone-17-pro, got %q", result.Device)
	}
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

func makeFrameImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Opaque bezel.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 12, G: 12, B: 12, A: 255})
		}
	}

	// Transparent inner screen cutout.
	left := width / 6
	right := width - left
	top := height / 8
	bottom := height - top
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 0})
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
