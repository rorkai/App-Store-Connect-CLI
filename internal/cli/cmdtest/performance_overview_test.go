package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const overviewFixture = `{"version":"1.0","appMetadata":{"appId":"123","bundleId":"com.example.app","latestVersion":"2.0","platform":"IOS"},"insights":{"regressions":[{"metric":"launch"}],"trendingUp":[]},"categories":[{"identifier":"launch","sections":[{"datasets":[{"filterCriteria":{"device":"iPhone15,2"},"points":[{"version":"2.0","value":0.5}]}]}]}],"telemetryIdentifier":"preserve-me"}`

func TestPerformanceOverview(t *testing.T) {
	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/v1/apps/123/performanceOverviews" {
					t.Errorf("request: %s %s", r.Method, r.URL.Path)
				}
				if got := r.Header.Get("Accept"); got != "application/vnd.apple.xcode-overview+json" {
					t.Errorf("Accept=%s", got)
				}
				if got := r.URL.Query().Get("filter[deviceType]"); got != "iPhone15,2" {
					t.Errorf("device type=%s", got)
				}
				w.Header().Set("Content-Type", "application/vnd.apple.xcode-overview+json")
				_, _ = io.WriteString(w, overviewFixture)
			}))
			defer server.Close()
			out, stderr, runErr := runRootWithDeps(t, stubServerClient(t, server), "performance", "overview", "--app", "123", "--device-type", "iPhone15,2", "--output", format)
			if runErr != nil {
				t.Fatalf("run: %v; stderr=%s", runErr, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr=%s", stderr)
			}
			if format == "json" {
				var got, want any
				if err := json.Unmarshal([]byte(out), &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(overviewFixture), &want); err != nil {
					t.Fatal(err)
				}
				gotJSON, _ := json.Marshal(got)
				wantJSON, _ := json.Marshal(want)
				if string(gotJSON) != string(wantJSON) {
					t.Errorf("JSON changed: %s", out)
				}
			} else if !strings.Contains(out, "com.example.app") || !strings.Contains(out, "Regressions") {
				t.Errorf("missing summary: %s", out)
			}
		})
	}
}

func TestPerformanceOverviewUsage(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")
	assertUsageExitWithDeps(t, nil, []string{"performance", "overview"}, "--app is required")
	assertUsageExitWithDeps(t, nil, []string{"performance", "overview", "--app", "123", "--device-type", " "}, "--device-type must not be empty")
	assertUsageExitWithDeps(t, nil, []string{"performance", "overview", "--app", "123", "extra"}, "unexpected argument")
}
