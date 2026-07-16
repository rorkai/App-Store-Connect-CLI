package assets

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
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

type assetsUploadRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn assetsUploadRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestUploadScreenshotsToSet_PreservesExistingOrderAndAppendsNewUploads(t *testing.T) {
	fileA := writeAssetsTestPNG(t, t.TempDir(), "01-home.png")
	fileB := writeAssetsTestPNG(t, t.TempDir(), "02-settings.png")
	files := []string{fileA, fileB}
	sizes := []int64{fileSize(t, fileA), fileSize(t, fileB)}
	relationshipPatchCalled := false

	origTransport := http.DefaultTransport
	http.DefaultTransport = assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appScreenshotSets/set-1/relationships/appScreenshots":
			return assetsJSONResponse(http.StatusOK, `{"data":[{"type":"appScreenshots","id":"old-1"},{"type":"appScreenshots","id":"old-2"}],"links":{}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/appScreenshots":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read create screenshot body: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode create screenshot body: %v", err)
			}
			id := "new-1"
			size := sizes[0]
			if strings.Contains(string(body), "02-settings.png") {
				id = "new-2"
				size = sizes[1]
			}
			return assetsJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/%s","length":%d,"offset":0}]}}}`, id, id, size))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return assetsJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"uploaded":true}}}`, id))
		case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/v1/appScreenshots/"):
			id := strings.TrimPrefix(req.URL.Path, "/v1/appScreenshots/")
			return assetsJSONResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appScreenshots","id":"%s","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`, id))
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
			wantIDs := []string{"old-1", "old-2", "new-1", "new-2"}
			if !reflect.DeepEqual(gotIDs, wantIDs) {
				t.Fatalf("relationship order = %v, want %v", gotIDs, wantIDs)
			}
			relationshipPatchCalled = true
			return assetsJSONResponse(http.StatusNoContent, "")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})
	t.Cleanup(func() {
		http.DefaultTransport = origTransport
	})

	client := newAssetsUploadTestClient(t)
	results, err := UploadScreenshotsToSet(context.Background(), client, "set-1", files, true)
	if err != nil {
		t.Fatalf("UploadScreenshotsToSet() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 uploaded screenshots, got %d", len(results))
	}
	if results[0].AssetID != "new-1" || results[1].AssetID != "new-2" {
		t.Fatalf("unexpected uploaded asset IDs: %#v", results)
	}
	if !relationshipPatchCalled {
		t.Fatal("expected screenshot relationship reorder PATCH to be called")
	}
}

func newAssetsUploadTestClient(t *testing.T) *asc.Client {
	t.Helper()
	pemBytes := newAssetsUploadTestPrivateKeyPEM(t)

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func newAssetsUploadTestServerClient(t *testing.T, handler http.Handler) *asc.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	httpClient := &http.Client{Transport: assetsUploadRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = target.Scheme
		cloned.URL.Host = target.Host
		cloned.Host = target.Host
		return serverTransport.RoundTrip(cloned)
	})}

	pemBytes := newAssetsUploadTestPrivateKeyPEM(t)
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient("KEY_ID", "ISSUER_ID", keyPath, httpClient)
	if err != nil {
		t.Fatalf("new client with test server: %v", err)
	}
	return client
}

func writeAssetsTestJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func newAssetsUploadTestPrivateKeyPEM(t *testing.T) []byte {
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
	return pemBytes
}

func assetsJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func writeAssetsTestPNG(t *testing.T, dir, name string) string {
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

func fileSize(t *testing.T, path string) int64 {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	return info.Size()
}
