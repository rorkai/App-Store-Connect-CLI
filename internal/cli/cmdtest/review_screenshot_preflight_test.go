package cmdtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

var (
	reviewScreenshotPNGOnce  sync.Once
	reviewScreenshotPNGBytes []byte
)

// reviewScreenshotPNG returns an opaque PNG at an accepted App Review
// screenshot size (iPhone 3.5-inch without status bar), encoded once.
func reviewScreenshotPNG(t *testing.T) []byte {
	t.Helper()
	reviewScreenshotPNGOnce.Do(func() {
		var buf bytes.Buffer
		if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 640, 920))); err != nil {
			panic(fmt.Sprintf("encode review screenshot fixture: %v", err))
		}
		reviewScreenshotPNGBytes = buf.Bytes()
	})
	return append([]byte(nil), reviewScreenshotPNGBytes...)
}

// writeReviewScreenshotPNG writes an opaque PNG that passes the local review
// screenshot preflight.
func writeReviewScreenshotPNG(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, reviewScreenshotPNG(t), 0o600); err != nil {
		t.Fatalf("write review screenshot fixture: %v", err)
	}
}

func writeOpaquePNG(t *testing.T, path string, width, height int) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}
}

// failOnAnyRequest installs a transport that records any App Store Connect
// request, proving the preflight rejected the file before contacting it.
func failOnAnyRequest(t *testing.T) *int {
	t.Helper()
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
	})
	return &requests
}

func writeJPEGNamed(t *testing.T, path string, width, height int) {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write jpeg: %v", err)
	}
}

// assertReviewScreenshotRejectedBeforeRequest runs args through the root
// entrypoint and asserts the review screenshot was rejected as a usage error
// (exit 2) with exactly the one diagnostic line wantError, without the
// command's usage page, and before any App Store Connect request.
func assertReviewScreenshotRejectedBeforeRequest(t *testing.T, args []string, requests *int, wantError string) {
	t.Helper()
	code := -1
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(args, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d (usage); stderr = %q", code, rootcmd.ExitUsage, stderr)
	}
	if *requests != 0 {
		t.Fatalf("expected no App Store Connect requests, got %d", *requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if want := "Error: " + wantError + "\n"; stderr != want {
		t.Fatalf("stderr = %q, want exactly %q", stderr, want)
	}
}

func TestReviewScreenshotUploadsRejectFormatProblemsBeforeAnyRequest(t *testing.T) {
	commands := []struct {
		name   string
		args   func(path string) []string
		prefix string
	}{
		{
			name: "subscriptions review screenshots create",
			args: func(path string) []string {
				return []string{"subscriptions", "review", "screenshots", "create", "--subscription-id", "8000000001", "--file", path}
			},
			prefix: "subscriptions review screenshots create: ",
		},
		{
			name: "iap review-screenshots create",
			args: func(path string) []string {
				return []string{"iap", "review-screenshots", "create", "--iap-id", "9000000001", "--file", path}
			},
			prefix: "iap review-screenshots create: ",
		},
		{
			name: "iap review-screenshots update",
			args: func(path string) []string {
				return []string{"iap", "review-screenshots", "update", "--screenshot-id", "shot-1", "--file", path}
			},
			prefix: "iap review-screenshots update: ",
		},
	}
	fixtures := []struct {
		name  string
		file  string
		write func(t *testing.T, path string)
		want  func(path string) string
	}{
		{
			name:  "JPEG data with .png extension",
			file:  "review.png",
			write: func(t *testing.T, path string) { writeJPEGNamed(t, path, 1290, 2796) },
			want: func(path string) string {
				return fmt.Sprintf("review screenshot %q is JPEG data but has a .png extension; rename it to review.jpg or re-export it as PNG", path)
			},
		},
		{
			name:  "GIF data with .png extension",
			file:  "review.png",
			write: writeGIF,
			want: func(path string) string {
				return fmt.Sprintf("review screenshot %q is GIF data; App Store Connect accepts PNG or JPEG screenshots (.png, .jpg, or .jpeg); re-export it as PNG or JPEG", path)
			},
		},
		{
			name:  "PNG data with .webp extension",
			file:  "review.webp",
			write: func(t *testing.T, path string) { writeReviewScreenshotPNG(t, path) },
			want: func(path string) string {
				return fmt.Sprintf("review screenshot %q has a .webp extension; App Store Connect accepts only .png, .jpg, or .jpeg file names; rename it to review.png", path)
			},
		},
	}
	for _, command := range commands {
		for _, fixture := range fixtures {
			t.Run(command.name+"/"+fixture.name, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
				requests := failOnAnyRequest(t)

				path := filepath.Join(t.TempDir(), fixture.file)
				fixture.write(t, path)

				assertReviewScreenshotRejectedBeforeRequest(t, command.args(path), requests, command.prefix+fixture.want(path))
			})
		}
	}
}

func TestSubscriptionsSetupRejectsMislabeledReviewScreenshotBeforeAnyRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := failOnAnyRequest(t)

	path := filepath.Join(t.TempDir(), "paywall.png")
	writeJPEGNamed(t, path, 1290, 2796)

	assertReviewScreenshotRejectedBeforeRequest(t, []string{
		"subscriptions", "setup",
		"--group-id", "GROUP_ID",
		"--reference-name", "Pro Monthly",
		"--product-id", "com.example.pro.monthly",
		"--review-screenshot", path,
	}, requests, fmt.Sprintf("invalid --review-screenshot: review screenshot %q is JPEG data but has a .png extension; rename it to paywall.jpg or re-export it as PNG", path))
}

func TestIAPImportRejectsNonPNGReviewScreenshotBeforeAnyRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	requests := failOnAnyRequest(t)

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	writeGIF(t, writeIAPImportScreenshot(t, dir))

	assertReviewScreenshotRejectedBeforeRequest(
		t,
		[]string{"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json"},
		requests,
		`iap import: products[0]: reviewScreenshot "shots/coins.png": review screenshot "shots/coins.png" is GIF data; App Store Connect accepts PNG or JPEG screenshots (.png, .jpg, or .jpeg); re-export it as PNG or JPEG`,
	)
}

// writeGIF writes GIF data, which App Store Connect does not accept as a
// review screenshot, to path whatever its extension.
func writeGIF(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, image.NewPaletted(image.Rect(0, 0, 640, 920), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write gif: %v", err)
	}
}

// runIAPReviewScreenshotCreateWithMockUpload runs a successful create against
// a mocked reservation, upload, commit, and delivery check.
func runIAPReviewScreenshotCreateWithMockUpload(t *testing.T, path string) (string, bool) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	size := info.Size()

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	uploaded := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":%q,"fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`, filepath.Base(path), size, size))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			uploaded = true
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, runErr := runRootCommand(t, []string{"iap", "review-screenshots", "create", "--iap-id", "9000000001", "--file", path, "--output", "json"})
	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	return stderr, uploaded
}

func TestIAPReviewScreenshotsCreateWarnsAboutUndocumentedSizeAndUploads(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	path := filepath.Join(t.TempDir(), "paywall.png")
	writeOpaquePNG(t, path, 1179, 2560)

	stderr, uploaded := runIAPReviewScreenshotCreateWithMockUpload(t, path)
	if !uploaded {
		t.Fatal("expected the screenshot to upload despite the size warning")
	}
	want := fmt.Sprintf("Warning: review screenshot %q is 1179x2560 pixels, which matches no documented App Store screenshot size (nearest documented size: 1179x2556)", path)
	if strings.Count(stderr, want) != 1 {
		t.Fatalf("stderr %q does not contain %q exactly once", stderr, want)
	}
}

func TestSubscriptionsReviewScreenshotsCreateWarnsAboutUndocumentedSizeAndUploads(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	path := filepath.Join(t.TempDir(), "paywall.png")
	writeOpaquePNG(t, path, 1024, 1024)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}
	checksum := reviewScreenshotFileMD5(t, path)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	uploaded := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			return jsonResponse(http.StatusOK, `{"data":null}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots":
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"paywall.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`, info.Size(), info.Size()))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			uploaded = true
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"paywall.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"paywall.png","sourceFileChecksum":%q,"assetDeliveryState":{"state":"COMPLETE"}}}}`, checksum))
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, runErr := runRootCommand(t, []string{"subscriptions", "review", "screenshots", "create", "--subscription-id", "8000000001", "--file", path, "--output", "json"})
	if runErr != nil {
		t.Fatalf("expected success, got %v", runErr)
	}
	if !uploaded {
		t.Fatal("expected the screenshot to upload despite the size warning")
	}
	want := fmt.Sprintf("Warning: review screenshot %q is 1024x1024 pixels, which matches no documented App Store screenshot size", path)
	if strings.Count(stderr, want) != 1 {
		t.Fatalf("stderr %q does not contain %q exactly once", stderr, want)
	}
}

func TestIAPReviewScreenshotsCreateWarnsAboutTruncatedImageAndUploads(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	path := filepath.Join(t.TempDir(), "review.png")
	data := reviewScreenshotPNG(t)
	if err := os.WriteFile(path, data[:len(data)/2], 0o600); err != nil {
		t.Fatalf("write truncated png: %v", err)
	}

	stderr, uploaded := runIAPReviewScreenshotCreateWithMockUpload(t, path)
	if !uploaded {
		t.Fatal("expected the screenshot to upload despite the decode warning")
	}
	want := fmt.Sprintf("Warning: review screenshot %q could not be fully decoded", path)
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr %q does not contain %q", stderr, want)
	}
}

func TestIAPReviewScreenshotsCreateWarnsAboutAlphaChannelAndUploads(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	path := filepath.Join(t.TempDir(), "review.png")
	img := image.NewNRGBA(image.Rect(0, 0, 640, 920))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}

	stderr, uploaded := runIAPReviewScreenshotCreateWithMockUpload(t, path)
	if !uploaded {
		t.Fatal("expected the screenshot to upload despite the warning")
	}
	want := fmt.Sprintf("Warning: review screenshot %q has an alpha channel", path)
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr %q does not contain %q", stderr, want)
	}
}

func TestIAPImportRechecksReviewScreenshotReplacedAfterPlanning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	screenshotPath := writeIAPImportScreenshot(t, dir)

	createdProduct := false
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			writeJPEGNamed(t, screenshotPath, 640, 920)
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			createdProduct = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		default:
			t.Fatalf("unexpected request after the screenshot was replaced: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, err := runIAPImport(t, []string{"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json"})
	if err == nil || !createdProduct {
		t.Fatalf("run error = %v, product created = %t, stderr = %q; want rejection after product creation", err, createdProduct, stderr)
	}
	if want := `review screenshot "shots/coins.png" is JPEG data but has a .png extension`; !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

func TestIAPImportWarnsOnceAtUpload(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	dir := t.TempDir()
	filePath := writeIAPImportFile(t, dir, `{"products":[{"type":"CONSUMABLE","referenceName":"Coins","productId":"com.example.coins","reviewScreenshot":"shots/coins.png"}]}`)
	screenshotPath := writeIAPImportScreenshot(t, dir)
	img := image.NewNRGBA(image.Rect(0, 0, 1000, 1000))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(screenshotPath, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	reserved := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/123456789/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v2/inAppPurchases":
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchases","id":"iap-1","attributes":{}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/inAppPurchaseAppStoreReviewScreenshots":
			reserved = true
			return jsonResponse(http.StatusCreated, `{"data":{"type":"inAppPurchaseAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"coins.png"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, _ := runIAPImport(t, []string{"iap", "import", "--app", "123456789", "--file", filePath, "--confirm", "--output", "json"})
	if !reserved {
		t.Fatalf("expected the screenshot upload to proceed despite the warning; stderr = %q", stderr)
	}
	for _, warning := range []string{
		`Warning: review screenshot "shots/coins.png" is 1000x1000 pixels, which matches no documented App Store screenshot size`,
		`Warning: review screenshot "shots/coins.png" has an alpha channel`,
	} {
		if got := strings.Count(stderr, warning); got != 1 {
			t.Fatalf("%q appeared %d times, want exactly 1; stderr = %q", warning, got, stderr)
		}
	}
}

func TestSubscriptionsReviewScreenshotsCreateUploadsTheCheckedBytes(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	path := filepath.Join(t.TempDir(), "review.png")
	writeReviewScreenshotPNG(t, path)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	checksum := reviewScreenshotFileMD5(t, path)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	var uploaded []byte
	var committedChecksum string
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptions/8000000001/appStoreReviewScreenshot":
			// Replace the file at its path after the local check passed.
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove checked file: %v", err)
			}
			writeJPEGNamed(t, path, 640, 920)
			return jsonResponse(http.StatusOK, `{"data":null}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots":
			return jsonResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","fileSize":%d,"uploadOperations":[{"method":"PUT","url":"https://upload.example.com/upload/shot-1","length":%d,"offset":0}]}}}`, len(original), len(original)))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example.com":
			body, readErr := io.ReadAll(req.Body)
			if readErr != nil {
				t.Fatalf("read upload body: %v", readErr)
			}
			uploaded = body
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			var payload struct {
				Data struct {
					Attributes struct {
						SourceFileChecksum string `json:"sourceFileChecksum"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Fatalf("decode commit: %v", err)
			}
			committedChecksum = payload.Data.Attributes.SourceFileChecksum
			return jsonResponse(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionAppStoreReviewScreenshots/shot-1":
			return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png","sourceFileChecksum":%q,"assetDeliveryState":{"state":"COMPLETE"}}}}`, checksum))
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, runErr := runRootCommand(t, []string{"subscriptions", "review", "screenshots", "create", "--subscription-id", "8000000001", "--file", path, "--output", "json"})
	if runErr != nil {
		t.Fatalf("expected success, got %v (stderr %q)", runErr, stderr)
	}
	if !bytes.Equal(uploaded, original) {
		t.Fatalf("uploaded %d bytes that differ from the %d checked bytes", len(uploaded), len(original))
	}
	if committedChecksum != checksum {
		t.Fatalf("committed checksum %q, want checksum of the checked bytes %q", committedChecksum, checksum)
	}
}

func TestSubscriptionsSetupRechecksReviewScreenshotReplacedAfterValidation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	path := filepath.Join(t.TempDir(), "review.png")
	writeReviewScreenshotPNG(t, path)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	createdSubscription := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/subscriptionGroups/group-1/subscriptions":
			return jsonResponse(http.StatusOK, `{"data":[],"links":{"next":""}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/subscriptions":
			createdSubscription = true
			writeJPEGNamed(t, path, 640, 920)
			return jsonResponse(http.StatusCreated, `{"data":{"type":"subscriptions","id":"sub-1","attributes":{"name":"Pro Monthly","productId":"com.example.pro.monthly","subscriptionPeriod":"ONE_MONTH","state":"MISSING_METADATA"}}}`)
		default:
			t.Fatalf("unexpected request after the screenshot was replaced: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	_, stderr, err := runRootCommand(t, []string{
		"subscriptions", "setup",
		"--group-id", "group-1",
		"--reference-name", "Pro Monthly",
		"--product-id", "com.example.pro.monthly",
		"--subscription-period", "ONE_MONTH",
		"--review-screenshot", path,
		"--output", "json",
	})
	if err == nil || !createdSubscription {
		t.Fatalf("run error = %v, subscription created = %t, stderr = %q; want rejection after subscription creation", err, createdSubscription, stderr)
	}
	if want := fmt.Sprintf("review screenshot %q is JPEG data but has a .png extension", path); !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}
