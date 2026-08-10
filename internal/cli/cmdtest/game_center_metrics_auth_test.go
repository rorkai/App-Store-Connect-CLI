package cmdtest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestGameCenterMetricsPropagateClientErrorsBeforeHTTP(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "matchmaking queue metrics",
			args: []string{"game-center", "matchmaking", "metrics", "queue-sizes", "--queue-id", "queue-1", "--granularity", "P1D", "--output", "json"},
		},
		{
			name: "matchmaking rule metrics",
			args: []string{"game-center", "matchmaking", "metrics", "rule-errors", "--rule-id", "rule-1", "--granularity", "P1D", "--output", "json"},
		},
		{
			name: "detail metrics",
			args: []string{"game-center", "details", "metrics", "classic-matchmaking", "--id", "detail-1", "--granularity", "P1D", "--output", "json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)

			requestCount := 0
			client, err := asc.NewClientWithHTTPClient(
				os.Getenv("ASC_KEY_ID"),
				os.Getenv("ASC_ISSUER_ID"),
				os.Getenv("ASC_PRIVATE_KEY_PATH"),
				&http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					requestCount++
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`)),
						Header:     http.Header{"Content-Type": []string{"application/json"}},
					}, nil
				})},
			)
			if err != nil {
				t.Fatalf("create poisoned metrics client: %v", err)
			}

			authErr := errors.New("metrics auth unavailable")
			factoryCalls := 0
			restore := shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				factoryCalls++
				// The error is authoritative even if a factory also returns a client.
				return client, authErr
			})
			t.Cleanup(restore)

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)
			var runErr error
			var recovered any
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				func() {
					defer func() {
						recovered = recover()
					}()
					runErr = root.Run(context.Background())
				}()
			})

			if recovered != nil {
				t.Fatalf("command panicked instead of returning the auth error: %v", recovered)
			}
			if !errors.Is(runErr, authErr) {
				t.Fatalf("run error = %v, want %v", runErr, authErr)
			}
			if got := rootcmd.ExitCodeFromError(runErr); got != rootcmd.ExitError {
				t.Fatalf("exit code = %d, want %d", got, rootcmd.ExitError)
			}
			if factoryCalls != 1 {
				t.Fatalf("client factory calls = %d, want 1", factoryCalls)
			}
			if requestCount != 0 {
				t.Fatalf("HTTP request count = %d, want 0", requestCount)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty before top-level error formatting", stderr)
			}
		})
	}
}
