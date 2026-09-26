package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppClipLayoutRejectsSymlinkedNestedFolder(t *testing.T) {
	metadata := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "subtitle.txt"), []byte("external"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(metadata, "en-US"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(metadata, "en-US", "app_clip")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, _, err := readAppClipLayout(metadata); err == nil {
		t.Fatal("read outside selected metadata root through nested symlink")
	}
}

func TestAppClipLayoutEmptyFolderIsPresent(t *testing.T) {
	metadata := t.TempDir()
	if err := os.MkdirAll(filepath.Join(metadata, "app_clip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, present, err := readAppClipLayout(metadata); err != nil || !present {
		t.Fatalf("present=%v err=%v; empty folder still requests an App Clip", present, err)
	}
}

func TestPreviewPosterFrameMatchesUploadContract(t *testing.T) {
	for _, tc := range []string{"00:00:01.000", "00:00:01:29"} {
		if !validPosterFrame(tc) {
			t.Errorf("rejects valid poster frame %q", tc)
		}
	}
	for _, tc := range []string{"00:99:00", "0:0:0", "00:00:01:30", "00:00:01"} {
		if validPosterFrame(tc) {
			t.Errorf("accepts invalid poster frame %q", tc)
		}
	}
}
