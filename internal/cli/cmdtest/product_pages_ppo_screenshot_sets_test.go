package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestProductPagesExperimentTreatmentLocalizationScreenshotSetsGroupHelpIncludesUploadAndSync(t *testing.T) {
	root := RootCommand("1.2.3")

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	help := stdout + stderr
	for _, token := range []string{"upload", "sync"} {
		if !strings.Contains(help, token) {
			t.Fatalf("expected help to contain %q, got %q", token, help)
		}
	}
}

func TestRunProductPagesExperimentTreatmentLocalizationScreenshotSetsUsageErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "upload missing localization",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--path", "./shots", "--device-type", "IPHONE_65"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "upload missing path",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--localization-id", "tloc-1", "--device-type", "IPHONE_65"},
			wantErr: "--path is required",
		},
		{
			name:    "upload missing device type",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--localization-id", "tloc-1", "--path", "./shots"},
			wantErr: "--device-type is required",
		},
		{
			name:    "sync missing confirm",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "sync", "--localization-id", "tloc-1", "--path", "./shots", "--device-type", "IPHONE_65"},
			wantErr: "--confirm is required",
		},
		{
			name:    "upload invalid device type",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload", "--localization-id", "tloc-1", "--path", "./shots", "--device-type", "NOT_A_DEVICE"},
			wantErr: "unsupported screenshot display type",
		},
		{
			name:    "preview upload missing localization",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload", "--path", "./previews", "--device-type", "IPHONE_65"},
			wantErr: "--localization-id is required",
		},
		{
			name:    "preview upload missing path",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload", "--localization-id", "tloc-1", "--device-type", "IPHONE_65"},
			wantErr: "--path is required",
		},
		{
			name:    "preview upload missing device type",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload", "--localization-id", "tloc-1", "--path", "./previews"},
			wantErr: "--device-type is required",
		},
		{
			name:    "preview upload invalid device type",
			args:    []string{"product-pages", "experiments", "treatments", "localizations", "preview-sets", "upload", "--localization-id", "tloc-1", "--path", "./previews", "--device-type", "NOT_A_DEVICE"},
			wantErr: "unsupported preview type",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				code := cmd.Run(tc.args, "1.2.3")
				if code != cmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
				}
			})

			if !strings.Contains(stderr, tc.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", tc.wantErr, stderr)
			}
		})
	}
}

func TestProductPagesExperimentTreatmentLocalizationScreenshotSetsUploadSuccessJSON(t *testing.T) {
	setupAuth(t)

	filePath := writePPOCmdtestPNG(t, t.TempDir(), "01-home.png", 1242, 2688)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionExperimentTreatmentLocalizations/tloc-1/appScreenshotSets":
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshotSets":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read screenshot-set body: %v", err)
			}
			if !strings.Contains(string(body), "appStoreVersionExperimentTreatmentLocalization") {
				t.Fatalf("expected PPO relationship in body, got %s", string(body))
			}
			return jsonHTTPResponse(http.StatusCreated, `{"data":{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return jsonHTTPResponse(http.StatusOK, `{"data":[]}`), nil
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return jsonHTTPResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/shot-1","length":%d,"offset":0}]}}}`, fileInfo.Size())), nil
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return jsonHTTPResponse(http.StatusOK, `{}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"uploaded":true}}}`), nil
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/shot-1":
			return jsonHTTPResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"sourceFileChecksum":"settled","assetDeliveryState":{"state":"COMPLETE"}}}}`), nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return jsonHTTPResponse(http.StatusNoContent, ""), nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	}))

	root := RootCommand("1.2.3")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"product-pages", "experiments", "treatments", "localizations", "screenshot-sets", "upload",
			"--localization-id", "tloc-1",
			"--path", filePath,
			"--device-type", "IPHONE_65",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if got := parsed["experimentTreatmentLocalizationId"]; got != "tloc-1" {
		t.Fatalf("expected experimentTreatmentLocalizationId=tloc-1, got %#v", got)
	}
	if got := parsed["setId"]; got != "set-1" {
		t.Fatalf("expected setId=set-1, got %#v", got)
	}
	if got := parsed["displayType"]; got != "APP_IPHONE_65" {
		t.Fatalf("expected displayType=APP_IPHONE_65, got %#v", got)
	}
	results, ok := parsed["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected one upload result, got %#v", parsed["results"])
	}
}

func writePPOCmdtestPNG(t *testing.T, dir, name string, width, height int) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}
