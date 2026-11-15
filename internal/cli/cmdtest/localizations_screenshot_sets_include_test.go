package cmdtest

import (
	"net/http"
	"strings"
	"testing"
)

func TestLocalizationsScreenshotSetsListInclude(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantQuery string
		wantErr   string
	}{
		{name: "include screenshots", args: []string{"--include", "appScreenshots"}, wantQuery: "include=appScreenshots&limit%5BappScreenshots%5D=50"},
		{name: "include screenshots paginated", args: []string{"--include", "appScreenshots", "--paginate"}, wantQuery: "include=appScreenshots&limit=200&limit%5BappScreenshots%5D=50"},
		{name: "include with next", args: []string{"--include", "appScreenshots", "--next", "https://api.appstoreconnect.apple.com/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets?cursor=AQ"}, wantErr: "Error: localizations screenshot-sets list: --next cannot be combined with --include"},
		{name: "unsupported include", args: []string{"--include", "appStoreVersionLocalization"}, wantErr: "Error: localizations screenshot-sets list: --include must be appScreenshots"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery string
			installScreenshotsListTestClient(t, func(w http.ResponseWriter, req *http.Request) {
				if tt.wantErr != "" || req.URL.Path != "/v1/appStoreVersionLocalizations/loc-1/appScreenshotSets" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL)
				}
				gotQuery = req.URL.RawQuery
				writeScreenshotsListJSON(t, w, `{"data":[{"type":"appScreenshotSets","id":"set-1","attributes":{"screenshotDisplayType":"APP_IPHONE_65"},"relationships":{"appScreenshots":{"data":[{"type":"appScreenshots","id":"shot-1"}]}}}],"included":[{"type":"appScreenshots","id":"shot-1","attributes":{"fileName":"home.png"}}],"links":{}}`)
			})

			stdout, stderr, err := runRootCommand(t, append([]string{"localizations", "screenshot-sets", "list", "--localization-id", "loc-1", "--output", "json"}, tt.args...))
			if tt.wantErr != "" {
				if !isUsageClassError(err) || stdout != "" || !strings.HasPrefix(stderr, tt.wantErr) {
					t.Fatalf("expected usage error, got err=%v stdout=%q stderr=%q", err, stdout, stderr)
				}
				return
			}
			if err != nil {
				t.Fatalf("list error: %v (stderr %q)", err, stderr)
			}
			if gotQuery != tt.wantQuery {
				t.Fatalf("query = %q, want %q", gotQuery, tt.wantQuery)
			}
			if !strings.Contains(stdout, `"included":[{"type":"appScreenshots","id":"shot-1"`) {
				t.Fatalf("stdout lost included screenshots: %s", stdout)
			}
		})
	}
}
