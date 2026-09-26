package apps

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesCommunityWallGitHubWritesButAllowsReads(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("mutating request left the GitHub client: %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"login":"tester"}`))
	}))
	defer server.Close()
	previousBase, previousClient := communityWallGitHubAPIBase, communityWallGitHubClient
	communityWallGitHubAPIBase = server.URL
	communityWallGitHubClient = func() *http.Client { return server.Client() }
	t.Cleanup(func() {
		communityWallGitHubAPIBase = previousBase
		communityWallGitHubClient = previousClient
	})

	client := communityWallGitHubClientAPI{Token: "token"}
	if _, _, err := client.request(context.Background(), http.MethodGet, "/user", nil); err != nil {
		t.Fatalf("GET /user error = %v, want nil", err)
	}
	_, _, err := client.request(context.Background(), http.MethodPost, "/repos/rorkai/App-Store-Connect-CLI/pulls", map[string]string{"title": "x"})
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("POST pulls error = %v, want readonly.ErrRefused", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("GitHub received %d requests, want 1 (the GET only)", got)
	}
}
