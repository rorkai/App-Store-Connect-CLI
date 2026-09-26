package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesUsageAlertWebhookPost(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		t.Errorf("webhook received a request under read-only mode: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := postUsageAlertJSON(context.Background(), server.URL+"/hooks/secret-path", nil, map[string]any{"event": "test"})
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("postUsageAlertJSON() error = %v, want readonly.ErrRefused", err)
	}
	if strings.Contains(err.Error(), "secret-path") {
		t.Fatalf("refusal leaked the webhook path: %v", err)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("webhook received %d requests, want 0", got)
	}
}
