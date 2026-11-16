package cmdtest

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

// frameTestFont builds the smallest sfnt asc accepts, with cmap and head
// tables inside the file; the renderer is mocked.
func frameTestFontBytes(signature string) []byte {
	data := []byte(signature)
	data = binary.BigEndian.AppendUint16(data, 2)
	data = append(data, make([]byte, 6)...)
	for index, tag := range []string{"cmap", "head"} {
		data = append(data, tag...)
		data = binary.BigEndian.AppendUint32(data, 0)
		data = binary.BigEndian.AppendUint32(data, uint32(44+4*index))
		data = binary.BigEndian.AppendUint32(data, 4)
	}
	return append(data, make([]byte, 8)...)
}

var frameTestFont = frameTestFontBytes("\x00\x01\x00\x00")

func writeFrameTestFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func describeTextBox(box *screenshots.TextBoxOptions) string {
	if box == nil {
		return "none"
	}
	optional := func(value *int) string {
		if value == nil {
			return "-"
		}
		return fmt.Sprint(*value)
	}
	return box.Color + "/" + optional(box.Padding) + "/" + optional(box.Radius)
}

func TestShotsFrame_RejectsInvalidTextBoxAndFontFile(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(20, 40))
	notFont := writeFrameTestFile(t, filepath.Join(dir, "notes.ttf"), []byte("not a font file"))
	badColorOverlay := writeFrameTestFile(t, filepath.Join(dir, "bad-color.json"), []byte(`{"default":{"title":"A","textBox":true,"textBoxColor":"navy"}}`))
	detailsOverlay := writeFrameTestFile(t, filepath.Join(dir, "details.json"), []byte(`{"default":{"title":"A"},"data":[{"filter":"raw","title":"B","textBoxPadding":4}]}`))
	noBoxOverlay := writeFrameTestFile(t, filepath.Join(dir, "no-box.json"), []byte(`{"default":{"title":"A"}}`))
	unmatchedBoxOverlay := writeFrameTestFile(t, filepath.Join(dir, "unmatched-box.json"), []byte(`{"default":{"title":"A"},"data":[{"filter":"paywall","title":"B","textBox":true}]}`))
	noTextOverlay := writeFrameTestFile(t, filepath.Join(dir, "no-text.json"), []byte(`{"default":{"background":"#111111"}}`))
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "invalid color", args: []string{"--title", "Home", "--text-box", "--text-box-color", "navy"}, wantErr: `--text-box-color must be a hex color in the form #RGB, #RRGGBB, or #RRGGBBAA, got "navy"`},
		{name: "negative padding", args: []string{"--title", "Home", "--text-box", "--text-box-padding", "-2"}, wantErr: "--text-box-padding must be >= 0"},
		{name: "negative radius", args: []string{"--title", "Home", "--text-box", "--text-box-radius", "-1"}, wantErr: "--text-box-radius must be >= 0"},
		{name: "box without text", args: []string{"--text-box"}, wantErr: "--text-box requires --title, --subtitle, or --overlay-config"},
		{name: "detail without box", args: []string{"--title", "Home", "--text-box-padding", "12"}, wantErr: "--text-box-padding requires --text-box or an --overlay-config textBox entry"},
		{name: "detail when overlay never enables box", args: []string{"--overlay-config", noBoxOverlay, "--text-box-color", "#000"}, wantErr: "--text-box-color requires --text-box or an --overlay-config textBox entry"},
		{name: "detail when matched entry leaves box off", args: []string{"--overlay-config", unmatchedBoxOverlay, "--text-box-radius", "3"}, wantErr: "--text-box-radius has no effect: no framed input has a title or keyword with a text box"},
		{name: "box with no overlay text", args: []string{"--overlay-config", noTextOverlay, "--text-box"}, wantErr: "--text-box has no effect: --overlay-config supplies no title or keyword"},
		{name: "overlay invalid color", args: []string{"--overlay-config", badColorOverlay}, wantErr: `--overlay-config: overlay config default.textBoxColor must be a hex color`},
		{name: "overlay details without box", args: []string{"--overlay-config", detailsOverlay}, wantErr: "--overlay-config: overlay config data[0] sets textBoxPadding without textBox: true"},
		{name: "missing font file", args: []string{"--title", "Home", "--font", filepath.Join(dir, "missing.otf")}, wantErr: "--font: read font file"},
		{name: "not a font", args: []string{"--title", "Home", "--font", notFont}, wantErr: "is not a TrueType or OpenType font"},
		{name: "font path without font extension", args: []string{"--title", "Home", "--font", filepath.Join(dir, "Brand")}, wantErr: "must end in .ttf, .otf, or .ttc"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installMockFrame(t, func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error) {
				t.Fatal("renderer must not run for a usage error")
				return nil, nil
			})
			args := append([]string{"screenshots", "frame", "--input", rawPath, "--output-dir", filepath.Join(dir, "out")}, test.args...)
			assertUsageExit(t, args, test.wantErr)
		})
	}
	for name, args := range map[string][]string{
		"config":         {"--config", "/tmp/frame.yaml", "--text-box"},
		"config details": {"--config", "/tmp/frame.yaml", "--text-box-radius", "4"},
	} {
		t.Run(name, func(t *testing.T) {
			assertUsageExit(t, append([]string{"screenshots", "frame"}, args...), "--text-box, --text-box-color, --text-box-padding, and --text-box-radius cannot be used with --config")
		})
	}
	t.Run("watch", func(t *testing.T) {
		assertUsageExit(t, []string{"screenshots", "frame", "--config", "/tmp/frame.yaml", "--watch", "--text-box", "--text-box-color", "#000"},
			"--text-box, --text-box-color cannot be used with --watch")
	})
}

func TestShotsFrame_TextBoxAndFontFileReachRenderer(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(20, 40))
	fontPath := writeFrameTestFile(t, filepath.Join(dir, "Brand.TTF"), frameTestFont)
	calls := 0
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		calls++
		canvas := req.Canvas
		if canvas == nil || canvas.Font != "" || canvas.FontFile == nil || canvas.FontFile.Ext != ".ttf" || !bytes.Equal(canvas.FontFile.Data, frameTestFont) {
			t.Fatalf("canvas font = %+v", canvas)
		}
		if got := describeTextBox(canvas.TextBox); got != "#1b3a5bcc/-/0" {
			t.Fatalf("text box = %s", got)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})
	_, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", filepath.Join(dir, "out"),
		"--title", "Home",
		"--font", fontPath,
		"--text-box",
		"--text-box-color", "#1b3a5bcc",
		"--text-box-radius", "0",
		"--output", "json",
	}...)
	if err != nil || calls != 1 {
		t.Fatalf("run error = %v calls = %d (stderr %q)", err, calls, stderr)
	}
}

func TestShotsFrame_OverlayConfigTextBoxPerFile(t *testing.T) {
	inputDir := t.TempDir()
	for _, name := range []string{"01-home.png", "02-paywall.png", "03-settings.png"} {
		writeFramePNG(t, filepath.Join(inputDir, name), makeRawImage(20, 40))
	}
	overlayPath := writeFrameTestFile(t, filepath.Join(t.TempDir(), "overlay.json"), []byte(`{
  "default": {"title": "App", "textBox": true, "textBoxColor": "#00000080"},
  "data": [
    {"filter": "paywall", "title": "Pro", "textBox": true, "textBoxColor": "#ffffffcc", "textBoxRadius": 4},
    {"filter": "settings", "title": "Settings"}
  ]
}`))
	run := func(extra ...string) map[string]string {
		var mu sync.Mutex
		seen := map[string]string{}
		installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
			mu.Lock()
			seen[filepath.Base(req.OutputPath)] = describeTextBox(req.Canvas.TextBox)
			mu.Unlock()
			return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
		})
		args := append([]string{
			"screenshots", "frame",
			"--input-dir", inputDir,
			"--output-dir", filepath.Join(t.TempDir(), "framed"),
			"--overlay-config", overlayPath,
			"--output", "json",
		}, extra...)
		if _, stderr, err := runRootWithDeps(t, nil, args...); err != nil {
			t.Fatalf("run error = %v (stderr %q)", err, stderr)
		}
		return seen
	}

	got := run()
	want := map[string]string{
		"01-home-iphone-air.png":     "#00000080/-/-",
		"02-paywall-iphone-air.png":  "#ffffffcc/-/4",
		"03-settings-iphone-air.png": "none",
	}
	for output, box := range want {
		if got[output] != box {
			t.Fatalf("overlay only: %s box = %q, want %q (all %v)", output, got[output], box, got)
		}
	}

	// Explicit flags override the matched entry key by key, and --text-box
	// turns the box on for entries that leave it off.
	got = run("--text-box", "--text-box-padding", "40")
	want = map[string]string{
		"01-home-iphone-air.png":     "#00000080/40/-",
		"02-paywall-iphone-air.png":  "#ffffffcc/40/4",
		"03-settings-iphone-air.png": "/40/-",
	}
	for output, box := range want {
		if got[output] != box {
			t.Fatalf("with flags: %s box = %q, want %q (all %v)", output, got[output], box, got)
		}
	}
}

func TestShotsFrame_ResumeRerendersWhenTextBoxOrFontFileChanges(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	rawPath := filepath.Join(dir, "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(20, 40))
	fontPath := writeFrameTestFile(t, filepath.Join(dir, "Brand.ttf"), frameTestFont)
	calls := 0
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		calls++
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})
	run := func(extra ...string) {
		t.Helper()
		args := append([]string{
			"screenshots", "frame",
			"--input", rawPath,
			"--output-dir", filepath.Join(dir, "framed"),
			"--title", "Home",
			"--resume",
			"--output", "json",
		}, extra...)
		if _, stderr, err := runRootWithDeps(t, nil, args...); err != nil {
			t.Fatalf("run error = %v (stderr %q)", err, stderr)
		}
	}
	steps := []struct {
		name      string
		args      []string
		wantCalls int
	}{
		{name: "first render", args: []string{"--text-box", "--font", fontPath}, wantCalls: 1},
		{name: "unchanged", args: []string{"--text-box", "--font", fontPath}, wantCalls: 1},
		{name: "box color", args: []string{"--text-box", "--text-box-color", "#fff", "--font", fontPath}, wantCalls: 2},
		{name: "box padding", args: []string{"--text-box", "--text-box-color", "#fff", "--text-box-padding", "9", "--font", fontPath}, wantCalls: 3},
		{name: "box radius", args: []string{"--text-box", "--text-box-color", "#fff", "--text-box-padding", "9", "--text-box-radius", "2", "--font", fontPath}, wantCalls: 4},
		{name: "box off", args: []string{"--font", fontPath}, wantCalls: 5},
	}
	for _, step := range steps {
		run(step.args...)
		if calls != step.wantCalls {
			t.Fatalf("%s: frame calls = %d, want %d", step.name, calls, step.wantCalls)
		}
	}
	// Same path, new font bytes.
	writeFrameTestFile(t, fontPath, frameTestFontBytes("OTTO"))
	run("--font", fontPath)
	if calls != 6 {
		t.Fatalf("font file change: frame calls = %d, want 6", calls)
	}
	run("--font", fontPath)
	if calls != 6 {
		t.Fatalf("unchanged font file: frame calls = %d, want 6", calls)
	}
}
