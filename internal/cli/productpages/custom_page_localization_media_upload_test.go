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
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type customPageUploadRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn customPageUploadRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestExecuteCustomPageScreenshotUpload_SyncDeletesExistingScreenshotsAndReordersUploads(t *testing.T) {
	dir := t.TempDir()
	fileA := writeCustomPageTestPNG(t, dir, "01-home.png", 1242, 2688)
	fileB := writeCustomPageTestPNG(t, dir, "02-settings.png", 1242, 2688)
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
			return customPageJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
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

func newCustomPageTestClient(t *testing.T) *asc.Client {
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

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
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

func writeCustomPageTestPNG(t *testing.T, dir, name string, width, height int) string {
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

func customPageFileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	return info.Size()
}
