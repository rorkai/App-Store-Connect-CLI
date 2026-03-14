package migrate

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

type migrateUploadRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn migrateUploadRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestUploadScreenshots_ReordersPlannedFilesBeforeUntouchedRemoteExtras(t *testing.T) {
	dir := t.TempDir()
	existingFile := writeMigrateTestPNG(t, dir, "01-home.png")
	newFile := writeMigrateTestPNG(t, dir, "02-settings.png")
	newFileSize := migrateFileSize(t, newFile)
	relationshipPatchCalled := false

	origTransport := http.DefaultTransport
	http.DefaultTransport = migrateUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/appScreenshots":
			return migrateJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":[{"type":"appScreenshots","id":"old-home","attributes":{"fileName":"%s"}},{"type":"appScreenshots","id":"old-extra","attributes":{"fileName":"99-legacy.png"}}]}`, filepath.Base(existingFile)))
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return migrateJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-extra"},{"type":"appScreenshots","id":"old-home"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			return migrateJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"new-settings","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/new-settings","length":%d,"offset":0}]}}}`, newFileSize))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return migrateJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appScreenshots/new-settings":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-settings","attributes":{"uploaded":true}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshots/new-settings":
			return migrateJSONResponse(http.StatusOK, `{"data":{"type":"appScreenshots","id":"new-settings","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
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
			wantIDs := []string{"old-home", "new-settings", "old-extra"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return migrateJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newMigrateUploadTestClient(t)
	results, err := uploadScreenshots(
		context.Background(),
		client,
		"version-1",
		map[string]string{"en-US": "loc-1"},
		[]ScreenshotPlan{{
			Locale:      "en-US",
			DisplayType: "APP_IPHONE_65",
			Files:       []string{existingFile, newFile},
		}},
	)
	if err != nil {
		t.Fatalf("uploadScreenshots() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 screenshot result, got %d", len(results))
	}
	if len(results[0].Skipped) != 1 || results[0].Skipped[0].Path != existingFile {
		t.Fatalf("expected existing file to be skipped, got %#v", results[0].Skipped)
	}
	if len(results[0].Uploaded) != 1 || results[0].Uploaded[0].AssetID != "new-settings" {
		t.Fatalf("expected uploaded screenshot new-settings, got %#v", results[0].Uploaded)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func newMigrateUploadTestClient(t *testing.T) *asc.Client {
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

func migrateJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func writeMigrateTestPNG(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer file.Close()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return path
}

func migrateFileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	return info.Size()
}
