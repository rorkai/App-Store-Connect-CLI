package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWebhookDeliveriesDateFiltersMatchAppleContract(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantRawQuery string
	}{
		{name: "no date filters"},
		{name: "created after", args: []string{"--created-after", "2026-01-01T00:00:00Z"}, wantRawQuery: "filter%5BcreatedDateGreaterThanOrEqualTo%5D=2026-01-01T00%3A00%3A00Z"},
		{name: "created before", args: []string{"--created-before", "2026-02-01T00:00:00Z"}, wantRawQuery: "filter%5BcreatedDateLessThan%5D=2026-02-01T00%3A00%3A00Z"},
		{name: "bounded range", args: []string{"--created-after", "2026-01-01T00:00:00Z", "--created-before", "2026-02-01T00:00:00Z"}, wantRawQuery: "filter%5BcreatedDateGreaterThanOrEqualTo%5D=2026-01-01T00%3A00%3A00Z&filter%5BcreatedDateLessThan%5D=2026-02-01T00%3A00%3A00Z"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				requestCount++
				if req.Method != http.MethodGet || req.URL.Path != "/v1/webhooks/wh-1/deliveries" {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				if req.URL.RawQuery != test.wantRawQuery {
					t.Fatalf("raw query = %q, want %q", req.URL.RawQuery, test.wantRawQuery)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[]}`)
			}))
			t.Cleanup(server.Close)

			args := append([]string{"webhooks", "deliveries", "--webhook-id", "wh-1", "--output", "json"}, test.args...)
			stdout, stderr, err := runRootWithDeps(t, stubServerClient(t, server), args...)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if requestCount != 1 {
				t.Fatalf("request count = %d, want 1", requestCount)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			var response struct {
				Data []json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("stdout is not JSON: %v; stdout=%q", err, stdout)
			}
		})
	}
}
