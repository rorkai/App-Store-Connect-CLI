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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shots"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
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
	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.InputPath != rawPath {
			t.Fatalf("req.InputPath = %q, want %q", req.InputPath, rawPath)
		}
		if req.ConfigPath != "" {
			t.Fatalf("req.ConfigPath = %q, want empty", req.ConfigPath)
		}
		if req.Device != "iphone-air" {
			t.Fatalf("req.Device = %q, want %q", req.Device, "iphone-air")
		}
		wantOutputPath := filepath.Join(outputDir, "raw-iphone-air.png")
		if req.OutputPath != wantOutputPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantOutputPath)
		}
		if req.Canvas != nil {
			t.Fatalf("req.Canvas = %+v, want nil", req.Canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			FramePath:    "iPhone Air - Light Gold - Portrait",
			Device:       "iphone-air",
			DisplayType:  "APP_IPHONE_69",
			UploadWidth:  1260,
			UploadHeight: 2736,
			Normalized:   true,
			Width:        1260,
			Height:       2736,
		}), nil
	})

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
	if result.UploadWidth != 1260 || result.UploadHeight != 2736 {
		t.Fatalf("expected upload target 1260x2736, got %dx%d", result.UploadWidth, result.UploadHeight)
	}
	if result.Width != 1260 || result.Height != 2736 {
		t.Fatalf("expected normalized output 1260x2736, got %dx%d", result.Width, result.Height)
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
	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.InputPath != rawPath {
			t.Fatalf("req.InputPath = %q, want %q", req.InputPath, rawPath)
		}
		if req.Device != "iphone-17-pro" {
			t.Fatalf("req.Device = %q, want %q", req.Device, "iphone-17-pro")
		}
		wantOutputPath := filepath.Join(outputDir, "raw-iphone-17-pro.png")
		if req.OutputPath != wantOutputPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantOutputPath)
		}
		if req.Canvas != nil {
			t.Fatalf("req.Canvas = %+v, want nil", req.Canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			FramePath:    "iPhone 17 Pro - Silver - Portrait",
			Device:       "iphone-17-pro",
			DisplayType:  "APP_IPHONE_61",
			UploadWidth:  1206,
			UploadHeight: 2622,
			Width:        1206,
			Height:       2622,
		}), nil
	})

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", outputDir,
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
		FramePath    string `json:"frame_path"`
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
	if result.FramePath != "iPhone 17 Pro - Silver - Portrait" {
		t.Fatalf("expected native iPhone 17 Pro frame, got %q", result.FramePath)
	}
	if result.DisplayType != "APP_IPHONE_61" {
		t.Fatalf("expected display type APP_IPHONE_61, got %q", result.DisplayType)
	}
	if result.UploadWidth != 1206 || result.UploadHeight != 2622 {
		t.Fatalf("expected upload target 1206x2622, got %dx%d", result.UploadWidth, result.UploadHeight)
	}
	if result.Width != 1206 || result.Height != 2622 {
		t.Fatalf("expected normalized output 1206x2622, got %dx%d", result.Width, result.Height)
	}
}

func TestShotsFrame_ConfigOnlyPath(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	writeFile(t, configPath, `project:
  name: "Demo"
  output_dir: "./out"
  device: "iPhone Air - Light Gold - Portrait"
  output_size: "iPhone6_9_alt"
screenshots:
  framed:
    content:
      - type: "image"
        asset: "screenshots/raw.png"
        frame: true
`)

	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.InputPath != "" {
			t.Fatalf("req.InputPath = %q, want empty", req.InputPath)
		}
		if req.ConfigPath != configPath {
			t.Fatalf("req.ConfigPath = %q, want %q", req.ConfigPath, configPath)
		}
		if req.Device != "iphone-air" {
			t.Fatalf("req.Device = %q, want %q", req.Device, "iphone-air")
		}
		wantOutputPath := filepath.Join(outputDir, "screenshot-iphone-air.png")
		if req.OutputPath != wantOutputPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantOutputPath)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			Device: "iphone-air",
			Width:  1260,
			Height: 2736,
		}), nil
	})

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
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected output file to exist at %q: %v", result.Path, err)
	}
	if result.Width != 1260 || result.Height != 2736 {
		t.Fatalf("expected output 1260x2736, got %dx%d", result.Width, result.Height)
	}
}

func TestShotsFrame_ConfigDefaultOutputUsesConfigDeviceInFilename(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	writeFile(t, configPath, `project:
  name: "Demo"
  output_dir: "./out"
  device: "iPhone 17 Pro - Silver - Portrait"
  output_size: "iPhone6_3"
screenshots:
  framed:
    content:
      - type: "image"
        asset: "screenshots/raw.png"
        frame: true
`)

	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.ConfigPath != configPath {
			t.Fatalf("req.ConfigPath = %q, want %q", req.ConfigPath, configPath)
		}
		wantPath := filepath.Join(outputDir, "screenshot-iphone-17-pro.png")
		if req.OutputPath != wantPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantPath)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			Device: "iphone-17-pro",
			Width:  1206,
			Height: 2622,
		}), nil
	})

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

func TestShotsFrame_MacDevice(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(2560, 1600))
	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.InputPath != rawPath {
			t.Fatalf("req.InputPath = %q, want %q", req.InputPath, rawPath)
		}
		if req.Device != "mac" {
			t.Fatalf("req.Device = %q, want %q", req.Device, "mac")
		}
		wantOutputPath := filepath.Join(outputDir, "raw-mac.png")
		if req.OutputPath != wantOutputPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantOutputPath)
		}
		if req.Canvas == nil {
			t.Fatal("expected canvas options for mac device")
		}
		if req.Canvas.Title != "My App" || req.Canvas.Subtitle != "Your tagline" {
			t.Fatalf("req.Canvas = %+v, want title/subtitle set", req.Canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			Device:       "mac",
			DisplayType:  "APP_DESKTOP",
			UploadWidth:  2880,
			UploadHeight: 1800,
			Width:        2880,
			Height:       1800,
		}), nil
	})

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", outputDir,
		"--device", "mac",
		"--title", "My App",
		"--subtitle", "Your tagline",
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
	if result.Device != "mac" {
		t.Fatalf("expected device mac, got %q", result.Device)
	}
	if result.DisplayType != "APP_DESKTOP" {
		t.Fatalf("expected display type APP_DESKTOP, got %q", result.DisplayType)
	}
	if result.UploadWidth != 2880 || result.UploadHeight != 1800 {
		t.Fatalf("expected upload target 2880x1800, got %dx%d", result.UploadWidth, result.UploadHeight)
	}
	if result.Width != 2880 || result.Height != 1800 {
		t.Fatalf("expected output 2880x1800, got %dx%d", result.Width, result.Height)
	}
}

func TestShotsFrame_MacDeviceNoText(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(2560, 1600))
	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.InputPath != rawPath {
			t.Fatalf("req.InputPath = %q, want %q", req.InputPath, rawPath)
		}
		if req.Device != "mac" {
			t.Fatalf("req.Device = %q, want %q", req.Device, "mac")
		}
		wantOutputPath := filepath.Join(outputDir, "raw-mac.png")
		if req.OutputPath != wantOutputPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantOutputPath)
		}
		if req.Canvas != nil {
			t.Fatalf("req.Canvas = %+v, want nil", req.Canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			Device:       "mac",
			DisplayType:  "APP_DESKTOP",
			UploadWidth:  2880,
			UploadHeight: 1800,
		}), nil
	})

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", outputDir,
		"--device", "mac",
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
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}
	if result.Device != "mac" {
		t.Fatalf("expected device mac, got %q", result.Device)
	}
	if result.DisplayType != "APP_DESKTOP" {
		t.Fatalf("expected display type APP_DESKTOP, got %q", result.DisplayType)
	}
	if result.UploadWidth != 2880 || result.UploadHeight != 1800 {
		t.Fatalf("expected upload target 2880x1800, got %dx%d", result.UploadWidth, result.UploadHeight)
	}
}

func TestShotsFrame_MacDeviceSubtitleOnly(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(2560, 1600))
	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.InputPath != rawPath {
			t.Fatalf("req.InputPath = %q, want %q", req.InputPath, rawPath)
		}
		if req.Device != "mac" {
			t.Fatalf("req.Device = %q, want %q", req.Device, "mac")
		}
		wantOutputPath := filepath.Join(outputDir, "raw-mac.png")
		if req.OutputPath != wantOutputPath {
			t.Fatalf("req.OutputPath = %q, want %q", req.OutputPath, wantOutputPath)
		}
		if req.Canvas == nil {
			t.Fatal("expected canvas options for subtitle-only mac frame")
		}
		if req.Canvas.Title != "" {
			t.Fatalf("req.Canvas.Title = %q, want empty", req.Canvas.Title)
		}
		if req.Canvas.Subtitle != "Just a tagline" {
			t.Fatalf("req.Canvas.Subtitle = %q, want %q", req.Canvas.Subtitle, "Just a tagline")
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			Device:      "mac",
			DisplayType: "APP_DESKTOP",
		}), nil
	})

	root := RootCommand("1.2.3")
	if err := root.Parse([]string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", outputDir,
		"--device", "mac",
		"--subtitle", "Just a tagline",
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
		Device      string `json:"device"`
		DisplayType string `json:"display_type"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal frame output: %v\nstdout=%q", err, stdout)
	}
	if result.Device != "mac" {
		t.Fatalf("expected device mac, got %q", result.Device)
	}
	if result.DisplayType != "APP_DESKTOP" {
		t.Fatalf("expected display type APP_DESKTOP, got %q", result.DisplayType)
	}
}

func TestShotsFrame_CanvasFlagsRejectNonCanvasDevice(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "title on iphone",
			args: []string{"screenshots", "frame", "--input", "/tmp/raw.png", "--title", "Hello"},
		},
		{
			name: "bg-color on iphone",
			args: []string{"screenshots", "frame", "--input", "/tmp/raw.png", "--bg-color", "#fff"},
		},
		{
			name: "title-color on iphone",
			args: []string{"screenshots", "frame", "--input", "/tmp/raw.png", "--title-color", "#000"},
		},
		{
			name: "subtitle on iphone",
			args: []string{"screenshots", "frame", "--input", "/tmp/raw.png", "--subtitle", "Tagline"},
		},
		{
			name: "subtitle-color on iphone",
			args: []string{"screenshots", "frame", "--input", "/tmp/raw.png", "--subtitle-color", "#333"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
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
			if !strings.Contains(stderr, "only apply to canvas devices") {
				t.Fatalf("expected canvas device error, got %q", stderr)
			}
		})
	}
}

func TestShotsFrame_CanvasFlagsRejectConfigMode(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots",
			"frame",
			"--config", "/tmp/frame.yaml",
			"--device", "mac",
			"--title", "Hello",
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
	if !strings.Contains(stderr, "cannot be used with --config") {
		t.Fatalf("expected config mode canvas error, got %q", stderr)
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

func installMockFrame(t *testing.T, fn func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error)) {
	t.Helper()

	restore := shots.SetFrameFunc(fn)
	t.Cleanup(restore)
}

func frameResultWithWrittenPNG(t *testing.T, outputPath string, result screenshots.FrameResult) *screenshots.FrameResult {
	t.Helper()

	path := strings.TrimSpace(result.Path)
	if path == "" {
		path = strings.TrimSpace(outputPath)
	}
	if path == "" {
		t.Fatal("expected frame result path")
	}

	width := result.Width
	if width <= 0 {
		width = result.UploadWidth
	}
	if width <= 0 {
		width = 1
	}

	height := result.Height
	if height <= 0 {
		height = result.UploadHeight
	}
	if height <= 0 {
		height = 1
	}

	writeFramePNG(t, path, makeRawImage(width, height))

	result.Path = path
	if result.Width == 0 {
		result.Width = width
	}
	if result.Height == 0 {
		result.Height = height
	}
	return &result
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
