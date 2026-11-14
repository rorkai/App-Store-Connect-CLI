package cmdtest

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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
			var query []string
			if encoded := test.wantQuery.Encode(); encoded != "" {
				query = strings.Split(encoded, "&")
			}
			matcher := asctest.GET(test.wantPath, query...)
			switch test.wantMethod {
			case http.MethodPost:
				matcher = asctest.POST(test.wantPath, query...)
			case http.MethodPatch:
				matcher = asctest.PATCH(test.wantPath, query...)
			case http.MethodDelete:
				matcher = asctest.DELETE(test.wantPath, query...)
			}
			if test.wantBody != "" {
				matcher = matcher.JSONBody(test.wantBody)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(matcher, asctest.JSON(status, test.respBody))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitSuccess {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitSuccess)
				}
			})

			requests := reg.Requests()
			if len(requests) != 1 {
				t.Fatalf("requests = %d, want 1", len(requests))
			}
			if test.wantBody == "" && len(requests[0].Body) != 0 {
				t.Fatalf("body = %s, want none", requests[0].Body)
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
