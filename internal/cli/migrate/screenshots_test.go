package migrate

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverScreenshotPlanForUploadRejectsReplacedFile(t *testing.T) {
	screenshotsDir := t.TempDir()
	localeDir := filepath.Join(screenshotsDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	path := filepath.Join(localeDir, "iphone_65_1.png")
	writePNG(t, path, 1242, 2688)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(original) error = %v", err)
	}

	plans, _, err := discoverScreenshotPlanForUpload(screenshotsDir)
	if err != nil {
		t.Fatalf("discoverScreenshotPlanForUpload() error = %v", err)
	}
	defer closeScreenshotPlans(plans)
	if len(plans) != 1 || len(plans[0].Files) != 1 {
		t.Fatalf("plans = %#v, want one screenshot", plans)
	}

	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	writePNG(t, path, 1290, 2796)

	opened, ok, err := plans[0].openedFile(path)
	if !ok {
		t.Fatal("openedFile() did not retain a rooted source")
	}
	if opened != nil {
		_ = opened.Close()
		t.Fatal("openedFile() returned a replaced screenshot")
	}
	if err == nil || !strings.Contains(err.Error(), "changed after discovery") {
		t.Fatalf("openedFile() error = %v, want changed-after-discovery rejection", err)
	}
	replacement, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(replacement) error = %v", err)
	}
	if bytes.Equal(replacement, original) {
		t.Fatal("test setup did not replace the pathname")
	}
}

func TestDiscoverScreenshotPlanForUploadRejectsFormatExtensionMismatch(t *testing.T) {
	screenshotsDir := t.TempDir()
	localeDir := filepath.Join(screenshotsDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeJPEG(t, filepath.Join(localeDir, "iphone_65_1.png"), 1242, 2688)

	plans, _, err := discoverScreenshotPlanForUpload(screenshotsDir)
	defer closeScreenshotPlans(plans)
	if err == nil {
		t.Fatal("discoverScreenshotPlanForUpload() = nil error, want format mismatch")
	}
	for _, want := range []string{"iphone_65_1.png", "JPEG", "iphone_65_1.jpg", "PNG"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("discoverScreenshotPlanForUpload() error = %v, want it to mention %q", err, want)
		}
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %#v, want none", plans)
	}
}

func TestInferScreenshotDisplayType_FromFilenameAndDimensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iphone_65_screen.png")
	writePNG(t, path, 1242, 2688)

	displayType, err := inferScreenshotDisplayTypeFromDimensions(path, 1242, 2688)
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

	displayType, err := inferScreenshotDisplayTypeFromDimensions(path, 1242, 2688)
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

	displayType, err := inferScreenshotDisplayTypeFromDimensions(path, 1242, 2688)
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

			displayType, err := inferScreenshotDisplayTypeFromDimensions(path, test.width, test.height)
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

			displayType, err := inferScreenshotDisplayTypeFromDimensions(path, test.width, test.height)
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

	displayType, err := inferScreenshotDisplayTypeFromDimensions(path, 1242, 2688)
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

	_, err := inferScreenshotDisplayTypeFromDimensions(path, 120, 240)
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

	plans, _, err := discoverScreenshotPlanWithOpenFiles(root, false)
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

func TestDiscoverScreenshotPlan_SkipsHiddenAndNonImageFiles(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	writePNG(t, filepath.Join(localeDir, "iPhone 17 Pro Max-1-main-screen.png"), 1320, 2868)
	if err := os.WriteFile(filepath.Join(localeDir, ".DS_Store"), []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write hidden file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localeDir, "notes.txt"), []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write notes file: %v", err)
	}

	plans, skipped, err := discoverScreenshotPlanWithOpenFiles(root, false)
	if err != nil {
		t.Fatalf("discoverScreenshotPlan() error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if len(plans[0].Files) != 1 {
		t.Fatalf("expected 1 image file, got %d", len(plans[0].Files))
	}
	if len(skipped) != 2 {
		t.Fatalf("expected 2 skipped files, got %+v", skipped)
	}
}

func TestDiscoverScreenshotPlan_IgnoresEmptyLocaleDirectories(t *testing.T) {
	root := t.TempDir()
	emptyLocaleDir := filepath.Join(root, "ja")
	if err := os.MkdirAll(emptyLocaleDir, 0o755); err != nil {
		t.Fatalf("mkdir empty locale dir: %v", err)
	}
	localeDir := filepath.Join(root, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir locale dir: %v", err)
	}
	writePNG(t, filepath.Join(localeDir, "iphone_65_screen.png"), 1242, 2688)

	plans, skipped, err := discoverScreenshotPlanWithOpenFiles(root, false)
	if err != nil {
		t.Fatalf("discoverScreenshotPlan() error: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped locale, got %+v", skipped)
	}
	if skipped[0].Path != emptyLocaleDir {
		t.Fatalf("expected skipped path %q, got %q", emptyLocaleDir, skipped[0].Path)
	}
}

func TestInferScreenshotDisplayType_IPadPro129GenerationDisambiguation(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		width  int
		height int
		want   string
	}{
		{
			name:   "13 inch M5 filename",
			path:   "iPad Pro 13-inch (M5)-1-main-screen.png",
			width:  2064,
			height: 2752,
			want:   "APP_IPAD_PRO_3GEN_129",
		},
		{
			name:   "modern 12.9 inch generation filename",
			path:   "iPad Pro (12.9-inch) (6th generation)-1-main-screen.png",
			width:  2048,
			height: 2732,
			want:   "APP_IPAD_PRO_3GEN_129",
		},
		{
			name:   "unhinted 13 inch dimensions",
			path:   "main-screen.png",
			width:  2064,
			height: 2752,
			want:   "APP_IPAD_PRO_3GEN_129",
		},
		{
			name:   "unhinted shared dimensions default modern",
			path:   "main-screen.png",
			width:  2048,
			height: 2732,
			want:   "APP_IPAD_PRO_3GEN_129",
		},
		{
			name:   "explicit legacy display type",
			path:   "APP_IPAD_PRO_129-1-main-screen.png",
			width:  2048,
			height: 2732,
			want:   "APP_IPAD_PRO_129",
		},
		{
			name:   "explicit second generation filename",
			path:   "iPad Pro (12.9-inch) (2nd generation)-1-main-screen.png",
			width:  2048,
			height: 2732,
			want:   "APP_IPAD_PRO_129",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			displayType, err := inferScreenshotDisplayTypeFromDimensions(test.path, test.width, test.height)
			if err != nil {
				t.Fatalf("inferScreenshotDisplayTypeFromDimensions() error: %v", err)
			}
			if displayType != test.want {
				t.Fatalf("display type = %q, want %q", displayType, test.want)
			}
		})
	}
}

func TestInferScreenshotDisplayType_WatchAndDesktopDimensions(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		width  int
		height int
		want   string
	}{
		{name: "watch series 3", path: "applewatch_1.png", width: 312, height: 390, want: "APP_WATCH_SERIES_3"},
		{name: "watch series 4", path: "applewatch_1.png", width: 368, height: 448, want: "APP_WATCH_SERIES_4"},
		{name: "watch series 7", path: "applewatch_1.png", width: 396, height: 484, want: "APP_WATCH_SERIES_7"},
		{name: "watch series 10", path: "applewatch_1.png", width: 416, height: 496, want: "APP_WATCH_SERIES_10"},
		{name: "watch ultra 410", path: "applewatch_1.png", width: 410, height: 502, want: "APP_WATCH_ULTRA"},
		{name: "watch ultra 422", path: "applewatch_1.png", width: 422, height: 514, want: "APP_WATCH_ULTRA"},
		{name: "desktop 1280x800", path: "01-main.png", width: 1280, height: 800, want: "APP_DESKTOP"},
		{name: "desktop 2880x1800", path: "01-main.png", width: 2880, height: 1800, want: "APP_DESKTOP"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			displayType, err := inferScreenshotDisplayTypeFromDimensions(test.path, test.width, test.height)
			if err != nil {
				t.Fatalf("inferScreenshotDisplayTypeFromDimensions() error: %v", err)
			}
			if displayType != test.want {
				t.Fatalf("display type = %q, want %q", displayType, test.want)
			}
		})
	}
}

func TestInferScreenshotDisplayType_AmbiguousUltraHighDefinitionDimensions(t *testing.T) {
	_, err := inferScreenshotDisplayTypeFromDimensions("01-main.png", 3840, 2160)
	if err == nil {
		t.Fatal("expected 3840x2160 to be reported as ambiguous")
	}
	for _, want := range []string{"ambiguous screenshot display type", "APP_APPLE_TV", "APP_APPLE_VISION_PRO"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}

	displayType, err := inferScreenshotDisplayTypeFromDimensions("vision_pro-01.png", 3840, 2160)
	if err != nil {
		t.Fatalf("inferScreenshotDisplayTypeFromDimensions() error: %v", err)
	}
	if displayType != "APP_APPLE_VISION_PRO" {
		t.Fatalf("display type = %q, want APP_APPLE_VISION_PRO", displayType)
	}

	displayType, err = inferScreenshotDisplayTypeFromDimensions("apple_tv-01.png", 3840, 2160)
	if err != nil {
		t.Fatalf("inferScreenshotDisplayTypeFromDimensions() error: %v", err)
	}
	if displayType != "APP_APPLE_TV" {
		t.Fatalf("display type = %q, want APP_APPLE_TV", displayType)
	}
}

func TestInferScreenshotDisplayType_IPad13InchFilenameHintWithoutCatalogDimensions(t *testing.T) {
	tests := []string{
		"ipad pro 13 marketing.png",
		"ipad-pro-13-inch-marketing.png",
		"ipad 12.9 marketing.png",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			displayType, err := inferScreenshotDisplayTypeFromDimensions(name, 1600, 1200)
			if err != nil {
				t.Fatalf("inferScreenshotDisplayTypeFromDimensions() error: %v", err)
			}
			if displayType != "APP_IPAD_PRO_3GEN_129" {
				t.Fatalf("display type = %q, want APP_IPAD_PRO_3GEN_129", displayType)
			}
		})
	}
}

func writeJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	defer file.Close()
	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
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
