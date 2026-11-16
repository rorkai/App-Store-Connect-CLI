package distribute

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/distribution"
)

// recordingObjectStoreTransport answers like an empty S3 bucket and records
// every request it receives.
type recordingObjectStoreTransport struct {
	mu       sync.Mutex
	requests []string
}

func (r *recordingObjectStoreTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.requests = append(r.requests, request.Method+" "+request.URL.Host)
	r.mu.Unlock()
	status := http.StatusNotFound
	if request.Method == http.MethodPut {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: http.NoBody, Request: request}, nil
}

func (r *recordingObjectStoreTransport) count(method string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, request := range r.requests {
		if strings.HasPrefix(request, method+" ") {
			n++
		}
	}
	return n
}

func TestPublishObjectStoreClientFollowsInjectedTransport(t *testing.T) {
	originalStore := newObjectStore
	t.Cleanup(func() { newObjectStore = originalStore })
	bundleDir := t.TempDir()
	writePrivatePublishPreparedBundle(t, bundleDir, []byte("ipa"), "11111111-2222-3333-4444-555555555555")
	t.Setenv("ASC_S3_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("ASC_S3_SECRET_ACCESS_KEY", "secret")
	t.Setenv("ASC_S3_SESSION_TOKEN", "")
	run := func(t *testing.T, d *shared.Deps) error {
		t.Helper()
		stateDir := t.TempDir()
		return newPublishCommand(d).ParseAndRun(context.Background(), []string{
			"--bundle-dir", bundleDir, "--endpoint", "https://objects.example.com", "--region", "auto", "--bucket", "bucket", "--prefix", "app",
			"--receipt", filepath.Join(stateDir, "receipt.json"), "--link-path", filepath.Join(stateDir, "link.json"), "--output", "json",
		})
	}

	t.Run("default deps leave the store on the SDK's client", func(t *testing.T) {
		t.Setenv("AWS_CA_BUNDLE", "")
		if client := objectStoreHTTPClient(shared.DefaultDeps()); client != nil {
			t.Fatalf("objectStoreHTTPClient(DefaultDeps()) = %v, want nil", client)
		}
		var config distribution.S3StoreConfig
		newObjectStore = func(_ context.Context, got distribution.S3StoreConfig) (distribution.ObjectStore, time.Time, error) {
			config = got
			return nil, time.Time{}, context.Canceled
		}
		_ = run(t, newDistributeTestDeps())
		if config.Endpoint == "" || config.HTTPClient != nil {
			t.Fatalf("store config = %+v, want an endpoint and no HTTP client", config)
		}
	})
	t.Run("an injected transport carries the S3 requests and the verification", func(t *testing.T) {
		t.Setenv("AWS_CA_BUNDLE", "")
		newObjectStore = originalStore
		transport := &recordingObjectStoreTransport{}
		d := newDistributeTestDeps()
		d.HTTPTransport = func() http.RoundTripper { return transport }
		runErr := run(t, d)
		if transport.count(http.MethodPut) == 0 || transport.count(http.MethodGet) == 0 {
			t.Fatalf("requests on the injected transport = %v (run error %v), want S3 uploads and a verification GET", transport.requests, runErr)
		}
	})
	t.Run("AWS_CA_BUNDLE with an injected transport fails the store setup before any upload", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer server.Close()
		caPath := filepath.Join(t.TempDir(), "ca.pem")
		if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("AWS_CA_BUNDLE", caPath)
		newObjectStore = originalStore
		transport := &recordingObjectStoreTransport{}
		d := newDistributeTestDeps()
		d.HTTPTransport = func() http.RoundTripper { return transport }
		err := run(t, d)
		if err == nil || !strings.Contains(err.Error(), "load S3 configuration failed") {
			t.Fatalf("run error = %v, want the S3 configuration error", err)
		}
		if len(transport.requests) != 0 {
			t.Fatalf("requests on the injected transport = %v, want none", transport.requests)
		}
	})
}

func TestPreflightDistributionPublicationObjectStoreClientFollowsInjectedTransport(t *testing.T) {
	original := newObjectStore
	t.Cleanup(func() { newObjectStore = original })
	var config distribution.S3StoreConfig
	newObjectStore = func(_ context.Context, got distribution.S3StoreConfig) (distribution.ObjectStore, time.Time, error) {
		config = got
		return nil, time.Time{}, nil
	}

	if err := preflightDistributionPublication(context.Background(), newDistributeTestDeps(), validDistributionOrchestrationConfig().Publication); err != nil {
		t.Fatal(err)
	}
	if config.HTTPClient != nil {
		t.Fatalf("store config with the default transport = %+v, want no HTTP client", config)
	}

	transport := &recordingObjectStoreTransport{}
	d := newDistributeTestDeps()
	d.HTTPTransport = func() http.RoundTripper { return transport }
	if err := preflightDistributionPublication(context.Background(), d, validDistributionOrchestrationConfig().Publication); err != nil {
		t.Fatal(err)
	}
	if config.HTTPClient == nil || config.HTTPClient.Transport != transport {
		t.Fatalf("store config with an injected transport = %+v, want a client on that transport", config)
	}
}
