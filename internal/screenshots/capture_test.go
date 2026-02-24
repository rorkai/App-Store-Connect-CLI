package screenshots

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapture_UnknownProvider(t *testing.T) {
	ctx := context.Background()
	req := CaptureRequest{
		Provider:  "unknown",
		BundleID:  "com.example.app",
		UDID:      "booted",
		Name:      "home",
		OutputDir: t.TempDir(),
	}

	result, err := Capture(ctx, req)
	if err == nil {
		t.Fatalf("expected error for unknown provider, got result %+v", result)
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("expected 'unknown provider' in error, got %q", err.Error())
	}
}

func TestCaptureWithProvider_RejectsNameWithPathSeparators(t *testing.T) {
	ctx := context.Background()
	req := CaptureRequest{
		Provider:  ProviderAXe,
		BundleID:  "com.example.app",
		UDID:      "booted",
		Name:      "../home",
		OutputDir: t.TempDir(),
	}

	fake := &fakeProvider{path: filepath.Join(t.TempDir(), "ignored.png")}
	_, err := CaptureWithProvider(ctx, req, fake)
	if err == nil {
		t.Fatal("expected validation error for path-like screenshot name")
	}
	if !strings.Contains(err.Error(), "file name without path separators") {
		t.Fatalf("expected screenshot name validation error, got %q", err.Error())
	}
}

func TestCaptureWithProvider_RequiresOutputDirectory(t *testing.T) {
	ctx := context.Background()
	req := CaptureRequest{
		Provider:  ProviderAXe,
		BundleID:  "com.example.app",
		UDID:      "booted",
		Name:      "home",
		OutputDir: "   ",
	}

	fake := &fakeProvider{path: filepath.Join(t.TempDir(), "ignored.png")}
	_, err := CaptureWithProvider(ctx, req, fake)
	if err == nil {
		t.Fatal("expected validation error for missing output directory")
	}
	if !strings.Contains(err.Error(), "output directory is required") {
		t.Fatalf("expected output directory validation error, got %q", err.Error())
	}
}

func TestCaptureWithProvider_MissingFile(t *testing.T) {
	ctx := context.Background()
	req := CaptureRequest{
		Provider:  ProviderAXe,
		BundleID:  "com.example.app",
		Name:      "home",
		OutputDir: t.TempDir(),
	}

	fake := &fakeProvider{path: filepath.Join(t.TempDir(), "nonexistent.png")}
	result, err := CaptureWithProvider(ctx, req, fake)
	if err == nil {
		t.Fatalf("expected validation error for missing file, got result %+v", result)
	}
	if !strings.Contains(err.Error(), "captured file invalid") && !strings.Contains(err.Error(), "no such file") {
		t.Fatalf("expected file invalid or not found error, got %q", err.Error())
	}
}

func TestCaptureWithProvider_ValidPNG(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "valid.png")
	writeMinimalPNG(t, pngPath, 100, 200)

	ctx := context.Background()
	req := CaptureRequest{
		Provider:  ProviderAXe,
		BundleID:  "com.example.app",
		UDID:      "booted",
		Name:      "valid",
		OutputDir: dir,
	}

	fake := &fakeProvider{path: pngPath}
	result, err := CaptureWithProvider(ctx, req, fake)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
	if result.Width != 100 || result.Height != 200 {
		t.Fatalf("expected 100x200, got %dx%d", result.Width, result.Height)
	}
	if result.Provider != ProviderAXe || result.BundleID != req.BundleID {
		t.Fatalf("unexpected result fields: %+v", result)
	}
	if result.Path == "" || !strings.HasSuffix(result.Path, "valid.png") {
		t.Fatalf("unexpected path: %q", result.Path)
	}
}

type fakeProvider struct {
	path string
}

func (f *fakeProvider) Capture(context.Context, CaptureRequest) (string, error) {
	return f.path, nil
}

// writeMinimalPNG writes a minimal valid PNG file for dimension tests.
func writeMinimalPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.Black)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create PNG file: %v", err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
}
