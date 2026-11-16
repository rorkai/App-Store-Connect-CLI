package shots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOutputPath_ConfigModeDefaultsToScreenshotName(t *testing.T) {
	outputDir := t.TempDir()

	got, err := resolveOutputPath(newShotsTestDepsOnly(), "", outputDir, "", "", "iphone-air")
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
		_, err := resolveOutputPath(newShotsTestDepsOnly(), "", outputDir, tc, "", "iphone-air")
		if err == nil {
			t.Fatalf("resolveOutputPath() error = nil for name %q", tc)
		}
		if !strings.Contains(err.Error(), "file name without path separators") {
			t.Fatalf("resolveOutputPath() error = %v, want name validation error for %q", err, tc)
		}
	}
}

func TestPathFoldsCaseMatchesFilesystem(t *testing.T) {
	base := t.TempDir()
	probe := filepath.Join(base, "Probe")
	if err := os.Mkdir(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(probe)
	if err != nil {
		t.Fatal(err)
	}
	swapped, swappedErr := os.Stat(filepath.Join(base, "pROBE"))
	want := swappedErr == nil && os.SameFile(original, swapped)

	if got := pathFoldsCase(probe); got != want {
		t.Fatalf("pathFoldsCase(existing) = %v, want %v", got, want)
	}
	// A missing output directory is judged by its nearest existing ancestor.
	if got := pathFoldsCase(filepath.Join(probe, "missing", "framed")); got != want {
		t.Fatalf("pathFoldsCase(missing) = %v, want %v", got, want)
	}
}
