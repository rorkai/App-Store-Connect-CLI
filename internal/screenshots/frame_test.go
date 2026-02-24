package screenshots

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseFrameDevice_DefaultIsIPhoneAir(t *testing.T) {
	device, err := ParseFrameDevice("")
	if err != nil {
		t.Fatalf("ParseFrameDevice() error = %v", err)
	}
	if device != DefaultFrameDevice() {
		t.Fatalf("expected default device %q, got %q", DefaultFrameDevice(), device)
	}
}

func TestFrameDeviceOptions_DefaultMarked(t *testing.T) {
	options := FrameDeviceOptions()
	if len(options) != len(FrameDeviceValues()) {
		t.Fatalf("expected %d options, got %d", len(FrameDeviceValues()), len(options))
	}

	defaultCount := 0
	for _, option := range options {
		if !option.Default {
			continue
		}
		defaultCount++
		if option.ID != string(DefaultFrameDevice()) {
			t.Fatalf("unexpected default option %q", option.ID)
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly 1 default option, got %d", defaultCount)
	}
}

func TestParseFrameDevice_NormalizesInput(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want FrameDevice
	}{
		{name: "underscores", raw: "iphone_17_pro", want: FrameDeviceIPhone17Pro},
		{name: "spaces mixed case", raw: " iPhone 17 Pro Max ", want: FrameDeviceIPhone17PM},
		{name: "hyphenated", raw: "iphone-16e", want: FrameDeviceIPhone16e},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseFrameDevice(test.raw)
			if err != nil {
				t.Fatalf("ParseFrameDevice(%q) error = %v", test.raw, err)
			}
			if got != test.want {
				t.Fatalf("ParseFrameDevice(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}

func TestParseFrameDevice_InvalidValue(t *testing.T) {
	_, err := ParseFrameDevice("iphone-se")
	if err == nil {
		t.Fatal("expected invalid device error")
	}
	if !strings.Contains(err.Error(), "allowed:") {
		t.Fatalf("expected allowed values in error, got %v", err)
	}
}

func TestResolveKoubouOutputSize(t *testing.T) {
	tests := []struct {
		name       string
		value      any
		wantWidth  int
		wantHeight int
		wantOK     bool
	}{
		{name: "named size", value: "iPhone6_9", wantWidth: 1320, wantHeight: 2868, wantOK: true},
		{name: "custom list", value: []any{1200, 2500}, wantWidth: 1200, wantHeight: 2500, wantOK: true},
		{name: "unknown name", value: "iphone7_2", wantOK: false},
		{name: "invalid list", value: []any{"bad", 2}, wantOK: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			width, height, ok := resolveKoubouOutputSize(test.value)
			if ok != test.wantOK {
				t.Fatalf("ok = %v, want %v", ok, test.wantOK)
			}
			if !ok {
				return
			}
			if width != test.wantWidth || height != test.wantHeight {
				t.Fatalf("dimensions = %dx%d, want %dx%d", width, height, test.wantWidth, test.wantHeight)
			}
		})
	}
}

func TestParseKoubouConfigMetadata(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	config := `project:
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
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	metadata := parseKoubouConfigMetadata(configPath)
	if metadata == nil {
		t.Fatal("expected parsed metadata")
	}
	if metadata.FrameRef != "iPhone 17 Pro - Silver - Portrait" {
		t.Fatalf("unexpected frame ref %q", metadata.FrameRef)
	}
	if metadata.DisplayType != "APP_IPHONE_67" {
		t.Fatalf("unexpected display type %q", metadata.DisplayType)
	}
	if metadata.UploadWidth != 1290 || metadata.UploadHeight != 2796 {
		t.Fatalf("unexpected upload dimensions %dx%d", metadata.UploadWidth, metadata.UploadHeight)
	}
}

func TestSelectGeneratedScreenshot_RelativePath(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "frame.yaml")
	if err := os.WriteFile(configPath, []byte("project: {}"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := selectGeneratedScreenshot(configPath, []koubouGenerateResult{
		{Name: "framed", Path: "output/framed.png", Success: true},
	})
	if err != nil {
		t.Fatalf("selectGeneratedScreenshot() error = %v", err)
	}
	want := filepath.Join(configDir, "output", "framed.png")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestSelectGeneratedScreenshot_RejectsEscapingRelativePath(t *testing.T) {
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "frame.yaml")
	if err := os.WriteFile(configPath, []byte("project: {}"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := selectGeneratedScreenshot(configPath, []koubouGenerateResult{
		{Name: "framed", Path: "../outside.png", Success: true},
	})
	if err == nil {
		t.Fatal("expected error for escaping output path")
	}
	if !strings.Contains(err.Error(), "escapes config directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFrame_ConfigModeReportsDeviceFromConfig(t *testing.T) {
	kouFixturePath := filepath.Join(t.TempDir(), "kou-fixture.png")
	writeFrameTestPNG(t, kouFixturePath, makeFrameTestImage(1290, 2796))
	installFrameTestMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	config := `project:
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
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := Frame(context.Background(), FrameRequest{
		ConfigPath: configPath,
		Device:     string(DefaultFrameDevice()),
	})
	if err != nil {
		t.Fatalf("Frame() error = %v", err)
	}
	if result.Device != string(FrameDeviceIPhone17Pro) {
		t.Fatalf("result.Device = %q, want %q", result.Device, FrameDeviceIPhone17Pro)
	}
}

func TestFrame_InputModeCleansTemporaryKoubouDirectory(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFrameTestPNG(t, rawPath, makeFrameTestImage(200, 300))

	kouFixturePath := filepath.Join(t.TempDir(), "kou-fixture.png")
	writeFrameTestPNG(t, kouFixturePath, makeFrameTestImage(1320, 2868))
	installFrameTestMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	before := listFrameTempWorkDirs(t)
	outputPath := filepath.Join(t.TempDir(), "framed", "home.png")
	result, err := Frame(context.Background(), FrameRequest{
		InputPath:  rawPath,
		OutputPath: outputPath,
		Device:     string(DefaultFrameDevice()),
	})
	if err != nil {
		t.Fatalf("Frame() error = %v", err)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected output file at %q: %v", result.Path, err)
	}

	for _, dir := range listFrameTempWorkDirs(t) {
		if slices.Contains(before, dir) {
			continue
		}
		t.Fatalf("found leaked temporary Koubou directory: %q", dir)
	}
}

func installFrameTestMockKou(t *testing.T, fixturePath, outputPath string) {
	t.Helper()

	binDir := t.TempDir()
	kouPath := filepath.Join(binDir, "kou")
	script := `#!/bin/sh
if [ "$1" = "--version" ]; then
  echo "kou 0.13.0"
  exit 0
fi
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

func writeFrameTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error: %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create(%q) error: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode(%q) error: %v", path, err)
	}
}

func makeFrameTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / max(width, 1)),
				G: uint8((y * 255) / max(height, 1)),
				B: 200,
				A: 255,
			})
		}
	}
	return img
}

func listFrameTempWorkDirs(t *testing.T) []string {
	t.Helper()

	pattern := filepath.Join(os.TempDir(), "asc-shots-kou-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("filepath.Glob(%q) error: %v", pattern, err)
	}
	dirs := make([]string, 0, len(matches))
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil || !info.IsDir() {
			continue
		}
		dirs = append(dirs, match)
	}
	return dirs
}

func TestRunKoubouGenerate_ParsesJSONFromStdoutWhenStderrHasWarnings(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.13.0"
  exit 0
fi
if [ "$1" != "generate" ]; then
  echo "unsupported args" >&2
  exit 1
fi
echo "warning: using fallback font" 1>&2
echo '[{"name":"framed","path":"output/framed.png","success":true,"error":""}]'
`)
	t.Setenv("PATH", binDir)

	results, err := runKoubouGenerate(context.Background(), "frame.yaml")
	if err != nil {
		t.Fatalf("runKoubouGenerate() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success || results[0].Path != "output/framed.png" {
		t.Fatalf("unexpected parsed result: %+v", results[0])
	}
}

func TestRunKoubouGenerate_RejectsUnpinnedKoubouVersion(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.12.0"
  exit 0
fi
if [ "$1" = "generate" ]; then
  echo '[{"name":"framed","path":"output/framed.png","success":true,"error":""}]'
  exit 0
fi
echo "unsupported args" >&2
exit 1
`)
	t.Setenv("PATH", binDir)

	_, err := runKoubouGenerate(context.Background(), "frame.yaml")
	if err == nil {
		t.Fatal("expected version pinning error")
	}
	if !strings.Contains(err.Error(), "unsupported Koubou version 0.12.0") {
		t.Fatalf("expected unsupported version error, got %v", err)
	}
	if !strings.Contains(err.Error(), "0.13.0") {
		t.Fatalf("expected pinned version in error, got %v", err)
	}
}

func TestRunKoubouGenerate_NotFoundIncludesPinnedInstallHint(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := runKoubouGenerate(context.Background(), "frame.yaml")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), "pip install koubou==0.13.0") {
		t.Fatalf("expected pinned install command in error, got %v", err)
	}
}
