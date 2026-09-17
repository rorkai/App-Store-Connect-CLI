package screenshots

import (
	"os"
	"path/filepath"
	"testing"
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
	state := FrameResumeState{Files: map[string]string{output: hash}}
	if ResumeSkip(state, output, hash) {
		t.Fatal("missing output must not skip")
	}
	if err := os.WriteFile(output, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !ResumeSkip(state, output, hash) {
		t.Fatal("matching hash and output should skip")
	}
	if ResumeSkip(state, output, "other") {
		t.Fatal("changed source must not skip")
	}
	path := filepath.Join(dir, FrameResumeStateRel)
	if err := SaveFrameResumeState(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadFrameResumeState(path)
	if err != nil || loaded.Files[output] != hash {
		t.Fatalf("loaded = %+v err=%v", loaded, err)
	}
}
