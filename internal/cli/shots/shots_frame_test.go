package shots

import (
	"path/filepath"
	"strings"
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

func TestResolveOutputPath_RejectsNameWithPathSeparators(t *testing.T) {
	outputDir := t.TempDir()

	testCases := []string{
		"../outside",
		"..\\outside",
		"nested/name",
		"nested\\name",
		".",
		"..",
	}

	for _, tc := range testCases {
		_, err := resolveOutputPath("", outputDir, tc, "", "iphone-air")
		if err == nil {
			t.Fatalf("resolveOutputPath() error = nil for name %q", tc)
		}
		if !strings.Contains(err.Error(), "file name without path separators") {
			t.Fatalf("resolveOutputPath() error = %v, want name validation error for %q", err, tc)
		}
	}
}
