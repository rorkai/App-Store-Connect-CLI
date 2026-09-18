package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAppClipAndPreviewLayoutRoundTrip(t *testing.T) {
	root := t.TempDir()
	metadata := filepath.Join(root, "metadata")
	header := filepath.Join(t.TempDir(), "header.png")
	if err := os.WriteFile(header, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(t.TempDir(), "preview.mp4")
	if err := os.WriteFile(video, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := AppClipLayout{
		Action:       "OPEN",
		Subtitles:    map[string]string{"en-US": "Clip subtitle"},
		HeaderImages: map[string]string{"en-US": header},
	}
	previews := []PreviewLayout{{Locale: "en-US", DeviceType: "iphone_65", FileName: "preview.mp4", Path: video, PosterFrame: "00:00:01"}}
	if err := writeAppClipLayout(metadata, original); err != nil {
		t.Fatal(err)
	}
	if err := writePreviewLayout(root, previews); err != nil {
		t.Fatal(err)
	}
	readClip, present, err := readAppClipLayout(metadata)
	if err != nil || !present || readClip.Action != "OPEN" || readClip.Subtitles["en-US"] != "Clip subtitle" {
		t.Fatalf("clip = %#v present=%v err=%v", readClip, present, err)
	}
	readPreviews, err := readPreviewLayout(root)
	if err != nil || len(readPreviews) != 1 || readPreviews[0].PosterFrame != "00:00:01" {
		t.Fatalf("previews = %#v err=%v", readPreviews, err)
	}
	exported := t.TempDir()
	exportedMeta := filepath.Join(exported, "metadata")
	if err := writeAppClipLayout(exportedMeta, readClip); err != nil {
		t.Fatal(err)
	}
	if err := writePreviewLayout(exported, readPreviews); err != nil {
		t.Fatal(err)
	}
	againClip, _, err := readAppClipLayout(exportedMeta)
	if err != nil {
		t.Fatal(err)
	}
	againPreviews, err := readPreviewLayout(exported)
	if err != nil {
		t.Fatal(err)
	}
	if againClip.Action != readClip.Action || againClip.Subtitles["en-US"] != readClip.Subtitles["en-US"] {
		t.Fatalf("clip diff %#v vs %#v", againClip, readClip)
	}
	if len(againPreviews) != 1 || againPreviews[0].FileName != readPreviews[0].FileName || againPreviews[0].PosterFrame != readPreviews[0].PosterFrame {
		t.Fatalf("preview diff %#v", againPreviews)
	}
	if warning := missingAppClipWarning(false); warning == nil {
		t.Fatal("expected missing App Clip warning")
	}
	if warning := missingAppClipWarning(true); warning != nil {
		t.Fatal("folder presence should not be only a warning")
	}
}
