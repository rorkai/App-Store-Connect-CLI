package cmdtest

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAppTagTerritoryDeprecation(t *testing.T) {
	for _, tt := range []struct {
		name          string
		args          []string
		warn, include bool
		query         []string
	}{
		{name: "view include", args: []string{"view", "--app", "app-1", "--id", "tag-1", "--include", "territories"}, warn: true, include: true, query: []string{"limit=200"}},
		{name: "view fields", args: []string{"view", "--app", "app-1", "--id", "tag-1", "--fields", "name,territories"}, warn: true, query: []string{"fields[appTags]=name,territories", "limit=200"}},
		{name: "list fields", args: []string{"list", "--app", "app-1", "--fields", "territories"}, warn: true, query: []string{"fields[appTags]=territories"}},
		{name: "next include", args: []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?include=territories&cursor=next"}, warn: true, query: []string{"include=territories", "cursor=next"}},
		{name: "next fields", args: []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?fields%5BappTags%5D=territories&cursor=next"}, warn: true, query: []string{"fields[appTags]=territories", "cursor=next"}},
		{name: "next territory parameters", args: []string{"list", "--next", "https://api.appstoreconnect.apple.com/v1/apps/app-1/appTags?fields%5Bterritories%5D=currency&limit%5Bterritories%5D=2&cursor=next"}, warn: true, query: []string{"fields[territories]=currency", "limit[territories]=2", "cursor=next"}},
		{name: "plain view", args: []string{"view", "--app", "app-1", "--id", "tag-1"}, query: []string{"limit=200"}},
		{name: "plain list", args: []string{"list", "--app", "app-1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps/app-1/appTags", tt.query...),
				asctest.JSON(http.StatusOK, `{"data":[{"type":"appTags","id":"tag-1","attributes":{"name":"Adventure","visibleInAppStore":true}}],"links":{}}`))
			if tt.include {
				reg.Register(asctest.GET("/v1/appTags/tag-1/territories"),
					asctest.JSON(http.StatusOK, `{"data":[{"type":"territories","id":"USA","attributes":{"currency":"USD"}}],"links":{}}`))
			}
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
			if calls := len(reg.Requests()); calls != wantCalls {
				t.Errorf("calls=%d want=%d", calls, wantCalls)
			}
		})
	}
}
