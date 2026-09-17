package screenshots

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestMatchOverlayPrefersFilterThenDefault(t *testing.T) {
	config := OverlayConfig{
		Default: OverlayEntry{Title: "App", Keyword: "ship", Background: "#111111"},
		Data: []OverlayEntry{
			{Filter: "home", Title: "Home", Keyword: "fast", Background: "#222222"},
		},
	}
	home := MatchOverlay(config, "/tmp/Home-Screen.PNG")
	if home.Title != "Home" || home.Keyword != "fast" {
		t.Fatalf("filter match = %+v", home)
	}
	other := OverlayToCanvas(MatchOverlay(config, "settings.png"))
	if other.Title != "App" || other.Subtitle != "ship" || other.BGColor != "#111111" {
		t.Fatalf("default overlay = %+v", other)
	}
}

func TestLoadOverlayConfigGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overlay.json")
	raw := `{
  "default": {"title": "Title", "keyword": "Keyword", "background": "#0d0c1e"},
  "data": [{"filter": "paywall", "title": "Upgrade", "keyword": "Plus", "background": "#140f2d"}]
}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	config, err := LoadOverlayConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	got := OverlayToCanvas(MatchOverlay(config, "paywall.png"))
	if got.Title != "Upgrade" || got.Subtitle != "Plus" || got.BGColor != "#140f2d" {
		t.Fatalf("golden overlay = %+v", got)
	}
}

func TestResumeSkipRequiresMatchingHashAndOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.png")
	output := filepath.Join(dir, "out.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := HashFile(input)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := FingerprintFrameResume(FrameResumeFingerprint{SourceHash: hash, Device: "iphone-air", Title: "Home"})
	state := FrameResumeState{Files: map[string]string{output: fingerprint}}
	if ResumeSkip(state, output, fingerprint) {
		t.Fatal("missing output must not skip")
	}
	if err := os.WriteFile(output, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ResumeSkip(state, output, fingerprint) {
		t.Fatal("matching fingerprint and output should skip")
	}
	changed := FingerprintFrameResume(FrameResumeFingerprint{SourceHash: hash, Device: "iphone-air", Title: "Other"})
	if ResumeSkip(state, output, changed) {
		t.Fatal("changed title must not skip")
	}
	path := FrameResumeStateRel
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := SaveFrameResumeState(root, path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFrameResumeState(root, path)
	if err != nil || loaded.Files[output] != fingerprint {
		t.Fatalf("loaded = %+v err=%v", loaded, err)
	}
}

func TestSaveFrameResumeStateRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".asc/reports/screenshots-frame"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, FrameResumeStateRel)
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = SaveFrameResumeState(root, FrameResumeStateRel, FrameResumeState{Files: map[string]string{"out": "fp"}})
	if err == nil {
		t.Fatal("expected symlink state file to be rejected")
	}
	data, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret" {
		t.Fatalf("symlink write changed outside file to %q", data)
	}
}
