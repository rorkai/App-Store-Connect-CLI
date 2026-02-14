package screenshots

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
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

func TestCollectAssetDirs_ResolvesRelativeAssetPathsFromConfigDir(t *testing.T) {
	baseDir := t.TempDir()
	projectDir := filepath.Join(baseDir, "project")
	rawDir := filepath.Join(projectDir, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(projectDir, "config.yaml")
	yaml := `screenshots:
  home:
    content:
      - type: "image"
        asset: "raw/home.png"
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if chdirErr := os.Chdir(cwd); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	}()
	otherDir := filepath.Join(baseDir, "other")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(otherDir); err != nil {
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

func TestGenerationCoalescer_TriggersSerialRuns(t *testing.T) {
	var runCount int32
	var concurrent int32
	var maxConcurrent int32

	firstRunStarted := make(chan struct{})
	releaseFirstRun := make(chan struct{})
	coalescer := newGenerationCoalescer(func() {
		current := atomic.AddInt32(&concurrent, 1)
		for {
			previous := atomic.LoadInt32(&maxConcurrent)
			if current <= previous || atomic.CompareAndSwapInt32(&maxConcurrent, previous, current) {
				break
			}
		}

		runNumber := atomic.AddInt32(&runCount, 1)
		if runNumber == 1 {
			close(firstRunStarted)
			<-releaseFirstRun
		}
		atomic.AddInt32(&concurrent, -1)
	})

	var firstTrigger sync.WaitGroup
	firstTrigger.Add(1)
	go func() {
		defer firstTrigger.Done()
		coalescer.Trigger()
	}()
	<-firstRunStarted

	var extraTriggers sync.WaitGroup
	for i := 0; i < 3; i++ {
		extraTriggers.Add(1)
		go func() {
			defer extraTriggers.Done()
			coalescer.Trigger()
		}()
	}
	extraTriggers.Wait()

	if got := atomic.LoadInt32(&runCount); got != 1 {
		t.Fatalf("expected first run still in progress, got %d run(s)", got)
	}

	close(releaseFirstRun)
	firstTrigger.Wait()

	if got := atomic.LoadInt32(&runCount); got != 2 {
		t.Fatalf("expected coalesced follow-up run, got %d run(s)", got)
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 1 {
		t.Fatalf("expected serialized execution, max concurrency %d", got)
	}
}
