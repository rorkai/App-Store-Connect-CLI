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

	"gopkg.in/yaml.v3"
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

func TestFrameDeviceKoubouSpecs_UsePinnedKoubouFrames(t *testing.T) {
	want := map[FrameDevice]struct {
		frameName   string
		outputSize  string
		displayType string
	}{
		FrameDeviceIPhoneAir:   {frameName: "iPhone Air - Light Gold - Portrait", outputSize: "iPhone6_9_alt", displayType: "APP_IPHONE_69"},
		FrameDeviceIPhone17PM:  {frameName: "iPhone 17 Pro Max - Silver - Portrait", outputSize: "iPhone6_9", displayType: "APP_IPHONE_69"},
		FrameDeviceIPhone17Pro: {frameName: "iPhone 17 Pro - Silver - Portrait", outputSize: "iPhone6_3", displayType: "APP_IPHONE_61"},
		FrameDeviceIPhone17:    {frameName: "iPhone 17 - White - Portrait", outputSize: "iPhone6_3", displayType: "APP_IPHONE_61"},
		FrameDeviceIPhone16e:   {frameName: "iPhone 16 - White - Portrait", outputSize: "iPhone6_1", displayType: "APP_IPHONE_61"},
		FrameDeviceMac:         {frameName: "Mac", outputSize: "AppDesktop_2880", displayType: "APP_DESKTOP"},
	}

	for device, wantSpec := range want {
		spec, ok := frameDeviceKoubouSpecs[device]
		if !ok {
			t.Fatalf("missing Koubou spec for %q", device)
		}
		if spec.FrameName != wantSpec.frameName {
			t.Fatalf("%q FrameName = %q, want %q", device, spec.FrameName, wantSpec.frameName)
		}
		if spec.OutputSize != wantSpec.outputSize {
			t.Fatalf("%q OutputSize = %q, want %q", device, spec.OutputSize, wantSpec.outputSize)
		}
		if spec.DisplayType != wantSpec.displayType {
			t.Fatalf("%q DisplayType = %q, want %q", device, spec.DisplayType, wantSpec.displayType)
		}
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
		{name: "alternate named size", value: "iPhone6_9_alt", wantWidth: 1260, wantHeight: 2736, wantOK: true},
		{name: "iphone 6.3 size", value: "iPhone6_3", wantWidth: 1206, wantHeight: 2622, wantOK: true},
		{name: "desktop named size", value: "AppDesktop_2880", wantWidth: 2880, wantHeight: 1800, wantOK: true},
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

func TestDisplayTypeForDimensions_Mac(t *testing.T) {
	for _, sz := range [][2]int{{1280, 800}, {1440, 900}, {2560, 1600}, {2880, 1800}} {
		displayType, ok := displayTypeForDimensions(sz[0], sz[1])
		if !ok || displayType != "APP_DESKTOP" {
			t.Fatalf("displayTypeForDimensions(%d, %d) = %q, %v; want APP_DESKTOP, true", sz[0], sz[1], displayType, ok)
		}
	}
}

func TestParseKoubouConfigMetadata(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	config := `project:
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
	if metadata.DisplayType != "APP_IPHONE_61" {
		t.Fatalf("unexpected display type %q", metadata.DisplayType)
	}
	if metadata.UploadWidth != 1206 || metadata.UploadHeight != 2622 {
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
	writeFrameTestPNG(t, kouFixturePath, makeFrameTestImage(1206, 2622))
	installFrameTestMockKou(t, kouFixturePath, filepath.Join(t.TempDir(), "kou-out", "framed.png"))

	configPath := filepath.Join(t.TempDir(), "frame.yaml")
	config := `project:
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

func TestResolveFrameDeviceFromConfig_LegacyFallbackAliases(t *testing.T) {
	tests := []struct {
		name     string
		frameRef string
		sizeName string
		want     FrameDevice
	}{
		{name: "iphone air", frameRef: "iPhone 16 Pro - White Titanium - Portrait", sizeName: "iPhone6_9", want: FrameDeviceIPhoneAir},
		{name: "iphone 17 pro max", frameRef: "iPhone 16 Pro Max - White Titanium - Portrait", sizeName: "iPhone6_9", want: FrameDeviceIPhone17PM},
		{name: "iphone 17 pro", frameRef: "iPhone 15 Pro - White Titanium - Portrait", sizeName: "iPhone6_7", want: FrameDeviceIPhone17Pro},
		{name: "iphone 17 teal alias", frameRef: "iPhone 17 - Teal - Portrait", sizeName: "iPhone6_7", want: FrameDeviceIPhone17},
		{name: "iphone 17", frameRef: "iPhone 14 Pro Portrait", sizeName: "iPhone6_7", want: FrameDeviceIPhone17},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "frame.yaml")
			config := `project:
  name: "Demo"
  output_dir: "./out"
  device: "` + tt.frameRef + `"
  output_size: "` + tt.sizeName + `"
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

			got := ResolveFrameDeviceFromConfig(configPath, string(DefaultFrameDevice()))
			if got != string(tt.want) {
				t.Fatalf("ResolveFrameDeviceFromConfig() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFrame_InputModeCleansTemporaryKoubouDirectory(t *testing.T) {
	// Isolate TempDir for this test process so concurrent package tests can't
	// create matching asc-shots-kou-* directories and cause false positives.
	processTmp := t.TempDir()
	t.Setenv("TMPDIR", processTmp)
	t.Setenv("TMP", processTmp)
	t.Setenv("TEMP", processTmp)

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
  echo "kou 0.18.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
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
	defer file.Close()

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

// parsedCanvasConfig is a lightweight struct for inspecting generated Koubou YAML
// in canvas tests without depending on the full koubouDefaultConfig type hierarchy.
type parsedCanvasConfig struct {
	Screenshots map[string]struct {
		Background *struct {
			Colors []string `yaml:"colors"`
		} `yaml:"background"`
		Content []struct {
			Type     string    `yaml:"type"`
			Content  string    `yaml:"content"`
			Position [2]string `yaml:"position"`
			Color    string    `yaml:"color"`
		} `yaml:"content"`
	} `yaml:"screenshots"`
}

func TestCreateDefaultKoubouConfig_CanvasNoText(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFrameTestPNG(t, rawPath, makeFrameTestImage(2560, 1600))

	spec := frameDeviceKoubouSpecs[FrameDeviceMac]
	configPath, metadata, workDir, err := createDefaultKoubouConfig(rawPath, spec, &CanvasOptions{})
	if err != nil {
		t.Fatalf("createDefaultKoubouConfig() error = %v", err)
	}
	defer os.RemoveAll(workDir)

	if metadata.DisplayType != "APP_DESKTOP" {
		t.Fatalf("DisplayType = %q, want APP_DESKTOP", metadata.DisplayType)
	}
	if metadata.UploadWidth != 2880 || metadata.UploadHeight != 1800 {
		t.Fatalf("dimensions = %dx%d, want 2880x1800", metadata.UploadWidth, metadata.UploadHeight)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var cfg parsedCanvasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	framed := cfg.Screenshots["framed"]
	// No text: exactly 1 content item (the image), window centered at 50%.
	if len(framed.Content) != 1 {
		t.Fatalf("expected 1 content item (image only), got %d", len(framed.Content))
	}
	item := framed.Content[0]
	if item.Type != "image" {
		t.Fatalf("expected image item, got %q", item.Type)
	}
	if item.Position[1] != canvasWindowCenterY {
		t.Fatalf("window Y = %q, want %q (center, no text)", item.Position[1], canvasWindowCenterY)
	}
}

func TestCreateDefaultKoubouConfig_CanvasSubtitleOnly(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFrameTestPNG(t, rawPath, makeFrameTestImage(2560, 1600))

	spec := frameDeviceKoubouSpecs[FrameDeviceMac]
	canvas := &CanvasOptions{Subtitle: "Just a tagline"}
	configPath, _, workDir, err := createDefaultKoubouConfig(rawPath, spec, canvas)
	if err != nil {
		t.Fatalf("createDefaultKoubouConfig() error = %v", err)
	}
	defer os.RemoveAll(workDir)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var cfg parsedCanvasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	framed := cfg.Screenshots["framed"]
	// Subtitle only: text item + image item = 2 items total.
	if len(framed.Content) != 2 {
		t.Fatalf("expected 2 content items (subtitle + image), got %d", len(framed.Content))
	}
	subtitleItem := framed.Content[0]
	if subtitleItem.Type != "text" {
		t.Fatalf("expected first item to be text, got %q", subtitleItem.Type)
	}
	if subtitleItem.Content != "Just a tagline" {
		t.Fatalf("subtitle content = %q, want %q", subtitleItem.Content, "Just a tagline")
	}
	// With no title, subtitle uses the solo Y position.
	if subtitleItem.Position[1] != canvasSubtitleSoloY {
		t.Fatalf("subtitle Y = %q, want %q (solo)", subtitleItem.Position[1], canvasSubtitleSoloY)
	}
	imageItem := framed.Content[1]
	// Has text => window is pushed down.
	if imageItem.Position[1] != canvasWindowTextY {
		t.Fatalf("window Y = %q, want %q (text present)", imageItem.Position[1], canvasWindowTextY)
	}
}

func TestCreateDefaultKoubouConfig_CanvasCustomColors(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFrameTestPNG(t, rawPath, makeFrameTestImage(2560, 1600))

	spec := frameDeviceKoubouSpecs[FrameDeviceMac]
	canvas := &CanvasOptions{
		Title:         "My App",
		Subtitle:      "Tagline",
		BGColor:       "#ffffff",
		TitleColor:    "#ff0000",
		SubtitleColor: "#00ff00",
	}
	configPath, _, workDir, err := createDefaultKoubouConfig(rawPath, spec, canvas)
	if err != nil {
		t.Fatalf("createDefaultKoubouConfig() error = %v", err)
	}
	defer os.RemoveAll(workDir)

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var cfg parsedCanvasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	framed := cfg.Screenshots["framed"]
	// Solid background: both gradient colors should be the solid color.
	if framed.Background == nil {
		t.Fatal("expected background config")
	}
	if len(framed.Background.Colors) != 2 {
		t.Fatalf("expected 2 background colors, got %d", len(framed.Background.Colors))
	}
	for i, c := range framed.Background.Colors {
		if c != "#ffffff" {
			t.Fatalf("background color[%d] = %q, want #ffffff", i, c)
		}
	}

	// title + subtitle + image = 3 items.
	if len(framed.Content) != 3 {
		t.Fatalf("expected 3 content items, got %d", len(framed.Content))
	}
	titleItem := framed.Content[0]
	if titleItem.Color != "#ff0000" {
		t.Fatalf("title color = %q, want #ff0000", titleItem.Color)
	}
	subtitleItem := framed.Content[1]
	if subtitleItem.Color != "#00ff00" {
		t.Fatalf("subtitle color = %q, want #00ff00", subtitleItem.Color)
	}
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
  echo "kou 0.18.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
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

func TestRunKoubouGenerate_RunsSetupFramesBeforeGenerate(t *testing.T) {
	resetKoubouVersionCacheForTest()

	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "kou.log")
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
  echo "setup-frames" >> "$KOU_LOG_PATH"
  exit 0
fi
if [ "$1" = "generate" ]; then
  echo "generate" >> "$KOU_LOG_PATH"
  echo '[{"name":"framed","path":"output/framed.png","success":true,"error":""}]'
  exit 0
fi
echo "unsupported args" >&2
exit 1
`)
	t.Setenv("KOU_LOG_PATH", logPath)
	t.Setenv("PATH", binDir)

	results, err := runKoubouGenerate(context.Background(), "frame.yaml")
	if err != nil {
		t.Fatalf("runKoubouGenerate() error = %v", err)
	}
	if len(results) != 1 || results[0].Path != "output/framed.png" {
		t.Fatalf("unexpected parsed results: %+v", results)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}
	if got := strings.TrimSpace(string(logBytes)); got != "setup-frames\ngenerate" {
		t.Fatalf("expected setup-frames before generate, got %q", got)
	}
}

func TestRunKoubouGenerate_SetupFramesFailureIncludesHint(t *testing.T) {
	resetKoubouVersionCacheForTest()

	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
  echo "network unavailable" >&2
  exit 1
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
		t.Fatal("expected setup-frames error")
	}
	if !strings.Contains(err.Error(), "kou setup-frames") {
		t.Fatalf("expected setup-frames command in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "run `kou setup-frames` with network access once before framing") {
		t.Fatalf("expected setup hint in error, got %v", err)
	}
}

func TestRunKoubouGenerate_RechecksSetupFramesWhenKouBinaryChanges(t *testing.T) {
	resetKoubouVersionCacheForTest()

	logPath := filepath.Join(t.TempDir(), "kou.log")

	firstBinDir := t.TempDir()
	writeExecutable(t, filepath.Join(firstBinDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
  echo "setup-first" >> "$KOU_LOG_PATH"
  exit 0
fi
if [ "$1" = "generate" ]; then
  echo "generate-first" >> "$KOU_LOG_PATH"
  echo '[{"name":"framed","path":"output/first.png","success":true,"error":""}]'
  exit 0
fi
echo "unsupported args" >&2
exit 1
`)

	secondBinDir := t.TempDir()
	writeExecutable(t, filepath.Join(secondBinDir, "kou"), `#!/bin/sh
set -eu
if [ "$1" = "--version" ]; then
  echo "kou 0.18.0"
  exit 0
fi
if [ "$1" = "setup-frames" ]; then
  echo "setup-second" >> "$KOU_LOG_PATH"
  exit 0
fi
if [ "$1" = "generate" ]; then
  echo "generate-second" >> "$KOU_LOG_PATH"
  echo '[{"name":"framed","path":"output/second.png","success":true,"error":""}]'
  exit 0
fi
echo "unsupported args" >&2
exit 1
`)

	t.Setenv("KOU_LOG_PATH", logPath)
	t.Setenv("PATH", firstBinDir)
	results, err := runKoubouGenerate(context.Background(), "frame.yaml")
	if err != nil {
		t.Fatalf("first runKoubouGenerate() error = %v", err)
	}
	if len(results) != 1 || results[0].Path != "output/first.png" {
		t.Fatalf("unexpected first parsed results: %+v", results)
	}

	t.Setenv("PATH", secondBinDir)
	results, err = runKoubouGenerate(context.Background(), "frame.yaml")
	if err != nil {
		t.Fatalf("second runKoubouGenerate() error = %v", err)
	}
	if len(results) != 1 || results[0].Path != "output/second.png" {
		t.Fatalf("unexpected second parsed results: %+v", results)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", logPath, err)
	}
	if got := strings.TrimSpace(string(logBytes)); got != "setup-first\ngenerate-first\nsetup-second\ngenerate-second" {
		t.Fatalf("expected setup-frames rerun after binary swap, got %q", got)
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
	if !strings.Contains(err.Error(), "0.18.0") {
		t.Fatalf("expected pinned version in error, got %v", err)
	}
}

func TestRunKoubouGenerate_NotFoundIncludesPinnedInstallHint(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := runKoubouGenerate(context.Background(), "frame.yaml")
	if err == nil {
		t.Fatal("expected not found error")
	}
	if !strings.Contains(err.Error(), "pip install koubou==0.18.0") {
		t.Fatalf("expected pinned install command in error, got %v", err)
	}
}
