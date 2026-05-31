package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScreenshotsDownload_ByID_WritesFile(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.appstoreconnect.apple.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/appScreenshots/shot-1" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			body := `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":7,"imageAsset":{"templateUrl":"https://example.com/assets/{w}x{h}bb.{f}","width":1242,"height":2688}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "example.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/assets/1242x2688bb.png" {
				t.Fatalf("unexpected asset path: %s", req.URL.Path)
			}
			if got := strings.TrimSpace(req.Header.Get("User-Agent")); got != "curl/8.7.1 App-Store-Connect-CLI/asset-download" {
				t.Fatalf("unexpected user agent: %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("PNGDATA")),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})

	outPath := filepath.Join(t.TempDir(), "shot.png")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type item struct {
		ID           string `json:"id"`
		OutputPath   string `json:"outputPath"`
		BytesWritten int64  `json:"bytesWritten"`
	}
	type result struct {
		Total      int    `json:"total"`
		Downloaded int    `json:"downloaded"`
		Failed     int    `json:"failed"`
		Items      []item `json:"items"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "download", "--id", "shot-1", "--output", outPath}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got result
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if got.Total != 1 || got.Downloaded != 1 || got.Failed != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(got.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got.Items))
	}
	if got.Items[0].ID != "shot-1" {
		t.Fatalf("expected item id shot-1, got %q", got.Items[0].ID)
	}
	if filepath.Clean(got.Items[0].OutputPath) != filepath.Clean(outPath) {
		t.Fatalf("expected outputPath %q, got %q", outPath, got.Items[0].OutputPath)
	}
	if got.Items[0].BytesWritten != int64(len("PNGDATA")) {
		t.Fatalf("expected bytesWritten %d, got %d", len("PNGDATA"), got.Items[0].BytesWritten)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestScreenshotsDownload_ByLocalization_WritesFiles(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.appstoreconnect.apple.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			switch req.URL.Path {
			case "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
				body := `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appScreenshotSets/set-1/appScreenshots":
				body := `{"data":[{"type":"appScreenshots","id":"shot-b","attributes":{"fileName":"01-home.png","fileSize":7,"imageAsset":{"templateUrl":"https://example.com/shot-b_{w}x{h}.{f}","width":100,"height":200}}},{"type":"appScreenshots","id":"shot-a","attributes":{"fileName":"02-paywall.png","fileSize":7,"imageAsset":{"templateUrl":"https://example.com/shot-a_{w}x{h}.{f}","width":100,"height":200}}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
				body := `{"data":[{"type":"appScreenshots","id":"shot-a"},{"type":"appScreenshots","id":"shot-b"}],"links":{}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected API path: %s", req.URL.Path)
				return nil, nil
			}
		case "example.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/shot-a_100x200.png" && req.URL.Path != "/shot-b_100x200.png" {
				t.Fatalf("unexpected asset path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("PNGDATA:" + req.URL.Path)),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})

	outDir := filepath.Join(t.TempDir(), "shots")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type result struct {
		Total      int `json:"total"`
		Downloaded int `json:"downloaded"`
		Failed     int `json:"failed"`
		Items      []struct {
			ID         string `json:"id"`
			OutputPath string `json:"outputPath"`
		} `json:"items"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "download", "--version-localization", "loc-1", "--output-dir", outDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got result
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if got.Total != 2 || got.Downloaded != 2 || got.Failed != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 result items, got %d", len(got.Items))
	}
	if got.Items[0].ID != "shot-a" || got.Items[1].ID != "shot-b" {
		t.Fatalf("expected relationship-ordered item IDs [shot-a shot-b], got [%s %s]", got.Items[0].ID, got.Items[1].ID)
	}

	wantFirstPath := filepath.Join(outDir, "APP_IPHONE_65", "01_shot-a_02-paywall.png")
	wantSecondPath := filepath.Join(outDir, "APP_IPHONE_65", "02_shot-b_01-home.png")
	if filepath.Clean(got.Items[0].OutputPath) != filepath.Clean(wantFirstPath) {
		t.Fatalf("expected first outputPath %q, got %q", wantFirstPath, got.Items[0].OutputPath)
	}
	if filepath.Clean(got.Items[1].OutputPath) != filepath.Clean(wantSecondPath) {
		t.Fatalf("expected second outputPath %q, got %q", wantSecondPath, got.Items[1].OutputPath)
	}

	data, err := os.ReadFile(wantFirstPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "PNGDATA:/shot-a_100x200.png" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
	data, err = os.ReadFile(wantSecondPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "PNGDATA:/shot-b_100x200.png" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestScreenshotsDownload_ByLocalization_RetriesTransientForbidden(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	assetAttempts := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.appstoreconnect.apple.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			switch req.URL.Path {
			case "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
				body := `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appScreenshotSets/set-1/appScreenshots":
				body := `{"data":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"screen.png","fileSize":7,"imageAsset":{"templateUrl":"https://example.com/screen_{w}x{h}.{f}","width":100,"height":200}}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
				body := `{"data":[{"type":"appScreenshots","id":"shot-1"}],"links":{}}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected API path: %s", req.URL.Path)
				return nil, nil
			}
		case "example.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/screen_100x200.png" {
				t.Fatalf("unexpected asset path: %s", req.URL.Path)
			}
			assetAttempts++
			if assetAttempts == 1 {
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Body:       io.NopCloser(strings.NewReader("403 Forbidden")),
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("PNGDATA")),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})

	outDir := filepath.Join(t.TempDir(), "shots")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	type result struct {
		Total      int `json:"total"`
		Downloaded int `json:"downloaded"`
		Failed     int `json:"failed"`
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "download", "--version-localization", "loc-1", "--output-dir", outDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got result
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if got.Total != 1 || got.Downloaded != 1 || got.Failed != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if assetAttempts != 2 {
		t.Fatalf("expected 2 download attempts, got %d", assetAttempts)
	}

	wantPath := filepath.Join(outDir, "APP_IPHONE_65", "01_shot-1_screen.png")
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestVideoPreviewsDownload_ByID_WritesFile(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.appstoreconnect.apple.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/v1/appPreviews/prev-1" {
				t.Fatalf("unexpected path: %s", req.URL.Path)
			}

			body := `{"data":{"type":"appPreviews","id":"prev-1","attributes":{"fileName":"preview.mov","fileSize":7,"mimeType":"video/quicktime","videoUrl":"https://example.com/preview.mov"}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "example.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/preview.mov" {
				t.Fatalf("unexpected asset path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("MOVDATA")),
				Header:     http.Header{"Content-Type": []string{"video/quicktime"}},
			}, nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})

	outPath := filepath.Join(t.TempDir(), "preview.mov")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"video-previews", "download", "--id", "prev-1", "--output", outPath}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"downloaded":1`) || !strings.Contains(stdout, `"failed":0`) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "MOVDATA" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestVideoPreviewsDownload_ByLocalization_WritesFiles(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.appstoreconnect.apple.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			switch req.URL.Path {
			case "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
				body := `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appPreviewSets/set-1/appPreviews":
				body := `{"data":[{"type":"appPreviews","id":"prev-1","attributes":{"fileName":"p.mov","fileSize":7,"mimeType":"video/quicktime","videoUrl":"https://example.com/p.mov"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			default:
				t.Fatalf("unexpected API path: %s", req.URL.Path)
				return nil, nil
			}
		case "example.com":
			if req.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", req.Method)
			}
			if req.URL.Path != "/p.mov" {
				t.Fatalf("unexpected asset path: %s", req.URL.Path)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("MOVDATA")),
				Header:     http.Header{"Content-Type": []string{"video/quicktime"}},
			}, nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})

	outDir := filepath.Join(t.TempDir(), "previews")

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"video-previews", "download", "--version-localization", "loc-1", "--output-dir", outDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"downloaded":1`) || !strings.Contains(stdout, `"failed":0`) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}

	wantPath := filepath.Join(outDir, "IPHONE_65", "01_prev-1_p.mov")
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != "MOVDATA" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}
