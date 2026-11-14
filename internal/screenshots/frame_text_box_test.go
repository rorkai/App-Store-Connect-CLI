package screenshots

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalTestFont builds the smallest sfnt asc accepts: a table directory
// with cmap and head tables that lie within the file. Koubou is never run in
// these tests.
func minimalTestFont(signature string) []byte {
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

var testFontBytes = minimalTestFont("\x00\x01\x00\x00")

// TestTextBoxAndFontFileConvertToRendererConfigGolden pins the Koubou 0.20.0
// text box and font file fields (TextBoxConfig and resolve_font_family in the
// upstream v0.20.0 source) generated for a phone and a tablet frame.
func TestTextBoxAndFontFileConvertToRendererConfigGolden(t *testing.T) {
	overlay, _, err := LoadOverlayConfig(filepath.Join("testdata", "overlay-config-text-box.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateOverlayTextBoxes(overlay, false); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		device FrameDevice
		input  string
		golden string
	}{
		{name: "phone", device: FrameDeviceIPhoneAir, input: "01-home.png", golden: "text-box-font-file.iphone-air.golden.yaml"},
		{name: "tablet", device: FrameDeviceIPadPro13, input: "03-paywall.png", golden: "text-box-font-file.ipad-pro-13.golden.yaml"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := MatchOverlay(overlay, test.input)
			canvas := OverlayToCanvas(entry)
			canvas.TextBox = OverlayTextBox(entry)
			canvas.FontFile = &FontFile{Ext: ".ttf", Data: testFontBytes}
			canvas.TextPosition = TextPositionBottom

			spec, _, err := resolveFrameSpec(test.device, "")
			if err != nil {
				t.Fatal(err)
			}
			workDir := t.TempDir()
			input := filepath.Join(workDir, test.input)
			if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
				t.Fatal(err)
			}
			configPath, _, err := createDefaultKoubouConfigAtRoot(input, spec, &canvas, workDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			font, err := os.ReadFile(filepath.Join(workDir, "font.ttf"))
			if err != nil {
				t.Fatalf("font file not copied into the Koubou work root: %v", err)
			}
			if !bytes.Equal(font, testFontBytes) {
				t.Fatalf("copied font = %x", font)
			}
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.ReplaceAll(string(data), workDir+string(filepath.Separator), "$WORK/")
			goldenPath := filepath.Join("testdata", test.golden)
			if os.Getenv("ASC_UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if wantText := strings.ReplaceAll(string(want), "\r\n", "\n"); got != wantText {
				t.Fatalf("renderer config mismatch\n--- got ---\n%s\n--- want ---\n%s", got, wantText)
			}
		})
	}
}

func TestTextBoxDefaultsScaleWithFontSize(t *testing.T) {
	items := bezelContentItems("/tmp/input.png", frameDeviceKoubouSpecs[FrameDeviceIPhoneAir], &CanvasOptions{
		Title:    "Title",
		Subtitle: "Subtitle",
		TextBox:  &TextBoxOptions{},
	})
	if len(items) != 3 {
		t.Fatalf("items = %#v", items)
	}
	title, subtitle, image := items[0], items[1], items[2]
	if title.Box == nil || title.Box.Color != DefaultTextBoxColor || title.Box.Padding != 22 || title.Box.CornerRadius != nil {
		t.Fatalf("title box = %#v", title.Box)
	}
	if title.Box.Level != "paragraph" || title.Box.Type != "rounded" {
		t.Fatalf("title box shape = %#v", title.Box)
	}
	if subtitle.Box == nil || subtitle.Box.Padding != 13 {
		t.Fatalf("subtitle box = %#v", subtitle.Box)
	}
	if image.Box != nil {
		t.Fatalf("image item must not carry a box: %#v", image)
	}
}

func TestTextBoxOmittedByDefault(t *testing.T) {
	items := bezelContentItems("/tmp/input.png", frameDeviceKoubouSpecs[FrameDeviceIPhoneAir], &CanvasOptions{Title: "Title"})
	if items[0].Box != nil {
		t.Fatalf("title box = %#v, want none", items[0].Box)
	}
}

func TestValidateTextBoxColor(t *testing.T) {
	for _, valid := range []string{"#fff", "#FFFFFF", "#00000099"} {
		if err := ValidateTextBoxColor(valid); err != nil {
			t.Fatalf("ValidateTextBoxColor(%q) = %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "fff", "#ffff", "#12345", "#gggggg", "black", "#0000009"} {
		if err := ValidateTextBoxColor(invalid); err == nil {
			t.Fatalf("ValidateTextBoxColor(%q) accepted", invalid)
		}
	}
}

func TestValidateOverlayTextBoxes(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		flagBox bool
		wantErr string
	}{
		{name: "bad color", raw: `{"default":{"title":"A","textBox":true,"textBoxColor":"black"}}`, wantErr: "default.textBoxColor"},
		{name: "negative padding", raw: `{"default":{"title":"A"},"data":[{"filter":"a","textBox":true,"textBoxPadding":-1}]}`, wantErr: "data[0].textBoxPadding must be >= 0"},
		{name: "negative radius", raw: `{"default":{"title":"A","textBox":true,"textBoxRadius":-4}}`, wantErr: "default.textBoxRadius must be >= 0"},
		{name: "details without box", raw: `{"default":{"title":"A","textBoxColor":"#000"}}`, wantErr: "default sets textBoxColor without textBox: true"},
		{name: "details enabled by flag", raw: `{"default":{"title":"A","textBoxColor":"#000"}}`, flagBox: true},
		{name: "valid", raw: `{"default":{"title":"A","textBox":true,"textBoxColor":"#000000aa","textBoxPadding":0,"textBoxRadius":0}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config, err := ParseOverlayConfig([]byte(test.raw))
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateOverlayTextBoxes(config, test.flagBox)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateOverlayTextBoxes() = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("ValidateOverlayTextBoxes() = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestIsFontFilePath(t *testing.T) {
	for _, value := range []string{"Brand.ttf", "fonts/Brand.OTF", `C:\fonts\brand.ttc`} {
		if !IsFontFilePath(value) {
			t.Fatalf("IsFontFilePath(%q) = false", value)
		}
	}
	for _, value := range []string{"Helvetica", "Helvetica Neue", "SF Pro Display"} {
		if IsFontFilePath(value) {
			t.Fatalf("IsFontFilePath(%q) = true", value)
		}
	}
}

func TestLoadFontFile(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "Brand.OTF")
	if err := os.WriteFile(valid, minimalTestFont("OTTO"), 0o644); err != nil {
		t.Fatal(err)
	}
	font, err := LoadFontFile(valid)
	if err != nil {
		t.Fatal(err)
	}
	if font.Ext != ".otf" || len(font.Data) != 52 || len(font.Hash()) != 64 {
		t.Fatalf("font = ext %q len %d hash %q", font.Ext, len(font.Data), font.Hash())
	}

	notFont := filepath.Join(dir, "notes.ttf")
	if err := os.WriteFile(notFont, []byte("plain text, not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	truncated := filepath.Join(dir, "truncated.ttf")
	if err := os.WriteFile(truncated, testFontBytes[:40], 0o644); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		filepath.Join(dir, "missing.ttf"): "read font file",
		notFont:                           "not a TrueType or OpenType font",
		truncated:                         "not a TrueType or OpenType font",
		filepath.Join(dir, "Brand.woff"):  "must end in .ttf, .otf, or .ttc",
	} {
		if _, err := LoadFontFile(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadFontFile(%q) = %v, want %q", path, err, want)
		}
	}
}

func TestFrameResumeFingerprintCoversTextBoxAndFontFile(t *testing.T) {
	base := FrameResumeFingerprint{SourceHash: "abc", Device: "iphone-air", Title: "Home"}
	withBox := base
	withBox.TextBox = "on|#000|8|"
	withFont := base
	withFont.FontFileHash = "deadbeef"
	seen := map[string]string{}
	for name, fp := range map[string]FrameResumeFingerprint{"base": base, "box": withBox, "font": withFont} {
		sum := FingerprintFrameResume(fp)
		if other, ok := seen[sum]; ok {
			t.Fatalf("%s and %s share fingerprint %s", name, other, sum)
		}
		seen[sum] = name
	}
}

// TestFrameResumeFingerprintUnchangedWithoutNewFields keeps existing resume
// state valid: inputs that use neither a text box nor a font file hash exactly
// as they did before those fields existed.
func TestFrameResumeFingerprintUnchangedWithoutNewFields(t *testing.T) {
	fp := FrameResumeFingerprint{SourceHash: "abc", Device: "iphone-air", Title: "Home", Font: "Helvetica", TextPosition: "top"}
	legacy := sha256.Sum256([]byte(strings.Join([]string{
		frameResumeSchema, pinnedKoubouVersion, fp.SourceHash, fp.Device, fp.Title, fp.Subtitle,
		fp.TitleColor, fp.SubtitleColor, fp.Background, fp.OverlayHash, fp.FrameColor, fp.Font, fp.TextPosition,
	}, "\x00")))
	if got := FingerprintFrameResume(fp); got != hex.EncodeToString(legacy[:]) {
		t.Fatalf("fingerprint changed for inputs without text box or font file: %s", got)
	}
}

func TestIsParseableFont(t *testing.T) {
	collection := []byte("ttcf\x00\x01\x00\x00")
	collection = binary.BigEndian.AppendUint32(collection, 1)
	collection = binary.BigEndian.AppendUint32(collection, 16)
	font := minimalTestFont("true")
	// Table offsets in a collection are relative to the file start.
	for index := range 2 {
		record := 12 + 16*index
		binary.BigEndian.PutUint32(font[record+8:], binary.BigEndian.Uint32(font[record+8:])+16)
	}
	collection = append(collection, font...)
	if !isParseableFont(collection) {
		t.Fatal("valid collection rejected")
	}
	noCmap := minimalTestFont("OTTO")
	copy(noCmap[12:16], "glyf")
	missingTable := minimalTestFont("OTTO")
	binary.BigEndian.PutUint32(missingTable[12+12:], 4096)
	for name, data := range map[string][]byte{
		"empty":              nil,
		"no tables":          append([]byte("OTTO\x00\x00"), make([]byte, 6)...),
		"no cmap":            noCmap,
		"table past the end": missingTable,
		"collection offset":  append([]byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x01"), 0xff, 0xff, 0xff, 0xff),
	} {
		if isParseableFont(data) {
			t.Fatalf("%s accepted", name)
		}
	}
	// Real system fonts, when present, must pass.
	for _, path := range []string{"/System/Library/Fonts/Supplemental/Chalkduster.ttf", "/System/Library/Fonts/Helvetica.ttc"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !isParseableFont(data) {
			t.Fatalf("system font %s rejected", path)
		}
	}
}
