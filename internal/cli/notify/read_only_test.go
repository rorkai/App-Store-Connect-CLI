package notify

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestNotifySlackRefusedInReadOnlyModeWithoutLeakingWebhookPath(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		t.Errorf("webhook received a request under read-only mode: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhookURL := server.URL + "/services/T000/B000/secret-token"
	t.Setenv(slackWebhookEnvVar, webhookURL)
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := SlackCommand()
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"--message", "Hello, Slack!"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	runErr := root.Run(context.Background())
	if !errors.Is(runErr, readonly.ErrRefused) {
		t.Fatalf("Run() error = %v, want readonly.ErrRefused", runErr)
	}
	if strings.Contains(runErr.Error(), "secret-token") {
		t.Fatalf("refusal leaked the webhook path: %v", runErr)
	}
	if !strings.Contains(runErr.Error(), "refusing POST "+server.URL) {
		t.Fatalf("refusal = %q, want the webhook host as target", runErr.Error())
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("webhook received %d requests, want 0", got)
	}
}
