package screenshots

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestCollectAssetDirs_ParsesKoubouYAML(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	yaml := `screenshots:
  home:
    content:
      - type: "image"
        asset: "` + filepath.Join(rawDir, "home.png") + `"
      - type: "text"
        content: "Hello"
  settings:
    content:
      - type: "image"
        asset: "` + filepath.Join(rawDir, "settings.png") + `"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	dirs := collectAssetDirs(configPath)
	if len(dirs) != 1 {
		t.Fatalf("expected 1 unique dir, got %d: %v", len(dirs), dirs)
	}
	if dirs[0] != rawDir {
		t.Fatalf("expected %q, got %q", rawDir, dirs[0])
	}
}

func TestCollectAssetDirs_EmptyOnMissingFile(t *testing.T) {
	dirs := collectAssetDirs("/nonexistent/config.yaml")
	if len(dirs) != 0 {
		t.Fatalf("expected 0 dirs for missing file, got %d", len(dirs))
	}
}

func TestIsRelevantChange_ConfigWrite(t *testing.T) {
	configPath := "/projects/screenshots/config.yaml"
	event := fsnotify.Event{
		Name: configPath,
		Op:   fsnotify.Write,
	}
	if !isRelevantChange(event, configPath, nil) {
		t.Fatal("expected config write to be relevant")
	}
}

func TestIsRelevantChange_AssetPNG(t *testing.T) {
	assetDir := "/projects/screenshots/raw"
	event := fsnotify.Event{
		Name: filepath.Join(assetDir, "home.png"),
		Op:   fsnotify.Create,
	}
	if !isRelevantChange(event, "/projects/screenshots/config.yaml", []string{assetDir}) {
		t.Fatal("expected PNG create in asset dir to be relevant")
	}
}

func TestIsRelevantChange_IgnoresUnrelatedFile(t *testing.T) {
	event := fsnotify.Event{
		Name: "/projects/screenshots/notes.txt",
		Op:   fsnotify.Write,
	}
	if isRelevantChange(event, "/projects/screenshots/config.yaml", []string{"/projects/screenshots/raw"}) {
		t.Fatal("expected .txt file to be ignored")
	}
}

func TestIsRelevantChange_IgnoresRemoveOp(t *testing.T) {
	event := fsnotify.Event{
		Name: "/projects/screenshots/config.yaml",
		Op:   fsnotify.Remove,
	}
	if isRelevantChange(event, "/projects/screenshots/config.yaml", nil) {
		t.Fatal("expected remove op to be ignored")
	}
}
