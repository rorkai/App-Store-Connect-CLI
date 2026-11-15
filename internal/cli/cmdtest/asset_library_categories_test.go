package cmdtest

import (
	"strings"
	"testing"
)

func TestAssetLibraryVideoUploadValidation(t *testing.T) {
	for _, args := range [][]string{
		{"asset-library", "videos", "upload"},
		{"asset-library", "videos", "upload", "--library-id", "lib", "--file", "video.mp4", "--category", "OTHER"},
		{"asset-library", "images", "upload", "--library-id", "lib", "--file", "image.png", "--category", "OTHER"},
	} {
		_, stderr, err := runAssetLibrary(t, nil, args...)
		if err == nil || !strings.Contains(stderr, "asset-library") {
			t.Fatalf("args %v: err=%v stderr=%q", args, err, stderr)
		}
		if strings.Contains(stderr, "Unknown") || strings.Contains(stderr, "not defined") {
			t.Fatalf("missing command/category: %s", stderr)
		}
	}
}
