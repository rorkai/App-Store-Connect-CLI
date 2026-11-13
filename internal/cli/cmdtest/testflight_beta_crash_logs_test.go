package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestTestFlightCrashesLogValidationErrors(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"testflight", "crashes", "log"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("expected ErrHelp, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: exactly one of --submission-id or --crash-log-id is required") {
		t.Fatalf("expected missing selector error, got %q", stderr)
	}
}

func TestTestFlightCrashesLogNotFoundPointsToCrashSubmissions(t *testing.T) {
	const hint = "Hint: Crash logs are fetched by crash submission ID: list them with `asc testflight crashes list --app APP_ID` and pass one to --submission-id.\n"
	tests := []struct {
		name string
		args []string
		path string
		kind string
	}{
		{name: "submission id", args: []string{"--submission-id", "missing"}, path: "/v1/betaFeedbackCrashSubmissions/missing/crashLog", kind: "betaFeedbackCrashSubmissions"},
		{name: "crash log id", args: []string{"--crash-log-id", "missing"}, path: "/v1/betaCrashLogs/missing", kind: "betaCrashLogs"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			detail := "There is no resource of type '" + test.kind + "' with id 'missing'"
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				}
				return jsonResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"The specified resource does not exist","detail":"`+detail+`"}]}`)
			}))

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(append([]string{"testflight", "crashes", "log"}, test.args...), "1.2.3")
			})
			if code != rootcmd.ExitNotFound {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, rootcmd.ExitNotFound, stderr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, detail) || !strings.HasSuffix(stderr, hint) {
				t.Fatalf("stderr = %q, want Apple's detail and hint %q", stderr, hint)
			}
		})
	}
}
