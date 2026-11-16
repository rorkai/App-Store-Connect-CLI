package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNotifySlackRefusesRedirectWithoutForwardingPayloadOrReferer(t *testing.T) {
	var redirectedRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		body, _ := io.ReadAll(r.Body)
		t.Errorf("redirect target received request: method=%s referer=%q body=%q", r.Method, r.Referer(), body)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture", http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	t.Setenv(slackWebhookEnvVar, "")
	t.Setenv(slackWebhookAllowLocalEnv, "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	d, _ := newNotifyTestDeps(source.Client().Transport)
	root := newSlackCommand(d)
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{
		"--webhook", source.URL + "/services/T/B/redirect-secret",
		"--message", "secret message body",
	}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	err := root.Run(context.Background())
	if err == nil {
		t.Fatal("expected redirect response to fail")
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests, want 0", got)
	}
	if !strings.Contains(err.Error(), "unexpected response 307") {
		t.Fatalf("unexpected redirect error: %v", err)
	}
}
