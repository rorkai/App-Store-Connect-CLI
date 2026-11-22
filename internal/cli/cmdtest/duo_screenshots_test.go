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
	cases := []struct {
		name        string
		displayType string
		wantType    string
		wantFamily  string
	}{
		{name: "app", displayType: "IPHONE_DUO", wantType: "APP_IPHONE_DUO", wantFamily: "APP"},
		{name: "imessage", displayType: "IMESSAGE_APP_IPHONE_DUO", wantType: "IMESSAGE_APP_IPHONE_DUO", wantFamily: "IMESSAGE"},
	}
	want := []asc.ScreenshotDimension{{Width: 1398, Height: 2034}, {Width: 2007, Height: 2853}, {Width: 2034, Height: 1398}, {Width: 2853, Height: 2007}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runRootWithDeps(t, nil, []string{"screenshots", "sizes", "--display-type", tc.displayType, "--output", "json"}...)
			if err != nil {
				t.Fatalf("sizes: %v; stderr: %s", err, stderr)
			}
			var result asc.ScreenshotSizesResult
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Sizes) != 1 || result.Sizes[0].DisplayType != tc.wantType || result.Sizes[0].Family != tc.wantFamily {
				t.Fatalf("unexpected entries: %+v", result.Sizes)
			}
			if len(result.Sizes[0].Dimensions) != len(want) {
				t.Fatalf("dimensions: %+v", result.Sizes[0].Dimensions)
			}
			for i, dim := range want {
				if result.Sizes[0].Dimensions[i] != dim {
					t.Fatalf("dimension %d = %+v, want %+v", i, result.Sizes[0].Dimensions[i], dim)
				}
			}
		})
	}

	stdout, stderr, err := runRootWithDeps(t, nil, []string{"screenshots", "sizes", "--all", "--output", "json"}...)
	if err != nil {
		t.Fatalf("sizes --all: %v; stderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, `"displayType":"IMESSAGE_APP_IPHONE_DUO"`) {
		t.Fatal("sizes --all omitted IMESSAGE_APP_IPHONE_DUO")
	}
}

func TestScreenshotsUploadIPhoneDuoDryRun(t *testing.T) {
	cases := []struct {
		name        string
		deviceType  string
		displayType string
	}{
		{name: "app", deviceType: "IPHONE_DUO", displayType: "APP_IPHONE_DUO"},
		{name: "imessage", deviceType: "IMESSAGE_APP_IPHONE_DUO", displayType: "IMESSAGE_APP_IPHONE_DUO"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			path := filepath.Join(t.TempDir(), "duo.png")
			writePNG(t, path, 2007, 2853)
			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/LOC_DUO/appScreenshotSets"),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-duo","attributes":{"screenshotDisplayType":"`+tc.displayType+`"}}],"links":{}}`))
			reg.Register(asctest.GET("/v1/appScreenshotSets/set-duo/appScreenshots"),
				asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))
			stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), []string{"screenshots", "upload", "--version-localization", "LOC_DUO", "--path", path, "--device-type", tc.deviceType, "--dry-run", "--output", "json"}...)
			if err != nil {
				t.Fatalf("upload: %v; stderr: %s", err, stderr)
			}
			if !strings.Contains(stdout, `"displayType":"`+tc.displayType+`"`) || !strings.Contains(stdout, `"state":"would-upload"`) {
				t.Fatalf("unexpected output: %s", stdout)
			}
		})
	}
}
