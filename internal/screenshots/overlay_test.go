package screenshots

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestLoadOverlayConfigRejectsOversizedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overlay.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxOverlayConfigBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadOverlayConfig(path); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("LoadOverlayConfig() error = %v, want size limit", err)
	}
}

func TestResumeSkipRequiresMatchingHashAndOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "in.png")
	output := filepath.Join(dir, "out.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := HashFile(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := FingerprintFrameResume(FrameResumeFingerprint{SourceHash: hash, Device: "iphone-air", Title: "Home"})
	stored := FrameResult{Path: output, Device: "iphone-air", Width: 20, Height: 40, FramePath: "frame"}
	state := FrameResumeState{Files: map[string]FrameResumeEntry{
		output: {Fingerprint: fingerprint, Result: stored},
	}}
	if _, ok := ResumeEntry(t.Context(), state, output, fingerprint); ok {
		t.Fatal("missing output must not skip")
	}
	if err := os.WriteFile(output, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := ResumeEntry(t.Context(), state, output, fingerprint); ok {
		t.Fatal("legacy output without a recorded hash must not skip")
	}
	outputHash, err := HashFile(t.Context(), output)
	if err != nil {
		t.Fatal(err)
	}
	entry := state.Files[output]
	entry.OutputHash = outputHash
	state.Files[output] = entry
	got, ok := ResumeEntry(t.Context(), state, output, fingerprint)
	if !ok || !got.Skipped || got.Width != 20 || got.FramePath != "frame" {
		t.Fatalf("resume entry = %+v ok=%v", got, ok)
	}
	changed := FingerprintFrameResume(FrameResumeFingerprint{SourceHash: hash, Device: "iphone-air", Title: "Other"})
	if _, ok := ResumeEntry(t.Context(), state, output, changed); ok {
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
	if _, ok := ResumeEntry(t.Context(), state, output, currentFingerprint); ok {
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

func TestLoadFrameResumeStateRejectsOversizedInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, FrameResumeStateRel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxFrameResumeStateBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := LoadFrameResumeState(root, FrameResumeStateRel); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("LoadFrameResumeState() error = %v, want size limit", err)
	}
}

func TestSaveFrameResumeStateRejectsOversizedOutput(t *testing.T) {
	dir := t.TempDir()
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	state := FrameResumeState{Files: map[string]FrameResumeEntry{
		"out.png": {Fingerprint: strings.Repeat("x", maxFrameResumeStateBytes)},
	}}
	if err := SaveFrameResumeState(root, FrameResumeStateRel, state); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("SaveFrameResumeState() error = %v, want size limit", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".asc")); !os.IsNotExist(err) {
		t.Fatalf(".asc stat error = %v, want not-exist", err)
	}
}

func TestWithFrameResumeLockKeepsPersistentLegacyLock(t *testing.T) {
	dir := t.TempDir()
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := WithFrameResumeLock(context.Background(), root, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(dir, frameResumeLockName))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("lock mode = %v, want regular file", info.Mode())
	}
}

func TestWithFrameResumeLockHonorsLegacyLock(t *testing.T) {
	dir := t.TempDir()
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	release, err := acquireMatrixNamedLock(context.Background(), root, frameResumeLockName)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := release(); err != nil {
			t.Error(err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = WithFrameResumeLock(ctx, root, func() error {
		t.Fatal("callback ran while the legacy lock was held")
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WithFrameResumeLock() error = %v, want context deadline", err)
	}
}

func TestWithFrameResumeLockCanceledContextDoesNotCreateStateDirectory(t *testing.T) {
	dir := t.TempDir()
	root, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WithFrameResumeLock(ctx, root, func() error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("WithFrameResumeLock() error = %v, want context canceled", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".asc")); !os.IsNotExist(err) {
		t.Fatalf(".asc stat error = %v, want not-exist", err)
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
	if _, err := HashFile(t.Context(), path); err == nil {
		t.Fatal("expected oversized input to be rejected")
	}
}

func TestResumeEntryRejectsSymlinkOutputForMatchingAndStaleState(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.png")
	if err := os.WriteFile(target, []byte("framed"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	outputHash, err := HashFile(t.Context(), target)
	if err != nil {
		t.Fatal(err)
	}
	entry := FrameResumeEntry{Fingerprint: "fp", OutputHash: outputHash, Result: FrameResult{Path: link}}
	state := FrameResumeState{Files: map[string]FrameResumeEntry{link: entry}}
	for _, test := range []struct {
		name        string
		fingerprint string
	}{
		{name: "matching", fingerprint: "fp"},
		{name: "stale", fingerprint: "stale"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := ResumeEntry(t.Context(), state, link, test.fingerprint); ok {
				t.Fatal("symlinked output must not be resumable")
			}
			contents, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(contents) != "framed" {
				t.Fatalf("symlink target changed to %q", contents)
			}
		})
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

func TestOpenFrameInputSnapshotRejectsReplacementBetweenLocks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the replacement seam relies on Unix rename semantics")
	}
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.png")
	writeFrameTestPNG(t, inputPath, makeFrameTestImage(20, 40))

	previous := matrixFrameInputAfterFileLockForTest
	matrixFrameInputAfterFileLockForTest = func(path string) {
		if err := os.Rename(path, path+".original"); err != nil {
			t.Errorf("rename prepared input: %v", err)
			return
		}
		writeFrameTestPNG(t, path, makeFrameTestImage(30, 50))
	}
	t.Cleanup(func() { matrixFrameInputAfterFileLockForTest = previous })

	snapshot, err := OpenFrameInputSnapshot(t.Context(), inputPath)
	if err == nil {
		if snapshot != nil {
			_ = snapshot.Close()
		}
		t.Fatal("OpenFrameInputSnapshot() error = nil, want replacement rejection")
	}
}

func TestHashFileHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.png")
	if err := os.WriteFile(path, []byte("framed"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := HashFile(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("HashFile error = %v, want context cancellation", err)
	}
}
