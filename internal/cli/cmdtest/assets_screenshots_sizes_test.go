package cmdtest

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAssetsScreenshotsSizesRejectsAllWithDisplayType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "sizes",
			"--all",
			"--display-type", "APP_IPHONE_65",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--display-type and --all are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error in stderr, got %q", stderr)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
}

func TestAssetsScreenshotsSizesRejectsInvalidDisplayTypeAsUsageError(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{
		"screenshots", "sizes",
		"--display-type", "not-a-device",
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "unsupported screenshot display type") {
		t.Fatalf("expected invalid display-type error, got %q", stderr)
	}
}

func TestAssetsScreenshotsUploadRejectsInvalidDimensionsBeforeNetwork(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.png")
	writePNG(t, path, 100, 100)

	reg := asctest.NewRegistry(t)
	reg.Install()

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
	if calls := len(reg.Requests()); calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
}

func TestAssetsScreenshotsUploadRejectsInvalidDeviceTypeAsUsageError(t *testing.T) {
	dir := t.TempDir()
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "upload",
			"--version-localization", "LOC_ID",
			"--path", dir,
			"--device-type", "not-a-device",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected flag.ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "unsupported screenshot display type") {
		t.Fatalf("expected invalid device-type error, got %q", stderr)
	}
}

func TestAssetsScreenshotsUploadSuggestsMatchingDisplayTypeBeforeNetwork(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	path := filepath.Join(dir, "known-size-wrong-type.png")
	writePNG(t, path, 1206, 2622)

	reg := asctest.NewRegistry(t)
	reg.Install()

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"screenshots", "upload",
			"--version-localization", "LOC_ID",
			"--path", path,
			"--device-type", "IPHONE_67",
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
	if !strings.Contains(runErr.Error(), "This size matches: APP_IPHONE_61") {
		t.Fatalf("expected display type suggestion in error, got %q", runErr.Error())
	}
	if calls := len(reg.Requests()); calls != 0 {
		t.Fatalf("expected no network calls, got %d", calls)
	}
}

func TestAssetsScreenshotsUploadAcceptsIPhone69AliasAndLatestDimensions(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	path := filepath.Join(dir, "valid-iphone69.png")
	writePNG(t, path, 1320, 2868)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/LOC_ID/appScreenshotSets"), func(*http.Request) (*http.Response, error) {
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
	if len(reg.Requests()) == 0 {
		t.Fatal("expected at least one network call after successful local validation")
	}
}

func writePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}
