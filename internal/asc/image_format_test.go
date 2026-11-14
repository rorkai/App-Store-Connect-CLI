package asc

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateImageFormatMatchesExtensionAcceptsAgreeingFiles(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		format string
	}{
		{name: "png", path: "/tmp/shot.png", format: "png"},
		{name: "uppercase png extension", path: "/tmp/shot.PNG", format: "png"},
		{name: "uppercase decoded format", path: "/tmp/shot.png", format: "PNG"},
		{name: "jpeg with jpg extension", path: "/tmp/shot.jpg", format: "jpeg"},
		{name: "jpeg with jpeg extension", path: "/tmp/shot.jpeg", format: "jpeg"},
		{name: "uppercase jpg extension", path: "/tmp/shot.JPG", format: "jpeg"},
		{name: "gif", path: "/tmp/shot.gif", format: "gif"},
		{name: "extension that names no image format", path: "/tmp/shot.bin", format: "png"},
		{name: "missing extension", path: "/tmp/shot", format: "jpeg"},
		{name: "unknown decoded format", path: "/tmp/shot.png", format: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateImageFormatMatchesExtension(test.path, test.format); err != nil {
				t.Fatalf("ValidateImageFormatMatchesExtension(%q, %q) error: %v", test.path, test.format, err)
			}
		})
	}
}

func TestValidateImageFormatMatchesExtensionRejectsContradictions(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		format      string
		contains    []string
		notContains []string
	}{
		{
			name:     "jpeg named png",
			path:     "/tmp/shots/01-home.png",
			format:   "jpeg",
			contains: []string{"01-home.png", "JPEG", ".png", "01-home.jpg", "PNG"},
		},
		{
			name:     "png named jpg",
			path:     "/tmp/shots/01-home.jpg",
			format:   "png",
			contains: []string{"01-home.jpg", "PNG", "01-home.png", "JPEG"},
		},
		{
			// Screenshot discovery only collects .png/.jpg/.jpeg, so a .gif
			// rename would make the file vanish from an upload directory.
			name:        "gif named png suggests re-export instead of rename",
			path:        "/tmp/shots/01-home.png",
			format:      "gif",
			contains:    []string{"01-home.png", "GIF", "re-export", "PNG"},
			notContains: []string{"01-home.gif", "rename"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateImageFormatMatchesExtension(test.path, test.format)
			if err == nil {
				t.Fatalf("ValidateImageFormatMatchesExtension(%q, %q) = nil, want mismatch error", test.path, test.format)
			}
			message := err.Error()
			for _, want := range test.contains {
				if !strings.Contains(message, want) {
					t.Fatalf("expected error to mention %q, got %q", want, message)
				}
			}
			for _, unwanted := range test.notContains {
				if strings.Contains(message, unwanted) {
					t.Fatalf("expected error not to mention %q, got %q", unwanted, message)
				}
			}
		})
	}
}

func TestValidateScreenshotDimensionsRejectsJPEGNamedPNG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "01-home.png")
	writeJPEG(t, path, 640, 960)

	err := ValidateScreenshotDimensions(path, "APP_IPHONE_35")
	if err == nil {
		t.Fatal("expected a format mismatch error for JPEG data named .png, got nil")
	}
	message := err.Error()
	for _, want := range []string{"JPEG", "01-home.jpg", "PNG"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected error to mention %q, got %q", want, message)
		}
	}
	if strings.Contains(message, "unsupported size") {
		t.Fatalf("expected the format mismatch to be reported, got a dimension error: %q", message)
	}
}

func TestValidateScreenshotDimensionsAcceptsAgreeingFormats(t *testing.T) {
	dir := t.TempDir()

	pngPath := filepath.Join(dir, "01-home.png")
	writePNG(t, pngPath, 640, 960)
	if err := ValidateScreenshotDimensions(pngPath, "APP_IPHONE_35"); err != nil {
		t.Fatalf("ValidateScreenshotDimensions(png) error: %v", err)
	}

	jpgPath := filepath.Join(dir, "02-settings.jpg")
	writeJPEG(t, jpgPath, 640, 960)
	if err := ValidateScreenshotDimensions(jpgPath, "APP_IPHONE_35"); err != nil {
		t.Fatalf("ValidateScreenshotDimensions(jpg) error: %v", err)
	}
}

func writeJPEG(t *testing.T, path string, width, height int) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
}
