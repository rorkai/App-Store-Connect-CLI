package assets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAssetUploadDeadlineScalesWithTheFilesItCovers(t *testing.T) {
	dir := t.TempDir()
	sizedFile := func(name string, size int64) string {
		t.Helper()
		path := filepath.Join(dir, name)
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		return path
	}
	screenshotA := sizedFile("a.png", 3<<19) // 1.5 MiB
	screenshotB := sizedFile("b.png", 3<<19)
	video := sizedFile("preview.mov", 500<<20)

	tests := []struct {
		name          string
		uploadTimeout string
		files         []string
		withoutWork   bool
		want          time.Duration
	}{
		{name: "no recorded work keeps the base", withoutWork: true, want: 10 * time.Minute},
		{name: "two 1.5 MiB screenshots", files: []string{screenshotA, screenshotB}, want: 10*time.Minute + 2*time.Minute + 3*30*time.Second},
		{name: "a 500 MiB preview hits the cap", files: []string{video}, want: time.Hour},
		{name: "ASC_UPLOAD_TIMEOUT replaces the scaled default", uploadTimeout: "2m", files: []string{screenshotA, screenshotB}, want: 2 * time.Minute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_UPLOAD_TIMEOUT", test.uploadTimeout)
			t.Setenv("ASC_UPLOAD_TIMEOUT_SECONDS", "")
			ctx := context.Background()
			if !test.withoutWork {
				ctx = withAssetUploadWork(ctx, test.files)
			}
			before := time.Now()
			uploadCtx, cancel := contextWithAssetUploadTimeout(ctx)
			defer cancel()
			deadline, ok := uploadCtx.Deadline()
			if !ok {
				t.Fatal("upload context has no deadline")
			}
			if got := deadline.Sub(before); got < test.want || got > test.want+time.Second {
				t.Fatalf("deadline = %s after start, want %s", got, test.want)
			}
		})
	}
}
