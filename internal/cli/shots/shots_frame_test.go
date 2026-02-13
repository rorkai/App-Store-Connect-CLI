package shots

import (
	"path/filepath"
	"testing"
)

func TestResolveOutputPath_ConfigModeDefaultsToScreenshotName(t *testing.T) {
	outputDir := t.TempDir()

	got, err := resolveOutputPath("", outputDir, "", "", "iphone-air")
	if err != nil {
		t.Fatalf("resolveOutputPath() error = %v", err)
	}

	want := filepath.Join(outputDir, "screenshot-iphone-air.png")
	if got != want {
		t.Fatalf("resolveOutputPath() = %q, want %q", got, want)
	}
}
