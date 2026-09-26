package cmdtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAppTagTerritoryDeprecation(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		warn, include bool
		fields        string
	}{
		{name: "view include", args: []string{"view", "--app", "app-1", "--id", "tag-1", "--include", "territories"}, warn: true, include: true},
		{name: "view fields", args: []string{"view", "--app", "app-1", "--id", "tag-1", "--fields", "name,territories"}, warn: true, fields: "name,territories"},
		{name: "list fields", args: []string{"list", "--app", "app-1", "--fields", "territories"}, warn: true, fields: "territories"},
		{name: "next include", args: []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?include=territories&cursor=next"}, warn: true},
		{name: "next fields", args: []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?fields%5BappTags%5D=territories&cursor=next"}, warn: true, fields: "territories"},
		{name: "next territory parameters", args: []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?fields%5Bterritories%5D=currency&limit%5Bterritories%5D=2&cursor=next"}, warn: true},
		{name: "plain view", args: []string{"view", "--app", "app-1", "--id", "tag-1"}},
		{name: "plain list", args: []string{"list", "--app", "app-1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			calls := 0
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != http.MethodGet {
					t.Errorf("method=%s", r.Method)
				}
				body := `{"data":[{"type":"appTags","id":"tag-1","attributes":{"name":"Adventure","visibleInAppStore":true}}],"links":{}}`
				if calls == 1 {
					if r.URL.Path != "/v1/apps/app-1/appTags" || r.URL.Query().Get("fields[appTags]") != tt.fields {
						t.Errorf("URL=%s", r.URL)
					}
					if strings.HasPrefix(tt.name, "next") && r.URL.Query().Get("cursor") != "next" {
						t.Errorf("lost continuation query: %s", r.URL)
					}
					if tt.name == "next territory parameters" && (r.URL.Query().Get("fields[territories]") != "currency" || r.URL.Query().Get("limit[territories]") != "2") {
						t.Errorf("lost territory query: %s", r.URL)
					}
					if tt.name == "next include" && r.URL.Query().Get("include") != "territories" {
						t.Errorf("lost include: %s", r.URL)
					}
				} else {
					if !tt.include || r.URL.Path != "/v1/appTags/tag-1/territories" {
						t.Errorf("unexpected request: %s", r.URL)
					}
					body = `{"data":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{}}`
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}))
			args := append([]string{"app-tags"}, tt.args...)
			args = append(args, "--output", "json")
			var runErr error
			stdout, stderr := captureOutput(t, func() { runErr = RootCommand("test").ParseAndRun(context.Background(), args) })
			want := ""
			if tt.warn {
				want = "Warning: App-tag territories are deprecated in API 4.5; remove territory selections and lookups. Requests are still forwarded for compatibility.\n"
			}
			if runErr != nil || stderr != want || !strings.Contains(stdout, `"id":"tag-1"`) {
				t.Fatalf("run=%v stdout=%s stderr=%q want=%q", runErr, stdout, stderr, want)
			}
			wantCalls := 1
			if tt.include {
				wantCalls = 2
				if !strings.Contains(stdout, `"id":"USA"`) {
					t.Fatalf("included territory lost: %s", stdout)
				}
			}
			if calls != wantCalls {
				t.Errorf("calls=%d want=%d", calls, wantCalls)
			}
		})
	}
}
