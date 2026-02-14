package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAssetsScreenshotsSizesOutput(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "sizes", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) == 0 {
		t.Fatal("expected sizes output, got empty list")
	}
	found := false
	for _, entry := range result.Sizes {
		if entry.DisplayType == "APP_IPHONE_65" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected APP_IPHONE_65 in sizes output")
	}
}

func TestAssetsScreenshotsSizesOutputSupportsIPhone69Alias(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "sizes", "--display-type", "IPHONE_69", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) != 1 {
		t.Fatalf("expected one filtered entry, got %d", len(result.Sizes))
	}
	if result.Sizes[0].DisplayType != "APP_IPHONE_69" {
		t.Fatalf("expected APP_IPHONE_69, got %q", result.Sizes[0].DisplayType)
	}
}

func TestAssetsScreenshotsSizesOutputSupportsIMessageIPhone69Alias(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "sizes", "--display-type", "IMESSAGE_APP_IPHONE_69", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(result.Sizes) != 1 {
		t.Fatalf("expected one filtered entry, got %d", len(result.Sizes))
	}
	if result.Sizes[0].DisplayType != "IMESSAGE_APP_IPHONE_69" {
		t.Fatalf("expected IMESSAGE_APP_IPHONE_69, got %q", result.Sizes[0].DisplayType)
	}
}

func TestAssetsScreenshotsSizesOutputIncludesMacWatchTVAndVisionDimensions(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "sizes", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}

	testCases := []struct {
		displayType string
		dimensions  []asc.ScreenshotDimension
	}{
		{
			displayType: "APP_DESKTOP",
			dimensions: []asc.ScreenshotDimension{
				{Width: 1280, Height: 800},
				{Width: 1440, Height: 900},
				{Width: 2560, Height: 1600},
				{Width: 2880, Height: 1800},
			},
		},
		{
			displayType: "APP_WATCH_ULTRA",
			dimensions: []asc.ScreenshotDimension{
				{Width: 422, Height: 514},
				{Width: 410, Height: 502},
			},
		},
		{
			displayType: "APP_APPLE_TV",
			dimensions: []asc.ScreenshotDimension{
				{Width: 1920, Height: 1080},
				{Width: 3840, Height: 2160},
			},
		},
		{
			displayType: "APP_APPLE_VISION_PRO",
			dimensions: []asc.ScreenshotDimension{
				{Width: 3840, Height: 2160},
			},
		},
	}

	for _, tc := range testCases {
		entry, found := screenshotEntryByDisplayType(result.Sizes, tc.displayType)
		if !found {
			t.Fatalf("expected %s in sizes output", tc.displayType)
		}
		for _, dim := range tc.dimensions {
			if !containsDimension(entry.Dimensions, dim) {
				t.Fatalf("expected %s to include %dx%d, got %v", tc.displayType, dim.Width, dim.Height, entry.Dimensions)
			}
		}
	}
}

func TestAssetsScreenshotsUploadRejectsInvalidDimensionsBeforeNetwork(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.png")
	writePNG(t, path, 100, 100)

	var calls int32
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return nil, fmt.Errorf("unexpected network request: %s %s", req.Method, req.URL.Path)
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "upload",
			"--version-localization", "LOC_ID",
			"--path", path,
			"--device-type", "IPHONE_35",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected validation error, got nil")
	}
	message := runErr.Error()
	if !strings.Contains(message, "100x100") {
		t.Fatalf("expected actual size in error, got %q", message)
	}
	if !strings.Contains(message, "640x960") {
		t.Fatalf("expected allowed size in error, got %q", message)
	}
	if !strings.Contains(message, "asc screenshots sizes") {
		t.Fatalf("expected hint in error, got %q", message)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
}

func TestAssetsScreenshotsUploadAcceptsIPhone69AliasAndLatestDimensions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	path := filepath.Join(dir, "valid-iphone69.png")
	writePNG(t, path, 1320, 2868)

	var calls int32
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return nil, fmt.Errorf("forced network failure after local validation")
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "upload",
			"--version-localization", "LOC_ID",
			"--path", path,
			"--device-type", "IPHONE_69",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runErr == nil {
		t.Fatal("expected network failure after validation, got nil")
	}
	if !strings.Contains(runErr.Error(), "forced network failure after local validation") {
		t.Fatalf("expected network failure error, got %q", runErr.Error())
	}
	if atomic.LoadInt32(&calls) == 0 {
		t.Fatal("expected at least one network call after successful local validation")
	}
}

func TestAssetsScreenshotsUploadAcceptsMacWatchTVAndVisionDimensions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	testCases := []struct {
		name       string
		deviceType string
		width      int
		height     int
	}{
		{name: "mac desktop", deviceType: "DESKTOP", width: 2880, height: 1800},
		{name: "watch ultra", deviceType: "WATCH_ULTRA", width: 422, height: 514},
		{name: "apple tv", deviceType: "APPLE_TV", width: 3840, height: 2160},
		{name: "vision pro", deviceType: "APPLE_VISION_PRO", width: 3840, height: 2160},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.name+".png")
			writePNG(t, path, tc.width, tc.height)

			var calls int32
			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				atomic.AddInt32(&calls, 1)
				return nil, fmt.Errorf("forced network failure after local validation")
			})

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{
					"screenshots", "upload",
					"--version-localization", "LOC_ID",
					"--path", path,
					"--device-type", tc.deviceType,
				}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if runErr == nil {
				t.Fatal("expected network failure after validation, got nil")
			}
			if !strings.Contains(runErr.Error(), "forced network failure after local validation") {
				t.Fatalf("expected network failure error, got %q", runErr.Error())
			}
			if atomic.LoadInt32(&calls) == 0 {
				t.Fatal("expected at least one network call after successful local validation")
			}
		})
	}
}

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func screenshotEntryByDisplayType(entries []asc.ScreenshotSizeEntry, displayType string) (asc.ScreenshotSizeEntry, bool) {
	for _, entry := range entries {
		if entry.DisplayType == displayType {
			return entry, true
		}
	}
	return asc.ScreenshotSizeEntry{}, false
}

func containsDimension(dimensions []asc.ScreenshotDimension, expected asc.ScreenshotDimension) bool {
	for _, dim := range dimensions {
		if dim.Width == expected.Width && dim.Height == expected.Height {
			return true
		}
	}
	return false
}
