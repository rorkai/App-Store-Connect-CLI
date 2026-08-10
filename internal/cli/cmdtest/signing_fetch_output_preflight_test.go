package cmdtest

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestSigningFetchPreflightsOutputBeforeProfileCreation(t *testing.T) {
	setupAuth(t)

	outputPath := filepath.Join(t.TempDir(), "not-a-directory")
	sentinel := []byte("keep this file")
	if err := os.WriteFile(outputPath, sentinel, 0o600); err != nil {
		t.Fatalf("write output sentinel: %v", err)
	}

	certificateContent := base64.StdEncoding.EncodeToString([]byte("certificate"))
	profileContent := base64.StdEncoding.EncodeToString([]byte("profile"))
	var bundleLookups atomic.Int32
	var profileLookups atomic.Int32
	var certificateLookups atomic.Int32
	var profileCreates atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds":
			bundleLookups.Add(1)
			if got := req.URL.Query().Get("filter[identifier]"); got != "com.example.app" {
				t.Errorf("bundle identifier filter = %q, want com.example.app", got)
			}
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[{"type":"bundleIds","id":"bundle-main","attributes":{"identifier":"com.example.app"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/bundleIds/bundle-main/profiles":
			profileLookups.Add(1)
			writeSigningFetchOutputJSON(t, w, http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/certificates":
			certificateLookups.Add(1)
			writeSigningFetchOutputJSON(t, w, http.StatusOK, fmt.Sprintf(
				`{"data":[{"type":"certificates","id":"cert-1","attributes":{"certificateType":"IOS_DISTRIBUTION","serialNumber":"CERT1","certificateContent":%q,"activated":true,"expirationDate":"2100-01-01T00:00:00Z"}}]}`,
				certificateContent,
			))
		case req.Method == http.MethodPost && req.URL.Path == "/v1/profiles":
			profileCreates.Add(1)
			writeSigningFetchOutputJSON(t, w, http.StatusCreated, fmt.Sprintf(
				`{"data":{"type":"profiles","id":"profile-created","attributes":{"name":"Created Profile","profileType":"IOS_APP_STORE","profileState":"ACTIVE","profileContent":%q}}}`,
				profileContent,
			))
		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	client, err := asc.NewClientWithHTTPClient(
		os.Getenv("ASC_KEY_ID"),
		os.Getenv("ASC_ISSUER_ID"),
		os.Getenv("ASC_PRIVATE_KEY_PATH"),
		&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			cloned := req.Clone(req.Context())
			cloned.URL.Scheme = serverURL.Scheme
			cloned.URL.Host = serverURL.Host
			return serverTransport.RoundTrip(cloned)
		})},
	)
	if err != nil {
		t.Fatalf("create signing fetch test client: %v", err)
	}
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{
			"signing", "fetch",
			"--bundle-id", "com.example.app",
			"--profile-type", "IOS_APP_STORE",
			"--create-missing",
			"--output", outputPath,
			"--format", "json",
		}); err != nil {
			t.Fatalf("parse signing fetch: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("signing fetch succeeded with a regular file as its output directory")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output", stdout)
	}
	if got := profileCreates.Load(); got != 0 {
		t.Fatalf("profile create requests = %d, want 0", got)
	}
	if got := bundleLookups.Load(); got != 1 {
		t.Fatalf("bundle ID lookups = %d, want 1", got)
	}
	if got := profileLookups.Load(); got != 1 {
		t.Fatalf("profile lookups = %d, want 1", got)
	}
	if got := certificateLookups.Load(); got != 1 {
		t.Fatalf("certificate lookups = %d, want 1", got)
	}
	gotSentinel, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output sentinel: %v", err)
	}
	if string(gotSentinel) != string(sentinel) {
		t.Fatalf("output sentinel = %q, want %q", gotSentinel, sentinel)
	}
}

func writeSigningFetchOutputJSON(t *testing.T, w http.ResponseWriter, status int, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := io.WriteString(w, body); err != nil {
		t.Errorf("write JSON response: %v", err)
	}
}
