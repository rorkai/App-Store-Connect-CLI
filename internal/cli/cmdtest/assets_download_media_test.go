package cmdtest

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	cmdpkg "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestMediaDownload_ByID_WritesFile(t *testing.T) {
	tests := []struct {
		name             string
		command          string
		id               string
		apiPath          string
		apiBody          string
		assetPath        string
		assetData        string
		contentType      string
		userAgent        string
		fileName         string
		reportsUnchanged bool
	}{
		{
			name:             "screenshot",
			command:          "screenshots",
			id:               "shot-1",
			apiPath:          "/v1/appScreenshots/shot-1",
			apiBody:          `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":7,"imageAsset":{"templateUrl":"https://example.com/assets/{w}x{h}bb.{f}","width":1242,"height":2688}}}}`,
			assetPath:        "/assets/1242x2688bb.png",
			assetData:        "PNGDATA",
			contentType:      "image/png",
			userAgent:        "curl/8.7.1 App-Store-Connect-CLI/asset-download",
			fileName:         "shot.png",
			reportsUnchanged: true,
		},
		{
			name:        "video preview",
			command:     "video-previews",
			id:          "prev-1",
			apiPath:     "/v1/appPreviews/prev-1",
			apiBody:     `{"data":{"type":"appPreviews","id":"prev-1","attributes":{"fileName":"preview.mov","fileSize":7,"mimeType":"video/quicktime","videoUrl":"https://example.com/preview.mov"}}}`,
			assetPath:   "/preview.mov",
			assetData:   "MOVDATA",
			contentType: "video/quicktime",
			fileName:    "preview.mov",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("expected GET, got %s", req.Method)
				}
				switch req.URL.Host {
				case "api.appstoreconnect.apple.com":
					if req.URL.Path != test.apiPath {
						t.Fatalf("unexpected path: %s", req.URL.Path)
					}
					return jsonResponse(http.StatusOK, test.apiBody)
				case "example.com":
					if req.URL.Path != test.assetPath {
						t.Fatalf("unexpected asset path: %s", req.URL.Path)
					}
					if test.userAgent != "" {
						if got := strings.TrimSpace(req.Header.Get("User-Agent")); got != test.userAgent {
							t.Fatalf("unexpected user agent: %q", got)
						}
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(test.assetData)),
						Header:     http.Header{"Content-Type": []string{test.contentType}},
					}, nil
				default:
					t.Fatalf("unexpected host: %s", req.URL.Host)
					return nil, nil
				}
			}))

			outPath := filepath.Join(t.TempDir(), test.fileName)
			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			type item struct {
				ID           string `json:"id"`
				OutputPath   string `json:"outputPath"`
				Unchanged    *bool  `json:"unchanged"`
				BytesWritten int64  `json:"bytesWritten"`
			}
			type result struct {
				Total      int    `json:"total"`
				Downloaded int    `json:"downloaded"`
				Failed     int    `json:"failed"`
				Items      []item `json:"items"`
			}

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{test.command, "download", "--id", test.id, "--output", outPath}); err != nil {
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
			if got.Total != 1 || got.Downloaded != 1 || got.Failed != 0 || len(got.Items) != 1 {
				t.Fatalf("unexpected result: %+v", got)
			}
			if got.Items[0].ID != test.id {
				t.Fatalf("expected item id %s, got %q", test.id, got.Items[0].ID)
			}
			if filepath.Clean(got.Items[0].OutputPath) != filepath.Clean(outPath) {
				t.Fatalf("expected outputPath %q, got %q", outPath, got.Items[0].OutputPath)
			}
			if got.Items[0].BytesWritten != int64(len(test.assetData)) {
				t.Fatalf("expected bytesWritten %d, got %d", len(test.assetData), got.Items[0].BytesWritten)
			}
			if test.reportsUnchanged && (got.Items[0].Unchanged == nil || *got.Items[0].Unchanged) {
				t.Fatalf("expected unchanged=false in item JSON, got %v", got.Items[0].Unchanged)
			}

			data, err := os.ReadFile(outPath)
			if err != nil {
				t.Fatalf("ReadFile() error: %v", err)
			}
			if string(data) != test.assetData {
				t.Fatalf("unexpected file contents: %q", string(data))
			}
		})
	}
}

func TestScreenshotsDownload_ByID_PreservesEquivalentPNGOnOverwrite(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	existingPNG := screenshotTestPNG(t, "asset-id-existing", "same-pixels")
	downloadedPNG := screenshotTestPNG(t, "asset-id-downloaded", "same-pixels")
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "api.appstoreconnect.apple.com":
			body := `{"data":{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"shot.png","fileSize":7,"imageAsset":{"templateUrl":"https://example.com/assets/{w}x{h}bb.{f}","width":1242,"height":2688}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		case "example.com":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(downloadedPNG))),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		default:
			t.Fatalf("unexpected host: %s", req.URL.Host)
			return nil, nil
		}
	})

	outPath := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(outPath, existingPNG, 0o600); err != nil {
		t.Fatalf("write existing screenshot: %v", err)
	}
	originalModTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(outPath, originalModTime, originalModTime); err != nil {
		t.Fatalf("set existing screenshot timestamps: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var got struct {
		Total      int `json:"total"`
		Downloaded int `json:"downloaded"`
		Unchanged  int `json:"unchanged"`
		Failed     int `json:"failed"`
		Items      []struct {
			Unchanged    bool  `json:"unchanged"`
			BytesWritten int64 `json:"bytesWritten"`
		} `json:"items"`
	}
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"screenshots", "download", "--id", "shot-1", "--output", outPath, "--overwrite"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v (stdout=%q, stderr=%q)", runErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if got.Total != 1 || got.Downloaded != 1 || got.Unchanged != 1 || got.Failed != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(got.Items) != 1 || !got.Items[0].Unchanged || got.Items[0].BytesWritten != 0 {
		t.Fatalf("unexpected item result: %+v", got.Items)
	}

	after, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read preserved screenshot: %v", err)
	}
	if string(after) != string(existingPNG) {
		t.Fatal("equivalent download replaced the existing screenshot")
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat preserved screenshot: %v", err)
	}
	if !info.ModTime().Equal(originalModTime) {
		t.Fatalf("modification time = %s, want %s", info.ModTime(), originalModTime)
	}
}

func TestScreenshotsDownload_ByLocalization_WritesFiles(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	var assetRequests sync.WaitGroup
	assetRequests.Add(2)
	bothAssetsRequested := make(chan struct{})
	go func() {
		assetRequests.Wait()
		close(bothAssetsRequested)
	}()

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
			assetRequests.Done()
			select {
			case <-bothAssetsRequested:
			case <-time.After(5 * time.Second):
				t.Error("screenshot downloads ran serially; want both in flight")
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
				body := `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"},"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"prev-2"},{"type":"appPreviews","id":"prev-1"}]}}},{"type":"appPreviewSets","id":"set-2","attributes":{"previewType":"IPAD_PRO_3GEN_129"}}],"included":[{"type":"appPreviews","id":"prev-1","attributes":{"fileName":"a.mov","videoUrl":"https://example.com/a.mov"}},{"type":"appPreviews","id":"prev-2","attributes":{"fileName":"z.mov","videoUrl":"https://example.com/z.mov"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appPreviewSets/set-2/appPreviews":
				body := `{"data":[{"type":"appPreviews","id":"prev-3","attributes":{"fileName":"a.mov","videoUrl":"https://example.com/a.mov"}},{"type":"appPreviews","id":"prev-4","attributes":{"fileName":"z.mov","videoUrl":"https://example.com/z.mov"}}]}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			case "/v1/appPreviewSets/set-2/relationships/appPreviews":
				body := `{"data":[{"type":"appPreviews","id":"prev-4"},{"type":"appPreviews","id":"prev-3"}],"links":{}}`
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
			if req.URL.Path != "/a.mov" && req.URL.Path != "/z.mov" {
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
	if !strings.Contains(stdout, `"downloaded":4`) || !strings.Contains(stdout, `"failed":0`) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}

	// Position prefixes follow App Store order, not file names, for both
	// included previews (set-1) and the per-set fallback (set-2).
	for _, wantPath := range []string{
		filepath.Join(outDir, "IPHONE_65", "01_prev-2_z.mov"),
		filepath.Join(outDir, "IPHONE_65", "02_prev-1_a.mov"),
		filepath.Join(outDir, "IPAD_PRO_3GEN_129", "01_prev-4_z.mov"),
		filepath.Join(outDir, "IPAD_PRO_3GEN_129", "02_prev-3_a.mov"),
	} {
		data, err := os.ReadFile(wantPath)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if string(data) != "MOVDATA" {
			t.Fatalf("unexpected file contents: %q", string(data))
		}
	}
}

func TestVideoPreviewsDownload_ByLocalization_ReadsEveryPreviewSetPage(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body := "MOVDATA"
		switch {
		case req.URL.Host == "example.com" && (req.URL.Path == "/one.mov" || req.URL.Path == "/two.mov"):
		case req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		case req.URL.Query().Get("cursor") == "page-2":
			body = `{"data":[{"type":"appPreviewSets","id":"set-2","attributes":{"previewType":"IPAD_PRO_3GEN_129"},"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"prev-2"}]}}}],"included":[{"type":"appPreviews","id":"prev-2","attributes":{"fileName":"two.mov","videoUrl":"https://example.com/two.mov"}}],"links":{}}`
		default:
			body = `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"},"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"prev-1"}]}}}],"included":[{"type":"appPreviews","id":"prev-1","attributes":{"fileName":"one.mov","videoUrl":"https://example.com/one.mov"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appPreviewSets?cursor=page-2&include=appPreviews"}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
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
	if !strings.Contains(stdout, `"downloaded":2`) {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
	for _, wantPath := range []string{
		filepath.Join(outDir, "IPHONE_65", "01_prev-1_one.mov"),
		filepath.Join(outDir, "IPAD_PRO_3GEN_129", "01_prev-2_two.mov"),
	} {
		if _, err := os.Stat(wantPath); err != nil {
			t.Fatalf("Stat() error: %v", err)
		}
	}
}

func TestVideoPreviewsDownload_ByLocalization_PreservesFallbackDetailErrors(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_MAX_RETRIES", "0")
	defaultTransport := http.DefaultTransport

	var serverURL string
	var requestMu sync.Mutex
	var requestPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestMu.Lock()
		requestPaths = append(requestPaths, req.URL.Path)
		requestMu.Unlock()
		if strings.HasPrefix(req.URL.Path, "/v1/") {
			authorization := req.Header.Get("Authorization")
			if !strings.HasPrefix(authorization, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer ")) == "" {
				t.Error("API request is missing a bearer Authorization header")
			}
		}
		if req.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", req.Method)
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		wantQuery := ""
		switch req.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
			wantQuery = "include=appPreviews&limit%5BappPreviews%5D=50"
		case "/v1/appPreviewSets/set-1/relationships/appPreviews":
			wantQuery = "limit=200"
		}
		if req.URL.RawQuery != wantQuery {
			t.Errorf("query = %q, want %q", req.URL.RawQuery, wantQuery)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}

		switch req.URL.Path {
		case "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
			writeJSONResponse(w, http.StatusOK, `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"}}],"links":{}}`)
		case "/v1/appPreviewSets/set-1/appPreviews":
			writeJSONResponse(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"auth","attributes":{"fileName":"a-auth.mov"}},{"type":"appPreviews","id":"empty","attributes":{"fileName":"b-empty.mov"}},{"type":"appPreviews","id":"good","attributes":{"fileName":"c-good.mov","videoUrl":"`+serverURL+`/media/good.mov"}}],"links":{}}`)
		case "/v1/appPreviewSets/set-1/relationships/appPreviews":
			writeJSONResponse(w, http.StatusOK, `{"data":[{"type":"appPreviews","id":"auth"},{"type":"appPreviews","id":"empty"},{"type":"appPreviews","id":"good"}],"links":{}}`)
		case "/v1/appPreviews/auth":
			writeJSONResponse(w, http.StatusUnauthorized, `{"errors":[{"status":"401","code":"NOT_AUTHORIZED","title":"Unauthorized","detail":"preview detail access denied\nforged-row"}]}`)
		case "/v1/appPreviews/empty":
			writeJSONResponse(w, http.StatusOK, `{"data":{"type":"appPreviews","id":"empty","attributes":{"fileName":"b-empty.mov"}},"links":{}}`)
		case "/media/good.mov":
			w.Header().Set("Content-Type", "video/quicktime")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "MOVDATA")
		default:
			t.Errorf("unexpected request path: %s", req.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	serverURL = server.URL

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	apiTransport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.appstoreconnect.apple.com" {
			return nil, fmt.Errorf("unexpected ASC client host: %s", req.URL.Host)
		}
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = target.Scheme
		cloned.URL.Host = target.Host
		cloned.Host = target.Host
		return serverTransport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: apiTransport},
	)
	if err != nil {
		t.Fatalf("create video preview detail test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	outDir := filepath.Join(t.TempDir(), "previews")
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"video-previews", "download", "--version-localization", "loc-1", "--output-dir", outDir}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if stderr != "" {
		t.Fatalf("stderr = %q, want empty because the JSON result already reports failures", stderr)
	}
	var reported ReportedError
	if !errors.As(runErr, &reported) {
		t.Fatalf("run error = %T %v, want ReportedError", runErr, runErr)
	}
	if got := cmdpkg.ExitCodeFromError(runErr); got != cmdpkg.ExitError {
		t.Fatalf("exit code = %d, want generic failure %d", got, cmdpkg.ExitError)
	}

	var result struct {
		Total      int `json:"total"`
		Downloaded int `json:"downloaded"`
		Failed     int `json:"failed"`
		Failures   []struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
	}
	if result.Total != 3 || result.Downloaded != 1 || result.Failed != 2 {
		t.Fatalf("unexpected result summary: %+v", result)
	}
	if len(result.Failures) != 2 {
		t.Fatalf("failures = %#v, want 2", result.Failures)
	}
	if got, want := result.Failures[0].ID, "auth"; got != want {
		t.Fatalf("first failure ID = %q, want %q", got, want)
	}
	if got, want := result.Failures[0].Error, "failed to fetch preview details: Unauthorized: preview detail access denied forged-row"; got != want {
		t.Fatalf("first failure error = %q, want %q", got, want)
	}
	if strings.ContainsAny(result.Failures[0].Error, "\r\n") {
		t.Fatalf("first failure contains an unsafe line break: %q", result.Failures[0].Error)
	}
	if got, want := result.Failures[1].ID, "empty"; got != want {
		t.Fatalf("second failure ID = %q, want %q", got, want)
	}
	if got, want := result.Failures[1].Error, "preview has no videoUrl"; got != want {
		t.Fatalf("second failure error = %q, want %q", got, want)
	}

	goodPath := filepath.Join(outDir, "IPHONE_65", "03_good_c-good.mov")
	data, err := os.ReadFile(goodPath)
	if err != nil {
		t.Fatalf("read successful sibling: %v", err)
	}
	if string(data) != "MOVDATA" {
		t.Fatalf("successful sibling contents = %q, want MOVDATA", data)
	}
	if http.DefaultTransport != defaultTransport {
		t.Fatal("test mutated http.DefaultTransport")
	}
	requestMu.Lock()
	gotRequests := strings.Join(requestPaths, "\n")
	requestMu.Unlock()
	wantRequests := strings.Join([]string{
		"/v1/appStoreVersionLocalizations/loc-1/appPreviewSets",
		"/v1/appPreviewSets/set-1/appPreviews",
		"/v1/appPreviewSets/set-1/relationships/appPreviews",
		"/v1/appPreviews/auth",
		"/v1/appPreviews/empty",
		"/media/good.mov",
	}, "\n")
	if gotRequests != wantRequests {
		t.Fatalf("request sequence = %q, want %q", gotRequests, wantRequests)
	}
}

func writeJSONResponse(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func screenshotTestPNG(t *testing.T, metadata, pixels string) []byte {
	t.Helper()

	signature := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	chunk := func(chunkType string, data []byte) []byte {
		t.Helper()
		result := make([]byte, 12+len(data))
		binary.BigEndian.PutUint32(result[:4], uint32(len(data)))
		copy(result[4:8], chunkType)
		copy(result[8:8+len(data)], data)
		binary.BigEndian.PutUint32(result[8+len(data):], crc32.ChecksumIEEE(result[4:8+len(data)]))
		return result
	}

	png := append([]byte(nil), signature...)
	header := make([]byte, 13)
	binary.BigEndian.PutUint32(header[:4], 1)
	binary.BigEndian.PutUint32(header[4:8], 1)
	header[8] = 8
	header[9] = 6
	row := make([]byte, 5)
	binary.BigEndian.PutUint32(row[1:], crc32.ChecksumIEEE([]byte(pixels)))
	var compressed bytes.Buffer
	compressor := zlib.NewWriter(&compressed)
	if _, err := compressor.Write(row); err != nil {
		t.Fatalf("compress screenshot PNG row: %v", err)
	}
	if err := compressor.Close(); err != nil {
		t.Fatalf("close screenshot PNG compressor: %v", err)
	}
	textData := append([]byte("date:modify"), 0, 0, 0, 0, 0)
	textData = append(textData, metadata...)
	png = append(png, chunk("IHDR", header)...)
	png = append(png, chunk("iTXt", textData)...)
	png = append(png, chunk("IDAT", compressed.Bytes())...)
	png = append(png, chunk("IEND", nil)...)
	return png
}

func TestVideoPreviewsDownload_HLSPlaylist(t *testing.T) {
	setupAuth(t)

	playlist := "#EXTM3U\n#EXT-X-VERSION:7\n"
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/stream" {
			t.Errorf("unexpected media path: %s", req.URL.Path)
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = io.WriteString(w, playlist)
	}))
	t.Cleanup(media.Close)

	preview := `{"type":"appPreviews","id":"prev-1","attributes":{"fileName":"preview.mp4","mimeType":"video/mp4","videoUrl":"` + media.URL + `/stream"}}`
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "api.appstoreconnect.apple.com" {
			return originalTransport.RoundTrip(req)
		}
		var body string
		switch req.URL.Path {
		case "/v1/appPreviews/prev-1":
			body = `{"data":` + preview + `}`
		case "/v1/appStoreVersionLocalizations/loc-1/appPreviewSets":
			body = `{"data":[{"type":"appPreviewSets","id":"set-1","attributes":{"previewType":"IPHONE_65"},"relationships":{"appPreviews":{"data":[{"type":"appPreviews","id":"prev-1"}]}}}],"included":[` + preview + `]}`
		default:
			t.Fatalf("unexpected API path: %s", req.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	run := func(args ...string) (string, string) {
		t.Helper()
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		return captureOutput(t, func() {
			if err := root.Parse(append([]string{"video-previews", "download"}, args...)); err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if err := root.Run(context.Background()); err != nil {
				t.Fatalf("run error: %v", err)
			}
		})
	}
	assertSaved := func(stdout, wantPath string) {
		t.Helper()
		var result struct {
			Items []struct {
				OutputPath string `json:"outputPath"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("decode stdout JSON: %v (stdout=%q)", err, stdout)
		}
		if len(result.Items) != 1 || result.Items[0].OutputPath != wantPath {
			t.Fatalf("items = %+v, want one item with outputPath %q", result.Items, wantPath)
		}
		data, err := os.ReadFile(wantPath)
		if err != nil {
			t.Fatalf("ReadFile() error: %v", err)
		}
		if string(data) != playlist {
			t.Fatalf("unexpected file contents: %q", string(data))
		}
	}

	explicit := filepath.Join(t.TempDir(), "preview.mp4")
	stdout, stderr := run("--id", "prev-1", "--output", explicit)
	assertSaved(stdout, explicit)
	wantWarning := "Warning: App Store Connect only exposes an HLS streaming playlist for previews; " + explicit + " is a .m3u8 playlist, not a video file\n"
	if stderr != wantWarning {
		t.Fatalf("stderr = %q, want %q", stderr, wantWarning)
	}

	outDir := t.TempDir()
	stdout, stderr = run("--version-localization", "loc-1", "--output-dir", outDir)
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	assertSaved(stdout, filepath.Join(outDir, "IPHONE_65", "01_prev-1_preview.m3u8"))
	if _, err := os.Stat(filepath.Join(outDir, "IPHONE_65", "01_prev-1_preview.mp4")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("playlist was saved under the .mp4 name (stat err = %v)", err)
	}
}
