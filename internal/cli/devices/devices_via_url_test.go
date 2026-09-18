package devices

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDeviceURLCallbackRegistersAndSkipsDuplicates(t *testing.T) {
	var posts int
	client := newDeviceURLTestClient(t, func(req *http.Request) *http.Response {
		switch {
		case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/devices"):
			return deviceURLJSON(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/devices":
			posts++
			return deviceURLJSON(http.StatusCreated, `{"data":{"type":"devices","id":"dev-1","attributes":{"name":"iPhone","udid":"ABCDEF0123456789","platform":"IOS"}}}`)
		default:
			t.Fatalf("unexpected %s %s", req.Method, req.URL.Path)
			return nil
		}
	})
	server := &deviceURLServer{
		token:    "token-1",
		platform: "IOS",
		confirm:  true,
		client:   client,
		seen:     map[string]struct{}{},
	}
	body := []byte(`{"UDID":"ABCDEF0123456789","PRODUCT":"iPhone"}`)
	first := httptest.NewRecorder()
	server.callback(first, httptest.NewRequest(http.MethodPost, "/callback?token=token-1", strings.NewReader(string(body))))
	if first.Code != http.StatusOK {
		t.Fatalf("first status %d body %s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	server.callback(second, httptest.NewRequest(http.MethodPost, "/callback?token=token-1", strings.NewReader(string(body))))
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"skipped"`) {
		t.Fatalf("second status %d body %s", second.Code, second.Body.String())
	}
	if posts != 1 || len(server.devices) != 1 || server.devices[0].Status != "registered" {
		t.Fatalf("devices = %#v", server.devices)
	}
	unknown := httptest.NewRecorder()
	server.callback(unknown, httptest.NewRequest(http.MethodPost, "/callback?token=other", strings.NewReader(string(body))))
	if unknown.Code != http.StatusForbidden {
		t.Fatalf("unknown token status = %d", unknown.Code)
	}
}

func TestDeviceURLCollectOnlyRoundTripsRegisterBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.tsv")
	records := []deviceBatchRecord{{UDID: "ABCDEF0123456789", Name: "iPhone abcdef01", Platform: "IOS"}}
	if err := writeDeviceRegistrationBatch(path, records); err != nil {
		t.Fatal(err)
	}
	parsed, err := readDeviceBatchTSV(path, "IOS")
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].UDID != records[0].UDID || parsed[0].Name != records[0].Name {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestDeviceURLListenStaysOnLoopback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "devices.tsv")
	result, err := serveDeviceRegistration(context.Background(), deviceURLServeOptions{
		Listen:     "127.0.0.1:0",
		TTL:        20 * time.Millisecond,
		Platform:   "IOS",
		OutputFile: path,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.URL, "127.0.0.1:") {
		t.Fatalf("url = %s", result.URL)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := validateDeviceRegisterListen("0.0.0.0:8080", false); err == nil {
		t.Fatal("expected public bind without explicit listen to be rejected")
	}
	if err := validateDeviceRegisterListen("0.0.0.0:8080", true); err != nil {
		t.Fatalf("explicit public bind rejected: %v", err)
	}
}

func newDeviceURLTestClient(t *testing.T, fn func(*http.Request) *http.Response) *asc.Client {
	t.Helper()
	transport := roundTripperFunc(fn)
	client, err := asc.NewClientWithHTTPClient("KEY", "ISSUER", writeDeviceURLKey(t), &http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

type roundTripperFunc func(*http.Request) *http.Response

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req), nil
}

func deviceURLJSON(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func writeDeviceURLKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "key.p8")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
