package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestIDsListsPaginateFromNext(t *testing.T) {
	runPaginateFromNextCases(t, []paginateFromNextCase{
		{
			args:     []string{"merchant-ids", "certificates", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/merchantIds/merchant-1/certificates?cursor=AQ&limit=200",
			dataType: "certificates",
		},
		{
			args:     []string{"merchant-ids", "certificates", "view"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/merchantIds/merchant-1/relationships/certificates?cursor=AQ&limit=200",
			dataType: "certificates",
		},
		{
			args:     []string{"pass-type-ids", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/passTypeIds?cursor=AQ&limit=200",
			dataType: "passTypeIds",
		},
		{
			args:     []string{"merchant-ids", "list"},
			firstURL: "https://api.appstoreconnect.apple.com/v1/merchantIds?cursor=AQ&limit=200",
			dataType: "merchantIds",
		},
	})
}

func TestPassTypeIDCertificatesListRejectsInvalidNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/certificates?cursor=AQ",
			wantErr: "pass-type-ids certificates list: --next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: "pass-type-ids certificates list: --next must be a valid URL:",
		},
		{
			name:    "wrong endpoint",
			next:    "https://api.appstoreconnect.apple.com/v1/users?cursor=AQ",
			wantErr: "--next must target the pass type ID certificates endpoint",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run([]string{"pass-type-ids", "certificates", "list", "--next", test.next}, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestPassTypeIDCertificatesRejectsConflictingPassTypeIDAndNext(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "list",
			args: []string{"pass-type-ids", "certificates", "list", "--pass-type-id", "pass-2", "--next", "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/certificates?cursor=AQ"},
		},
		{
			name: "view",
			args: []string{"pass-type-ids", "certificates", "view", "--pass-type-id", "pass-2", "--next", "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/relationships/certificates?cursor=AQ"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--pass-type-id must match the pass type ID in --next") {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
		})
	}
}

func TestPassTypeIDCertificatesListPaginateFromNextWithoutPassTypeID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/certificates?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/certificates?cursor=BQ&limit=200"

	requestCount := 0
	installAppStoreConnectTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.RequestURI() != "/v1/passTypeIds/pass-1/certificates?cursor=AQ&limit=200" {
				t.Errorf("unexpected first request: %s %s", req.Method, req.URL.String())
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			body := `{"data":[{"type":"certificates","id":"pass-cert-next-1"}],"links":{"next":"` + secondURL + `"}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		case 2:
			if req.Method != http.MethodGet || req.URL.RequestURI() != "/v1/passTypeIds/pass-1/certificates?cursor=BQ&limit=200" {
				t.Errorf("unexpected second request: %s %s", req.Method, req.URL.String())
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			body := `{"data":[{"type":"certificates","id":"pass-cert-next-2"}],"links":{"next":""}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		default:
			t.Errorf("unexpected extra request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pass-type-ids", "certificates", "list", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"pass-cert-next-1"`) || !strings.Contains(stdout, `"id":"pass-cert-next-2"`) {
		t.Fatalf("expected paginated pass type certificates in output, got %q", stdout)
	}
}

func TestPassTypeIDCertificatesGetRejectsInvalidNextURL(t *testing.T) {
	tests := []struct {
		name    string
		next    string
		wantErr string
	}{
		{
			name:    "invalid scheme",
			next:    "http://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/relationships/certificates?cursor=AQ",
			wantErr: "pass-type-ids certificates view: --next must be an App Store Connect URL",
		},
		{
			name:    "malformed URL",
			next:    "https://api.appstoreconnect.apple.com/%zz",
			wantErr: "pass-type-ids certificates view: --next must be a valid URL:",
		},
		{
			name:    "wrong endpoint",
			next:    "https://api.appstoreconnect.apple.com/v1/users?cursor=AQ",
			wantErr: "--next must target the pass type ID certificates endpoint",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run([]string{"pass-type-ids", "certificates", "view", "--next", test.next}, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr %q, got %q", test.wantErr, stderr)
			}
		})
	}
}

func TestPassTypeIDCertificatesGetPaginateFromNextWithoutPassTypeID(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const firstURL = "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/relationships/certificates?cursor=AQ&limit=200"
	const secondURL = "https://api.appstoreconnect.apple.com/v1/passTypeIds/pass-1/relationships/certificates?cursor=BQ&limit=200"

	requestCount := 0
	installAppStoreConnectTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			if req.Method != http.MethodGet || req.URL.RequestURI() != "/v1/passTypeIds/pass-1/relationships/certificates?cursor=AQ&limit=200" {
				t.Errorf("unexpected first request: %s %s", req.Method, req.URL.String())
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			body := `{"data":[{"type":"certificates","id":"pass-rel-next-1"}],"links":{"next":"` + secondURL + `"}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		case 2:
			if req.Method != http.MethodGet || req.URL.RequestURI() != "/v1/passTypeIds/pass-1/relationships/certificates?cursor=BQ&limit=200" {
				t.Errorf("unexpected second request: %s %s", req.Method, req.URL.String())
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			body := `{"data":[{"type":"certificates","id":"pass-rel-next-2"}],"links":{"next":""}}`
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, body)
		default:
			t.Errorf("unexpected extra request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"pass-type-ids", "certificates", "view", "--paginate", "--next", firstURL}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"pass-rel-next-1"`) || !strings.Contains(stdout, `"id":"pass-rel-next-2"`) {
		t.Fatalf("expected paginated pass type certificate relationships in output, got %q", stdout)
	}
}

func installAppStoreConnectTestServer(t *testing.T, handler http.Handler) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}

	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.appstoreconnect.apple.com" {
			return nil, fmt.Errorf("unexpected App Store Connect URL: %s", req.URL.String())
		}

		rewritten := req.Clone(req.Context())
		rewritten.URL.Scheme = serverURL.Scheme
		rewritten.URL.Host = serverURL.Host
		rewritten.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(rewritten)
	}))
}
