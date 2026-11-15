package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shots"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/screenshots"
)

func TestShotsFrame_FrameColorAndTextControlsReachRenderer(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))

	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(100, 140))
	outputDir := filepath.Join(t.TempDir(), "framed")
	calls := 0
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		calls++
		if req.Device != "ipad-pro-13" || req.FrameColor != "space-gray" {
			t.Fatalf("device/color = %q/%q", req.Device, req.FrameColor)
		}
		if req.OutputPath != filepath.Join(outputDir, "raw-ipad-pro-13.png") {
			t.Fatalf("output path = %q", req.OutputPath)
		}
		canvas := req.Canvas
		if canvas == nil || canvas.Title != "Home" || canvas.Font != "Helvetica" || canvas.TextPosition != screenshots.TextPositionBottom || canvas.BGColor != "#111111" || canvas.TitleColor != "#fafafa" {
			t.Fatalf("canvas = %+v", canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device, UploadWidth: 2064, UploadHeight: 2752}), nil
	})

	stdout, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input", rawPath,
		"--output-dir", outputDir,
		"--device", "iPad Pro 13",
		"--frame-color", "Space Gray",
		"--title", "Home",
		"--title-color", "#fafafa",
		"--font", "Helvetica",
		"--text-position", "bottom",
		"--bg-color", "#111111",
		"--output", "json",
	}...)
	if err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
	if calls != 1 {
		t.Fatalf("frame calls = %d", calls)
	}
	var result screenshots.FrameResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("stdout is not a frame result: %v (%q)", err, stdout)
	}
	if result.Device != "ipad-pro-13" {
		t.Fatalf("result = %+v", result)
	}
}

func TestShotsFrame_BGColorAppliesToBezelDevice(t *testing.T) {
	rawPath := filepath.Join(t.TempDir(), "raw.png")
	writeFramePNG(t, rawPath, makeRawImage(100, 220))
	outputDir := filepath.Join(t.TempDir(), "framed")
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		if req.Device != "iphone-air" || req.Canvas == nil || req.Canvas.BGColor != "#fff" {
			t.Fatalf("request = %+v canvas %+v", req, req.Canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})
	if _, stderr, err := runRootWithDeps(t, nil, []string{"screenshots", "frame", "--input", rawPath, "--output-dir", outputDir, "--bg-color", "#fff", "--output", "json"}...); err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
}

func TestShotsFrame_RejectsUnsupportedFrameAndTextOptions(t *testing.T) {
	inputDir := t.TempDir()
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown device lists tablet and watch frames",
			args:    []string{"--input", "/tmp/raw.png", "--device", "ipad"},
			wantErr: "ipad-pro-13, ipad-pro-11, ipad-air-13, ipad-air-11, ipad-mini, watch-series-11, watch-ultra-3, apple-tv",
		},
		{
			name:    "unknown frame color",
			args:    []string{"--input", "/tmp/raw.png", "--device", "iphone-17-pro", "--frame-color", "gold"},
			wantErr: `--frame-color: unsupported frame color "gold" for device iphone-17-pro (allowed: silver, cosmic-orange, deep-blue)`,
		},
		{
			name:    "frame color on canvas device",
			args:    []string{"--input", "/tmp/raw.png", "--device", "mac", "--frame-color", "silver"},
			wantErr: "--frame-color: device mac has no frame color variants",
		},
		{
			name:    "invalid text position",
			args:    []string{"--input", "/tmp/raw.png", "--title", "Home", "--text-position", "middle"},
			wantErr: `--text-position: unsupported text position "middle" (allowed: top, bottom)`,
		},
		{
			name:    "font without text",
			args:    []string{"--input", "/tmp/raw.png", "--font", "Helvetica"},
			wantErr: "--font requires --title, --subtitle, or --overlay-config",
		},
		{
			name:    "text position without text",
			args:    []string{"--input", "/tmp/raw.png", "--text-position", "bottom"},
			wantErr: "--text-position requires --title, --subtitle, or --overlay-config",
		},
		{
			name:    "frame color with config",
			args:    []string{"--config", "/tmp/frame.yaml", "--frame-color", "silver"},
			wantErr: "--frame-color, --font, and --text-position cannot be used with --config",
		},
		{
			name:    "input dir with input",
			args:    []string{"--input-dir", inputDir, "--input", "/tmp/raw.png"},
			wantErr: "use either --input, --input-dir, or --config",
		},
		{
			name:    "input dir with config",
			args:    []string{"--input-dir", inputDir, "--config", "/tmp/frame.yaml"},
			wantErr: "use either --input, --input-dir, or --config",
		},
		{
			name:    "input dir with output path",
			args:    []string{"--input-dir", inputDir, "--output-path", "/tmp/out.png"},
			wantErr: "--output-path and --name cannot be used with --input-dir",
		},
		{
			name:    "input dir with name",
			args:    []string{"--input-dir", inputDir, "--name", "home"},
			wantErr: "--output-path and --name cannot be used with --input-dir",
		},
		{
			name:    "output dir equals input dir",
			args:    []string{"--input-dir", inputDir, "--output-dir", inputDir},
			wantErr: "--output-dir must differ from --input-dir",
		},
		{
			name:    "output dir aliases input dir",
			args:    []string{"--input-dir", inputDir, "--output-dir", filepath.Join(inputDir, ".")},
			wantErr: "--output-dir must differ from --input-dir",
		},
		{
			name:    "input dir without png files",
			args:    []string{"--input-dir", inputDir},
			wantErr: "--input-dir contains no PNG files",
		},
		{
			name:    "watch rejects batch and style flags",
			args:    []string{"--config", "/tmp/frame.yaml", "--watch", "--frame-color", "silver", "--font", "Arial", "--input-dir", inputDir, "--text-position", "top"},
			wantErr: "--font, --frame-color, --input-dir, --text-position cannot be used with --watch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			installMockFrame(t, func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error) {
				t.Fatal("renderer must not run for a usage error")
				return nil, nil
			})
			assertUsageExit(t, append([]string{"screenshots", "frame"}, test.args...), test.wantErr)
		})
	}
}

func TestShotsFrame_RejectsTextStyleWhenOverlayHasNoText(t *testing.T) {
	overlayPath := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(overlayPath, []byte(`{"default":{"background":"#111111"},"data":[{"filter":"paywall","title":"Upgrade"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rawPath := filepath.Join(t.TempDir(), "home.png")
	writeFramePNG(t, rawPath, makeRawImage(20, 40))
	inputDir := t.TempDir()
	writeFramePNG(t, filepath.Join(inputDir, "home.png"), makeRawImage(20, 40))
	installMockFrame(t, func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		t.Fatal("renderer must not run for a usage error")
		return nil, nil
	})
	assertUsageExit(t, []string{"screenshots", "frame", "--input", rawPath, "--output-dir", t.TempDir(), "--overlay-config", overlayPath, "--font", "Helvetica"},
		"--font has no effect: --overlay-config supplies no title or keyword for the framed input")
	assertUsageExit(t, []string{"screenshots", "frame", "--input-dir", inputDir, "--output-dir", t.TempDir(), "--overlay-config", overlayPath, "--text-position", "bottom"},
		"--text-position has no effect: --overlay-config supplies no title or keyword for the framed input")
}

func TestShotsFrame_InputDirFramesEveryPNG(t *testing.T) {
	inputDir := t.TempDir()
	writeFramePNG(t, filepath.Join(inputDir, "b-settings.png"), makeRawImage(20, 40))
	writeFramePNG(t, filepath.Join(inputDir, "a-home.PNG"), makeRawImage(20, 40))
	if err := os.WriteFile(filepath.Join(inputDir, "notes.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(inputDir, "nested.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(overlayPath, []byte(`{"default":{"title":"App"},"data":[{"filter":"settings","title":"Settings","keyword":"Yours"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(t.TempDir(), "framed")

	var seen []string
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		seen = append(seen, filepath.Base(req.OutputPath)+"="+req.Canvas.Title+"/"+req.Canvas.Subtitle)
		if req.Device != "watch-ultra-3" || req.FrameColor != "black-ocean-band-black" || req.Canvas.TextPosition != screenshots.TextPositionTop {
			t.Fatalf("request = %+v canvas %+v", req, req.Canvas)
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{
			Device:      req.Device,
			FramePath:   "AW Ultra 3 - Black + Ocean Band Black",
			DisplayType: "APP_WATCH_ULTRA",
			Width:       422,
			Height:      514,
		}), nil
	})

	stdout, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input-dir", inputDir,
		"--output-dir", outputDir,
		"--device", "watch-ultra-3",
		"--overlay-config", overlayPath,
		"--output", "json",
	}...)
	if err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
	if want := []string{"a-home-watch-ultra-3.png=App/", "b-settings-watch-ultra-3.png=Settings/Yours"}; !slices.Equal(seen, want) {
		t.Fatalf("framed inputs = %v, want %v", seen, want)
	}
	var receipt asc.ScreenshotFrameBatchResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
	}
	if receipt.Total != 2 || receipt.Framed != 2 || receipt.Failed != 0 || receipt.Skipped != 0 || receipt.Device != "watch-ultra-3" || receipt.FrameColor != "black-ocean-band-black" || receipt.Resume {
		t.Fatalf("receipt = %+v", receipt)
	}
	wantPaths := []string{
		filepath.Join(outputDir, "a-home-watch-ultra-3.png"),
		filepath.Join(outputDir, "b-settings-watch-ultra-3.png"),
	}
	for index, file := range receipt.Files {
		if file.Path != wantPaths[index] || file.Status != asc.ScreenshotFrameBatchStatusFramed || file.DisplayType != "APP_WATCH_ULTRA" || file.Width != 422 {
			t.Fatalf("file %d = %+v", index, file)
		}
	}
}

func TestShotsFrame_InputDirRendersEnumeratedBytesNotLaterSymlink(t *testing.T) {
	inputDir := t.TempDir()
	writeFramePNG(t, filepath.Join(inputDir, "01-home.png"), makeRawImage(20, 40))
	writeFramePNG(t, filepath.Join(inputDir, "02-search.png"), makeRawImage(20, 40))
	outside := filepath.Join(t.TempDir(), "outside.png")
	writeFramePNG(t, outside, makeRawImage(30, 60))
	outsideBytes, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}

	var rendered []string
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		name := filepath.Base(req.OutputPath)
		rendered = append(rendered, name)
		data, err := os.ReadFile(req.InputPath)
		if err != nil {
			t.Fatalf("read render input: %v", err)
		}
		if string(data) == string(outsideBytes) {
			t.Fatalf("%s rendered bytes from outside --input-dir", name)
		}
		if strings.HasPrefix(name, "01-home") {
			// Swap the next enumerated input for a symlink that escapes
			// --input-dir before the batch reaches it.
			second := filepath.Join(inputDir, "02-search.png")
			if err := os.Remove(second); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, second); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})

	stdout, stderr, err := runRootWithDeps(t, nil, []string{
		"screenshots", "frame",
		"--input-dir", inputDir,
		"--output-dir", filepath.Join(t.TempDir(), "framed"),
		"--output", "json",
	}...)
	if err == nil {
		t.Fatalf("expected the replaced input to fail (stdout %q)", stdout)
	}
	if code := rootcmd.ExitCodeFromError(err); code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d (err %v, stderr %q)", code, rootcmd.ExitError, err, stderr)
	}
	if want := []string{"01-home-iphone-air.png"}; !slices.Equal(rendered, want) {
		t.Fatalf("rendered %v, want %v", rendered, want)
	}
	var receipt asc.ScreenshotFrameBatchResult
	if err := json.Unmarshal([]byte(stdout), &receipt); err != nil {
		t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
	}
	if receipt.Framed != 1 || receipt.Failed != 1 || receipt.Files[1].Status != asc.ScreenshotFrameBatchStatusFailed || !strings.Contains(receipt.Files[1].Error, "snapshot input") {
		t.Fatalf("receipt = %+v", receipt)
	}
}

func writeCaseDistinctFrameInputs(t *testing.T) string {
	t.Helper()
	inputDir := t.TempDir()
	writeFramePNG(t, filepath.Join(inputDir, "home.png"), makeRawImage(20, 40))
	if _, err := os.Stat(filepath.Join(inputDir, "HOME.png")); err == nil {
		t.Skip("case-insensitive filesystem cannot hold both inputs")
	}
	writeFramePNG(t, filepath.Join(inputDir, "HOME.png"), makeRawImage(20, 40))
	return inputDir
}

func TestShotsFrame_InputDirRejectsCaseInsensitiveOutputCollision(t *testing.T) {
	inputDir := writeCaseDistinctFrameInputs(t)
	t.Cleanup(shots.SetFrameOutputFoldsCase(true))
	installMockFrame(t, func(context.Context, screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		t.Fatal("renderer must not run when outputs collide")
		return nil, nil
	})
	assertUsageExit(t, []string{"screenshots", "frame", "--input-dir", inputDir, "--output-dir", t.TempDir()}, "--input-dir inputs HOME.png and home.png map to the same output")
}

func TestShotsFrame_InputDirKeepsCaseDistinctOutputsOnCaseSensitiveFilesystem(t *testing.T) {
	inputDir := writeCaseDistinctFrameInputs(t)
	outputDir := t.TempDir()
	var rendered []string
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		rendered = append(rendered, filepath.Base(req.OutputPath))
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})
	_, stderr, err := runRootWithDeps(t, nil, []string{"screenshots", "frame", "--input-dir", inputDir, "--output-dir", outputDir, "--output", "json"}...)
	if err != nil {
		t.Fatalf("run error = %v (stderr %q)", err, stderr)
	}
	if want := []string{"HOME-iphone-air.png", "home-iphone-air.png"}; !slices.Equal(rendered, want) {
		t.Fatalf("rendered %v, want %v", rendered, want)
	}
}

func TestShotsFrame_InputDirReportsPartialFailureAndResumes(t *testing.T) {
	workDir := t.TempDir()
	t.Chdir(workDir)
	inputDir := filepath.Join(workDir, "raw")
	if err := os.Mkdir(inputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"01-home.png", "02-search.png", "03-paywall.png"} {
		writeFramePNG(t, filepath.Join(inputDir, name), makeRawImage(20, 40))
	}
	outputDir := filepath.Join(workDir, "framed")

	failSearch := true
	var rendered []string
	installMockFrame(t, func(_ context.Context, req screenshots.FrameRequest) (*screenshots.FrameResult, error) {
		name := filepath.Base(req.OutputPath)
		rendered = append(rendered, name)
		if failSearch && strings.HasPrefix(name, "02-search") {
			return nil, errors.New("koubou generation failed: font not found")
		}
		return frameResultWithWrittenPNG(t, req.OutputPath, screenshots.FrameResult{Device: req.Device}), nil
	})

	args := []string{
		"screenshots", "frame",
		"--input-dir", inputDir,
		"--output-dir", outputDir,
		"--device", "ipad-mini",
		"--title", "Plan",
		"--resume",
		"--output", "json",
	}
	stdout, stderr, err := runRootWithDeps(t, nil, args...)
	if err == nil {
		t.Fatal("expected partial batch failure")
	}
	if code := rootcmd.ExitCodeFromError(err); code != rootcmd.ExitError {
		t.Fatalf("exit code = %d, want %d (err %v)", code, rootcmd.ExitError, err)
	}
	if !strings.Contains(stderr, "1 of 3 screenshots failed to frame") {
		t.Fatalf("stderr = %q", stderr)
	}
	var first asc.ScreenshotFrameBatchResult
	if err := json.Unmarshal([]byte(stdout), &first); err != nil {
		t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
	}
	if first.Framed != 2 || first.Failed != 1 || first.Files[1].Status != asc.ScreenshotFrameBatchStatusFailed || !strings.Contains(first.Files[1].Error, "font not found") {
		t.Fatalf("first receipt = %+v", first)
	}
	if want := []string{"01-home-ipad-mini.png", "02-search-ipad-mini.png", "03-paywall-ipad-mini.png"}; !slices.Equal(rendered, want) {
		t.Fatalf("first run rendered %v, want %v", rendered, want)
	}

	failSearch = false
	rendered = nil
	stdout, stderr, err = runRootWithDeps(t, nil, args...)
	if err != nil {
		t.Fatalf("resume run error = %v (stderr %q)", err, stderr)
	}
	if want := []string{"02-search-ipad-mini.png"}; !slices.Equal(rendered, want) {
		t.Fatalf("resume rendered %v, want only the failed input %v", rendered, want)
	}
	var second asc.ScreenshotFrameBatchResult
	if err := json.Unmarshal([]byte(stdout), &second); err != nil {
		t.Fatalf("stdout is not a batch receipt: %v (%q)", err, stdout)
	}
	if second.Skipped != 2 || second.Framed != 1 || second.Failed != 0 || !second.Resume {
		t.Fatalf("second receipt = %+v", second)
	}
	statuses := []string{second.Files[0].Status, second.Files[1].Status, second.Files[2].Status}
	if want := []string{"skipped", "framed", "skipped"}; !slices.Equal(statuses, want) {
		t.Fatalf("statuses = %v, want %v", statuses, want)
	}
	if _, err := os.Stat(filepath.Join(workDir, screenshots.FrameResumeStateRel)); err != nil {
		t.Fatalf("resume state missing: %v", err)
	}
}
