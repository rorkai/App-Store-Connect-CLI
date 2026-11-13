package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

type testFlightRequestCase struct {
	name       string
	args       []string
	wantMethod string
	wantPath   string
	wantQuery  url.Values
	wantBody   string
	respStatus int
	respBody   string
	wantStdout []string
	wantStderr string
}

func runTestFlightRequestCases(t *testing.T, tests []testFlightRequestCase) {
	t.Helper()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_APP_ID", "")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			status := test.respStatus
			if status == 0 {
				status = http.StatusOK
			}
			var requests []*http.Request
			var bodies []string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				payload := ""
				if req.Body != nil {
					data, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read body: %v", err)
					}
					payload = string(data)
				}
				requests = append(requests, req)
				bodies = append(bodies, payload)
				return &http.Response{
					StatusCode: status,
					Body:       io.NopCloser(strings.NewReader(test.respBody)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				}, nil
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})

			if len(requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(requests))
			}
			req := requests[0]
			if req.Method != test.wantMethod || req.URL.Path != test.wantPath {
				t.Fatalf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.wantMethod, test.wantPath)
			}
			if got := req.URL.Query().Encode(); got != test.wantQuery.Encode() {
				t.Fatalf("query = %q, want %q", got, test.wantQuery.Encode())
			}
			if test.wantBody == "" {
				if bodies[0] != "" {
					t.Fatalf("body = %s, want none", bodies[0])
				}
			} else {
				var got, want any
				if err := json.Unmarshal([]byte(bodies[0]), &got); err != nil {
					t.Fatalf("decode body %q: %v", bodies[0], err)
				}
				if err := json.Unmarshal([]byte(test.wantBody), &want); err != nil {
					t.Fatalf("decode wantBody: %v", err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("body = %s, want %s", bodies[0], test.wantBody)
				}
			}
			if test.wantStderr == "" && stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			for _, want := range test.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout = %q, want %q", stdout, want)
				}
			}
		})
	}
}
