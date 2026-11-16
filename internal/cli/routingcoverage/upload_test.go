package routingcoverage

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type routingCoverageRoundTripFunc func(*http.Request) (*http.Response, error)

func (f routingCoverageRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newRoutingCoverageTestClient(t *testing.T, handler http.Handler) (*asc.Client, string) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	httpClient := &http.Client{Transport: routingCoverageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = target.Scheme
		cloned.URL.Host = target.Host
		cloned.Host = target.Host
		return serverTransport.RoundTrip(cloned)
	})}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	client, err := asc.NewClientWithHTTPClient("KEY_ID", "ISSUER_ID", keyPath, httpClient)
	if err != nil {
		t.Fatalf("new client with test server: %v", err)
	}
	return client, server.URL
}

// TestUploadPreparedRoutingCoverageFileUploadsSnapshotWhileSourceChanges pins the
// isolation the upload actually provides: bytes come from an unlinked snapshot
// taken before the reservation, so rewriting the operator's file mid-upload can
// neither corrupt the upload nor abort it.
func TestUploadPreparedRoutingCoverageFileUploadsSnapshotWhileSourceChanges(t *testing.T) {
	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	t.Chdir(filepath.Dir(coveragePath))

	prepared, err := PrepareRoutingCoverageFile(shared.DefaultDeps(), coveragePath)
	if err != nil {
		t.Fatalf("PrepareRoutingCoverageFile() error: %v", err)
	}

	var mu sync.Mutex
	var uploaded string
	var committedChecksum string
	var uploadURL string

	client, serverURL := newRoutingCoverageTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			mu.Lock()
			target := uploadURL
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_1","attributes":{"uploadOperations":[{"method":"PUT","url":%q,"offset":0,"length":%d}]}}}`, target, prepared.FileSize)
		case req.Method == http.MethodPut && req.URL.Path == "/upload":
			// Rewrite the operator's file while its bytes are being uploaded.
			if err := os.WriteFile(coveragePath, []byte(`{"type":"MultiPolygon","coordinates":[]}`), 0o600); err != nil {
				t.Errorf("rewrite routing coverage fixture: %v", err)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read upload body: %v", err)
			}
			mu.Lock()
			uploaded = string(body)
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_1":
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read commit body: %v", err)
			}
			mu.Lock()
			committedChecksum = string(body)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	mu.Lock()
	uploadURL = serverURL + "/upload"
	mu.Unlock()

	committed, err := UploadPreparedRoutingCoverageFileWith(context.Background(), shared.DefaultDeps(), client, "VERSION_1", prepared)
	if err != nil {
		t.Fatalf("UploadPreparedRoutingCoverageFile() error: %v", err)
	}
	if committed == nil || committed.Data.ID != "COVERAGE_1" {
		t.Fatalf("committed response = %+v, want COVERAGE_1", committed)
	}

	mu.Lock()
	defer mu.Unlock()
	if uploaded != validRoutingCoverageGeoJSON {
		t.Fatalf("uploaded body = %q, want the validated snapshot %q", uploaded, validRoutingCoverageGeoJSON)
	}
	if !strings.Contains(committedChecksum, prepared.Checksum) {
		t.Fatalf("commit body = %q, want prepared checksum %q", committedChecksum, prepared.Checksum)
	}
}
