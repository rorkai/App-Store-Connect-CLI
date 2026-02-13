package screenshots

import (
	"image"
	"image/color"
	"image/draw"
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

func TestComposeFramedImage_OuterAntialiasCompositesOnWhite(t *testing.T) {
	raw := image.NewRGBA(image.Rect(0, 0, 2, 2))
	draw.Draw(raw, raw.Bounds(), &image.Uniform{C: color.RGBA{200, 20, 20, 255}}, image.Point{}, draw.Src)

	frame := image.NewRGBA(image.Rect(0, 0, 9, 9))
	for y := 1; y <= 7; y++ {
		for x := 1; x <= 7; x++ {
			px := color.RGBA{80, 80, 80, 255}
			if x == 1 || x == 7 || y == 1 || y == 7 {
				px.A = 160 // anti-aliased outer edge
			}
			frame.SetRGBA(x, y, px)
		}
	}
	for y := 3; y <= 5; y++ {
		for x := 3; x <= 5; x++ {
			frame.SetRGBA(x, y, color.RGBA{})
		}
	}

	composed, err := composeFramedImage(raw, frame, 0)
	if err != nil {
		t.Fatalf("composeFramedImage() error = %v", err)
	}

	got := composed.RGBAAt(1, 4)
	want := compositeOnWhite(color.RGBA{80, 80, 80, 160})
	if got != want {
		t.Fatalf("outer anti-aliased edge = %#v, want %#v", got, want)
	}
}

func writeTextFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func compositeOnWhite(src color.RGBA) color.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, 1, 1))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	overlay := image.NewRGBA(image.Rect(0, 0, 1, 1))
	overlay.SetRGBA(0, 0, src)
	draw.Draw(dst, dst.Bounds(), overlay, image.Point{}, draw.Over)
	return dst.RGBAAt(0, 0)
}
