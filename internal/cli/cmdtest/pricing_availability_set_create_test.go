package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestAvailabilitySet_MissingAvailabilityReturnsUpdateOnlyError(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantPrefix string
		wantHint   string
	}{
		{
			name: "pricing availability edit",
			args: []string{
				"pricing", "availability", "edit",
				"--app", "app-1",
				"--territory", "usa,gbr",
				"--available", "true",
				"--available-in-new-territories", "false",
				"--output", "json",
			},
			wantPrefix: `pricing availability edit: app availability not found for app "app-1"; this command only updates existing app availability`,
			wantHint:   `use "asc pricing availability create" first. If Apple rejects public-API bootstrap`,
		},
		{
			name: "app-setup availability edit",
			args: []string{
				"app-setup", "availability", "edit",
				"--app", "app-1",
				"--territory", "usa,gbr",
				"--available", "true",
				"--available-in-new-territories", "false",
				"--output", "json",
			},
			wantPrefix: `app-setup availability edit: app availability not found for app "app-1"; this command only updates existing app availability`,
			wantHint:   `use "asc pricing availability create" first. If Apple rejects public-API bootstrap`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if requestCount > 1 {
					t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.Path)
				}
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appAvailabilityV2" {
					t.Fatalf("unexpected initial availability request: %s %s", req.Method, req.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found","detail":"missing"}]}`))
			}))
			t.Cleanup(server.Close)
			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse server URL: %v", err)
			}
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			})
			client, err := asc.NewClientWithHTTPClient(
				"TEST_KEY",
				"TEST_ISSUER",
				os.Getenv("ASC_PRIVATE_KEY_PATH"),
				&http.Client{Transport: transport},
			)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			restore := shared.SetAvailabilityClientFactory(func() (*asc.Client, error) {
				return client, nil
			})
			t.Cleanup(restore)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tc.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected missing-availability error")
				}
				if !errors.Is(err, asc.ErrNotFound) {
					t.Fatalf("expected asc.ErrNotFound, got %v", err)
				}
				var apiErr *asc.APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
					t.Fatalf("expected wrapped 404 API error, got %v", err)
				}
				if got := cmd.ExitCodeFromError(err); got != cmd.ExitNotFound {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, got)
				}
				if !strings.Contains(err.Error(), tc.wantPrefix) {
					t.Fatalf("expected update-only error, got %q", err.Error())
				}
				if !strings.Contains(err.Error(), tc.wantHint) {
					t.Fatalf("expected bootstrap hint in error, got %q", err.Error())
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if requestCount != 1 {
				t.Fatalf("expected only the missing-availability lookup request, got %d requests", requestCount)
			}
		})
	}
}
