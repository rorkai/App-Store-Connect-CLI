package cmdtest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestWebProviderSwitchRefusedEndToEnd runs real web commands against a
// cached session on provider 111 with --provider-id 222. Only HTTP is
// stubbed: Apple's session endpoint answers reads, and any other request,
// including the provider-switch POST, fails the test. Read-only mode exits 6,
// --dry-run exits 2, and the cache stays on provider 111.
func TestWebProviderSwitchRefusedEndToEnd(t *testing.T) {
	const current = "111 / TEAM111 / Team A"
	const requested = "222 / TEAM222 / Team B"
	tests := []struct {
		name       string
		args       []string
		env        string
		wantExit   int
		wantStderr string
	}{
		{
			name:       "read-only flag",
			args:       []string{"--read-only", "web", "removed-apps", "list"},
			wantExit:   rootcmd.ExitReadOnly,
			wantStderr: "Error: --read-only is set; refusing to switch the web session from provider " + current + " to provider " + requested + "\n",
		},
		{
			name:       "ASC_READ_ONLY",
			args:       []string{"web", "removed-apps", "list"},
			env:        "1",
			wantExit:   rootcmd.ExitReadOnly,
			wantStderr: "Error: ASC_READ_ONLY is set; refusing to switch the web session from provider " + current + " to provider " + requested + "\n",
		},
		{
			name:       "dry-run",
			args:       []string{"web", "apps", "delete", "--app", "1234567890", "--dry-run"},
			wantExit:   rootcmd.ExitUsage,
			wantStderr: "Error: --dry-run is set; refusing to switch the web session from provider " + current + " to provider " + requested + "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "config.json"))
			t.Setenv("ASC_WEB_SESSION_CACHE_BACKEND", "file")
			t.Setenv("ASC_WEB_SESSION_CACHE_DIR", dir)
			t.Setenv("ASC_WEB_SESSION_CACHE", "1")
			t.Setenv("ASC_READ_ONLY", test.env)
			t.Setenv(webPasswordEnvNameForTest(), "")
			cachePath := writeCachedWebSessionOnProvider(t, dir, "user@example.com", 111)

			var mu sync.Mutex
			var requests []string
			asctest.SwapDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				mu.Lock()
				requests = append(requests, req.Method+" "+req.URL.Path)
				mu.Unlock()
				if req.Method != http.MethodGet || req.URL.Host != "appstoreconnect.apple.com" || req.URL.Path != "/olympus/v1/session" {
					t.Errorf("unexpected request %s %s", req.Method, req.URL)
					return &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("{}")), Request: req}, nil
				}
				body := `{"provider":{"providerId":111,"publicProviderId":"TEAM111","name":"Team A"},` +
					`"availableProviders":[{"providerId":111,"publicProviderId":"TEAM111","name":"Team A"},{"providerId":222,"publicProviderId":"TEAM222","name":"Team B"}],` +
					`"user":{"emailAddress":"user@example.com"}}`
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			}))

			args := append(append([]string{}, test.args...), "--apple-id", "user@example.com", "--provider-id", "222")
			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})

			if code != test.wantExit {
				t.Fatalf("exit code = %d, want %d; stderr %q", code, test.wantExit, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want it to start with %q", stderr, test.wantStderr)
			}
			for _, request := range requests {
				if request != "GET /olympus/v1/session" {
					t.Fatalf("requests = %v, want only session reads", requests)
				}
			}
			if got := cachedWebSessionProvider(t, cachePath); got != 111 {
				t.Fatalf("cached session is on provider %d, want 111", got)
			}
		})
	}
}

func writeCachedWebSessionOnProvider(t *testing.T, dir, email string, providerID int64) string {
	t.Helper()
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	raw, err := json.Marshal(map[string]any{
		"version":     1,
		"user_email":  email,
		"provider_id": providerID,
		"cookies": map[string]any{
			"https://appstoreconnect.apple.com": []map[string]any{{"name": "myacinfo", "value": "cookie-value"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(dir, "session-"+hex.EncodeToString(sum[:])+".json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func cachedWebSessionProvider(t *testing.T, path string) int64 {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cached session: %v", err)
	}
	var cached struct {
		ProviderID int64 `json:"provider_id"`
	}
	if err := json.Unmarshal(raw, &cached); err != nil {
		t.Fatalf("decode cached session: %v", err)
	}
	return cached.ProviderID
}
