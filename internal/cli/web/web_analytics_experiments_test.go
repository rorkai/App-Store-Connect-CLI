package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

// These fixtures are synthetic wire responses, including unknown fields that must survive JSON output.
const (
	experimentListFixture    = `{"size":1,"results":[{"testName":"Preview test","testId":"11111111-1111-1111-1111-111111111111","startDate":"2026-01-01T00:00:00Z","endDate":null,"status":"RUNNING","future":{"value":1234567890123456789}}],"extra":false}`
	experimentResultsFixture = `{"baseline":"default","size":2,"treatments":[{"id":"default","name":"Original","conversionRate":{"lower":0.02,"central":0.085,"upper":0.19},"improvement":{"lower":0,"central":0,"upper":0},"uniqueImpressions":42,"confidence":0,"outcome":"NA","data":[]},{"id":"22222222-2222-2222-2222-222222222222","name":"Preview","conversionRate":{"lower":0.0456,"central":0.1229,"upper":0.2475},"improvement":{"lower":-0.5934,"central":0.4455,"upper":4.8659},"uniqueImpressions":41,"confidence":0.0002,"outcome":"COLLECTING","future":true,"data":[{"date":"2026-01-02T00:00:00Z","conversionRate":{"central":0.1229},"improvement":{"central":0.4455},"uniqueImpressions":7,"valid":true,"future":1234567890123456789}]}],"extra":null}`
)

func TestWebAnalyticsExperimentsReadContract(t *testing.T) {
	original := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = original })
	for _, tc := range []struct {
		name     string
		args     []string
		path     string
		query    string
		response string
		wantRow  string
		tableErr string
	}{
		{"list", []string{"list", "--app", "123456789"}, "/analytics/api/v1/app-info/123456789/ab-tests", "", experimentListFixture, "Preview test", ""},
		{"results", []string{"results", "--experiment", "11111111-1111-1111-1111-111111111111"}, "/analytics/api/v1/ab-tests/11111111-1111-1111-1111-111111111111", "baseline=default", experimentResultsFixture, "12.29%", ""},
		{"alternate baseline", []string{"results", "--experiment", "11111111-1111-1111-1111-111111111111", "--baseline", "22222222-2222-2222-2222-222222222222"}, "/analytics/api/v1/ab-tests/11111111-1111-1111-1111-111111111111", "baseline=22222222-2222-2222-2222-222222222222", `{"baseline":"22222222-2222-2222-2222-222222222222","size":0,"treatments":[]}`, "", ""},
		{"empty list", []string{"list", "--app", "123456789"}, "/analytics/api/v1/app-info/123456789/ab-tests", "", `{"size":0,"results":[]}`, "", ""},
		{"unavailable metrics", []string{"results", "--experiment", "11111111-1111-1111-1111-111111111111"}, "/analytics/api/v1/ab-tests/11111111-1111-1111-1111-111111111111", "baseline=default", `{"baseline":"default","size":1,"treatments":[{"id":"default","name":"No metrics","conversionRate":null,"improvement":null,"uniqueImpressions":null,"confidence":null,"outcome":"NA","data":[]}]}`, "No metrics", ""},
		{"float-encoded count", []string{"results", "--experiment", "11111111-1111-1111-1111-111111111111"}, "/analytics/api/v1/ab-tests/11111111-1111-1111-1111-111111111111", "baseline=default", `{"baseline":"default","treatments":[{"id":"22222222-2222-2222-2222-222222222222","name":"Preview","uniqueImpressions":41.0,"outcome":"NA"}]}`, "41.0", ""},
		{"metric the table cannot read", []string{"results", "--experiment", "11111111-1111-1111-1111-111111111111"}, "/analytics/api/v1/ab-tests/11111111-1111-1111-1111-111111111111", "baseline=default", `{"baseline":"default","treatments":[{"id":"22222222-2222-2222-2222-222222222222","confidence":"0.9"}]}`, "", "failed to parse experiment results"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requests.Add(1)
				body, err := io.ReadAll(req.Body)
				if err != nil || len(body) != 0 || req.Method != http.MethodGet || req.URL.Path != tc.path || req.URL.RawQuery != tc.query {
					t.Errorf("unexpected request: %s %s body=%q err=%v", req.Method, req.URL, body, err)
				}
				if req.Header.Get("X-Requested-By") != "appstoreconnect.apple.com" || req.Header.Get("Accept") != "application/json" {
					t.Errorf("missing analytics headers: %v", req.Header)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tc.response)
			}))
			t.Cleanup(server.Close)
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{Client: &http.Client{Transport: customAppUsersRewriteTransport{target: target, base: server.Client().Transport}}}, "fixture", nil
			}
			for _, format := range []string{"json", "table", "markdown"} {
				d, stdout, stderr := newWebTestDeps()
				args := append([]string{"analytics", "product-pages", "experiments"}, tc.args...)
				args = append(args, "--output", format)
				err := NewWebCommand(d).ParseAndRun(context.Background(), args)
				if format != "json" && tc.tableErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.tableErr) || stdout.Len() != 0 {
						t.Fatalf("%s command: err=%v stdout=%q, want error %q and no output", format, err, stdout, tc.tableErr)
					}
					continue
				}
				if err != nil || stderr.Len() != 0 {
					t.Fatalf("%s command: err=%v stderr=%q", format, err, stderr)
				}
				if format == "json" {
					decode := func(text string) any {
						decoder := json.NewDecoder(strings.NewReader(text))
						decoder.UseNumber()
						var value any
						if err := decoder.Decode(&value); err != nil {
							t.Fatal(err)
						}
						return value
					}
					if got, want := decode(stdout.String()), decode(tc.response); !reflect.DeepEqual(got, want) {
						t.Fatalf("JSON envelope changed: got %s want %s", stdout, tc.response)
					}
				} else {
					if !strings.Contains(stdout.String(), tc.wantRow) {
						t.Fatalf("missing row %q: %s", tc.wantRow, stdout)
					}
					if tc.name == "results" && (!strings.Contains(stdout.String(), "44.55%") || !strings.Contains(stdout.String(), "0.02%") || !strings.Contains(stdout.String(), "COLLECTING")) {
						t.Fatalf("lost precision, percentage scale, or raw outcome: %s", stdout)
					}
					if tc.name == "unavailable metrics" && strings.Contains(stdout.String(), "0.00%") {
						t.Fatalf("null metrics rendered as zero: %s", stdout)
					}
				}
			}
			if requests.Load() != 3 {
				t.Fatalf("requests=%d, want one per command", requests.Load())
			}
		})
	}
}

func TestWebAnalyticsExperimentsReadFailures(t *testing.T) {
	original := resolveSessionFn
	t.Cleanup(func() { resolveSessionFn = original })
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		canceled bool
		wantErr  string
	}{
		{"expired session", http.StatusUnauthorized, `{"errors":[{"status":"401"}]}`, false, "run 'asc web auth login'"},
		{"malformed response", http.StatusOK, `<html>Sign in</html>`, false, "failed to parse analytics experiment response"},
		{"canceled", http.StatusOK, experimentResultsFixture, true, "context canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			t.Cleanup(server.Close)
			target, _ := url.Parse(server.URL)
			resolveSessionFn = func(context.Context, string, string, string) (*webcore.AuthSession, string, error) {
				return &webcore.AuthSession{Client: &http.Client{Transport: customAppUsersRewriteTransport{target: target, base: server.Client().Transport}}}, "fixture", nil
			}
			d, stdout, _ := newWebTestDeps()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			err := NewWebCommand(d).ParseAndRun(ctx, []string{"analytics", "product-pages", "experiments", "results", "--experiment", "11111111-1111-1111-1111-111111111111", "--output", "json"})
			if err == nil || stdout.Len() != 0 {
				t.Fatalf("expected failure without data: err=%v stdout=%s", err, stdout)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
			if tc.canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if tc.status == http.StatusUnauthorized {
				var apiErr *webcore.APIError
				if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
					t.Fatalf("auth error lost: %v", err)
				}
			}
		})
	}
}
