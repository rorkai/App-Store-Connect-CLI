package migrate

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestInferScreenshotDisplayType_FromFilenameAndDimensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iphone_65_screen.png")
	writePNG(t, path, 1242, 2688)

	displayType, err := inferScreenshotDisplayType(path)
	if err != nil {
		t.Fatalf("inferScreenshotDisplayType() error: %v", err)
	}
	if displayType != "APP_IPHONE_65" {
		t.Fatalf("expected APP_IPHONE_65, got %q", displayType)
	}
}

func TestInferScreenshotDisplayType_FromDimensionsOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screen.png")
	writePNG(t, path, 1242, 2688)

	displayType, err := inferScreenshotDisplayType(path)
	if err != nil {
		t.Fatalf("inferScreenshotDisplayType() error: %v", err)
	}
	if displayType != "APP_IPHONE_65" {
		t.Fatalf("expected APP_IPHONE_65, got %q", displayType)
	}
}

func TestInferScreenshotDisplayType_IgnoresPathSegments(t *testing.T) {
	dir := t.TempDir()
	nestedDir := filepath.Join(dir, "desktop")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	path := filepath.Join(nestedDir, "screen.png")
	writePNG(t, path, 1242, 2688)

	displayType, err := inferScreenshotDisplayType(path)
	if err != nil {
		t.Fatalf("inferScreenshotDisplayType() error: %v", err)
	}
	if displayType != "APP_IPHONE_65" {
		t.Fatalf("expected APP_IPHONE_65, got %q", displayType)
	}
}

func TestInferScreenshotDisplayType_ProMaxDimensions(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		want   string
	}{
		{name: "iphone_67", width: 1290, height: 2796, want: "APP_IPHONE_67"},
		{name: "iphone_69", width: 1320, height: 2868, want: "APP_IPHONE_69"},
		{name: "1260x2736", width: 1260, height: 2736, want: "APP_IPHONE_69"},
		{name: "1284x2778 maps to iphone_65", width: 1284, height: 2778, want: "APP_IPHONE_65"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "screen.png")
			writePNG(t, path, test.width, test.height)

			displayType, err := inferScreenshotDisplayType(path)
			if err != nil {
				t.Fatalf("inferScreenshotDisplayType() error: %v", err)
			}
			if displayType != test.want {
				t.Fatalf("expected %s, got %q", test.want, displayType)
			}
		})
	}
}

func TestInferScreenshotDisplayType_ModernDimensions(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		want   string
	}{
		{name: "1206x2622 maps to iphone_61", width: 1206, height: 2622, want: "APP_IPHONE_61"},
		{name: "1170x2532 maps to iphone_58", width: 1170, height: 2532, want: "APP_IPHONE_58"},
		{name: "1080x2340 maps to iphone_58", width: 1080, height: 2340, want: "APP_IPHONE_58"},
		{name: "1488x2266 maps to ipad_11", width: 1488, height: 2266, want: "APP_IPAD_PRO_3GEN_11"},
		{name: "1640x2360 maps to ipad_11", width: 1640, height: 2360, want: "APP_IPAD_PRO_3GEN_11"},
		{name: "1668x2420 maps to ipad_11", width: 1668, height: 2420, want: "APP_IPAD_PRO_3GEN_11"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "screen.png")
			writePNG(t, path, test.width, test.height)

			displayType, err := inferScreenshotDisplayType(path)
			if err != nil {
				t.Fatalf("inferScreenshotDisplayType() error: %v", err)
			}
			if displayType != test.want {
				t.Fatalf("expected %s, got %q", test.want, displayType)
			}
		})
	}
}

func TestInferScreenshotDisplayType_FromFilenameHintNoSpace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iphone6.9_screen.png")
	writePNG(t, path, 1242, 2688)

	displayType, err := inferScreenshotDisplayType(path)
	if err != nil {
		t.Fatalf("inferScreenshotDisplayType() error: %v", err)
	}
	if displayType != "APP_IPHONE_69" {
		t.Fatalf("expected APP_IPHONE_69, got %q", displayType)
	}
}

func TestInferScreenshotDisplayType_UnknownSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screen.png")
	writePNG(t, path, 120, 240)

	_, err := inferScreenshotDisplayType(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDiscoverScreenshotPlan_NormalizesLocale(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "en_US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	writePNG(t, filepath.Join(localeDir, "iphone_65_screen.png"), 1242, 2688)

	plans, _, err := discoverScreenshotPlan(root)
	if err != nil {
		t.Fatalf("discoverScreenshotPlan() error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if plans[0].Locale != "en-US" {
		t.Fatalf("expected locale en-US, got %q", plans[0].Locale)
	}
	if plans[0].DisplayType != "APP_IPHONE_65" {
		t.Fatalf("expected display type APP_IPHONE_65, got %q", plans[0].DisplayType)
	}
	if len(plans[0].Files) != 1 {
		t.Fatalf("expected 1 screenshot file, got %d", len(plans[0].Files))
	}
}

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}
