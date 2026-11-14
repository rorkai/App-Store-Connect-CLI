package cmdtest

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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

			var query []string
			for key, value := range test.wantQuery {
				query = append(query, key+"="+value)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/betaGroups", query...),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"betaGroups","id":"bg-1","attributes":{"name":"Global Beta"}}]}`))

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

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/betaGroups"), func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	_, _, runErr := runCommand(t, []string{"testflight", "groups", "list", "--global"})

	if runErr == nil {
		t.Fatal("expected run error")
	}
	if message := runErr.Error(); !strings.Contains(message, `Get "https://api.appstoreconnect.apple.com/v1/betaGroups`) {
		t.Fatalf("error = %q, want the HTTP verb and URL unchanged", message)
	}
}
