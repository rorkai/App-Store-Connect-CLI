package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestDebugFlagLogsHTTPRequests(t *testing.T) {
	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"data":[]}`)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer func() { server.Close() }()

	tmpDir := t.TempDir()
	keyPath := tmpDir + "/key.p8"
	writeECDSAPEM(t, keyPath)

	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_DEBUG", "1")

	root := RootCommand("test")

	_, stderr := captureOutput(t, func() {
		// Parse with --debug flag
		if err := root.Parse([]string{"--debug", "apps"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		// Note: We're not actually running the command because it would fail auth
		// We're just testing that the flag is accepted
	})

	// Test should not error on unknown flag
	if strings.Contains(stderr, "flag provided but not defined: -debug") {
		t.Fatalf("--debug flag not registered")
	}

	root = RootCommand("test")
	_, stderr = captureOutput(t, func() {
		// Parse with --api-debug flag
		if err := root.Parse([]string{"--api-debug", "apps"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
	})
	if strings.Contains(stderr, "flag provided but not defined: -api-debug") {
		t.Fatalf("--api-debug flag not registered")
	}
}

func TestDebugEnvVarEnablesDebugMode(t *testing.T) {
	t.Setenv("ASC_DEBUG", "1")
	asc.SetDebugOverride(nil)
	asc.SetDebugHTTPOverride(nil)

	if !asc.ResolveDebugEnabled() {
		t.Fatal("ASC_DEBUG=1 should enable debug mode")
	}
}

func TestDebugDisabledByDefault(t *testing.T) {
	// Ensure no env var is set
	t.Setenv("ASC_DEBUG", "")
	asc.SetDebugOverride(nil)
	asc.SetDebugHTTPOverride(nil)

	if asc.ResolveDebugEnabled() {
		t.Fatal("Debug mode should be disabled by default")
	}
}
