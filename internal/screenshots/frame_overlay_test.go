package screenshots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBezelConfigIncludesTitleOverlay(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "home.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath, _, err := createDefaultKoubouConfigAtRoot(input, frameDeviceKoubouSpecs[FrameDeviceIPhoneAir], &CanvasOptions{
		Title:    "Home",
		Subtitle: "Fast",
	}, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "content: Home") || !strings.Contains(text, "content: Fast") {
		t.Fatalf("phone frame config missing overlay text:\n%s", text)
	}
}

func TestBezelSubtitleOnlyUsesSoloPosition(t *testing.T) {
	items := bezelContentItems("/tmp/input.png", frameDeviceKoubouSpecs[FrameDeviceIPhoneAir], &CanvasOptions{Subtitle: "Fast"})
	if len(items) < 2 {
		t.Fatalf("bezel content items = %#v", items)
	}
	if items[0].Content != "Fast" || items[0].Position[1] != bezelTopSubtitleSoloY {
		t.Fatalf("subtitle-only item = %#v, want Y %q", items[0], bezelTopSubtitleSoloY)
	}
}

func TestBezelWithoutTextKeepsFullSizeFrame(t *testing.T) {
	items := bezelContentItems("/tmp/input.png", frameDeviceKoubouSpecs[FrameDeviceIPadPro13], nil)
	if len(items) != 1 {
		t.Fatalf("bezel content items = %#v, want image only", items)
	}
	image := items[0]
	if image.Position != [2]string{"50%", "50%"} || image.Scale != 1 || image.Frame == nil || !*image.Frame {
		t.Fatalf("image item = %#v, want full-size framed image", image)
	}
}

func TestBezelTextReservesBandAndScalesFrame(t *testing.T) {
	tests := []struct {
		name         string
		device       FrameDevice
		position     TextPosition
		wantTitleY   string
		wantImageY   string
		wantScale    float64
		wantTitle    int
		wantSubtitle int
		wantMaxWidth int
	}{
		// iPhone Air: canvas 1260x2736, frame 1380x2880.
		// scale = min(0.90*1260/1380, 0.74*2736/2880) = min(0.8217, 0.7030).
		{name: "phone top", device: FrameDeviceIPhoneAir, position: TextPositionTop, wantTitleY: "8%", wantImageY: "61%", wantScale: 0.703, wantTitle: 88, wantSubtitle: 50, wantMaxWidth: 1134},
		// Apple Watch Series 11: canvas 416x496, frame 560x880.
		{name: "watch bottom", device: FrameDeviceWatchSeries11, position: TextPositionBottom, wantTitleY: "85%", wantImageY: "39%", wantScale: 0.4171, wantTitle: 29, wantSubtitle: 17, wantMaxWidth: 374},
		// Apple TV: canvas 3840x2160, frame 4300x2780; font scales from the short edge.
		{name: "tv default", device: FrameDeviceAppleTV, position: "", wantTitleY: "8%", wantImageY: "61%", wantScale: 0.575, wantTitle: 151, wantSubtitle: 86, wantMaxWidth: 3456},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := bezelContentItems("/tmp/input.png", frameDeviceKoubouSpecs[test.device], &CanvasOptions{
				Title:        "Title",
				Subtitle:     "Subtitle",
				Font:         "Helvetica",
				TextPosition: test.position,
			})
			if len(items) != 3 {
				t.Fatalf("items = %#v", items)
			}
			title, subtitle, image := items[0], items[1], items[2]
			if title.Position[1] != test.wantTitleY || title.Size != test.wantTitle || title.MaxWidth != test.wantMaxWidth || title.FontFamily != "Helvetica" {
				t.Fatalf("title = %#v", title)
			}
			if subtitle.Size != test.wantSubtitle || subtitle.FontFamily != "Helvetica" || subtitle.MaxWidth != test.wantMaxWidth {
				t.Fatalf("subtitle = %#v", subtitle)
			}
			if image.Position[1] != test.wantImageY || image.Scale != test.wantScale {
				t.Fatalf("image = %#v, want Y %q scale %v", image, test.wantImageY, test.wantScale)
			}
		})
	}
}

func TestCanvasTextPositionBottomMovesWindowUp(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFrameTestPNG(t, rawPath, makeFrameTestImage(2560, 1600))

	configPath, _, workDir, err := createDefaultKoubouConfig(rawPath, frameDeviceKoubouSpecs[FrameDeviceMac], &CanvasOptions{
		Title:        "My App",
		Subtitle:     "Tagline",
		Font:         "Georgia",
		TextPosition: TextPositionBottom,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Screenshots map[string]struct {
			Content []koubouDefaultContentItem `yaml:"content"`
		} `yaml:"screenshots"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	content := cfg.Screenshots["framed"].Content
	if len(content) != 3 {
		t.Fatalf("content = %#v", content)
	}
	if content[0].Position[1] != canvasBottomTitleY || content[0].FontFamily != "Georgia" {
		t.Fatalf("title = %#v", content[0])
	}
	if content[1].Position[1] != canvasBottomSubtitleY || content[1].FontFamily != "Georgia" {
		t.Fatalf("subtitle = %#v", content[1])
	}
	if content[2].Position[1] != canvasWindowBottomTextY {
		t.Fatalf("window = %#v", content[2])
	}
}

func TestBezelBackgroundAppliesToEveryDevice(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "home.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		canvas     *CanvasOptions
		wantColors []string
	}{
		{name: "no text keeps transparent canvas", canvas: nil},
		{name: "solid background", canvas: &CanvasOptions{BGColor: "#123456"}, wantColors: []string{"#123456", "#123456"}},
		{name: "text defaults to dark gradient", canvas: &CanvasOptions{Title: "Home"}, wantColors: []string{canvasBGColorFrom, canvasBGColorTo}},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workDir := filepath.Join(dir, "work", string(rune('a'+index)))
			if err := os.MkdirAll(workDir, 0o700); err != nil {
				t.Fatal(err)
			}
			configPath, _, err := createDefaultKoubouConfigAtRoot(input, frameDeviceKoubouSpecs[FrameDeviceIPhone17Pro], test.canvas, workDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			var cfg parsedCanvasConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			background := cfg.Screenshots["framed"].Background
			if test.wantColors == nil {
				if background != nil {
					t.Fatalf("background = %+v, want none", background)
				}
				return
			}
			if background == nil || strings.Join(background.Colors, ",") != strings.Join(test.wantColors, ",") {
				t.Fatalf("background = %+v, want %v", background, test.wantColors)
			}
		})
	}
}

// TestOverlayFixtureConvertsToRendererConfigGolden pins the complete
// conversion from an --overlay-config fixture, through overlay matching, to the
// Koubou YAML handed to the renderer.
func TestOverlayFixtureConvertsToRendererConfigGolden(t *testing.T) {
	overlay, _, err := LoadOverlayConfig(filepath.Join("testdata", "overlay-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	canvas := OverlayToCanvas(MatchOverlay(overlay, "03-paywall.png"))
	canvas.Font = "Helvetica"
	canvas.TextPosition = TextPositionBottom
	canvas.TitleColor = "#fafafa"

	spec, colorID, err := resolveFrameSpec(FrameDeviceIPadPro13, "space-gray")
	if err != nil {
		t.Fatal(err)
	}
	if colorID != "space-gray" {
		t.Fatalf("color = %q", colorID)
	}

	workDir := t.TempDir()
	input := filepath.Join(workDir, "03-paywall.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath, metadata, err := createDefaultKoubouConfigAtRoot(input, spec, &canvas, workDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.FrameRef != "iPad Pro 13 - M4 - Space Gray - Portrait" || metadata.DisplayType != "APP_IPAD_PRO_3GEN_129" || metadata.UploadWidth != 2064 || metadata.UploadHeight != 2752 {
		t.Fatalf("metadata = %+v", metadata)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// Pin separators too: the generated YAML carries native paths, so a
	// Windows run joins $WORK and file names with backslashes.
	got := strings.ReplaceAll(string(data), workDir+string(filepath.Separator), "$WORK/")
	goldenPath := filepath.Join("testdata", "overlay-config.ipad-pro-13.golden.yaml")
	if os.Getenv("ASC_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	// Git may check the golden out with CRLF line endings on Windows
	// (core.autocrlf); the renderer config itself always uses LF.
	wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
	if got != wantText {
		t.Fatalf("renderer config mismatch\n--- got ---\n%s\n--- want ---\n%s", got, wantText)
	}
}

// TestGeneratedContentItemsAnchorOnCenter pins the horizontal anchor of every
// generated text and image item. Since Koubou 0.19.0 content alignment is the
// true horizontal anchor, so a 50% x position only centers an item when its
// alignment is center.
func TestGeneratedContentItemsAnchorOnCenter(t *testing.T) {
	tests := []struct {
		name   string
		device FrameDevice
		canvas *CanvasOptions
		items  int
	}{
		{name: "bezel without text", device: FrameDeviceIPhoneAir, items: 1},
		{name: "bezel text top", device: FrameDeviceIPadAir13, canvas: &CanvasOptions{Title: "Plan", Subtitle: "Trips"}, items: 3},
		{name: "bezel text bottom", device: FrameDeviceWatchUltra3, canvas: &CanvasOptions{Title: "Plan", Subtitle: "Trips", TextPosition: TextPositionBottom}, items: 3},
		{name: "landscape bezel", device: FrameDeviceAppleTV, canvas: &CanvasOptions{Title: "Plan"}, items: 2},
		{name: "canvas without text", device: FrameDeviceMac, items: 1},
		{name: "canvas text", device: FrameDeviceMac, canvas: &CanvasOptions{Title: "Plan", Subtitle: "Trips", TextPosition: TextPositionBottom}, items: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workDir := t.TempDir()
			input := filepath.Join(workDir, "home.png")
			if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
				t.Fatal(err)
			}
			configPath, _, err := createDefaultKoubouConfigAtRoot(input, frameDeviceKoubouSpecs[test.device], test.canvas, workDir, nil)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			var parsed struct {
				Screenshots map[string]struct {
					Content []struct {
						Type      string    `yaml:"type"`
						Position  [2]string `yaml:"position"`
						Alignment string    `yaml:"alignment"`
					} `yaml:"content"`
				} `yaml:"screenshots"`
			}
			if err := yaml.Unmarshal(data, &parsed); err != nil {
				t.Fatal(err)
			}
			content := parsed.Screenshots["framed"].Content
			if len(content) != test.items {
				t.Fatalf("content items = %d, want %d:\n%s", len(content), test.items, data)
			}
			for index, item := range content {
				if item.Alignment != "center" || item.Position[0] != "50%" {
					t.Fatalf("item %d (%s) alignment %q x %q, want center at 50%%:\n%s", index, item.Type, item.Alignment, item.Position[0], data)
				}
			}
		})
	}
}

// TestWatchAndTVRendererConfigGolden pins the Koubou YAML for native Apple
// Watch and Apple TV captures with titles above and below the device. Each
// golden was rendered with Koubou 0.20.0 from native-size captures (416x496,
// 422x514, and 3840x2160) and checked visually: the whole capture lands inside
// the detected screen at a uniform scale, and the text clears the device.
func TestWatchAndTVRendererConfigGolden(t *testing.T) {
	tests := []struct {
		device   FrameDevice
		input    string
		title    string
		subtitle string
	}{
		{device: FrameDeviceWatchSeries11, input: "watch.png", title: "Roll the die", subtitle: "Right on your wrist"},
		{device: FrameDeviceWatchUltra3, input: "watch.png", title: "Roll the die", subtitle: "Built for Ultra"},
		{device: FrameDeviceAppleTV, input: "tv.png", title: "Dice Oracle on TV", subtitle: "Big screen answers"},
	}
	for _, test := range tests {
		for _, position := range []TextPosition{TextPositionTop, TextPositionBottom} {
			name := string(test.device) + "." + string(position)
			t.Run(name, func(t *testing.T) {
				spec, _, err := resolveFrameSpec(test.device, "")
				if err != nil {
					t.Fatal(err)
				}
				workDir := t.TempDir()
				input := filepath.Join(workDir, test.input)
				if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
					t.Fatal(err)
				}
				canvas := &CanvasOptions{Title: test.title, Subtitle: test.subtitle, TextPosition: position}
				configPath, metadata, err := createDefaultKoubouConfigAtRoot(input, spec, canvas, workDir, nil)
				if err != nil {
					t.Fatal(err)
				}
				wantWidth, wantHeight, _ := spec.outputDimensions()
				if metadata.UploadWidth != wantWidth || metadata.UploadHeight != wantHeight || metadata.DisplayType != spec.DisplayType {
					t.Fatalf("metadata = %+v, want %dx%d %s", metadata, wantWidth, wantHeight, spec.DisplayType)
				}
				data, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				got := strings.ReplaceAll(string(data), workDir+string(filepath.Separator), "$WORK/")
				goldenPath := filepath.Join("testdata", "frame-config."+name+".golden.yaml")
				if os.Getenv("ASC_UPDATE_GOLDEN") == "1" {
					if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatal(err)
				}
				wantText := strings.ReplaceAll(string(want), "\r\n", "\n")
				if got != wantText {
					t.Fatalf("renderer config mismatch\n--- got ---\n%s\n--- want ---\n%s", got, wantText)
				}
			})
		}
	}
}
