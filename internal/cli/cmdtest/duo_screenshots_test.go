package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestScreenshotsSizesIPhoneDuo(t *testing.T) {
	stdout, stderr, err := runRootCommand(t, []string{"screenshots", "sizes", "--display-type", "IPHONE_DUO", "--output", "json"})
	if err != nil {
		t.Fatalf("sizes: %v; stderr: %s", err, stderr)
	}
	var result asc.ScreenshotSizesResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Sizes) != 1 || result.Sizes[0].DisplayType != "APP_IPHONE_DUO" {
		t.Fatalf("unexpected entries: %+v", result.Sizes)
	}
	want := []asc.ScreenshotDimension{{Width: 1398, Height: 2034}, {Width: 2007, Height: 2853}, {Width: 2034, Height: 1398}, {Width: 2853, Height: 2007}}
	if len(result.Sizes[0].Dimensions) != len(want) {
		t.Fatalf("dimensions: %+v", result.Sizes[0].Dimensions)
	}
	for i, dim := range want {
		if result.Sizes[0].Dimensions[i] != dim {
			t.Fatalf("dimension %d = %+v, want %+v", i, result.Sizes[0].Dimensions[i], dim)
		}
	}
}

func TestScreenshotsUploadIPhoneDuoDryRun(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	path := filepath.Join(t.TempDir(), "duo.png")
	writePNG(t, path, 2007, 2853)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/LOC_DUO/appScreenshotSets"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-duo","attributes":{"screenshotDisplayType":"APP_IPHONE_DUO"}}],"links":{}}`))
	reg.Register(asctest.GET("/v1/appScreenshotSets/set-duo/appScreenshots"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
	stdout, stderr, err := runRootCommand(t, []string{"screenshots", "upload", "--version-localization", "LOC_DUO", "--path", path, "--device-type", "IPHONE_DUO", "--dry-run", "--output", "json"})
	if err != nil {
		t.Fatalf("upload: %v; stderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, `"displayType":"APP_IPHONE_DUO"`) || !strings.Contains(stdout, `"state":"would-upload"`) {
		t.Fatalf("unexpected output: %s", stdout)
	}
}
