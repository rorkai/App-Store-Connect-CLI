package cmdtest

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestRunScreenshotsUploadRejectsFormatExtensionMismatchBeforeAnyRequest(t *testing.T) {
	fanoutArgs := func(t *testing.T, extra ...string) []string {
		root := t.TempDir()
		localeDir := filepath.Join(root, "en-US")
		if err := os.MkdirAll(localeDir, 0o755); err != nil {
			t.Fatalf("create locale dir: %v", err)
		}
		writeCmdtestScreenshotJPEG(t, localeDir, "01-home.png")
		return append([]string{"--app", "123456789", "--version", "1.2.3", "--path", root, "--device-type", "IPHONE_65"}, extra...)
	}
	resumeArgs := func(t *testing.T, displayType string) []string {
		dir := t.TempDir()
		pendingPath := filepath.Join(dir, "01-home.png")
		// The original upload reserved a real PNG; the file on disk was
		// replaced with JPEG bytes under the same name before the resume.
		writeCmdtestScreenshotPNG(t, dir, "01-home.png")
		writeCmdtestScreenshotJPEG(t, dir, "01-home.png")

		artifact := map[string]any{
			"versionLocalizationId": "LOC_123",
			"rootPath":              dir,
			"setId":                 "set-1",
			"pendingFiles":          []string{pendingPath},
			"generatedAt":           "2026-08-19T00:00:00Z",
		}
		if displayType != "" {
			artifact["displayType"] = displayType
		}
		payload, err := json.Marshal(artifact)
		if err != nil {
			t.Fatalf("marshal artifact: %v", err)
		}
		artifactPath := filepath.Join(dir, "artifact.json")
		if err := os.WriteFile(artifactPath, payload, 0o600); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
		return []string{"--resume", artifactPath}
	}

	tests := []struct {
		name       string
		args       func(t *testing.T) []string
		wantStderr []string
	}{
		{
			name: "single localization",
			args: func(t *testing.T) []string {
				workDir := t.TempDir()
				writeCmdtestScreenshotJPEG(t, workDir, "01-home.png")
				return []string{"--version-localization", "LOC_123", "--path", workDir, "--device-type", "IPHONE_65"}
			},
			wantStderr: []string{"01-home.png", "JPEG", "01-home.jpg", "PNG"},
		},
		{
			name:       "plain fan-out",
			args:       func(t *testing.T) []string { return fanoutArgs(t) },
			wantStderr: []string{"en-US", "01-home.png", "JPEG", "01-home.jpg", "PNG"},
		},
		// --replace deletes a locale's existing screenshots before uploading,
		// so the preflight must reject the file before the first request.
		{
			name:       "replace fan-out",
			args:       func(t *testing.T) []string { return fanoutArgs(t, "--replace", "--confirm") },
			wantStderr: []string{"en-US", "01-home.png", "JPEG", "01-home.jpg", "PNG"},
		},
		// The pending file was swapped for JPEG bytes after the original
		// upload failed, which is what the resume must catch.
		{
			name:       "resume artifact with display type",
			args:       func(t *testing.T) []string { return resumeArgs(t, "APP_IPHONE_65") },
			wantStderr: []string{"01-home.png", "JPEG", "01-home.jpg", "PNG"},
		},
		// The check reads the bytes and the file name only, so an artifact
		// written before the display type was recorded is rejected too.
		{
			name:       "resume artifact without display type",
			args:       func(t *testing.T) []string { return resumeArgs(t, "") },
			wantStderr: []string{"01-home.png", "JPEG", "01-home.jpg", "PNG"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			args := append(append([]string{"screenshots", "upload"}, test.args(t)...), "--output", "json")

			var requests atomic.Int32
			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests.Add(1)
				t.Errorf("unexpected request before upload preflight: %s %s", req.Method, req.URL.String())
				return nil, errors.New("unexpected request")
			})

			_, stderr := captureOutput(t, func() {
				if code := cmd.Run(args, "1.2.3"); code == cmd.ExitSuccess {
					t.Fatal("expected the upload to fail for JPEG data named .png")
				}
			})

			if got := requests.Load(); got != 0 {
				t.Fatalf("expected zero HTTP requests, got %d", got)
			}
			for _, want := range test.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("expected stderr to mention %q, got %q", want, stderr)
				}
			}
		})
	}
}

func writeCmdtestScreenshotJPEG(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 1242, 2688))
	for y := 0; y < 2688; y++ {
		for x := 0; x < 1242; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
}
