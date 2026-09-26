package snitch

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesGitHubIssueWrites(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		t.Errorf("GitHub received a request under read-only mode: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	if _, err := createIssue(t.Context(), "test-token", LogEntry{Description: "x", Severity: "bug"}); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("createIssue() error = %v, want readonly.ErrRefused", err)
	}
	if err := addIssueLabels(t.Context(), "test-token", 42, []string{"bug"}); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("addIssueLabels() error = %v, want readonly.ErrRefused", err)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("GitHub received %d requests, want 0", got)
	}
}
