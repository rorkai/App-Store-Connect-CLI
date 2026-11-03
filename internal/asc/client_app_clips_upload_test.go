package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func newUploadTestClient(t *testing.T, handler func(*http.Request) (*http.Response, error)) *Client {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return handler(req)
	})

	return &Client{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		privateKey: key,
	}
}

func assertAppClipImageCommit(t *testing.T, req *http.Request, content []byte) {
	t.Helper()

	var payload struct {
		Data struct {
			Attributes struct {
				SourceFileChecksum string `json:"sourceFileChecksum"`
				Uploaded           bool   `json:"uploaded"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		t.Fatalf("decode commit body: %v", err)
	}
	sum := md5.Sum(content)
	if got, want := payload.Data.Attributes.SourceFileChecksum, hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("sourceFileChecksum = %q, want %q", got, want)
	}
	if !payload.Data.Attributes.Uploaded {
		t.Fatal("uploaded = false, want true")
	}
}

func TestUploadAppClipHeaderImage(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "header.png")
	if err := os.WriteFile(filePath, []byte("header-image"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalled = true
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT upload, got %s", r.Method)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	client := newUploadTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appClipHeaderImages":
			body := fmt.Sprintf(`{"data":{"type":"appClipHeaderImages","id":"img-1","attributes":{"uploadOperations":[{"method":"PUT","url":"%s","length":%d,"offset":0}]}}}`, server.URL, fileInfo.Size())
			return jsonResponse(http.StatusCreated, body), nil
		case "/v1/appClipHeaderImages/img-1":
			assertAppClipImageCommit(t, req, []byte("header-image"))
			body := `{"data":{"type":"appClipHeaderImages","id":"img-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`
			return jsonResponse(http.StatusOK, body), nil
		default:
			return jsonResponse(http.StatusNotFound, `{"errors":[{"title":"not found"}]}`), nil
		}
	})

	result, err := client.UploadAppClipHeaderImage(context.Background(), "loc-1", filePath)
	if err != nil {
		t.Fatalf("UploadAppClipHeaderImage() error: %v", err)
	}
	if !uploadCalled {
		t.Fatalf("expected upload operation to be called")
	}
	if result.ID != "img-1" {
		t.Fatalf("expected result id img-1, got %s", result.ID)
	}
	if result.LocalizationID != "loc-1" {
		t.Fatalf("expected localization id loc-1, got %s", result.LocalizationID)
	}
	if result.AssetDeliveryState != "COMPLETE" {
		t.Fatalf("expected state COMPLETE, got %s", result.AssetDeliveryState)
	}
}

func TestUploadAppClipAdvancedExperienceImage(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "advanced.png")
	if err := os.WriteFile(filePath, []byte("advanced-image"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	uploadCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uploadCalled = true
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT upload, got %s", r.Method)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}

	client := newUploadTestClient(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/v1/appClipAdvancedExperienceImages":
			body := fmt.Sprintf(`{"data":{"type":"appClipAdvancedExperienceImages","id":"img-2","attributes":{"uploadOperations":[{"method":"PUT","url":"%s","length":%d,"offset":0}]}}}`, server.URL, fileInfo.Size())
			return jsonResponse(http.StatusCreated, body), nil
		case "/v1/appClipAdvancedExperienceImages/img-2":
			assertAppClipImageCommit(t, req, []byte("advanced-image"))
			body := `{"data":{"type":"appClipAdvancedExperienceImages","id":"img-2","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`
			return jsonResponse(http.StatusOK, body), nil
		default:
			return jsonResponse(http.StatusNotFound, `{"errors":[{"title":"not found"}]}`), nil
		}
	})

	result, err := client.UploadAppClipAdvancedExperienceImage(context.Background(), filePath)
	if err != nil {
		t.Fatalf("UploadAppClipAdvancedExperienceImage() error: %v", err)
	}
	if !uploadCalled {
		t.Fatalf("expected upload operation to be called")
	}
	if result.ID != "img-2" {
		t.Fatalf("expected result id img-2, got %s", result.ID)
	}
	if result.AssetDeliveryState != "COMPLETE" {
		t.Fatalf("expected state COMPLETE, got %s", result.AssetDeliveryState)
	}
}
