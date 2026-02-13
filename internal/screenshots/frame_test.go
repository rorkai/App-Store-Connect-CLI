package screenshots

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
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

func TestFrameUploadTargetForDevice(t *testing.T) {
	tests := []struct {
		name            string
		device          FrameDevice
		wantDisplayType string
		wantWidth       int
		wantHeight      int
	}{
		{
			name:            "iphone-air maps to APP_IPHONE_69",
			device:          FrameDeviceIPhoneAir,
			wantDisplayType: "APP_IPHONE_69",
			wantWidth:       1320,
			wantHeight:      2868,
		},
		{
			name:            "iphone-17-pro maps to APP_IPHONE_67",
			device:          FrameDeviceIPhone17Pro,
			wantDisplayType: "APP_IPHONE_67",
			wantWidth:       1320,
			wantHeight:      2868,
		},
		{
			name:            "iphone-16e maps to APP_IPHONE_61",
			device:          FrameDeviceIPhone16e,
			wantDisplayType: "APP_IPHONE_61",
			wantWidth:       1179,
			wantHeight:      2556,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, ok, err := frameUploadTargetForDevice(test.device)
			if err != nil {
				t.Fatalf("frameUploadTargetForDevice() error = %v", err)
			}
			if !ok {
				t.Fatalf("expected upload target for %q", test.device)
			}
			if target.DisplayType != test.wantDisplayType {
				t.Fatalf("display type = %q, want %q", target.DisplayType, test.wantDisplayType)
			}
			if target.Dimension.Width != test.wantWidth || target.Dimension.Height != test.wantHeight {
				t.Fatalf(
					"target dimensions = %dx%d, want %dx%d",
					target.Dimension.Width,
					target.Dimension.Height,
					test.wantWidth,
					test.wantHeight,
				)
			}
		})
	}
}

func TestNormalizeFramedForUpload_IPhoneAir(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1380, 2880))
	for y := 0; y < 2880; y++ {
		for x := 0; x < 1380; x++ {
			src.SetRGBA(x, y, color.RGBA{
				R: uint8((x * 255) / 1380),
				G: uint8((y * 255) / 2880),
				B: 128,
				A: 255,
			})
		}
	}

	normalized, target, changed, err := normalizeFramedForUpload(src, FrameDeviceIPhoneAir)
	if err != nil {
		t.Fatalf("normalizeFramedForUpload() error = %v", err)
	}
	if !changed {
		t.Fatal("expected normalization to be applied")
	}
	if target.DisplayType != "APP_IPHONE_69" {
		t.Fatalf("display type = %q, want APP_IPHONE_69", target.DisplayType)
	}
	if normalized.Bounds().Dx() != 1320 || normalized.Bounds().Dy() != 2868 {
		t.Fatalf("normalized dimensions = %dx%d, want 1320x2868", normalized.Bounds().Dx(), normalized.Bounds().Dy())
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

func TestResolveFramePath_PrefersAirLightGoldPortrait(t *testing.T) {
	root := t.TempDir()
	pngDir := filepath.Join(root, "iphone-air", "png")
	if err := os.MkdirAll(pngDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	writeTextFile(t, filepath.Join(pngDir, "iPhone Air - Silver - Portrait.png"), "x")
	writeTextFile(t, filepath.Join(pngDir, defaultIPhoneAirPortrait), "y")

	got, err := resolveFramePath(root, FrameDeviceIPhoneAir)
	if err != nil {
		t.Fatalf("resolveFramePath() error = %v", err)
	}
	if filepath.Base(got) != defaultIPhoneAirPortrait {
		t.Fatalf("expected %q, got %q", defaultIPhoneAirPortrait, filepath.Base(got))
	}
}

func TestResolveFramePath_FallsBackToSortedPortrait(t *testing.T) {
	root := t.TempDir()
	pngDir := filepath.Join(root, "iphone-17-pro", "png")
	if err := os.MkdirAll(pngDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	writeTextFile(t, filepath.Join(pngDir, "iPhone 17 Pro - Silver - Portrait.png"), "x")
	writeTextFile(t, filepath.Join(pngDir, "iPhone 17 Pro - Deep Blue - Portrait.png"), "y")

	got, err := resolveFramePath(root, FrameDeviceIPhone17Pro)
	if err != nil {
		t.Fatalf("resolveFramePath() error = %v", err)
	}
	if filepath.Base(got) != "iPhone 17 Pro - Deep Blue - Portrait.png" {
		t.Fatalf("unexpected selected portrait: %q", filepath.Base(got))
	}
}

func writeTextFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
