package screenshots

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFrameCanvasAllowsBezelText(t *testing.T) {
	spec := frameDeviceKoubouSpecs[FrameDeviceIPhoneAir]
	if err := validateFrameCanvas(spec, &CanvasOptions{Title: "Home", Subtitle: "Fast"}); err != nil {
		t.Fatal(err)
	}
	if err := validateFrameCanvas(spec, &CanvasOptions{BGColor: "#111111"}); err == nil {
		t.Fatal("expected bezel background to be rejected")
	}
}

func TestBezelConfigIncludesTitleOverlay(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "home.png")
	if err := os.WriteFile(input, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath, _, err := createDefaultKoubouConfigAt(input, frameDeviceKoubouSpecs[FrameDeviceIPhoneAir], &CanvasOptions{
		Title:    "Home",
		Subtitle: "Fast",
	}, dir)
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
