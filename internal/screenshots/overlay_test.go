package screenshots

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestParseOverlayConfigRejectsUnknownFields(t *testing.T) {
	_, err := ParseOverlayConfig([]byte(`{"default":{"titel":"Nope"}}`))
	if err == nil {
		t.Fatal("expected unknown overlay field to be rejected")
	}
	if _, err := ParseOverlayConfig([]byte(`{"default":{"title":"A"}}{"default":{"title":"B"}}`)); err == nil {
		t.Fatal("expected trailing overlay JSON to be rejected")
	}
}

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
	config, _, err := LoadOverlayConfig(path)
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
	stored := FrameResult{Path: output, Device: "iphone-air", Width: 20, Height: 40, FramePath: "frame"}
	state := FrameResumeState{Files: map[string]FrameResumeEntry{
		output: {Fingerprint: fingerprint, Result: stored},
	}}
	if _, ok := ResumeEntry(state, output, fingerprint); ok {
		t.Fatal("missing output must not skip")
	}
	if err := os.WriteFile(output, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := ResumeEntry(state, output, fingerprint)
	if !ok || !got.Skipped || got.Width != 20 || got.FramePath != "frame" {
		t.Fatalf("resume entry = %+v ok=%v", got, ok)
	}
	changed := FingerprintFrameResume(FrameResumeFingerprint{SourceHash: hash, Device: "iphone-air", Title: "Other"})
	if _, ok := ResumeEntry(state, output, changed); ok {
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
	if err != nil || loaded.Files[output].Fingerprint != fingerprint || loaded.Files[output].Result.Width != 20 {
		t.Fatalf("loaded = %+v err=%v", loaded, err)
	}
}

func TestResumeFingerprintInvalidatesLegacySchema(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "out.png")
	if err := os.WriteFile(output, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	fingerprintInput := FrameResumeFingerprint{
		SourceHash: "source",
		Device:     "iphone-air",
		Subtitle:   "keyword",
	}
	legacySum := sha256.Sum256([]byte(strings.Join([]string{
		"1",
		pinnedKoubouVersion,
		fingerprintInput.SourceHash,
		fingerprintInput.Device,
		fingerprintInput.Title,
		fingerprintInput.Subtitle,
		fingerprintInput.TitleColor,
		fingerprintInput.SubtitleColor,
		fingerprintInput.Background,
		fingerprintInput.OverlayHash,
	}, "\x00")))
	legacyFingerprint := hex.EncodeToString(legacySum[:])
	currentFingerprint := FingerprintFrameResume(fingerprintInput)
	if currentFingerprint == legacyFingerprint {
		t.Fatal("resume fingerprint did not change after schema bump")
	}
	state := FrameResumeState{Files: map[string]FrameResumeEntry{
		output: {Fingerprint: legacyFingerprint, Result: FrameResult{Path: output}},
	}}
	if _, ok := ResumeEntry(state, output, currentFingerprint); ok {
		t.Fatal("legacy resume state must not be reused after fingerprint schema change")
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
	err = SaveFrameResumeState(root, FrameResumeStateRel, FrameResumeState{Files: map[string]FrameResumeEntry{
		"out": {Fingerprint: "fp"},
	}})
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

func TestHashFileRejectsOversizedInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oversized.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxMatrixArtifactBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := HashFile(path); err == nil {
		t.Fatal("expected oversized input to be rejected")
	}
}

func TestResumeEntryRejectsSymlinkOutput(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.png")
	if err := os.WriteFile(target, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	entry := FrameResumeEntry{Fingerprint: "fp", Result: FrameResult{Path: link}}
	state := FrameResumeState{Files: map[string]FrameResumeEntry{link: entry}}
	if _, ok := ResumeEntry(state, link, "fp"); ok {
		t.Fatal("symlinked output must not be resumable")
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatal(err)
	}
}

func TestOpenFrameInputSnapshotPinsBytes(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.png")
	writeFrameTestPNG(t, inputPath, makeFrameTestImage(20, 40))
	original, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := OpenFrameInputSnapshot(t.Context(), inputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := snapshot.Close(); err != nil {
			t.Error(err)
		}
	}()
	if snapshot.SourceHash() == "" || snapshot.Path() == "" {
		t.Fatal("snapshot did not expose a path and digest")
	}
	writeFrameTestPNG(t, inputPath, makeFrameTestImage(30, 50))
	pinned, err := os.ReadFile(snapshot.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(pinned) != string(original) {
		t.Fatal("snapshot bytes changed after source replacement")
	}
}
