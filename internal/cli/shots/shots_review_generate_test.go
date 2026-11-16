package shots

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShotsReviewGenerateResolvesRelativePathsAgainstDepsWorkingDirectory(t *testing.T) {
	workingDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workingDir, "framed"), 0o755); err != nil {
		t.Fatal(err)
	}
	d, _, _ := newShotsTestDeps()
	d.Getwd = func() (string, error) { return workingDir, nil }

	err := NewShotsReviewGenerateCommand(d).ParseAndRun(context.Background(), []string{"--framed-dir", "framed", "--raw-dir", "", "--output-dir", "review"})
	if err != nil {
		t.Fatalf("review-generate error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "review", "manifest.json")); err != nil {
		t.Fatalf("manifest not written under the deps working directory: %v", err)
	}
}
