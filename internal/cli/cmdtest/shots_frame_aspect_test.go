package cmdtest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

const frameAspectWarningText = "will be letterboxed"

func installAspectMockFrame(t *testing.T) *int {
	t.Helper()
	calls := 0
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		calls++
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})
	return &calls
}

func TestShotsFrame_WarnsWhenInputAspectMismatchesDeviceScreen(t *testing.T) {
	dir := t.TempDir()
	rawPath := filepath.Join(dir, "home.png")
	writeFramePNG(t, rawPath, makeRawImage(120, 260))
	outputDir := filepath.Join(dir, "framed")
	calls := installAspectMockFrame(t)

	stdout, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", outputDir,
		"--device", "watch-ultra-3",
		"--output", "json",
	}...)
	if err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
	if *calls != 1 {
		t.Fatalf("frame calls = %d, want 1", *calls)
	}
	want := "Warning: " + rawPath + " is 120x260, but watch-ultra-3 screenshots are 422x514; the image " + frameAspectWarningText + " inside the device screen\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	var result screenshots.FrameResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not a frame result: %v (%q)", err, stdout)
	}
}

func TestShotsFrame_NoAspectWarningForNativeCapture(t *testing.T) {
	for _, test := range []struct {
		device        string
		width, height int
	}{
		{device: "watch-series-11", width: 416, height: 496},
		{device: "watch-ultra-3", width: 422, height: 514},
		{device: "apple-tv", width: 192, height: 108},
	} {
		t.Run(test.device, func(t *testing.T) {
			dir := t.TempDir()
			rawPath := filepath.Join(dir, "home.png")
			writeFramePNG(t, rawPath, makeRawImage(test.width, test.height))
			installAspectMockFrame(t)

			_, stderr, err := runRootWithDeps(t, nil, []string{
				"screenshots", "frame",
				"--input", rawPath,
				"--output-dir", filepath.Join(dir, "framed"),
				"--device", test.device,
				"--output", "json",
			}...)
			if err != nil {
				t.Fatalf("run error = %v (stderr %q)", err, stderr)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want no aspect warning", stderr)
			}
		})
	}
}

func TestShotsFrame_ResumeWarnsWhenInputAspectMismatchesDeviceScreen(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(dir, "config.json"))
	rawPath := filepath.Join(dir, "home.png")
	writeFramePNG(t, rawPath, makeRawImage(300, 300))
	calls := installAspectMockFrame(t)

	args := []string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", filepath.Join(dir, "framed"),
		"--device", "apple-tv",
		"--resume",
		"--output", "json",
	}
	_, stderr, err := runRootWithDeps(t, nil, args...)
	if err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
	if !strings.Contains(stderr, rawPath+" is 300x300, but apple-tv screenshots are 3840x2160") {
		t.Fatalf("stderr = %q, want aspect warning", stderr)
	}
	// A resumed skip renders nothing, so it does not repeat the warning.
	_, stderr, err = runRootWithDeps(t, nil, args...)
	if err != nil {
		t.Fatalf("resume run error = %v (stderr %q)", err, stderr)
	}
	if *calls != 1 {
		t.Fatalf("frame calls = %d, want 1", *calls)
	}
	if stderr != "" {
		t.Fatalf("resumed stderr = %q, want no warning", stderr)
	}
}

func TestShotsFrame_InputDirWarnsOnlyForMismatchedInputs(t *testing.T) {
	dir := t.TempDir()
	inputDir := filepath.Join(dir, "raw")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mismatched := filepath.Join(inputDir, "a-iphone.png")
	native := filepath.Join(inputDir, "b-watch.png")
	writeFramePNG(t, mismatched, makeRawImage(120, 260))
	writeFramePNG(t, native, makeRawImage(416, 496))
	calls := installAspectMockFrame(t)

	_, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input-dir", inputDir,
		"--output-dir", filepath.Join(dir, "framed"),
		"--device", "watch-series-11",
		"--output", "json",
	}...)
	if err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
	if *calls != 2 {
		t.Fatalf("frame calls = %d, want 2", *calls)
	}
	want := "Warning: " + mismatched + " is 120x260, but watch-series-11 screenshots are 416x496; the image " + frameAspectWarningText + " inside the device screen\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
