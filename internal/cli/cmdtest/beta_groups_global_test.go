package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestBetaGroupsListGlobalRequestWiring(t *testing.T) {
	tests := []struct {
		name      string
		appEnv    string
		args      []string
		wantQuery map[string]string
	}{
		{name: "global", args: []string{"--global"}},
		{name: "ignores ASC_APP_ID", appEnv: "app-from-env", args: []string{"--global"}},
		{name: "limit", args: []string{"--global", "--limit", "50"}, wantQuery: map[string]string{"limit": "50"}},
		{name: "internal filter", args: []string{"--global", "--internal"}, wantQuery: map[string]string{"filter[isInternalGroup]": "true"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", test.appEnv)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/betaGroups" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				query := req.URL.Query()
				if got := query.Get("filter[app]"); got != "" {
					t.Fatalf("filter[app] = %q, want none for --global", got)
				}
				for key, want := range test.wantQuery {
					if got := query.Get(key); got != want {
						t.Fatalf("%s = %q, want %q", key, got, want)
					}
				}
				return betaGroupsJSONResponse(`{"data":[{"type":"betaGroups","id":"bg-1","attributes":{"name":"Global Beta"}}]}`), nil
			}))

			stdout, stderr := runBetaGroupsList(t, test.args...)

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"id":"bg-1"`) {
				t.Fatalf("expected beta group id in output, got %q", stdout)
			}
		})
	}
}

func TestTestFlightGroupsErrorKeepsTransportVerbAndURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "groups", "list", "--global"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected run error")
	}
	if message := runErr.Error(); !strings.Contains(message, `Get "https://api.appstoreconnect.apple.com/v1/betaGroups`) {
		t.Fatalf("error = %q, want the HTTP verb and URL unchanged", message)
	}
}
