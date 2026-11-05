package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebAppsCreateIfExistsRejectsUnsupportedModesBeforeHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		http.Error(w, "no HTTP expected", http.StatusInternalServerError)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	for _, value := range []string{"update", "overwrite", ""} {
		t.Run("value="+value, func(t *testing.T) {
			assertUsageExit(t, webAppsCreateAccessArgs("--if-exists", value), `--if-exists must be one of fail, skip (got "`+value+`")`)
		})
	}
	if requests != 0 {
		t.Fatalf("expected no HTTP, got %d requests", requests)
	}
}

func TestWebAppsCreateIfExistsSkipRejectsAccessBeforeHTTP(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests++
		http.Error(w, "no HTTP expected", http.StatusInternalServerError)
	}))
	defer server.Close()
	setWebAppsCreateASCClient(t, server)

	assertUsageExit(t, webAppsCreateAccessArgs("--if-exists", "skip", "--access", "full"), "--if-exists skip cannot be combined with --access")
	if requests != 0 {
		t.Fatalf("expected no HTTP, got %d requests", requests)
	}
}
