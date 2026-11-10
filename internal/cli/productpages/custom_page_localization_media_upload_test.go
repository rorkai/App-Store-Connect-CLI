package productpages

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type customPageUploadRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn customPageUploadRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type customPageBaseURLRoundTripper struct {
	target *url.URL
	next   http.RoundTripper
}

func (t customPageBaseURLRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	requestURL := *cloned.URL
	requestURL.Scheme = t.target.Scheme
	requestURL.Host = t.target.Host
	cloned.URL = &requestURL
	cloned.Host = t.target.Host
	return t.next.RoundTrip(cloned)
}

func TestExecuteCustomPageScreenshotUpload_SyncDeletesExistingScreenshotsAndReordersUploads(t *testing.T) {
	dir := t.TempDir()
	fileA := writeCustomPageTestPNG(t, dir, "01-home.png")
	fileB := writeCustomPageTestPNG(t, dir, "02-settings.png")
	sizes := map[string]int64{
		"new-1": customPageFileSize(t, fileA),
		"new-2": customPageFileSize(t, fileB),
	}

	deletedExisting := false
	relationshipPatchCalled := false
	origTransport := http.DefaultTransport
	http.DefaultTransport = customPageUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return customPageJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1","attributes":{"fileName":"legacy.png"}}]}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/appScreenshots/old-1":
			deletedExisting = true
			return customPageJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot body: %v", err)
			}
			id := "new-1"
			if strings.Contains(string(body), "02-settings.png") {
				id = "new-2"
			}
			return customPageJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, sizes[id]))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return customPageJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"},"sourceFileChecksum":"settled"}}}`, id))
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read relationship patch body: %v", err)
			}
			var payload asc.RelationshipRequest
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode relationship patch body: %v", err)
			}
			gotIDs := make([]string, 0, len(payload.Data))
			for _, item := range payload.Data {
				gotIDs = append(gotIDs, item.ID)
			}
			wantIDs := []string{"new-1", "new-2"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return customPageJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newCustomPageTestClient(t)
	origFactory := customPageMediaClientFactory
	customPageMediaClientFactory = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() {
		customPageMediaClientFactory = origFactory
	})

	result, err := executeCustomPageScreenshotUpload(context.Background(), "loc-1", dir, "IPHONE_65", true)
	if err != nil {
		t.Fatalf("executeCustomPageScreenshotUpload() error: %v", err)
	}
	if result.SetID != "set-1" {
		t.Fatalf("expected set ID set-1, got %q", result.SetID)
	}
	if len(result.Results) != 2 {
		t.Fatalf("expected 2 uploaded screenshots, got %d", len(result.Results))
	}
	if !deletedExisting {
		t.Fatal("expected sync to delete existing screenshots before upload")
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func TestExecuteCustomPageScreenshotUploadFullSetQuotesReplacementPath(t *testing.T) {
	rootDir := t.TempDir()
	trickyPath := filepath.Join(rootDir, "$HOME", "$(touch pwned)")
	if err := os.MkdirAll(trickyPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	writeCustomPageTestPNG(t, trickyPath, "01-home.png")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appCustomProductPageLocalizations/loc-1/appScreenshotSets":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[{"type":"appScreenshots","id":"old-1"},{"type":"appScreenshots","id":"old-2"},{"type":"appScreenshots","id":"old-3"},{"type":"appScreenshots","id":"old-4"},{"type":"appScreenshots","id":"old-5"},{"type":"appScreenshots","id":"old-6"},{"type":"appScreenshots","id":"old-7"},{"type":"appScreenshots","id":"old-8"},{"type":"appScreenshots","id":"old-9"},{"type":"appScreenshots","id":"old-10"}]}`)
		default:
			t.Fatalf("full-set remediation must not mutate remote assets: %s %s", req.Method, req.URL.String())
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(func() {
		server.Close()
	})

	serverURL, parseErr := url.Parse(server.URL)
	if parseErr != nil {
		t.Fatalf("parse test server URL: %v", parseErr)
	}
	httpClient := server.Client()
	httpClient.Transport = customPageBaseURLRoundTripper{target: serverURL, next: httpClient.Transport}
	client := newCustomPageTestClientWithHTTPClient(t, httpClient)
	origFactory := customPageMediaClientFactory
	customPageMediaClientFactory = func() (*asc.Client, error) { return client, nil }
	t.Cleanup(func() {
		customPageMediaClientFactory = origFactory
	})

	_, err := executeCustomPageScreenshotUpload(context.Background(), "loc-1", trickyPath, "IPHONE_65", false)
	if err == nil {
		t.Fatal("expected full screenshot set error")
	}
	const marker = "or rerun with "
	commandIndex := strings.Index(err.Error(), marker)
	if commandIndex < 0 {
		t.Fatalf("expected replacement command in error: %v", err)
	}
	command := strings.TrimSpace(err.Error()[commandIndex+len(marker):])
	args, splitErr := shellquote.Split(command)
	if splitErr != nil {
		t.Fatalf("replacement command is not shell-parseable: %v", splitErr)
	}
	wantArgs := []string{
		"asc", "product-pages", "custom-pages", "localizations", "screenshot-sets", "sync",
		"--localization-id", "loc-1", "--path", trickyPath, "--device-type", "IPHONE_65", "--confirm",
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("replacement command args = %#v, want %#v (error: %v)", args, wantArgs, err)
	}
}

func newCustomPageTestClient(t *testing.T) *asc.Client {
	return newCustomPageTestClientWithTimeout(t, asc.ResolveTimeout())
}

func newCustomPageTestClientWithTimeout(t *testing.T, timeout time.Duration) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("encode pem: nil")
	}

	client, err := asc.NewClientFromPEMWithTimeout("KEY_ID", "ISSUER_ID", string(pemBytes), timeout)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func newCustomPageTestClientWithHTTPClient(t *testing.T, httpClient *http.Client) *asc.Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("encode pem: nil")
	}
	keyPath := filepath.Join(t.TempDir(), "test-key.p8")
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	client, err := asc.NewClientWithHTTPClient("KEY_ID", "ISSUER_ID", keyPath, httpClient)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func customPageJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func writeCustomPageTestPNG(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	const (
		width  = 1242
		height = 2688
	)

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	// Distinct names get distinct pixels: uploads reject duplicate screenshots.
	sum := crc32.ChecksumIEEE([]byte(name))
	img.Set(0, 0, color.RGBA{R: byte(sum), G: byte(sum >> 8), B: byte(sum >> 16), A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func writeDisplayTypeTestPNG(t *testing.T, dir, name, displayType string) string {
	t.Helper()

	dimensions, ok := asc.ScreenshotDimensions(displayType)
	if !ok || len(dimensions) == 0 {
		t.Fatalf("dimensions unavailable for display type %q", displayType)
	}

	return writePNGWithDimensions(t, dir, name, dimensions[0].Width, dimensions[0].Height)
}

func writePNGWithDimensions(t *testing.T, dir, name string, width, height int) string {
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
	// Distinct names get distinct pixels: uploads reject duplicate screenshots.
	sum := crc32.ChecksumIEEE([]byte(name))
	img.Set(0, 0, color.RGBA{R: byte(sum), G: byte(sum >> 8), B: byte(sum >> 16), A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func customPageFileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	return info.Size()
}
