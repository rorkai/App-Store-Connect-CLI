package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
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
			wantHint:   `use the experimental "asc web apps availability create" flow`,
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
			wantHint:   `use the experimental "asc web apps availability create" flow`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			originalTransport := http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultTransport = originalTransport
			})

			requestCount := 0
			http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++
				if requestCount > 1 {
					t.Fatalf("unexpected extra request: %s %s", req.Method, req.URL.Path)
				}
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appAvailabilityV2" {
					t.Fatalf("unexpected initial availability request: %s %s", req.Method, req.URL.Path)
				}
				return jsonHTTPResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"not found","detail":"missing"}]}`), nil
			})

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
