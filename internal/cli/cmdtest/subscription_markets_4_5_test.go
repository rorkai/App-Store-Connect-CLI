package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSubscriptionOrganizationSettings(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		attributes map[string]any
		fields     string
		table      bool
	}{
		{name: "both settings", args: []string{"update", "--id", "sub-45", "--market-settings", "APP_STORE,APPLE_SCHOOL,APPLE_BUSINESS", "--multi-seat-status", "ENABLED"}, attributes: map[string]any{"marketSettings": []string{"APP_STORE", "APPLE_SCHOOL", "APPLE_BUSINESS"}, "multiSeatStatus": "ENABLED"}},
		{name: "markets only", args: []string{"update", "--id", "sub-45", "--market-settings", "APPLE_BUSINESS"}, attributes: map[string]any{"marketSettings": []string{"APPLE_BUSINESS"}}},
		{name: "multi seat only", args: []string{"update", "--id", "sub-45", "--multi-seat-status", "DISABLED"}, attributes: map[string]any{"multiSeatStatus": "DISABLED"}},
		{name: "sparse read", args: []string{"view", "--id", "sub-45", "--fields", "marketSettings,multiSeatStatus"}, fields: "marketSettings,multiSeatStatus"},
		{name: "table read", args: []string{"view", "--id", "sub-45"}, table: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				method := http.MethodGet
				if tt.attributes != nil {
					method = http.MethodPatch
				}
				if r.Method != method || r.URL.Path != "/v1/subscriptions/sub-45" {
					t.Errorf("request %s %s", r.Method, r.URL.Path)
				}
				if tt.attributes != nil {
					var payload struct {
						Data struct {
							Type       string         `json:"type"`
							ID         string         `json:"id"`
							Attributes map[string]any `json:"attributes"`
						} `json:"data"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					got, _ := json.Marshal(payload.Data.Attributes)
					want, _ := json.Marshal(tt.attributes)
					if string(got) != string(want) || payload.Data.Type != "subscriptions" || payload.Data.ID != "sub-45" {
						t.Errorf("payload=%+v, want attributes=%s", payload, want)
					}
				}
				if got := r.URL.Query().Get("fields[subscriptions]"); got != tt.fields {
					t.Errorf("fields=%q", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":{"type":"subscriptions","id":"sub-45","attributes":{"name":"Business","productId":"business","marketSettings":["APP_STORE","APPLE_SCHOOL","APPLE_BUSINESS"],"multiSeatStatus":"ENABLED"}}}`)
			}))
			defer server.Close()
			u, _ := url.Parse(server.URL)
			installDefaultTransport(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
				c := r.Clone(r.Context())
				c.URL.Scheme = u.Scheme
				c.URL.Host = u.Host
				return server.Client().Transport.RoundTrip(c)
			}))
			format := "json"
			if tt.table {
				format = "table"
			}
			args := append([]string{"subscriptions"}, tt.args...)
			args = append(args, "--output", format)
			root := RootCommand("test")
			var runErr error
			stdout, stderr := captureOutput(t, func() { runErr = root.ParseAndRun(context.Background(), args) })
			if runErr != nil {
				t.Fatalf("run: %v; stderr=%s", runErr, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr=%s", stderr)
			}
			if tt.table {
				if !strings.Contains(stdout, "APPLE_BUSINESS") || !strings.Contains(stdout, "ENABLED") {
					t.Errorf("missing settings: %s", stdout)
				}
				return
			}
			var result struct {
				Data struct {
					Attributes map[string]any `json:"attributes"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatal(err)
			}
			markets, ok := result.Data.Attributes["marketSettings"].([]any)
			if !ok || len(markets) != 3 || result.Data.Attributes["multiSeatStatus"] != "ENABLED" {
				t.Errorf("settings lost: %s", stdout)
			}
		})
	}
}

func TestSubscriptionOrganizationSettingsUsage(t *testing.T) {
	for _, tt := range []struct{ flag, value, want string }{
		{"--market-settings", "RETAIL", "--market-settings must be one of:"},
		{"--market-settings", " ", "--market-settings must not be empty"},
		{"--multi-seat-status", "true", "--multi-seat-status must be one of:"},
		{"--multi-seat-status", " ", "--multi-seat-status must not be empty"},
	} {
		t.Run(tt.flag+tt.value, func(t *testing.T) {
			assertUsageExit(t, []string{"subscriptions", "update", "--id", "sub-45", tt.flag, tt.value}, tt.want)
		})
	}
}
