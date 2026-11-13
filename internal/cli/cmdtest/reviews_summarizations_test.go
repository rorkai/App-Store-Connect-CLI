package cmdtest

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestReviewsSummarizationsUsageErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	runUsageExitCases(t, []usageExitCase{
		{
			name:    "requires app",
			args:    []string{"reviews", "summarizations"},
			wantErr: "Error: --app is required (or set ASC_APP_ID)",
		},
		{
			name:    "requires platform without next",
			args:    []string{"reviews", "summarizations", "--app", "app-1"},
			wantErr: "Error: --platform is required",
		},
	})
}

func TestReviewsSummarizationsNextDoesNotRequirePlatform(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_APP_ID", "ambient-app-must-not-affect-continuation")

	const nextURL = "https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviewSummarizations?cursor=abc"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		actualURL := "https://" + req.Host + req.URL.RequestURI()
		if req.Method != http.MethodGet || actualURL != nextURL {
			t.Errorf("expected GET %s, got %s %s", nextURL, req.Method, actualURL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[],"links":{"self":"`+nextURL+`"}}`)
	}))
	t.Cleanup(server.Close)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		cloned.Host = req.URL.Host
		return server.Client().Transport.RoundTrip(cloned)
	}))

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"reviews", "summarizations", "--next", nextURL, "--output", "json"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if !strings.Contains(stdout, `"data":[]`) {
		t.Fatalf("expected JSON response on stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}
